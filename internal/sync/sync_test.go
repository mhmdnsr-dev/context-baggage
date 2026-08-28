package sync

import (
	"errors"
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
	if err := b.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", LocalPaths: []string{"/machine-b/foo"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	got, err := b.ReadWorkspace("w_shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LocalPaths) != 1 || got.LocalPaths[0] != "/machine-b/foo" {
		t.Fatalf("pull did not preserve LocalPaths: %v", got.LocalPaths)
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

func TestNamespaceStateMatrix(t *testing.T) {
	folder := t.TempDir()
	state, err := NamespaceState(folder)
	if err != nil || state != NamespaceNone {
		t.Fatalf("none: got %v, %v", state, err)
	}
	if err := os.MkdirAll(filepath.Join(folder, exportDir), 0o700); err != nil {
		t.Fatal(err)
	}
	state, _ = NamespaceState(folder)
	if state != NamespaceLegacyOnly {
		t.Fatalf("legacy-only: got %v", state)
	}
	if err := os.MkdirAll(filepath.Join(folder, exportDirV2), 0o700); err != nil {
		t.Fatal(err)
	}
	state, _ = NamespaceState(folder)
	if state != NamespaceBoth {
		t.Fatalf("both: got %v", state)
	}
	if err := os.RemoveAll(filepath.Join(folder, exportDir)); err != nil {
		t.Fatal(err)
	}
	state, _ = NamespaceState(folder)
	if state != NamespaceV2Only {
		t.Fatalf("v2-only: got %v", state)
	}
}

func TestEmptyV2BeatsPopulatedLegacy(t *testing.T) {
	folder := t.TempDir()
	// Populated legacy.
	writeLegacyWorkspace(t, folder, "w_legacy", "id: w_legacy\nname: legacy\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n", nil)
	// Empty v2 present.
	if err := os.MkdirAll(filepath.Join(folder, exportDirV2), 0o700); err != nil {
		t.Fatal(err)
	}
	ws, _, err := ListPortableWorkspaces(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 0 {
		t.Fatalf("empty v2 must not fall back to populated legacy, got %d", len(ws))
	}
}

func TestCorruptV2NoFallback(t *testing.T) {
	folder := t.TempDir()
	// Corrupt v2 entry.
	corrupt := filepath.Join(folder, exportDirV2, "workspaces", "w_bad")
	if err := os.MkdirAll(corrupt, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, "workspace.yaml"), []byte("name: bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Valid legacy.
	writeLegacyWorkspace(t, folder, "w_good", "id: w_good\nname: good\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n", nil)
	ws, warnings, err := ListPortableWorkspaces(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 0 {
		t.Fatalf("corrupt v2 must not fall back to legacy, got %d valid", len(ws))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 corrupt warning, got %d", len(warnings))
	}
}

func TestSyncUpgradeFormatOnly(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	// Seed a legacy workspace with machine-local + continuity + unknown files.
	legacyYAML := "id: w_local\nname: notes\nidentity:\n  type: local-directory\n  value: \nlocalPaths:\n  - /home/alice/private/work/foo\nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\nupdatedAt: 2026-01-02T00:00:00Z\n"
	writeLegacyWorkspace(t, folder, "w_local", legacyYAML, map[string]string{
		"active-task":                 "t_1\n",
		"tasks/t_1/task.yaml":         "id: t_1\nname: one\n",
		"tasks/t_1/checkpoints.jsonl": "{}\n",
		"tasks/t_1/handoff.md":        "# handoff\n",
		"local-debug.json":            "debug-root",
		"tasks/t_1/local-debug.json":  "debug-task",
	})
	if err := os.WriteFile(filepath.Join(folder, exportDir, "config.yaml"), []byte("version: 0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stBefore, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncUpgrade(s); err != nil {
		t.Fatal(err)
	}
	// Format-only: local store unchanged.
	if _, err := s.ReadWorkspace("w_local"); err == nil {
		t.Fatalf("upgrade must not import into local canonical store")
	}
	// Sync hashes unchanged.
	stAfter, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if stAfter.BaseHash != stBefore.BaseHash || stAfter.LastPush != stBefore.LastPush {
		t.Fatalf("upgrade changed sync bookkeeping")
	}
	// Legacy namespace preserved.
	if _, err := os.Stat(filepath.Join(folder, exportDir, "workspaces", "w_local", "workspace.yaml")); err != nil {
		t.Fatalf("legacy namespace modified: %v", err)
	}
	// v2 created and sanitized.
	content := readExportWorkspace(t, folder, "w_local")
	if strings.Contains(content, "/home/alice") || strings.Contains(content, "localPaths") || strings.Contains(content, "updatedAt") {
		t.Fatalf("legacy machine-local fields leaked into v2:\n%s", content)
	}
	// Known continuity preserved.
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_local", "active-task")); err != nil {
		t.Fatalf("active-task not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_local", "tasks", "t_1", "handoff.md")); err != nil {
		t.Fatalf("handoff not preserved: %v", err)
	}
	// Unknown files excluded.
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_local", "local-debug.json")); !os.IsNotExist(err) {
		t.Fatalf("unknown root file exported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "workspaces", "w_local", "tasks", "t_1", "local-debug.json")); !os.IsNotExist(err) {
		t.Fatalf("unknown task file exported: %v", err)
	}
	// config.yaml absent.
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config.yaml exported: %v", err)
	}
}

func TestUpgradeProjectionEquivalence(t *testing.T) {
	// Same logical workspace exported via normal push vs legacy upgrade should
	// produce an equivalent v2 projection.
	folderUp := t.TempDir()
	sUp, _ := newStore(t)
	if _, err := Init(sUp, folderUp); err != nil {
		t.Fatal(err)
	}
	legacyYAML := "id: w_x\nname: x\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n"
	writeLegacyWorkspace(t, folderUp, "w_x", legacyYAML, map[string]string{
		"tasks/t_1/task.yaml":         "id: t_1\nname: one\nworkspaceId: w_x\nstatus: active\ncreatedAt: 2026-01-01T00:00:00Z\nupdatedAt: 2026-01-01T00:00:00Z\n",
		"tasks/t_1/checkpoints.jsonl": "",
	})
	if err := SyncUpgrade(sUp); err != nil {
		t.Fatal(err)
	}
	upHash, err := store.HashDir(filepath.Join(folderUp, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}

	folderPush := t.TempDir()
	sPush, _ := newStore(t)
	if _, err := Init(sPush, folderPush); err != nil {
		t.Fatal(err)
	}
	if err := sPush.WriteWorkspace(store.Workspace{ID: "w_x", Name: "x", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := sPush.WriteTask(store.Task{ID: "t_1", Name: "one", WorkspaceID: "w_x", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(sPush); err != nil {
		t.Fatal(err)
	}
	pushHash, err := store.HashDir(filepath.Join(folderPush, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	if upHash != pushHash {
		t.Fatalf("upgrade projection != normal push projection: %s != %s", upHash, pushHash)
	}
}

func TestPushRefusesLegacyOnly(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(folder, exportDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err == nil || !strings.Contains(err.Error(), "sync upgrade") {
		t.Fatalf("expected push to refuse in legacy-only, got %v", err)
	}
}

func TestNoBasePush(t *testing.T) {
	folder := t.TempDir()
	// Seed one workspace into the shared folder.
	producer := store.New(t.TempDir())
	if err := producer.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(producer, folder); err != nil {
		t.Fatal(err)
	}
	if err := producer.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	// LOCAL=X, REMOTE=empty -> allowed.
	if _, err := Push(producer); err != nil {
		t.Fatalf("LOCAL=X REMOTE=empty push should succeed: %v", err)
	}

	// LOCAL=empty, REMOTE=X -> refuse (no shared baseline, differing state).
	emptyStore := store.New(t.TempDir())
	if err := emptyStore.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(emptyStore, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(emptyStore); err == nil || !strings.Contains(err.Error(), "no shared baseline") {
		t.Fatalf("LOCAL=empty REMOTE=X push should refuse, got %v", err)
	}

	// LOCAL=X, REMOTE=X -> allowed (base still empty for this store, but equal).
	sameStore := store.New(t.TempDir())
	if err := sameStore.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(sameStore, folder); err != nil {
		t.Fatal(err)
	}
	if err := sameStore.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(sameStore); err != nil {
		t.Fatalf("LOCAL=X REMOTE=X push should succeed: %v", err)
	}

	// LOCAL=X, REMOTE=Y -> refuse.
	diffStore := store.New(t.TempDir())
	if err := diffStore.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(diffStore, folder); err != nil {
		t.Fatal(err)
	}
	if err := diffStore.WriteWorkspace(store.Workspace{ID: "w_other", Name: "other", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(diffStore); err == nil || !strings.Contains(err.Error(), "no shared baseline") {
		t.Fatalf("LOCAL=X REMOTE=Y push should refuse, got %v", err)
	}
}

func TestNoBasePull(t *testing.T) {
	// LOCAL=empty, REMOTE=X -> allowed and converges
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
	if _, err := Pull(b); err != nil {
		t.Fatalf("LOCAL=empty REMOTE=X pull should succeed: %v", err)
	}

	// LOCAL=X, REMOTE=Y -> refuse
	c := store.New(t.TempDir())
	if err := c.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(c, folder); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteWorkspace(store.Workspace{ID: "w_other", Name: "other", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(c); err == nil || !strings.Contains(err.Error(), "no shared baseline") {
		t.Fatalf("LOCAL=X REMOTE=Y pull should refuse, got %v", err)
	}
}

func TestPostUpgradeFirstPullConverges(t *testing.T) {
	folder := t.TempDir()
	s, _ := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	writeLegacyWorkspace(t, folder, "w_local", "id: w_local\nname: notes\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n", nil)
	if err := SyncUpgrade(s); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(s); err != nil {
		t.Fatalf("post-upgrade fresh pull should succeed: %v", err)
	}
	if _, err := s.ReadWorkspace("w_local"); err != nil {
		t.Fatalf("expected imported workspace: %v", err)
	}
	exported, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	local, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if local != exported {
		t.Fatalf("post-upgrade pull LOCAL != REMOTE: %s != %s", local, exported)
	}
}

func TestListOnlyAttachable(t *testing.T) {
	folder := t.TempDir()
	for _, id := range []string{"w_local", "w_gitlocal", "w_gitremote"} {
		typ := "local-directory"
		if id == "w_gitlocal" {
			typ = "git-local"
		}
		if id == "w_gitremote" {
			typ = "git-remote"
		}
		ws := filepath.Join(folder, exportDirV2, "workspaces", id)
		if err := os.MkdirAll(ws, 0o700); err != nil {
			t.Fatal(err)
		}
		yaml := "id: " + id + "\nname: " + id + "\nidentity:\n  type: " + typ + "\n  value: x\nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n"
		if err := os.WriteFile(filepath.Join(ws, "workspace.yaml"), []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ws, warnings, err := ListPortableWorkspaces(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 3 || len(warnings) != 0 {
		t.Fatalf("expected 3 workspaces no warnings, got %d/%d", len(ws), len(warnings))
	}
}

func TestFindPortableWorkspace(t *testing.T) {
	folder := t.TempDir()
	good := filepath.Join(folder, exportDirV2, "workspaces", "w_good")
	if err := os.MkdirAll(good, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, "workspace.yaml"), []byte("id: w_good\nname: good\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, err := FindPortableWorkspace(folder, "w_good"); err != nil || p.ID != "w_good" {
		t.Fatalf("expected found w_good, got %+v %v", p, err)
	}
	if _, err := FindPortableWorkspace(folder, "w_missing"); !errors.Is(err, errPortableNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
	// Directory-ID mismatch is corrupt.
	bad := filepath.Join(folder, exportDirV2, "workspaces", "w_dir")
	if err := os.MkdirAll(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "workspace.yaml"), []byte("id: w_other\nname: bad\nidentity:\n  type: local-directory\n  value: \nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FindPortableWorkspace(folder, "w_dir"); !errors.Is(err, errPortableCorrupt) {
		t.Fatalf("expected corrupt for directory/metadata mismatch, got %v", err)
	}
}

func TestDiscoveryIsReadOnly(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_shared", Name: "example", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(s); err != nil {
		t.Fatal(err)
	}
	stBefore, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	v2HashBefore, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ListPortableWorkspaces(folder); err != nil {
		t.Fatal(err)
	}
	stAfter, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if stAfter != stBefore {
		t.Fatalf("discovery modified sync state: %+v -> %+v", stBefore, stAfter)
	}
	v2HashAfter, err := store.HashDir(filepath.Join(folder, exportDirV2))
	if err != nil {
		t.Fatal(err)
	}
	if v2HashAfter != v2HashBefore {
		t.Fatalf("discovery modified v2 namespace")
	}
}
