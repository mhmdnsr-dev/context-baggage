package githubsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func managedPullStore(t *testing.T) store.Store {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSync(store.SyncState{
		FormatVersion:          store.SyncStateFormatVersion,
		DestinationType:        store.DestinationGitHub,
		GitHubLocator:          "https://github.com/neutral/repository",
		GitHubRepository:       "github.com/neutral/repository",
		ManagedDestinationID:   neutralDestinationID,
		LastObservedRemoteHash: "",
		LastRefresh:            "",
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func managedPullFixtureRuntime(t *testing.T, runner GitRunner, remote string) pullRuntime {
	t.Helper()
	return pullRuntime{
		materialize: func(ctx context.Context, git GitRunner, locator Locator) (RepositorySnapshot, string, func(), error) {
			return materializeLocalRemote(ctx, git, locator, remote)
		},
	}
}

func materializeLocalRemote(ctx context.Context, git GitRunner, locator Locator, remote string) (RepositorySnapshot, string, func(), error) {
	commitID, empty, err := git.observeManagedRef(ctx, remote)
	if err != nil {
		return RepositorySnapshot{}, "", nil, err
	}
	if empty {
		return RepositorySnapshot{RepositoryIdentity: locator.identity, State: RepositoryEmpty}, "", nil, nil
	}
	root, err := os.MkdirTemp("", "ctx-bag-managed-pull-*")
	if err != nil {
		return RepositorySnapshot{}, "", nil, ErrTransportUnavailable
	}
	snapshot, portableDir, err := git.inspectAndMaterialize(ctx, remote, locator.identity, commitID, root)
	if err != nil {
		_ = os.RemoveAll(root)
		return RepositorySnapshot{}, "", nil, err
	}
	return snapshot, portableDir, func() { _ = os.RemoveAll(root) }, nil
}

func advanceManagedRemote(t *testing.T, remote string, files map[string]string) string {
	t.Helper()
	work := createWorkFixture(t, "context-baggage", files)
	commitID := strings.TrimSpace(runFixtureGit(t, "-C", work, "rev-parse", "HEAD"))
	runFixtureGit(t, "--git-dir", remote, "fetch", "--quiet", work, commitID)
	runFixtureGit(t, "--git-dir", remote, "update-ref", managedRef, commitID)
	return commitID
}

func portableWorkspaceYAMLWithName(name string) string {
	return fmt.Sprintf("id: w_one\nname: %s\nidentity:\n  type: local-directory\n  value: neutral\nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n", name)
}

func canonicalWorkspaceHash(t *testing.T, s store.Store) string {
	t.Helper()
	hash, err := store.HashDir(filepath.Join(s.Home, "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestManagedPullRoundTrip(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	hash, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	local, err := portablesync.EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if local != hash {
		t.Fatalf("LOCAL != pulled hash: %s != %s", local, hash)
	}
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if !state.BasePresent || state.BaseHash != hash {
		t.Fatalf("BASE not bound after pull: %+v", state)
	}
	if state.BaseDestinationType != store.DestinationGitHub || state.BaseDestinationIdentity != neutralDestinationID {
		t.Fatalf("BASE destination binding wrong: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after success")
	}

	yFiles := validPortableFiles()
	yFiles[portableRootName+"/workspaces/w_one/workspace.yaml"] = portableWorkspaceYAMLWithName("next")
	advanceManagedRemote(t, remote, yFiles)
	hash2, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	if hash2 == hash {
		t.Fatalf("second pull did not advance remote: %s", hash2)
	}
	local2, err := portablesync.EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if local2 != hash2 {
		t.Fatalf("LOCAL after second pull: %s != %s", local2, hash2)
	}
	state2, _ := s.ReadSync()
	if state2.BaseHash != hash2 {
		t.Fatalf("BASE not advanced after second pull: %+v", state2)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after second pull")
	}
}

func TestManagedPullConflict(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	hash, err := pullManaged(context.Background(), s, runner, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_one", Name: "local-change", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	zFiles := validPortableFiles()
	zFiles[portableRootName+"/workspaces/w_one/workspace.yaml"] = portableWorkspaceYAMLWithName("remote-change")
	advanceManagedRemote(t, remote, zFiles)
	_, err = pullManaged(context.Background(), s, runner, runtime)
	if !errors.Is(err, ErrPullConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	got, err := s.ReadWorkspace("w_one")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "local-change" {
		t.Fatalf("local workspace was overwritten after conflict: %s", got.Name)
	}
	state, _ := s.ReadSync()
	if state.BaseHash != hash {
		t.Fatalf("BASE changed after conflict: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record created on conflict")
	}
}

func TestManagedPullNoOp(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_one", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory", Value: "neutral"},
		Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	runner := localFixtureRunner(t)
	hash, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("no-op pull created a recovery record")
	}
	state, _ := s.ReadSync()
	if !state.BasePresent || state.BaseHash != hash {
		t.Fatalf("no-op pull did not set BASE: %+v", state)
	}
}

func TestManagedPullDestinationIdentity(t *testing.T) {
	runner := localFixtureRunner(t)
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())

	t.Run("matching id", func(t *testing.T) {
		s := managedPullStore(t)
		if _, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote)); err != nil {
			t.Fatalf("matching destination refused: %v", err)
		}
	})
	t.Run("missing local id", func(t *testing.T) {
		s := managedPullStore(t)
		state, _ := s.ReadSync()
		state.ManagedDestinationID = ""
		if err := s.WriteSync(state); err != nil {
			t.Fatal(err)
		}
		_, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
		if !errors.Is(err, ErrManagedDestinationAdoptionRequired) {
			t.Fatalf("expected adoption required, got %v", err)
		}
	})
	t.Run("mismatching id", func(t *testing.T) {
		s := managedPullStore(t)
		state, _ := s.ReadSync()
		state.ManagedDestinationID = "dst_ffffffffffffffffffffffffffffffff"
		if err := s.WriteSync(state); err != nil {
			t.Fatal(err)
		}
		_, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
		if !errors.Is(err, ErrManagedDestinationMismatch) {
			t.Fatalf("expected mismatch, got %v", err)
		}
	})
	t.Run("remote empty after claim", func(t *testing.T) {
		emptyRemote, _ := createBareFixture(t, "", nil)
		s := managedPullStore(t)
		_, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, emptyRemote))
		if !errors.Is(err, ErrManagedDestinationLost) {
			t.Fatalf("expected identity loss, got %v", err)
		}
	})
}

