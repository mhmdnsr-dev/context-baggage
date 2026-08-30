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
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_private")); !os.IsNotExist(err) {
		t.Fatalf("private workspace exported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_shared")); err != nil {
		t.Fatalf("shared workspace missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDir)); !os.IsNotExist(err) {
		t.Fatalf("legacy namespace should not be created by v2 push: %v", err)
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

func readExportWorkspace(t *testing.T, folder, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(folder, exportDirV2, "workspaces", id, "workspace.yaml"))
	if err != nil {
		t.Fatalf("read exported workspace.yaml: %v", err)
	}
	return string(b)
}

func newStore(t *testing.T) (store.Store, string) {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s, t.TempDir()
}

func TestProjectionOmitsLocalPathsAndUpdatedAt(t *testing.T) {
	s, folder := newStore(t)
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory"},
		LocalPaths: []string{"/home/alice/work/example"}, Sync: true,
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	content := readExportWorkspace(t, folder, "w_shared")
	if strings.Contains(content, "/home/alice") || strings.Contains(content, "localPaths") {
		t.Fatalf("LocalPaths leaked into projection:\n%s", content)
	}
	if strings.Contains(content, "updatedAt") || strings.Contains(content, "2026-01-02") {
		t.Fatalf("UpdatedAt leaked into projection:\n%s", content)
	}
	if !strings.Contains(content, "sync: true") || !strings.Contains(content, "createdAt: 2026-01-01T00:00:00Z") {
		t.Fatalf("portable fields missing:\n%s", content)
	}
}

func TestProjectionPreservesIdentity(t *testing.T) {
	s, folder := newStore(t)
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_git", Name: "repo", Identity: store.WorkspaceIdentity{Type: "git-remote", Value: "example.com/org/repo"},
		Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	content := readExportWorkspace(t, folder, "w_git")
	if !strings.Contains(content, "type: git-remote") || !strings.Contains(content, "value: example.com/org/repo") {
		t.Fatalf("Identity not preserved:\n%s", content)
	}
}

func TestPortableHashIgnoresLocalPaths(t *testing.T) {
	s1, _ := newStore(t)
	s2, _ := newStore(t)
	for _, s := range []store.Store{s1, s2} {
		if err := s.WriteWorkspace(store.Workspace{
			ID: "w_shared", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory"},
			Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s1.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory"},
		LocalPaths: []string{"/home/a/foo"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s2.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Identity: store.WorkspaceIdentity{Type: "local-directory"},
		LocalPaths: []string{`D:\work\foo`}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	h1, err := eligibleHash(s1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := eligibleHash(s2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("LocalPaths changed portable hash: %s != %s", h1, h2)
	}
}

func TestPortableHashIgnoresUpdatedAt(t *testing.T) {
	s1, _ := newStore(t)
	s2, _ := newStore(t)
	if err := s1.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s2.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-05-05T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	h1, _ := eligibleHash(s1)
	h2, _ := eligibleHash(s2)
	if h1 != h2 {
		t.Fatalf("UpdatedAt changed portable hash: %s != %s", h1, h2)
	}
}

func TestUnknownFilesIgnored(t *testing.T) {
	s, folder := newStore(t)
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteTask(store.Task{ID: "t_1", Name: "task one", WorkspaceID: "w_shared", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	before, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	// Add unknown files at workspace root and inside a task dir.
	unknownRoot := filepath.Join(s.WorkspaceDir("w_shared"), "local-debug.json")
	unknownTask := filepath.Join(s.WorkspaceDir("w_shared"), "tasks", "t_1", "local-debug.json")
	for _, p := range []string{unknownRoot, unknownTask} {
		if err := os.WriteFile(p, []byte("debug"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	after, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unknown files changed portable hash: %s != %s", before, after)
	}
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_shared", "local-debug.json")); !os.IsNotExist(err) {
		t.Fatalf("unknown root file exported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_shared", "tasks", "t_1", "local-debug.json")); !os.IsNotExist(err) {
		t.Fatalf("unknown task file exported: %v", err)
	}
}

func TestConfigYAMLNotExported(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config.yaml exported into v2: %v", err)
	}
}

func TestLegacyIsolationOnPush(t *testing.T) {
	s, folder := newStore(t)
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	// Establish v2 first, then seed legacy so the folder is in the nsBoth state.
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(folder, exportDir), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(folder, exportDir, "legacy-sentinel")
	if err := os.WriteFile(legacy, []byte("do-not-touch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatalf("expected v2 push with both namespaces: %v", err)
	}
	b, err := os.ReadFile(legacy)
	if err != nil || string(b) != "do-not-touch" {
		t.Fatalf("v2 push modified legacy namespace: %v %q", err, string(b))
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2)); err != nil {
		t.Fatalf("v2 namespace missing: %v", err)
	}
}

func TestPullPreservesLocalPaths(t *testing.T) {
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
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(b, folder); err != nil {
		t.Fatal(err)
	}
	if err := b.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", LocalPaths: []string{"/machine-b/foo"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-05-05T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	hash, err := Pull(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.ReadWorkspace("w_shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LocalPaths) != 1 || got.LocalPaths[0] != "/machine-b/foo" {
		t.Fatalf("pull did not preserve LocalPaths: %v", got.LocalPaths)
	}
	if got.UpdatedAt != "2026-05-05T00:00:00Z" {
		t.Fatalf("pull did not preserve UpdatedAt: %q", got.UpdatedAt)
	}
	state, err := b.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if state.BaseHash != hash || state.LastPullHash != hash {
		t.Fatalf("successful pull did not update BASE: %+v, hash %s", state, hash)
	}
}

func TestPullRemovesStalePortableState(t *testing.T) {
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
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(b, folder); err != nil {
		t.Fatal(err)
	}
	// Establish the shared baseline, then diverge locally: the stale task is
	// portable-owned and must be removed by a later pull.
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	if err := b.WriteTask(store.Task{ID: "t_old", Name: "old", WorkspaceID: "w_shared", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetActiveTask("w_shared", "t_old"); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadTask("w_shared", "t_old"); err == nil {
		t.Fatalf("stale task not removed by pull")
	}
	if _, err := os.Stat(filepath.Join(b.WorkspaceDir("w_shared"), "active-task")); !os.IsNotExist(err) {
		t.Fatalf("stale active-task not removed by pull: %v", err)
	}
}
