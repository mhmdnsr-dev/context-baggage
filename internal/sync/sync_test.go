package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestPushSkipsSyncFalseWorkspace(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_private", Name: "private", Sync: false, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "shared", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDir, "workspaces", "w_private")); !os.IsNotExist(err) {
		t.Fatalf("private workspace exported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDir, "workspaces", "w_shared")); err != nil {
		t.Fatalf("shared workspace missing: %v", err)
	}
}

func TestPullDetectsConflict(t *testing.T) {
	folder := t.TempDir()
	s1 := store.New(t.TempDir())
	s2 := store.New(t.TempDir())
	for _, s := range []store.Store{s1, s2} {
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		if _, err := Init(s, folder); err != nil {
			t.Fatal(err)
		}
	}
	w := store.Workspace{ID: "w_shared", Name: "shared", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}
	if err := s1.WriteWorkspace(w); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s1); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(s2); err != nil {
		t.Fatal(err)
	}
	w.Name = "remote"
	if err := s1.WriteWorkspace(w); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s1); err != nil {
		t.Fatal(err)
	}
	w.Name = "local"
	if err := s2.WriteWorkspace(w); err != nil {
		t.Fatal(err)
	}
	_, err := Pull(s2)
	if err == nil || !strings.Contains(err.Error(), "CONFLICT DETECTED") {
		t.Fatalf("expected conflict, got %v", err)
	}
	got, readErr := s2.ReadWorkspace("w_shared")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got.Name != "local" {
		t.Fatalf("local workspace was overwritten after conflict: %s", got.Name)
	}
}

func TestPortableHashIgnoresMachineLocalMetadata(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDevice(store.Device{ID: "d_before", Name: "before", OS: "linux", Arch: "amd64", CreatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "shared", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	before, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDevice(store.Device{ID: "d_after", Name: "after", OS: "linux", Arch: "amd64", CreatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSync(store.SyncState{
		Folder:       t.TempDir(),
		LastPush:     store.Now(),
		LastPull:     store.Now(),
		LastPushHash: "push",
		LastPullHash: "pull",
		BaseHash:     "base",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("machine-local metadata changed portable hash: %s != %s", before, after)
	}
}