func TestManagedPullRecoveryRecordPrecedesMutation(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	recordWritten := false
	runtime.writeRecovery = func(s store.Store, r store.PullRecovery) error {
		recordWritten = true
		return s.WritePullRecovery(r)
	}
	runtime.apply = func(s store.Store, dir string) error {
		if !recordWritten {
			t.Fatal("apply ran before recovery record was written")
		}
		return portablesync.ApplyPortableSnapshot(s, dir)
	}
	if _, err := pullManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after success")
	}
}

func TestManagedPullRecoveryWriteFailure(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	runtime.writeRecovery = func(s store.Store, r store.PullRecovery) error {
		return errors.New("recovery write failed")
	}
	before := canonicalWorkspaceHash(t, s)
	_, err := pullManaged(context.Background(), s, runner, runtime)
	if err == nil {
		t.Fatal("expected recovery write failure")
	}
	if canonicalWorkspaceHash(t, s) != before {
		t.Fatal("canonical state mutated after recovery write failure")
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("BASE set despite recovery write failure: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record persisted despite write failure")
	}
}

func TestManagedPullMidApplyFailure(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	runtime.apply = func(s store.Store, dir string) error {
		if err := s.WriteWorkspace(store.Workspace{ID: "w_one", Name: "partial", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: store.Now()}); err != nil {
			return err
		}
		return errors.New("mid-apply failure")
	}
	_, err := pullManaged(context.Background(), s, runner, runtime)
	if err == nil {
		t.Fatal("expected mid-apply failure")
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("BASE set despite mid-apply failure: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record not retained after mid-apply failure")
	}
}

func TestManagedPullMarkerOnlyRemovesSyncedContent(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	if _, err := pullManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadWorkspace("w_one"); err != nil {
		t.Fatalf("baseline pull did not import workspace: %v", err)
	}
	advanceManagedRemote(t, remote, markerFiles())
	hash, err := pullManaged(context.Background(), s, runner, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadWorkspace("w_one"); !os.IsNotExist(err) {
		t.Fatalf("synced workspace not removed by marker-only pull: %v", err)
	}
	local, _ := portablesync.EligibleHash(s)
	if local != hash {
		t.Fatalf("LOCAL != marker-only hash: %s != %s", local, hash)
	}
	state, _ := s.ReadSync()
	if state.BaseHash != hash {
		t.Fatalf("BASE not set on marker-only pull: %+v", state)
	}
}

func TestManagedEmptyPortableIdentity(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", markerFiles())
	snapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.PortableHash != emptyPortableHash {
		t.Fatalf("marker-only REMOTE identity %s != %s", snapshot.PortableHash, emptyPortableHash)
	}
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	hash, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	if hash != emptyPortableHash {
		t.Fatalf("empty managed Pull identity %s != %s", hash, emptyPortableHash)
	}
	state, _ := s.ReadSync()
	if !state.BasePresent || state.BaseHash != emptyPortableHash {
		t.Fatalf("empty managed Pull did not bind BASE to empty identity: %+v", state)
	}
}

func TestManagedPullRefusesWhileRecoveryPending(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	record := recoveryRecordFixture(t, strings.Repeat("a", 40), strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err := s.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	_, err := pullManaged(context.Background(), s, runner, managedPullFixtureRuntime(t, runner, remote))
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("expected recovery required, got %v", err)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record dropped on pending-recovery refusal")
	}
}

func TestManagedPushRefusesWhileRecoveryPending(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	record := recoveryRecordFixture(t, strings.Repeat("a", 40), strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err := s.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	_, err := publishManaged(context.Background(), s, runner, localPublicationRuntime(t, runner, remote))
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("expected recovery required, got %v", err)
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("publication happened despite pending recovery: empty=%t err=%v", empty, err)
	}
}

func TestManagedPullBaseWriteFailure(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	runtime := managedPullFixtureRuntime(t, runner, remote)
	runtime.persist = func(s store.Store, state store.SyncState, hash string) (string, error) {
		return "", errors.New("sync write failed")
	}
	_, err := pullManaged(context.Background(), s, runner, runtime)
	if err == nil {
		t.Fatal("expected BASE write failure")
	}
	local, err := portablesync.EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("BASE falsely advanced: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record not retained after BASE failure")
	}
	recoveryRuntime := recoveryFixtureRuntime(t, runner, remote)
	if _, err := recoverManagedPull(context.Background(), s, runner, recoveryRuntime); err != nil {
		t.Fatal(err)
	}
	state2, _ := s.ReadSync()
	if !state2.BasePresent || state2.BaseHash != local {
		t.Fatalf("recovery did not finalize BASE: %+v", state2)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after recovery")
	}
}
