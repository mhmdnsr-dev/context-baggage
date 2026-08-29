package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	"github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func TestAttachFreshNonGit(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := store.PortableWorkspace{ID: "w_A", Name: "notes", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	w, changed, err := Attach(s, dir, target)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected attach to change state")
	}
	if w.ID != "w_A" || w.Name != "notes" || w.Identity.Type != "local-directory" {
		t.Fatalf("unexpected attached workspace: %+v", w)
	}
	if w.Sync {
		t.Fatalf("fresh attach must be Sync:false staging: %+v", w)
	}
	if len(w.LocalPaths) != 1 || w.LocalPaths[0] != dir {
		t.Fatalf("LocalPaths wrong: %v", w.LocalPaths)
	}
}

func TestAttachIdempotent(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := store.PortableWorkspace{ID: "w_A", Name: "notes", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, changed, err := Attach(s, dir, target); err != nil || !changed {
		t.Fatalf("first attach: changed=%v err=%v", changed, err)
	}
	_, changed, err := Attach(s, dir, target)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second attach to same target must be a no-op")
	}
}

func TestAttachPopulatedSourceRefused(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := s.WriteWorkspace(store.Workspace{ID: "w_old", Name: "old", Sync: false, LocalPaths: []string{dir}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteTask(store.Task{ID: "t_1", Name: "one", WorkspaceID: "w_old", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, dir, target); err == nil || !strings.Contains(err.Error(), "populated") {
		t.Fatalf("expected populated-source refusal, got %v", err)
	}
	if _, err := s.ReadWorkspace("w_old"); err != nil {
		t.Fatalf("source workspace must be unchanged: %v", err)
	}
}

func TestAttachSyncTrueSourceRefused(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := s.WriteWorkspace(store.Workspace{ID: "w_old", Name: "old", Sync: true, LocalPaths: []string{dir}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, dir, target); err == nil || !strings.Contains(err.Error(), "sync-enabled") {
		t.Fatalf("expected sync-source refusal, got %v", err)
	}
}

func TestAttachUnknownFileSourceRefused(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := s.WriteWorkspace(store.Workspace{ID: "w_old", Name: "old", Sync: false, LocalPaths: []string{dir}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.WorkspaceDir("w_old"), "local-debug.json"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, dir, target); err == nil {
		t.Fatal("expected refusal for unknown-file source")
	}
}

func TestAttachMultiLocalPath(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	foo := filepath.Join(t.TempDir(), "foo")
	bar := filepath.Join(t.TempDir(), "bar")
	if err := os.MkdirAll(foo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bar, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_old", Name: "old", Sync: false, LocalPaths: []string{foo, bar}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, foo, target); err != nil {
		t.Fatal(err)
	}
	old, err := s.ReadWorkspace("w_old")
	if err != nil {
		t.Fatal(err)
	}
	if len(old.LocalPaths) != 1 || old.LocalPaths[0] != bar {
		t.Fatalf("unrelated path not preserved: %v", old.LocalPaths)
	}
	tgt, err := s.ReadWorkspace("w_A")
	if err != nil {
		t.Fatal(err)
	}
	if len(tgt.LocalPaths) != 1 || tgt.LocalPaths[0] != foo {
		t.Fatalf("target path wrong: %v", tgt.LocalPaths)
	}
}

func TestAttachExistingTargetSyncTrue(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	cur := filepath.Join(t.TempDir(), "cur")
	if err := os.MkdirAll(cur, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_A", Name: "a", Sync: true, LocalPaths: []string{other}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteTask(store.Task{ID: "t_1", Name: "one", WorkspaceID: "w_A", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, cur, target); err != nil {
		t.Fatal(err)
	}
	w, err := s.ReadWorkspace("w_A")
	if err != nil {
		t.Fatal(err)
	}
	if !w.Sync {
		t.Fatal("existing Sync:true target must not be reset to false")
	}
	if !hasPath(w.LocalPaths, cur) || !hasPath(w.LocalPaths, other) {
		t.Fatalf("target LocalPaths not preserved+added: %v", w.LocalPaths)
	}
	if _, err := s.ReadTask("w_A", "t_1"); err != nil {
		t.Fatalf("target task lost: %v", err)
	}
}

func TestAttachGitRemoteRefused(t *testing.T) {
	repo := gitRepo(t, "https://example.com/org/repo.git")
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	target := store.PortableWorkspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	if _, _, err := Attach(s, repo, target); err == nil || !strings.Contains(err.Error(), "deterministic Git identity") {
		t.Fatalf("expected git-remote attach refusal, got %v", err)
	}
}

func TestResolveGainsRemoteKeepsBinding(t *testing.T) {
	repo := gitRepo(t, "") // git-local, no remote
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(s, repo, nil); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "remote", "add", "origin", "https://example.com/org/repo.git")
	w, _, err := Current(s, repo)
	if err != nil {
		t.Fatal(err)
	}
	derived := store.StableID("w", "git-remote:"+NormalizeRemote("https://example.com/org/repo.git"))
	if w.ID == derived {
		t.Fatalf("workspace silently re-keyed to derived Git ID: %s", w.ID)
	}
	if w.ID == "" {
		t.Fatal("expected an established ID")
	}
}

func TestFreshGitLocalGainsRemoteKeepsInitIdentity(t *testing.T) {
	repo := gitRepo(t, "")
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	first, err := Init(s, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "remote", "add", "origin", "https://example.com/org/repo.git")
	second, err := Init(s, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Name != first.Name || second.Identity != first.Identity {
		t.Fatalf("Init re-keyed established binding: %+v -> %+v", first, second)
	}
}

func TestFreshCloneNoBindingDeterministic(t *testing.T) {
	repo := gitRepo(t, "https://example.com/org/project.git")
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != store.StableID("w", "git-remote:"+NormalizeRemote("https://example.com/org/project.git")) {
		t.Fatalf("fresh clone must derive deterministic Git ID: %s", r.ID)
	}
}

func TestAttachThenPull(t *testing.T) {
	folder := t.TempDir()
	producer := store.New(t.TempDir())
	if err := producer.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Init(producer, folder); err != nil {
		t.Fatal(err)
	}
	if err := producer.WriteWorkspace(store.Workspace{ID: "w_A", Name: "a", Identity: store.WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := producer.WriteTask(store.Task{ID: "t_1", Name: "one", WorkspaceID: "w_A", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := producer.SetActiveTask("w_A", "t_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Push(producer); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Init(b, folder); err != nil {
		t.Fatal(err)
	}
	target, err := sync.FindPortableWorkspace(folder, "w_A")
	if err != nil {
		t.Fatal(err)
	}
	if _, changed, err := Attach(b, dir, target); err != nil || !changed {
		t.Fatalf("attach: changed=%v err=%v", changed, err)
	}
	w, _ := b.ReadWorkspace("w_A")
	if w.Sync {
		t.Fatal("staged attach must be Sync:false")
	}
	stBefore, _ := b.ReadSync()
	if stBefore.BaseHash != "" {
		t.Fatal("attach must not set BaseHash")
	}
	if _, err := sync.Pull(b); err != nil {
		t.Fatal(err)
	}
	w, _ = b.ReadWorkspace("w_A")
	if !w.Sync {
		t.Fatal("after pull Sync must be true")
	}
	if !hasPath(w.LocalPaths, dir) {
		t.Fatalf("LocalPaths not preserved: %v", w.LocalPaths)
	}
	if _, err := b.ReadTask("w_A", "t_1"); err != nil {
		t.Fatalf("task not imported: %v", err)
	}
	if at, err := b.ActiveTask("w_A"); err != nil || at != "t_1" {
		t.Fatalf("active-task not imported: %q %v", at, err)
	}
	stAfter, _ := b.ReadSync()
	exported, _ := store.HashDir(filepath.Join(folder, "context-baggage-state-v2"))
	if stAfter.BaseHash != exported {
		t.Fatalf("BASE does not equal remote: %s != %s", stAfter.BaseHash, exported)
	}
}

func TestAttachThenPullRefusesStagedLocalContext(t *testing.T) {
	folder := t.TempDir()
	producer := store.New(t.TempDir())
	if err := producer.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Init(producer, folder); err != nil {
		t.Fatal(err)
	}
	if err := producer.WriteWorkspace(store.Workspace{ID: "w_A", Name: "a", Sync: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Push(producer); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	b := store.New(t.TempDir())
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Init(b, folder); err != nil {
		t.Fatal(err)
	}
	target, err := sync.FindPortableWorkspace(folder, "w_A")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Attach(b, dir, target); err != nil {
		t.Fatal(err)
	}
	if err := b.WriteTask(store.Task{ID: "local_work", Name: "local", WorkspaceID: "w_A", Status: "active", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := sync.Pull(b); err == nil || !strings.Contains(err.Error(), "unshared local context") {
		t.Fatalf("expected staged-context refusal, got %v", err)
	}
	if _, err := b.ReadTask("w_A", "local_work"); err != nil {
		t.Fatalf("local context was lost: %v", err)
	}
}
