package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestLockCanonicalMutationRefusesBeforeWorkspaceWrite(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	runCLI(t, home, cwd, "init")
	s := store.New(home)
	release := holdAppLock(t, s.AcquireCanonicalShared)

	_, runErr := runCLIErr(t, home, cwd, "workspace", "init", "--sync")
	if !errors.Is(runErr, store.ErrOperationBusy) {
		t.Fatalf("contended workspace init error = %v", runErr)
	}
	release()
	workspaces, err := s.ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("contended workspace init wrote %d workspaces", len(workspaces))
	}
}

func TestLockSyncStatusUsesSharedLockWithoutRewritingState(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	folder := t.TempDir()
	runCLI(t, home, cwd, "init")
	runCLI(t, home, cwd, "sync", "init", folder)
	s := store.New(home)
	before, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	sharedRelease := holdAppLock(t, s.AcquireSyncShared)
	var out bytes.Buffer
	if err := runSyncStatus(s, &out); err != nil {
		t.Fatalf("shared sync status failed: %v", err)
	}
	sharedRelease()
	after, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("sync status rewrote product state")
	}

	exclusiveRelease := holdAppLock(t, s.AcquireSyncExclusive)
	out.Reset()
	if err := runSyncStatus(s, &out); !errors.Is(err, store.ErrOperationBusy) {
		t.Fatalf("status under exclusive sync owner error = %v", err)
	}
	exclusiveRelease()
}

type appLockAcquirer func(context.Context) (func() error, error)

func holdAppLock(t *testing.T, acquire appLockAcquirer) func() {
	t.Helper()
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		unlock, err := acquire(context.Background())
		if err != nil {
			t.Errorf("acquire test lock: %v", err)
			close(held)
			return
		}
		close(held)
		<-release
		if err := unlock(); err != nil {
			t.Errorf("release test lock: %v", err)
		}
	}()
	<-held
	var released bool
	releaseLock := func() {
		if released {
			return
		}
		released = true
		close(release)
		<-done
	}
	t.Cleanup(releaseLock)
	return releaseLock
}
