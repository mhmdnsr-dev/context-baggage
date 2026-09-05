package githubsync

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func TestFirstClaimPublishesMarkerOnlyAndPortableState(t *testing.T) {
	for _, withWorkspace := range []bool{false, true} {
		t.Run(map[bool]string{false: "marker only", true: "marker and v2"}[withWorkspace], func(t *testing.T) {
			assertFirstClaimRoundTrip(t, withWorkspace)
		})
	}
}

func assertFirstClaimRoundTrip(t *testing.T, withWorkspace bool) {
	t.Helper()
	s, remote := publicationFixture(t, withWorkspace)
	runner := localFixtureRunner(t)
	hash, err := publishManaged(context.Background(), s, runner, localPublicationRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	observed, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	if observed.State != RepositoryInitialized || observed.ManagedDestinationID != state.ManagedDestinationID || observed.PortableHash != hash {
		t.Fatalf("round trip mismatch: state=%+v remote=%+v hash=%q", state, observed, hash)
	}
	if state.BaseHash != hash || !state.BasePresent || state.BaseDestinationIdentity != state.ManagedDestinationID {
		t.Fatalf("BASE not bound after confirmation: %+v", state)
	}
	if !withWorkspace && observed.PortablePresent {
		t.Fatal("empty LOCAL did not produce a marker-only repository")
	}
}

func TestFirstClaimRandomFailureDoesNotPublish(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.random = failingReader{}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err == nil {
		t.Fatal("expected random generation failure")
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("random failure changed remote: empty=%t error=%v", empty, err)
	}
	assertUnadvancedState(t, s)
}

func TestGeneratedManagedDestinationID(t *testing.T) {
	id, err := generateManagedDestinationID(strings.NewReader("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "dst_30313233343536373839616263646566" {
		t.Fatalf("unexpected generated ID %q", id)
	}
}

func TestPublicationDestinationIdentityRefusals(t *testing.T) {
	initialized := RepositorySnapshot{State: RepositoryInitialized, CommitID: strings.Repeat("a", 40), ManagedDestinationID: neutralDestinationID}
	if _, _, err := publicationIdentity(store.SyncState{}, initialized, nil); !errors.Is(err, ErrManagedDestinationAdoptionRequired) {
		t.Fatalf("missing identity: %v", err)
	}
	state := store.SyncState{ManagedDestinationID: "dst_ffffffffffffffffffffffffffffffff"}
	if _, _, err := publicationIdentity(state, initialized, nil); !errors.Is(err, ErrManagedDestinationMismatch) {
		t.Fatalf("mismatching identity: %v", err)
	}
	if _, _, err := publicationIdentity(state, RepositorySnapshot{State: RepositoryEmpty}, nil); !errors.Is(err, ErrManagedDestinationLost) {
		t.Fatalf("lost identity: %v", err)
	}
}

func TestPublicationRechecksPrivacyEveryTime(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	checks := 0
	runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
		checks++
		if checks == 1 {
			return VerifiedNonPublic, nil
		}
		return VerifiedPublic, nil
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPrivacyRefused) {
		t.Fatalf("second fresh privacy refusal = %v", err)
	}
	if checks != 2 {
		t.Fatalf("privacy checks = %d", checks)
	}
}

func TestFirstClaimPrivacyRefusalFollowsCandidatePreparation(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	prepared := false
	runtime.afterPrepare = func() { prepared = true }
	runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
		return VerifiedPublic, nil
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPrivacyRefused) {
		t.Fatalf("first-claim privacy result = %v", err)
	}
	if !prepared {
		t.Fatal("privacy ran before candidate commit preparation")
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("privacy refusal changed remote: empty=%t error=%v", empty, err)
	}
	assertUnadvancedState(t, s)
}

func TestFirstClaimObservablePublicationOrder(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	events := make([]string, 0, 4)
	runtime.afterPrepare = func() { events = append(events, "prepared") }
	runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
		events = append(events, "privacy")
		return VerifiedNonPublic, nil
	}
	runtime.observe = func(ctx context.Context, git GitRunner, remoteURL string) (string, bool, error) {
		events = append(events, "empty-observation")
		return git.observeManagedRef(ctx, remoteURL)
	}
	runtime.push = func(ctx context.Context, root string, prepared preparedPublication) error {
		events = append(events, "push")
		return runner.pushPrepared(ctx, root, prepared)
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	want := []string{"prepared", "privacy", "empty-observation", "push"}
	if strings.Join(events, ",") != strings.Join(want, ",") {
		t.Fatalf("publication order = %v, want %v", events, want)
	}
}

func TestLocalMutationAfterSnapshotPublishesCapturedState(t *testing.T) {
	s, remote := publicationFixture(t, true)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.afterSnapshot = func() {
		workspace, err := s.ReadWorkspace("w_shared")
		if err != nil {
			t.Fatal(err)
		}
		workspace.Name = "changed after snapshot"
		if err := s.WriteWorkspace(workspace); err != nil {
			t.Fatal(err)
		}
	}
	publishedHash, err := publishManaged(context.Background(), s, runner, runtime)
	if err != nil {
		t.Fatal(err)
	}
	currentDir := t.TempDir()
	currentHash, err := portablesync.BuildPushSnapshot(s, currentDir)
	if err != nil {
		t.Fatal(err)
	}
	state, _ := s.ReadSync()
	if currentHash == publishedHash || state.BaseHash != publishedHash {
		t.Fatalf("snapshot semantics failed: published=%q current=%q BASE=%q", publishedHash, currentHash, state.BaseHash)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func publicationFixture(t *testing.T, withWorkspace bool) (store.Store, string) {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if withWorkspace {
		workspace := store.Workspace{ID: "w_shared", Name: "shared", Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
		if err := s.WriteWorkspace(workspace); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.WriteSync(store.SyncState{
		FormatVersion: store.SyncStateFormatVersion, DestinationType: store.DestinationGitHub,
		GitHubLocator: "https://github.com/neutral/repository", GitHubRepository: "github.com/neutral/repository",
	}); err != nil {
		t.Fatal(err)
	}
	remote, _ := createBareFixture(t, "", nil)
	return s, remote
}

func localPublicationRuntime(t *testing.T, runner GitRunner, remote string) publicationRuntime {
	t.Helper()
	return publicationRuntime{
		random: strings.NewReader("0123456789abcdef"), remoteURL: remote,
		classify: func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
			return VerifiedNonPublic, nil
		},
		inspect: func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
			return inspectLocalFixture(t, remote)
		},
	}
}

func assertUnadvancedState(t *testing.T, s store.Store) {
	t.Helper()
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if state.ManagedDestinationID != "" || state.BasePresent || state.LastPush != "" {
		t.Fatalf("publication failure advanced state: %+v", state)
	}
}
