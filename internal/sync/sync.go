package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

const exportDir = "context-baggage-state"

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
	dest := filepath.Join(st.Folder, exportDir)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
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
	if err := copyEligible(s, tmp); err != nil {
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
	src := filepath.Join(st.Folder, exportDir)
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
	if err := copyDir(src, s.Home, false); err != nil {
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
	if err := copyEligible(s, tmp); err != nil {
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

func copyEligible(s store.Store, dest string) error {
	// Device identity and sync configuration are machine-local. Only eligible
	// portable workspace state is exported.
	for _, name := range []string{"config.yaml"} {
		src := filepath.Join(s.Home, name)
		if _, err := os.Stat(src); err == nil {
			if err := copyDir(src, filepath.Join(dest, name), true); err != nil {
				return err
			}
		}
	}
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
		src := s.WorkspaceDir(w.ID)
		dst := filepath.Join(dest, "workspaces", w.ID)
		if err := copyDir(src, dst, true); err != nil {
			return err
		}
	}
	return nil
}

func copyDir(src, dst string, clean bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if clean {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if strings.Contains(rel, "..") {
			return fmt.Errorf("unsafe relative path: %s", rel)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		return copyFile(path, target)
	})
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
