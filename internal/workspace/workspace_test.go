package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
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
