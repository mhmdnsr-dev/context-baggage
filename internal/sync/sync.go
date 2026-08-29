package sync

import (
	"errors"
	"fmt"
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
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return "", err
	}
	if state == NamespaceLegacyOnly {
		return "", errors.New("legacy sync state detected\nrun: ctx-bag sync upgrade")
	}
	dest := filepath.Join(st.Folder, exportDirV2)
	if err := os.MkdirAll(st.Folder, 0o700); err != nil {
		return "", err
	}
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	remoteHash, err := store.HashDir(dest)
	if err != nil {
		return "", err
	}
	if base := sharedBase(st); base == "" {
		// First-sync safety: with no shared baseline a push may only establish
		// v2 from an empty or already-equivalent remote state.
		if remoteHash != "" && localHash != remoteHash {
			return "", noBaseErr()
		}
	} else if hasConflict(base, localHash, remoteHash) {
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
	hash, err := replaceV2(tmp, dest)
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
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return "", err
	}
	if state == NamespaceLegacyOnly {
		return "", errors.New("legacy sync state detected\nrun: ctx-bag sync upgrade")
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
	if base := sharedBase(st); base == "" {
		// First-sync safety: with no shared baseline a pull may only import when
		// the local side is empty or already equals the remote state.
		localNonEmpty, err := hasEligibleWorkspaces(s)
		if err != nil {
			return "", err
		}
		if localNonEmpty && localHash != incomingHash {
			return "", noBaseErr()
		}
	} else if hasConflict(base, localHash, incomingHash) {
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: local store\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pulling", localHash, incomingHash, src)
	}
	if err := preflightPortable(s, src); err != nil {
		return "", err
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

func noBaseErr() error {
	return errors.New("no shared baseline exists\nlocal and shared portable state differ\nautomatic direction cannot be determined safely")
}

// replaceV2 atomically replaces the v2 namespace with a prebuilt temporary
// tree. The hash is computed before replacement so a hash failure leaves the
// current authoritative v2 namespace untouched.
func replaceV2(tmp, dest string) (string, error) {
	hash, err := store.HashDir(tmp)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return hash, nil
}

func hasEligibleWorkspaces(s store.Store) (bool, error) {
	ws, err := s.ListWorkspaces()
	if err != nil {
		return false, err
	}
	for _, w := range ws {
		if w.Sync {
			return true, nil
		}
	}
	return false, nil
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
