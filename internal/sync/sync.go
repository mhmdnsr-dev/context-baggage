package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

const (
	exportDir   = "context-baggage-state"
	exportDirV2 = "context-baggage-state-v2"
)

func Init(s store.Store, folder string) (store.SyncState, error) {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return store.SyncState{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return store.SyncState{}, fmt.Errorf("sync folder is unavailable: %s; check that the folder exists and is mounted", abs)
	}
	if !info.IsDir() {
		return store.SyncState{}, fmt.Errorf("sync target is not a directory: %s", abs)
	}
	st := store.SyncState{Folder: abs}
	if old, err := s.ReadSync(); err == nil {
		st.LastPush = old.LastPush
		st.LastPull = old.LastPull
		st.LastPushHash = old.LastPushHash
		st.LastPullHash = old.LastPullHash
		st.BaseHash = old.BaseHash
	}
	return st, s.WriteSync(st)
}

func Push(s store.Store) (string, error) {
	st, err := s.ReadSync()
	if err != nil {
		return "", errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	dest := filepath.Join(st.Folder, exportDirV2)
	if err := os.MkdirAll(st.Folder, 0o700); err != nil {
		return "", err
	}
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	remoteHash, _ := store.HashDir(dest)
	base := sharedBase(st)
	if hasConflict(base, localHash, remoteHash) {
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: sync folder\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pushing", localHash, remoteHash, dest)
	}
	tmp, err := os.MkdirTemp(st.Folder, ".ctx-bag-push-*")
	if err != nil {
		return "", err
	}
	defer func() {
		// Temporary-directory cleanup is best-effort; it does not change the
		// result of a completed or failed push.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExport(s, tmp); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	hash, err := store.HashDir(dest)
	if err != nil {
		return "", err
	}
	st.LastPush, st.LastPushHash, st.BaseHash = store.Now(), hash, hash
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return hash, nil
}

func Pull(s store.Store) (string, error) {
	st, err := s.ReadSync()
	if err != nil {
		return "", errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	src := filepath.Join(st.Folder, exportDirV2)
	incomingHash, err := store.HashDir(src)
	if err != nil {
		return "", err
	}
	if incomingHash == "" {
		return "", fmt.Errorf("sync folder has no exported state: %s", src)
	}
	// Sync bookkeeping is machine-local and must not affect the portable-state
	// hash. Otherwise a successful push would make the next pull look local.
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	base := sharedBase(st)
	if hasConflict(base, localHash, incomingHash) {
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: local store\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pulling", localHash, incomingHash, src)
	}
	if err := importPortable(s, src); err != nil {
		return "", err
	}
	st.LastPull, st.LastPullHash, st.BaseHash = store.Now(), incomingHash, incomingHash
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return incomingHash, nil
}

func eligibleHash(s store.Store) (string, error) {
	tmp, err := os.MkdirTemp("", "ctx-bag-hash-*")
	if err != nil {
		return "", err
	}
	defer func() {
		// Temporary hash input cleanup is best-effort after the hash result is known.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExport(s, tmp); err != nil {
		return "", err
	}
	return store.HashDir(tmp)
}

func sharedBase(st store.SyncState) string {
	if st.BaseHash != "" {
		return st.BaseHash
	}
	if st.LastPullHash != "" {
		return st.LastPullHash
	}
	return st.LastPushHash
}

func hasConflict(base, localHash, remoteHash string) bool {
	if base == "" || localHash == remoteHash {
		return false
	}
	// A conflict exists only when both local and remote state changed from the
	// last shared baseline and the resulting portable states differ.
	return localHash != base && remoteHash != base
}

// buildPortableExport writes the explicit portable projection of all Sync:true
// workspaces into dest. Machine-local fields (LocalPaths, UpdatedAt) and
// config.yaml are intentionally excluded.
func buildPortableExport(s store.Store, dest string) error {
	workspaces, err := s.ListWorkspaces()
	if err != nil {
		return err
	}
	for _, w := range workspaces {
		if !w.Sync {
			// sync:false is a privacy boundary, not a hint. Never export that
			// workspace during push.
			continue
		}
		dst := filepath.Join(dest, "workspaces", w.ID)
		if err := copyProjectedWorkspace(s.WorkspaceDir(w.ID), dst, w); err != nil {
			return err
		}
	}
	return nil
}

// copyProjectedWorkspace writes the portable projection of one workspace. Only
// the allowlisted portable paths are copied; unknown files are ignored.
func copyProjectedWorkspace(srcDir, dstDir string, w store.Workspace) error {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	if err := store.WritePortableWorkspace(dstDir, w.Portable()); err != nil {
		return err
	}
	if has, err := pathExists(filepath.Join(srcDir, "active-task")); err == nil && has {
		if err := copyFile(filepath.Join(srcDir, "active-task"), filepath.Join(dstDir, "active-task")); err != nil {
			return err
		}
	}
	return copyTaskTree(srcDir, dstDir)
}

// copyTaskTree copies only the allowlisted task filenames from the source
// tasks directory to the destination tasks directory.
func copyTaskTree(srcWsDir, dstWsDir string) error {
	srcTasks := filepath.Join(srcWsDir, "tasks")
	entries, err := os.ReadDir(srcTasks)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		srcTask := filepath.Join(srcTasks, e.Name())
		dstTask := filepath.Join(dstWsDir, "tasks", e.Name())
		if err := os.MkdirAll(dstTask, 0o700); err != nil {
			return err
		}
		for _, name := range []string{"task.yaml", "checkpoints.jsonl", "handoff.md"} {
			if has, err := pathExists(filepath.Join(srcTask, name)); err == nil && has {
				if err := copyFile(filepath.Join(srcTask, name), filepath.Join(dstTask, name)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// importPortable replaces portable-owned workspace state from the v2 export
// while preserving machine-local fields (LocalPaths, UpdatedAt).
func importPortable(s store.Store, src string) error {
	wsEntries, err := os.ReadDir(filepath.Join(src, "workspaces"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range wsEntries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		exportWsDir := filepath.Join(src, "workspaces", id)
		p, err := store.ReadPortableWorkspace(exportWsDir)
		if err != nil {
			return err
		}
		localDir := s.WorkspaceDir(p.ID)
		// Preserve machine-local fields from an existing local workspace.
		var localPaths []string
		var updatedAt string
		if w, err := s.ReadWorkspace(p.ID); err == nil {
			localPaths = w.LocalPaths
			updatedAt = w.UpdatedAt
		}
		// Replace only the known portable-owned paths.
		if err := os.RemoveAll(filepath.Join(localDir, "tasks")); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(localDir, "active-task"))
		if err := os.MkdirAll(localDir, 0o700); err != nil {
			return err
		}
		if has, err := pathExists(filepath.Join(exportWsDir, "active-task")); err == nil && has {
			if err := copyFile(filepath.Join(exportWsDir, "active-task"), filepath.Join(localDir, "active-task")); err != nil {
				return err
			}
		}
		// Import the remote allowlisted task state.
		if err := copyTaskTree(exportWsDir, localDir); err != nil {
			return err
		}
		// Rebuild the full local workspace record: portable fields from remote,
		// machine-local fields preserved.
		w := store.Workspace{}.ApplyPortable(p)
		w.LocalPaths = localPaths
		w.UpdatedAt = updatedAt
		if w.UpdatedAt == "" {
			w.UpdatedAt = store.Now()
		}
		if err := s.WriteWorkspace(w); err != nil {
			return err
		}
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		// Read-only close is best-effort; copy errors are returned from io.Copy.
		_ = in.Close()
	}()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup. Rename removes the temporary path on success.
		_ = os.Remove(tmpName)
	}()
	if _, err := io.Copy(tmp, in); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return fmt.Errorf("copy file: %w; close temporary file: %v", err, closeErr)
		}
		return err
	}
	// Close must succeed before rename so a late write/flush error cannot look
	// like a successful copied file.
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
