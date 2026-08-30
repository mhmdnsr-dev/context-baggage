package sync

import (
	"errors"
	"fmt"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// Init configures the existing filesystem sync destination. The current
// public CLI historically permits replacing its folder; explicit --replace
// parsing is introduced only with the later v0.3 CLI integration slice.
func Init(s store.Store, folder string) (store.SyncState, error) {
	return initFilesystem(s, folder, true)
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
		return store.SyncState{}, errors.New("a different sync destination is already configured")
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
