package sync

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// ErrDestinationAlreadyConfigured reports a requested change that requires
// explicit replacement permission.
var ErrDestinationAlreadyConfigured = errors.New("a different sync destination is already configured")

// Init preserves the internal legacy behavior that authorizes filesystem
// replacement. Public CLI callers use InitWithReplacement for explicit policy.
func Init(s store.Store, folder string) (store.SyncState, error) {
	return InitWithReplacement(s, folder, true)
}

// InitWithReplacement configures a filesystem destination while requiring an
// explicit permission before changing its normalized identity. Reinitializing
// the same destination preserves destination-bound bookkeeping.
func InitWithReplacement(s store.Store, folder string, allowReplacement bool) (store.SyncState, error) {
	unlock, err := s.AcquireSyncExclusive(context.Background())
	if err != nil {
		return store.SyncState{}, err
	}
	defer func() { _ = unlock() }()
	if err := ensureNoPendingRecovery(s); err != nil {
		return store.SyncState{}, err
	}
	return initFilesystem(s, folder, allowReplacement)
}

// ensureNoPendingRecovery refuses to change the active destination while an
// interrupted managed Pull has left a recovery record bound to another
// destination. Replacing the destination would make the record permanently
// non-recoverable. It does not block read-only or offline operations.
func ensureNoPendingRecovery(s store.Store) error {
	exists, err := s.PullRecoveryExists()
	if err != nil {
		return err
	}
	if exists {
		return errors.New("managed pull recovery is pending\nresolve the interrupted pull before reconfiguring sync")
	}
	return nil
}

// initFilesystem validates and activates a filesystem destination. Replacement
// permission authorizes a different identity but never determines sameness.
func initFilesystem(s store.Store, folder string, allowReplacement bool) (store.SyncState, error) {
	identity, err := store.NormalizeFilesystemDestination(folder)
	if err != nil {
		return store.SyncState{}, err
	}
	info, err := os.Stat(identity)
	if err != nil {
		return store.SyncState{}, fmt.Errorf("sync folder is unavailable: %s; check that the folder exists and is mounted", identity)
	}
	if !info.IsDir() {
		return store.SyncState{}, fmt.Errorf("sync target is not a directory: %s", identity)
	}

	old, err := s.ReadSync()
	if errors.Is(err, os.ErrNotExist) {
		return writeFilesystemDestination(s, newFilesystemState(identity))
	}
	if err != nil {
		return store.SyncState{}, fmt.Errorf("read existing sync state: %w", err)
	}
	if sameFilesystemDestination(old, identity) {
		old.FormatVersion = store.SyncStateFormatVersion
		old.DestinationType = store.DestinationFilesystem
		old.Folder = identity
		return writeFilesystemDestination(s, old)
	}
	if !allowReplacement {
		return store.SyncState{}, ErrDestinationAlreadyConfigured
	}
	return writeFilesystemDestination(s, newFilesystemState(identity))
}

func newFilesystemState(identity string) store.SyncState {
	return store.SyncState{
		FormatVersion:   store.SyncStateFormatVersion,
		DestinationType: store.DestinationFilesystem,
		Folder:          identity,
	}
}

func sameFilesystemDestination(state store.SyncState, identity string) bool {
	return state.DestinationType == store.DestinationFilesystem && state.Folder == identity
}

func writeFilesystemDestination(s store.Store, state store.SyncState) (store.SyncState, error) {
	if err := s.WriteSync(state); err != nil {
		return store.SyncState{}, err
	}
	return state, nil
}
