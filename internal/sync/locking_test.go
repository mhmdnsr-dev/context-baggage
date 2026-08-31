package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestLockPushRequiresExclusiveSyncOwnership(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(lockTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	release := holdLock(t, s.AcquireSyncShared)

	if _, err := Push(s); !errors.Is(err, store.ErrOperationBusy) {
		t.Fatalf("contended Push error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("contended Push mutated destination: %v", err)
	}
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastPush != "" || state.BasePresent {
		t.Fatalf("contended Push mutated bookkeeping: %+v", state)
	}
	release()
}

func TestLockPushBuildsSnapshotUnderCanonicalSharedLock(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(lockTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	readRelease := holdLock(t, s.AcquireCanonicalShared)
	if _, err := Push(s); err != nil {
		t.Fatalf("Push did not coexist with canonical reader: %v", err)
	}
	readRelease()

	beforeState, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	beforeRemote, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	writeRelease := holdLock(t, s.AcquireCanonicalExclusive)
	if _, err := Push(s); !errors.Is(err, store.ErrOperationBusy) {
		t.Fatalf("Push without canonical snapshot ownership error = %v", err)
	}
	writeRelease()
	assertSyncAndRemoteUnchanged(t, s, folder, beforeState, beforeRemote)
}

func TestLockPullRequiresCanonicalExclusiveBeforeMutation(t *testing.T) {
	folder := t.TempDir()
	producer := store.New(t.TempDir())
	consumer := store.New(t.TempDir())
	for _, s := range []store.Store{producer, consumer} {
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		if _, err := Init(s, folder); err != nil {
			t.Fatal(err)
		}
	}
	if err := producer.WriteWorkspace(lockTestWorkspace()); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(producer); err != nil {
		t.Fatal(err)
	}
	release := holdLock(t, consumer.AcquireCanonicalShared)

	if _, err := Pull(consumer); !errors.Is(err, store.ErrOperationBusy) {
		t.Fatalf("Pull without canonical mutation ownership error = %v", err)
	}
	if _, err := consumer.ReadWorkspace("w_lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("contended Pull mutated canonical state: %v", err)
	}
	state, err := consumer.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastPull != "" || state.BasePresent {
		t.Fatalf("contended Pull mutated bookkeeping: %+v", state)
	}
	release()
	if _, err := Pull(consumer); err != nil {
		t.Fatalf("Pull failed after canonical lock release: %v", err)
	}
}

type testLockAcquirer func(context.Context) (func() error, error)

func holdLock(t *testing.T, acquire testLockAcquirer) func() {
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

func lockTestWorkspace() store.Workspace {
	now := store.Now()
	return store.Workspace{ID: "w_lock", Name: "lock", Sync: true, CreatedAt: now, UpdatedAt: now}
}

func assertSyncAndRemoteUnchanged(t *testing.T, s store.Store, folder string, wantState []byte, wantRemote string) {
	t.Helper()
	gotState, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(gotState) != string(wantState) {
		t.Fatal("contended Push changed sync state")
	}
	gotRemote, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	if gotRemote != wantRemote {
		t.Fatalf("contended Push changed remote: %s != %s", gotRemote, wantRemote)
	}
}
