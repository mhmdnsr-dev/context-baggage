package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestPullImportsRemoteTasks(t *testing.T) {
	folder := t.TempDir()
	a := store.New(t.TempDir())
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(a, folder); err != nil {
		t.Fatal(err)
	}
	if err := a.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := a.WriteTask(store.Task{ID: "t_1", Name: "task one", WorkspaceID: "w_shared", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(a); err != nil {
		t.Fatal(err)
	}
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(b, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadTask("w_shared", "t_1"); err != nil {
		t.Fatalf("remote task not imported: %v", err)
	}
}

func TestEligibleHashEqualsExportedHash(t *testing.T) {
	s, folder := newStore(t)
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteTask(store.Task{ID: "t_1", Name: "one", WorkspaceID: "w_shared", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	e, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	h, err := Push(s)
	if err != nil {
		t.Fatal(err)
	}
	if e != h {
		t.Fatalf("eligibleHash != exported hash: %s != %s", e, h)
	}
}

func TestPullConverges(t *testing.T) {
	folder := t.TempDir()
	a := store.New(t.TempDir())
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(a, folder); err != nil {
		t.Fatal(err)
	}
	if err := a.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(a); err != nil {
		t.Fatal(err)
	}
	exported, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(b, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	local, err := eligibleHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if local != exported {
		t.Fatalf("after pull LOCAL != REMOTE: %s != %s", local, exported)
	}
}

// writeLegacyWorkspace seeds a v0.1-style legacy workspace export.
func writeLegacyWorkspace(t *testing.T, folder, id, yaml string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(folder, exportDir, "workspaces", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
