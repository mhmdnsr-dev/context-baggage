package githubsync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func TestPrivacyRefusalPerformsNoRemoteWrite(t *testing.T) {
	s, remote := publicationFixture(t, true)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
		return Unverifiable, ErrPrivacyUnverifiable
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); !errors.Is(err, ErrPrivacyRefused) {
		t.Fatalf("privacy result = %v", err)
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("privacy refusal changed remote: empty=%t error=%v", empty, err)
	}
	assertUnadvancedState(t, s)
}

func TestPublicationMaterializationLimitUsesBoundedSeam(t *testing.T) {
	s, _ := publicationFixture(t, true)
	if _, err := portablesync.BuildPushSnapshotBounded(s, filepath.Join(t.TempDir(), "bounded"), 1); !errors.Is(err, portablesync.ErrPortableExportLimit) {
		t.Fatalf("bounded snapshot capture = %v", err)
	}
	root := filepath.Join(t.TempDir(), "portable")
	hash, err := portablesync.BuildPushSnapshot(s, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePublicationSnapshotWithLimit(root, hash, 1); !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("small materialization limit = %v", err)
	}
	if err := validatePublicationSnapshot(root, hash); err != nil {
		t.Fatalf("production materialization limit rejected fixture: %v", err)
	}
}

func TestPublicationTemporaryGuardPrecedesRemoteWrite(t *testing.T) {
	s, remote := publicationFixture(t, true)
	runner := localFixtureRunner(t)
	runner.testTemporaryLimit = 1
	if _, err := publishManaged(context.Background(), s, runner, localPublicationRuntime(t, runner, remote)); !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("temporary guard = %v", err)
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("temporary refusal changed remote: empty=%t error=%v", empty, err)
	}
}

func TestInvalidLocalSnapshotPerformsNoRemoteObservationOrWrite(t *testing.T) {
	s, remote := publicationFixture(t, true)
	if err := os.WriteFile(s.ActiveTaskPath("w_shared"), []byte("missing-task\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	inspections := 0
	runtime.inspect = func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
		inspections++
		return RepositorySnapshot{State: RepositoryEmpty}, nil
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err == nil {
		t.Fatal("invalid LOCAL snapshot was accepted")
	}
	if inspections != 0 {
		t.Fatalf("invalid LOCAL reached remote inspection %d times", inspections)
	}
	if _, empty, err := runner.observeManagedRef(context.Background(), remote); err != nil || !empty {
		t.Fatalf("invalid LOCAL changed remote: empty=%t error=%v", empty, err)
	}
}

func TestStateWriteFailureDoesNotRollBackConfirmedRemote(t *testing.T) {
	s, remote := publicationFixture(t, false)
	runner := localFixtureRunner(t)
	runtime := localPublicationRuntime(t, runner, remote)
	runtime.push = func(ctx context.Context, root string, prepared preparedPublication) error {
		if err := runner.pushPrepared(ctx, root, prepared); err != nil {
			return err
		}
		if err := os.Remove(s.SyncPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(s.SyncPath(), 0o700); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	if _, err := publishManaged(context.Background(), s, runner, runtime); err == nil {
		t.Fatal("expected sync-state persistence failure")
	}
	observed, err := inspectLocalFixture(t, remote)
	if err != nil || observed.State != RepositoryInitialized {
		t.Fatalf("confirmed remote was rolled back: %+v error=%v", observed, err)
	}
}

func TestManagedPublicationUsesCrossProcessSyncLock(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	unlock, err := s.AcquireSyncExclusive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), publicationLockHelperEnvironment+"="+s.Home)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("publication lock helper: %v: %s", err, output)
	}
}

func runPublicationLockHelper() {
	s := store.New(os.Getenv(publicationLockHelperEnvironment))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := PublishManaged(ctx, s, GitRunner{})
	if err == nil || time.Since(started) < 100*time.Millisecond {
		os.Exit(2)
	}
}
