package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	"github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func TestNormalizeRemoteEquivalentFormsAndCredentialSanitization(t *testing.T) {
	ssh := NormalizeRemote("git@example.com:example-org/example-repo.git")
	https := NormalizeRemote("https://user:abc123@example.com/example-org/example-repo.git")
	if ssh != https {
		t.Fatalf("expected equivalent remotes, got %q and %q", ssh, https)
	}
	if strings.Contains(https, "abc123") || strings.Contains(https, "user:") {
		t.Fatalf("credentials leaked in normalized remote: %q", https)
	}
}

func TestGitHTTPSRemoteNameAndIdentity(t *testing.T) {
	dir := gitRepo(t, "https://example.com/org/example-repo.git")
	got, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity.Type != "git-remote" || got.Identity.Value != "example.com/org/example-repo" {
		t.Fatalf("unexpected identity: %#v", got.Identity)
	}
	if got.Name != "example-repo" {
		t.Fatalf("expected repository-derived name, got %q", got.Name)
	}
}

func TestGitSSHRemoteNameAndIdentity(t *testing.T) {
	dir := gitRepo(t, "git@example.com:org/example-repo.git")
	got, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity.Type != "git-remote" || got.Identity.Value != "example.com/org/example-repo" {
		t.Fatalf("unexpected identity: %#v", got.Identity)
	}
	if got.Name != "example-repo" {
		t.Fatalf("expected repository-derived name, got %q", got.Name)
	}
}

func TestWorkspaceIDIgnoresLocalPathForRemote(t *testing.T) {
	parent := t.TempDir()
	dirA := gitRepoAt(t, filepath.Join(parent, "workspace-a"), "git@example.com:example-org/example-repo.git")
	dirB := gitRepoAt(t, filepath.Join(parent, "workspace-b"), "https://example.com/example-org/example-repo.git")
	gotA, err := Resolve(dirA)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := Resolve(dirB)
	if err != nil {
		t.Fatal(err)
	}
	if gotA.ID != gotB.ID {
		t.Fatalf("IDs differ: %s != %s", gotA.ID, gotB.ID)
	}
	if gotA.Name != "example-repo" || gotB.Name != "example-repo" {
		t.Fatalf("expected repository-derived names, got %q and %q", gotA.Name, gotB.Name)
	}
	if gotA.Root == gotB.Root {
		t.Fatalf("expected different local roots, got %q", gotA.Root)
	}
}

func TestGitRepositoryWithoutRemoteUsesLocalDisplayNameAndLocalOnlyIdentity(t *testing.T) {
	parent := t.TempDir()
	repo := gitRepoAt(t, filepath.Join(parent, "local-git-project"), "")
	resolved, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Name != "local-git-project" {
		t.Fatalf("expected local display name, got %q", resolved.Name)
	}
	if resolved.Identity.Type != "git-local" || resolved.Identity.Value != "" {
		t.Fatalf("unexpected local Git identity: %#v", resolved.Identity)
	}
	if resolved.ID != "" {
		t.Fatalf("local-only Git workspace should not derive ID from path, got %q", resolved.ID)
	}

	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	w, err := Init(s, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID == "" || !strings.HasPrefix(w.ID, "w_") {
		t.Fatalf("expected generated Context Baggage ID, got %q", w.ID)
	}
	current, _, err := Current(s, repo)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != w.ID {
		t.Fatalf("current workspace ID differs: %s != %s", current.ID, w.ID)
	}
}

func TestNonGitFolderInitializesAndUsesBasename(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "local-folder-project")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	w, err := Init(s, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "local-folder-project" {
		t.Fatalf("expected folder basename, got %q", w.Name)
	}
	if w.Identity.Type != "local-directory" || w.Identity.Value != "" {
		t.Fatalf("unexpected non-Git identity: %#v", w.Identity)
	}
	if w.ID == "" || !strings.HasPrefix(w.ID, "w_") {
		t.Fatalf("expected generated Context Baggage ID, got %q", w.ID)
	}
	current, _, err := Current(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != w.ID {
		t.Fatalf("current workspace ID differs: %s != %s", current.ID, w.ID)
	}
}

func TestNonGitSameBasenameDoesNotAutoLink(t *testing.T) {
	parent := t.TempDir()
	dirA := filepath.Join(parent, "a", "project")
	dirB := filepath.Join(parent, "b", "project")
	for _, dir := range []string{dirA, dirB} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	wA, err := Init(s, dirA, nil)
	if err != nil {
		t.Fatal(err)
	}
	wB, err := Init(s, dirB, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wA.ID == wB.ID {
		t.Fatalf("same basename folders should not auto-link: %s", wA.ID)
	}
	if wA.Name != "project" || wB.Name != "project" {
		t.Fatalf("expected display basename for both folders, got %q and %q", wA.Name, wB.Name)
	}
}

func TestWorkspaceInitDefaultSyncFalse(t *testing.T) {
	repo := gitRepo(t, "git@example.com:example-org/example-repo.git")
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	w, err := Init(s, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.Sync {
		t.Fatal("workspace sync default should be false")
	}
	if len(w.LocalPaths) != 1 || !filepath.IsAbs(w.LocalPaths[0]) {
		t.Fatalf("local path not recorded: %#v", w.LocalPaths)
	}
}

func TestWorkspaceInitCanOptIntoSync(t *testing.T) {
	repo := gitRepo(t, "git@example.com:example-org/example-repo.git")
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	enabled := true
	w, err := Init(s, repo, &enabled)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Sync {
		t.Fatal("workspace sync should be enabled")
	}
}

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

func hasPath(paths []string, path string) bool {
	for _, p := range paths {
		if filepath.Clean(p) == filepath.Clean(path) {
			return true
		}
	}
	return false
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

func gitRepo(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, remote)
	return dir
}

func gitRepoAt(t *testing.T, dir, remote string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, dir, remote)
	return dir
}

func initGitRepo(t *testing.T, dir, remote string) {
	t.Helper()
	run(t, dir, "git", "init")
	if remote != "" {
		run(t, dir, "git", "remote", "add", "origin", remote)
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
