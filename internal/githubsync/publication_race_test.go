package githubsync

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCompetingFirstClaimUsesExpectedAbsenceLease(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	winnerID := "dst_ffffffffffffffffffffffffffffffff"
	winnerCommit := prepareFixtureCommit(t, remote, map[string]string{
		managedMarkerName: "format: 1\ndestinationId: " + winnerID + "\n",
	})
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.beforePush = func() {
		runFixtureGit(t, "--git-dir", remote, "update-ref", managedRef, winnerCommit)
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("losing first claim = %v", err)
	}
	if got := strings.TrimSpace(runFixtureGit(t, "--git-dir", remote, "rev-parse", managedRef)); got != winnerCommit {
		t.Fatalf("loser overwrote winner: %s", got)
	}
	assertUnadvancedState(t, s)
}

func TestNormalPublicationExactLeasePreservesConcurrentWriter(t *testing.T) {
	s, remote := publicationFixture(t, true)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	before, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	workspace, _ := s.ReadWorkspace("w_shared")
	workspace.Name = "local next"
	if err := s.WriteWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	concurrentFiles := validPortableFiles()
	concurrentFiles[managedMarkerName] = "format: 1\ndestinationId: " + before.ManagedDestinationID + "\n"
	concurrentCommit := prepareFixtureCommit(t, remote, concurrentFiles)
	runtime.beforePush = func() {
		runFixtureGit(t, "--git-dir", remote, "update-ref", managedRef, concurrentCommit)
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("lease loss = %v", err)
	}
	if got := strings.TrimSpace(runFixtureGit(t, "--git-dir", remote, "rev-parse", managedRef)); got != concurrentCommit {
		t.Fatalf("concurrent commit overwritten: %s", got)
	}
	after, _ := s.ReadSync()
	if after.BaseHash != before.BaseHash || after.LastPushHash != before.LastPushHash {
		t.Fatalf("lease loss advanced BASE: before=%+v after=%+v", before, after)
	}
}

func TestPushErrorConfirmedByFreshObservationSucceeds(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.push = func(ctx context.Context, root string, prepared preparedPublication) error {
		if err := runner.pushPrepared(ctx, root, prepared); err != nil {
			t.Fatal(err)
		}
		return ErrTransportUnavailable
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatalf("fresh confirmation did not override transport result: %v", err)
	}
	state, _ := s.ReadSync()
	if !state.BasePresent {
		t.Fatal("confirmed publication did not advance BASE")
	}
}

func TestUnavailableConfirmationIsAmbiguousAndDoesNotAdvanceBase(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	inspections := 0
	runtime.inspect = func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
		inspections++
		if inspections == 1 {
			return RepositorySnapshot{State: RepositoryEmpty, RepositoryIdentity: "github.com/neutral/repository"}, nil
		}
		return RepositorySnapshot{}, ErrTransportUnavailable
	}
	runtime.push = func(context.Context, string, preparedPublication) error { return ErrTransportUnavailable }
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPublicationAmbiguous) {
		t.Fatalf("ambiguous publication = %v", err)
	}
	assertUnadvancedState(t, s)
}

func TestConcurrentUnrelatedRefPreventsBaseAdvancement(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.push = func(ctx context.Context, root string, prepared preparedPublication) error {
		if err := runner.pushPrepared(ctx, root, prepared); err != nil {
			return err
		}
		runFixtureGit(t, "--git-dir", remote, "update-ref", "refs/tags/unrelated", prepared.commitID)
		return nil
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrRepositoryIncompatible) {
		t.Fatalf("unrelated ref result = %v", err)
	}
	if runFixtureGit(t, "--git-dir", remote, "rev-parse", "refs/tags/unrelated") == "" {
		t.Fatal("publication deleted unrelated ref")
	}
	assertUnadvancedState(t, s)
}

func TestNormalPublicationPreservesMarkerIdentity(t *testing.T) {
	s, remote := publicationFixture(t, true)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	first, _ := s.ReadSync()
	firstSnapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	workspace, _ := s.ReadWorkspace("w_shared")
	workspace.Name = "next"
	if err := s.WriteWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err != nil {
		t.Fatal(err)
	}
	second, _ := s.ReadSync()
	if second.ManagedDestinationID != first.ManagedDestinationID {
		t.Fatalf("normal publication changed marker: %q != %q", second.ManagedDestinationID, first.ManagedDestinationID)
	}
	secondSnapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	commit := runFixtureGit(t, "--git-dir", remote, "cat-file", "-p", secondSnapshot.CommitID)
	if !strings.Contains(commit, "parent "+firstSnapshot.CommitID+"\n") {
		t.Fatalf("normal publication did not parent observed commit %s", firstSnapshot.CommitID)
	}
}

func prepareFixtureCommit(t *testing.T, remote string, files map[string]string) string {
	t.Helper()
	work := createWorkFixture(t, "context-baggage", files)
	commitID := strings.TrimSpace(runFixtureGit(t, "-C", work, "rev-parse", "HEAD"))
	runFixtureGit(t, "--git-dir", remote, "fetch", "--quiet", work, commitID)
	return commitID
}
