package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

var ErrPortableExportLimit = errors.New("portable export exceeds byte limit")

const (
	exportDir   = "context-baggage-state"
	exportDirV2 = "context-baggage-state-v2"
)

func Push(s store.Store) (string, error) {
	// Operations that need both locks always acquire sync before canonical.
	// The canonical shared phase ends once the immutable export is built.
	unlock, err := s.AcquireSyncExclusive(context.Background())
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	return pushFilesystem(s)
}

// pushFilesystem publishes one immutable LOCAL snapshot while the caller owns
// the sync-operation lock.
func pushFilesystem(s store.Store) (string, error) {
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
	tmp, err := os.MkdirTemp(st.Folder, ".ctx-bag-push-*")
	if err != nil {
		return "", err
	}
	defer func() {
		// Temporary-directory cleanup is best-effort; it does not change the
		// result of a completed or failed push.
		_ = os.RemoveAll(tmp)
	}()
	localHash, err := BuildPushSnapshot(s, tmp)
	if err != nil {
		return "", err
	}
	remoteHash, err := store.HashDir(dest)
	if err != nil {
		return "", err
	}
	if PushWouldConflict(st, localHash, remoteHash) {
		if !st.BasePresent {
			return "", noBaseErr()
		}
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: sync folder\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pushing", localHash, remoteHash, dest)
	}
	hash, err := replaceV2(tmp, dest)
	if err != nil {
		return "", err
	}
	st.LastPush, st.LastPushHash = store.Now(), hash
	if err := BindBaseToActiveDestination(&st, hash); err != nil {
		return "", err
	}
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return hash, nil
}

func Pull(s store.Store) (string, error) {
	// Pull observes REMOTE while holding only sync. Canonical is acquired
	// exclusively afterward and remains held through BASE persistence.
	unlock, err := s.AcquireSyncExclusive(context.Background())
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	return pullFilesystem(s)
}

// pullFilesystem observes REMOTE before taking canonical ownership, then keeps
// canonical stable through LOCAL validation, import, and BASE persistence.
func pullFilesystem(s store.Store) (string, error) {
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
	canonicalUnlock, err := s.AcquireCanonicalExclusive(context.Background())
	if err != nil {
		return "", err
	}
	defer func() { _ = canonicalUnlock() }()
	// Sync bookkeeping is machine-local and must not affect the portable-state
	// hash. Otherwise a successful push would make the next pull look local.
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	if base, hasBase := sharedBase(st); !hasBase {
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
	st.LastPull, st.LastPullHash = store.Now(), incomingHash
	if err := BindBaseToActiveDestination(&st, incomingHash); err != nil {
		return "", err
	}
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return incomingHash, nil
}

// BuildPushSnapshot exports and hashes one coherent LOCAL view. Returning from
// this helper releases canonical ownership before destination publication.
func BuildPushSnapshot(s store.Store, dest string) (string, error) {
	return buildPushSnapshot(s, dest, nil)
}

// BuildPushSnapshotBounded prevents the immutable export itself from growing
// past the caller's provider limit while canonical shared ownership is held.
func BuildPushSnapshotBounded(s store.Store, dest string, maxBytes int64) (string, error) {
	return buildPushSnapshot(s, dest, &exportBudget{remaining: maxBytes})
}

func buildPushSnapshot(s store.Store, dest string, budget *exportBudget) (string, error) {
	unlock, err := s.AcquireCanonicalShared(context.Background())
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	if err := buildPortableExportBounded(s, dest, budget); err != nil {
		return "", err
	}
	return store.HashDir(dest)
}

// PushWouldConflict applies the provider-independent LOCAL/REMOTE/BASE push
// matrix. A missing BASE permits only an empty or already-equivalent REMOTE.
func PushWouldConflict(state store.SyncState, localHash, remoteHash string) bool {
	if !state.BasePresent {
		return remoteHash != "" && localHash != remoteHash
	}
	return hasConflict(state.BaseHash, localHash, remoteHash)
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

func sharedBase(st store.SyncState) (string, bool) {
	return st.BaseHash, st.BasePresent
}

func hasConflict(base, localHash, remoteHash string) bool {
	if localHash == remoteHash {
		return false
	}
	// A conflict exists only when both local and remote state changed from the
	// last shared baseline and the resulting portable states differ.
	return localHash != base && remoteHash != base
}

// BindBaseToActiveDestination records a portable baseline together with the
// identity that makes it safe to reuse for the active destination.
func BindBaseToActiveDestination(state *store.SyncState, hash string) error {
	identity := state.Folder
	if state.DestinationType == store.DestinationGitHub {
		identity = state.ManagedDestinationID
	}
	if identity == "" {
		return fmt.Errorf("cannot bind sync BASE: active destination has no durable identity")
	}
	state.BasePresent = true
	state.BaseHash = hash
	state.BaseDestinationType = state.DestinationType
	state.BaseDestinationIdentity = identity
	return nil
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
