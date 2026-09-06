package githubsync

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func recoveryFixtureRuntime(t *testing.T, runner GitRunner, remote string) recoveryRuntime {
	t.Helper()
	return recoveryRuntime{
		materialize: func(ctx context.Context, git GitRunner, locator Locator, commitID string) (RepositorySnapshot, string, func(), error) {
			root, err := os.MkdirTemp("", "ctx-bag-recover-*")
			if err != nil {
				return RepositorySnapshot{}, "", nil, ErrTransportUnavailable
			}
			snapshot, portableDir, err := git.inspectAndMaterialize(ctx, remote, locator.identity, commitID, root)
			if err != nil {
				_ = os.RemoveAll(root)
				return RepositorySnapshot{}, "", nil, err
			}
			return snapshot, portableDir, func() { _ = os.RemoveAll(root) }, nil
		},
	}
}

func recoveryRecordFixture(t *testing.T, targetCommitID, preLocalHash, targetPortableHash string) store.PullRecovery {
	t.Helper()
	return store.PullRecovery{
		Format:             store.PullRecoveryFormatVersion,
		DestinationType:    store.DestinationGitHub,
		DestinationID:      neutralDestinationID,
		RepositoryIdentity: "github.com/neutral/repository",
		TargetCommitID:     targetCommitID,
		PreLocalHash:       preLocalHash,
		TargetPortableHash: targetPortableHash,
	}
}

func TestRecoveryBeforeApply(t *testing.T) {
	remote, commitA := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	snapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	preHash, err := portablesync.EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WritePullRecovery(recoveryRecordFixture(t, commitA, preHash, snapshot.PortableHash)); err != nil {
		t.Fatal(err)
	}
	hash, err := recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	if hash != snapshot.PortableHash {
		t.Fatalf("recovery returned %s, want %s", hash, snapshot.PortableHash)
	}
	local, _ := portablesync.EligibleHash(s)
	if local != snapshot.PortableHash {
		t.Fatalf("LOCAL != target after recovery: %s != %s", local, snapshot.PortableHash)
	}
	state, _ := s.ReadSync()
	if !state.BasePresent || state.BaseHash != snapshot.PortableHash {
		t.Fatalf("recovery did not persist BASE: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after recovery")
	}
}

func TestRecoveryAfterApply(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	snapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_one", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory", Value: "neutral"},
		Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePullRecovery(recoveryRecordFixture(t, snapshot.CommitID, strings.Repeat("b", 64), snapshot.PortableHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote)); err != nil {
		t.Fatal(err)
	}
	state, _ := s.ReadSync()
	if !state.BasePresent || state.BaseHash != snapshot.PortableHash {
		t.Fatalf("recovery after apply did not finalize BASE: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("recovery record not cleared after apply recovery")
	}
}

func TestRecoveryAmbiguous(t *testing.T) {
	remote, commitA := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	snapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	preHash, _ := portablesync.EligibleHash(s)
	if err := s.WriteWorkspace(store.Workspace{ID: "w_one", Name: "ambiguous", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePullRecovery(recoveryRecordFixture(t, commitA, preHash, snapshot.PortableHash)); err != nil {
		t.Fatal(err)
	}
	_, err = recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote))
	if !errors.Is(err, ErrRecoveryAmbiguous) {
		t.Fatalf("expected ambiguous refusal, got %v", err)
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("BASE set on ambiguous recovery: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record dropped on ambiguous recovery")
	}
}

func TestRecoveryRemoteMovedUsesRecordedTarget(t *testing.T) {
	remote, commitA := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	snapshotA, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	bFiles := validPortableFiles()
	bFiles[portableRootName+"/workspaces/w_one/workspace.yaml"] = portableWorkspaceYAMLWithName("moved")
	advanceManagedRemote(t, remote, bFiles)
	preHash, _ := portablesync.EligibleHash(s)
	if err := s.WritePullRecovery(recoveryRecordFixture(t, commitA, preHash, snapshotA.PortableHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote)); err != nil {
		t.Fatal(err)
	}
	local, _ := portablesync.EligibleHash(s)
	if local != snapshotA.PortableHash {
		t.Fatalf("recovery followed remote head instead of recorded target: %s != %s", local, snapshotA.PortableHash)
	}
	state, _ := s.ReadSync()
	if state.BaseHash != snapshotA.PortableHash {
		t.Fatalf("BASE set to wrong target: %+v", state)
	}
}

func TestRecoveryRefusesUnfetchableTarget(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	snapshot, err := inspectLocalFixture(t, remote)
	if err != nil {
		t.Fatal(err)
	}
	preHash, _ := portablesync.EligibleHash(s)
	missingCommit := strings.Repeat("f", 40)
	if err := s.WritePullRecovery(recoveryRecordFixture(t, missingCommit, preHash, snapshot.PortableHash)); err != nil {
		t.Fatal(err)
	}
	_, err = recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote))
	if err == nil {
		t.Fatal("expected refusal for unfetchable target")
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record dropped when target unfetchable")
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("BASE set despite unfetchable target: %+v", state)
	}
}

func TestRecoveryBindingMismatchRefuses(t *testing.T) {
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	record := recoveryRecordFixture(t, strings.Repeat("a", 40), strings.Repeat("b", 64), strings.Repeat("c", 64))
	record.DestinationID = "dst_ffffffffffffffffffffffffffffffff"
	if err := s.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	_, err := recoverManagedPull(context.Background(), s, runner, recoveryFixtureRuntime(t, runner, remote))
	if !errors.Is(err, ErrRecoveryBindingMismatch) {
		t.Fatalf("expected binding mismatch, got %v", err)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record dropped on binding mismatch")
	}
}
