package sync

import (
	"os"
	"path/filepath"
	"strings"
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

func TestPullPreflightRejectsWorkspaceDirectoryMetadataIDMismatch(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{
		ID: "w_metadata", Name: "local", LocalPaths: []string{"/machine/local"},
		Sync: false, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteTask(store.Task{
		ID: "t_local", Name: "local task", WorkspaceID: "w_metadata", Status: "active",
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	writePortableV2Workspace(t, folder, "w_directory", store.PortableWorkspace{
		ID: "w_metadata", Name: "remote", Sync: true, CreatedAt: "2026-01-01T00:00:00Z",
	}, nil)

	establishLocalBase(t, s)
	beforeHash, beforeSync := snapshotPullState(t, s)
	_, err := Pull(s)
	if err == nil || !strings.Contains(err.Error(), `directory ID "w_directory" does not match workspace metadata ID "w_metadata"`) {
		t.Fatalf("expected workspace identity mismatch, got %v", err)
	}
	assertPullStateUnchanged(t, s, beforeHash, beforeSync)
	if _, err := s.ReadTask("w_metadata", "t_local"); err != nil {
		t.Fatalf("metadata ID workspace was mutated: %v", err)
	}
	if _, err := s.ReadWorkspace("w_directory"); !os.IsNotExist(err) {
		t.Fatalf("directory ID workspace was created: %v", err)
	}
}

func TestPullPreflightRejectsMalformedExistingLocalWorkspace(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	localWorkspacePath := s.WorkspacePath("w_shared")
	if err := os.MkdirAll(filepath.Dir(localWorkspacePath), 0o700); err != nil {
		t.Fatal(err)
	}
	malformed := []byte("name: malformed local workspace\n")
	if err := os.WriteFile(localWorkspacePath, malformed, 0o600); err != nil {
		t.Fatal(err)
	}
	localTaskPath := s.TaskPath("w_shared", "t_local")
	if err := os.MkdirAll(filepath.Dir(localTaskPath), 0o700); err != nil {
		t.Fatal(err)
	}
	localTask := []byte("local task must survive\n")
	if err := os.WriteFile(localTaskPath, localTask, 0o600); err != nil {
		t.Fatal(err)
	}
	writePortableV2Workspace(t, folder, "w_shared", store.PortableWorkspace{
		ID: "w_shared", Name: "remote", Sync: true, CreatedAt: "2026-01-01T00:00:00Z",
	}, map[string]string{
		"tasks/t_remote/task.yaml": "id: t_remote\nname: remote task\nworkspaceId: w_shared\nstatus: active\ncreatedAt: 2026-01-01T00:00:00Z\nupdatedAt: 2026-01-01T00:00:00Z\n",
		"active-task":              "t_remote\n",
	})

	establishLocalBase(t, s)
	beforeHash, beforeSync := snapshotPullState(t, s)
	_, err := Pull(s)
	if err == nil || !strings.Contains(err.Error(), `read local workspace "w_shared" before pull`) || !strings.Contains(err.Error(), "workspace id missing") {
		t.Fatalf("expected malformed local workspace refusal, got %v", err)
	}
	assertPullStateUnchanged(t, s, beforeHash, beforeSync)
	gotWorkspace, err := os.ReadFile(localWorkspacePath)
	if err != nil || string(gotWorkspace) != string(malformed) {
		t.Fatalf("malformed local workspace was overwritten: %v %q", err, gotWorkspace)
	}
	gotTask, err := os.ReadFile(localTaskPath)
	if err != nil || string(gotTask) != string(localTask) {
		t.Fatalf("related local task state was changed: %v %q", err, gotTask)
	}
}

func TestPullPreflightRejectsInvalidPortableTaskState(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name: "task directory metadata mismatch",
			files: map[string]string{
				"tasks/t_directory/task.yaml": "id: t_metadata\nname: task\nworkspaceId: w_shared\n",
			},
			wantErr: `task directory ID "t_directory" does not match task metadata ID "t_metadata"`,
		},
		{
			name: "task workspace mismatch",
			files: map[string]string{
				"tasks/t_task/task.yaml": "id: t_task\nname: task\nworkspaceId: w_other\n",
			},
			wantErr: `task "t_task" workspace ID "w_other" does not match workspace "w_shared"`,
		},
		{
			name: "missing task metadata ID",
			files: map[string]string{
				"tasks/t_task/task.yaml": "name: task\nworkspaceId: w_shared\n",
			},
			wantErr: `task "t_task" for workspace "w_shared" has an empty metadata ID`,
		},
		{
			name: "invalid active task reference",
			files: map[string]string{
				"tasks/t_task/task.yaml": "id: t_task\nname: task\nworkspaceId: w_shared\n",
				"active-task":            "t_missing\n",
			},
			wantErr: `active-task "t_missing" does not reference a valid portable task`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, folder := newStore(t)
			if _, err := Init(s, folder); err != nil {
				t.Fatal(err)
			}
			writePortableV2Workspace(t, folder, "w_shared", store.PortableWorkspace{
				ID: "w_shared", Name: "remote", Sync: true, CreatedAt: "2026-01-01T00:00:00Z",
			}, tt.files)

			establishLocalBase(t, s)
			beforeHash, beforeSync := snapshotPullState(t, s)
			_, err := Pull(s)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
			assertPullStateUnchanged(t, s, beforeHash, beforeSync)
			if _, err := s.ReadWorkspace("w_shared"); !os.IsNotExist(err) {
				t.Fatalf("workspace imported despite semantic preflight failure: %v", err)
			}
		})
	}
}

func writePortableV2Workspace(t *testing.T, folder, dirID string, portable store.PortableWorkspace, files map[string]string) {
	t.Helper()
	dir := filepath.Join(folder, exportDirV2, "workspaces", dirID)
	if err := store.WritePortableWorkspace(dir, portable); err != nil {
		t.Fatal(err)
	}
	for relativePath, content := range files {
		path := filepath.Join(dir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func snapshotPullState(t *testing.T, s store.Store) (string, store.SyncState) {
	t.Helper()
	hash, err := store.HashDir(filepath.Join(s.Home, "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	syncState, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	return hash, syncState
}

func establishLocalBase(t *testing.T, s store.Store) {
	t.Helper()
	hash, err := eligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if err := bindBaseToActiveDestination(&state, hash); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
}

func assertPullStateUnchanged(t *testing.T, s store.Store, wantHash string, wantSync store.SyncState) {
	t.Helper()
	gotHash, gotSync := snapshotPullState(t, s)
	if gotHash != wantHash {
		t.Fatalf("canonical workspace state changed on preflight failure: %s -> %s", wantHash, gotHash)
	}
	if gotSync != wantSync {
		t.Fatalf("sync BASE/bookkeeping changed on preflight failure: %+v -> %+v", wantSync, gotSync)
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
