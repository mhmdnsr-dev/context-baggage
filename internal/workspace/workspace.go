package workspace

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

type Resolved struct {
	Root     string
	Name     string
	Identity store.WorkspaceIdentity
	ID       string
}

func Resolve(cwd string) (Resolved, error) {
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve current directory: %w", err)
	}
	root, err := git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		root = filepath.Clean(absCwd)
		return Resolved{
			Root:     root,
			Name:     filepath.Base(root),
			Identity: store.WorkspaceIdentity{Type: "local-directory"},
		}, nil
	}
	root = filepath.Clean(root)
	remote, _ := git(root, "config", "--get", "remote.origin.url")
	if strings.TrimSpace(remote) == "" {
		// Without a Git remote there is no portable repository identity.
		// The directory basename is only display metadata; init assigns a
		// Context Baggage workspace ID instead of deriving one from the path.
		return Resolved{
			Root:     root,
			Name:     filepath.Base(root),
			Identity: store.WorkspaceIdentity{Type: "git-local"},
		}, nil
	}

	// The same repository can be cloned with SSH on one machine and HTTPS
	// on another. Normalize the remote before deriving the workspace ID.
	value := NormalizeRemote(remote)
	// The display name also comes from the portable repository identity, not
	// the local checkout folder, so it remains stable across machines.
	name := repositoryName(value)
	id := store.StableID("w", "git-remote:"+value)
	return Resolved{Root: root, Name: name, Identity: store.WorkspaceIdentity{Type: "git-remote", Value: value}, ID: id}, nil
}

func Init(s store.Store, cwd string, syncPreference *bool) (store.Workspace, error) {
	r, err := Resolve(cwd)
	if err != nil {
		return store.Workspace{}, err
	}
	now := store.Now()
	w, found, err := existingWorkspace(s, r)
	if err != nil {
		return store.Workspace{}, err
	}
	if !found {
		id := r.ID
		if id == "" {
			id, err = store.NewID("w")
			if err != nil {
				return store.Workspace{}, fmt.Errorf("generate workspace id: %w", err)
			}
		}
		w = store.Workspace{ID: id, Sync: false, CreatedAt: now}
	}
	w.Name = r.Name
	w.Identity = r.Identity
	w.UpdatedAt = now
	if !containsPath(w.LocalPaths, r.Root) {
		w.LocalPaths = append(w.LocalPaths, r.Root)
	}
	if syncPreference != nil {
		w.Sync = *syncPreference
	}
	if err := s.WriteWorkspace(w); err != nil {
		return store.Workspace{}, err
	}
	return w, nil
}

func Current(s store.Store, cwd string) (store.Workspace, Resolved, error) {
	r, err := Resolve(cwd)
	if err != nil {
		return store.Workspace{}, Resolved{}, err
	}
	w, found, err := existingWorkspace(s, r)
	if err != nil {
		return store.Workspace{}, r, err
	}
	if !found {
		return store.Workspace{}, r, fmt.Errorf("workspace not initialized for current directory\nrun: ctx-bag workspace init")
	}
	return w, r, nil
}

func existingWorkspace(s store.Store, r Resolved) (store.Workspace, bool, error) {
	if r.ID != "" {
		w, err := s.ReadWorkspace(r.ID)
		if err != nil {
			return store.Workspace{}, false, nil
		}
		return w, true, nil
	}
	return workspaceByLocalPath(s, r.Root)
}

func workspaceByLocalPath(s store.Store, root string) (store.Workspace, bool, error) {
	workspaces, err := s.ListWorkspaces()
	if err != nil {
		return store.Workspace{}, false, err
	}
	for _, w := range workspaces {
		if containsPath(w.LocalPaths, root) {
			return w, true, nil
		}
	}
	return store.Workspace{}, false, nil
}

func NormalizeRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	// Git remotes may embed credentials. Never persist those as workspace
	// identity or display them back to the user.
	remote = stripCredentials(remote)
	if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@"), ":", 2)
		if len(parts) == 2 {
			return strings.ToLower(parts[0] + "/" + strings.TrimPrefix(parts[1], "/"))
		}
	}
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err == nil {
			host := strings.ToLower(u.Hostname())
			path := strings.TrimPrefix(u.EscapedPath(), "/")
			path, _ = url.PathUnescape(path)
			return strings.ToLower(host + "/" + strings.TrimSuffix(path, ".git"))
		}
	}
	remote = strings.TrimPrefix(remote, "ssh://")
	remote = strings.TrimPrefix(remote, "git@")
	remote = strings.ReplaceAll(remote, ":", "/")
	return strings.ToLower(remote)
}

func Slug(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	out := strings.Trim(re.ReplaceAllString(strings.ToLower(name), "-"), "-._")
	if out == "" {
		return "task"
	}
	return out
}

func repositoryName(identity string) string {
	identity = strings.Trim(strings.TrimSuffix(strings.TrimSpace(identity), ".git"), "/")
	if identity == "" {
		return "workspace"
	}
	parts := strings.Split(identity, "/")
	return parts[len(parts)-1]
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func stripCredentials(raw string) string {
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil {
			u.User = nil
			return u.String()
		}
	}
	return raw
}

func containsPath(paths []string, path string) bool {
	want, err := filepath.Abs(path)
	if err != nil {
		want = path
	}
	for _, p := range paths {
		got, err := filepath.Abs(p)
		if err != nil {
			got = p
		}
		if filepath.Clean(got) == filepath.Clean(want) {
			return true
		}
	}
	return false
}
