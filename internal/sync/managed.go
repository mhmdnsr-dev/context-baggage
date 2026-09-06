package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// EligibleHash re-derives the current LOCAL portable identity under canonical
// shared ownership. It is the provider-independent LOCAL identity used by both
// filesystem and managed Pull.
func EligibleHash(s store.Store) (string, error) {
	return eligibleHash(s)
}

// HasEligibleWorkspaces reports whether any Sync:true workspace exists locally.
// A machine with only Sync:false workspaces has no portable content.
func HasEligibleWorkspaces(s store.Store) (bool, error) {
	return hasEligibleWorkspaces(s)
}

// PreflightPortable validates the target portable snapshot and the local
// preservation prerequisites before any canonical mutation. A malformed target
// or a locally protected Sync:false workspace causes a refusal with no mutation.
func PreflightPortable(s store.Store, src string) error {
	return preflightPortable(s, src)
}

// PullWouldConflict applies the provider-independent LOCAL/REMOTE/BASE pull
// matrix. A missing BASE permits only an empty or already-equivalent LOCAL.
func PullWouldConflict(state store.SyncState, localHash, remoteHash string, localNonEmpty bool) bool {
	if !state.BasePresent {
		return localNonEmpty && localHash != remoteHash
	}
	return hasConflict(state.BaseHash, localHash, remoteHash)
}

// ApplyPortableSnapshot reconciles canonical workspace and task state to match
// the target portable snapshot rooted at src. It preserves machine-local fields
// (LocalPaths, UpdatedAt) and deletes portable-owned Sync:true workspaces that
// are absent from the target, without ever touching Sync:false or other
// machine-local data. It must run only after the recovery record is persisted.
func ApplyPortableSnapshot(s store.Store, src string) error {
	targetIDs := make(map[string]struct{})
	wsDir := filepath.Join(src, "workspaces")
	entries, err := os.ReadDir(wsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		targetIDs[entry.Name()] = struct{}{}
		if err := importPortableWorkspace(s, filepath.Join(wsDir, entry.Name())); err != nil {
			return err
		}
	}
	local, err := s.ListWorkspaces()
	if err != nil {
		return err
	}
	for _, workspace := range local {
		if !workspace.Sync {
			continue
		}
		if _, present := targetIDs[workspace.ID]; present {
			continue
		}
		if err := os.RemoveAll(s.WorkspaceDir(workspace.ID)); err != nil {
			return fmt.Errorf("remove stale portable workspace %s: %w", workspace.ID, err)
		}
	}
	return nil
}
