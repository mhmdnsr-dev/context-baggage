package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// SyncUpgrade converts a legacy-only shared namespace into a sanitized v2
// namespace. It is a format conversion only: it never mutates the local
// canonical store or the sync BASE bookkeeping.
func SyncUpgrade(s store.Store) error {
	unlock, err := s.AcquireSyncExclusive(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	return syncUpgrade(s)
}

// syncUpgrade performs the filesystem namespace conversion while its caller
// owns the sync-operation lock.
func syncUpgrade(s store.Store) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return err
	}
	switch state {
	case NamespaceV2Only, NamespaceBoth:
		return errors.New("sync state already uses the v2 format")
	case NamespaceNone:
		return errors.New("no shared sync state to upgrade\nrun: ctx-bag sync push")
	}
	tmp, err := os.MkdirTemp(st.Folder, ".ctx-bag-upgrade-*")
	if err != nil {
		return err
	}
	defer func() {
		// Temporary-directory cleanup is best-effort; it does not change the
		// result of a completed or failed upgrade.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExportFromLegacy(filepath.Join(st.Folder, exportDir), tmp); err != nil {
		return err
	}
	dest := filepath.Join(st.Folder, exportDirV2)
	if _, err := replaceV2(tmp, dest); err != nil {
		return err
	}
	// Format conversion is not a synchronization: leave sync BASE bookkeeping
	// unchanged until a real sync operation establishes the relationship.
	return nil
}

// buildPortableExportFromLegacy writes the sanitized v2 projection of a legacy
// workspace set into dest. The legacy source differs, but the v2 output
// contract is identical to normal push.
func buildPortableExportFromLegacy(legacyDir, dest string) error {
	wsDir := filepath.Join(legacyDir, "workspaces")
	entries, err := os.ReadDir(wsDir)
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
		srcWsDir := filepath.Join(wsDir, e.Name())
		p, err := store.ReadPortableWorkspace(srcWsDir)
		if err != nil {
			return err
		}
		dstWsDir := filepath.Join(dest, "workspaces", p.ID)
		if err := copyPortableWorkspace(srcWsDir, dstWsDir, p); err != nil {
			return err
		}
	}
	return nil
}

// preflightPortable validates incoming portable identities and local
// preservation prerequisites before any import mutation. It also refuses to
// overwrite a staged (Sync:false) workspace with unshared local context.
func preflightPortable(s store.Store, src string) error {
	wsDir := filepath.Join(src, "workspaces")
	portableStore := store.New(src)
	entries, err := os.ReadDir(wsDir)
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
		if err := preflightPortableWorkspace(s, portableStore, wsDir, e.Name()); err != nil {
			return err
		}
	}
	return nil
}

// preflightPortableWorkspace validates one portable workspace and confirms any
// existing local record can safely supply the fields import must preserve.
func preflightPortableWorkspace(s, portableStore store.Store, wsDir, dirID string) error {
	exportWsDir := filepath.Join(wsDir, dirID)
	portable, err := store.ReadPortableWorkspace(exportWsDir)
	if err != nil {
		return fmt.Errorf("read portable workspace %q: %w", dirID, err)
	}
	if portable.ID != dirID {
		return fmt.Errorf("portable workspace directory ID %q does not match workspace metadata ID %q", dirID, portable.ID)
	}
	if err := preflightPortableTasks(portableStore, exportWsDir, dirID); err != nil {
		return err
	}
	w, err := s.ReadWorkspace(dirID)
	if errors.Is(err, os.ErrNotExist) {
		return nil // no local record; nothing stale to protect
	}
	if err != nil {
		return fmt.Errorf("read local workspace %q before pull: %w", dirID, err)
	}
	if w.Sync {
		return nil // sync-enabled: the normal conflict model governs
	}
	empty, err := s.IsWorkspaceEmpty(dirID)
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("local workspace %s has unshared local context\nrefusing to overwrite it\nsafe next action: resolve the local context before pulling", dirID)
	}
	return nil
}

// preflightPortableTasks validates task identities and the active-task
// reference using the existing portable task layout before import can delete
// or replace any local task state.
func preflightPortableTasks(portableStore store.Store, exportWsDir, workspaceID string) error {
	taskIDs := make(map[string]struct{})
	tasksDir := filepath.Join(exportWsDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read portable tasks for workspace %q: %w", workspaceID, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirID := e.Name()
		task, err := portableStore.ReadTask(workspaceID, dirID)
		if err != nil {
			return fmt.Errorf("read portable task %q for workspace %q: %w", dirID, workspaceID, err)
		}
		if task.ID == "" {
			return fmt.Errorf("portable task %q for workspace %q has an empty metadata ID", dirID, workspaceID)
		}
		if task.ID != dirID {
			return fmt.Errorf("portable task directory ID %q does not match task metadata ID %q in workspace %q", dirID, task.ID, workspaceID)
		}
		if task.WorkspaceID != "" && task.WorkspaceID != workspaceID {
			return fmt.Errorf("portable task %q workspace ID %q does not match workspace %q", dirID, task.WorkspaceID, workspaceID)
		}
		taskIDs[dirID] = struct{}{}
	}
	return preflightPortableActiveTask(exportWsDir, workspaceID, taskIDs)
}

// preflightPortableActiveTask ensures the optional active-task marker names a
// task that was validated in the same portable workspace.
func preflightPortableActiveTask(exportWsDir, workspaceID string, taskIDs map[string]struct{}) error {
	data, err := os.ReadFile(filepath.Join(exportWsDir, "active-task"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read active task for portable workspace %q: %w", workspaceID, err)
	}
	activeTaskID := strings.TrimSpace(string(data))
	if activeTaskID == "" {
		return fmt.Errorf("portable workspace %q has an empty active-task reference", workspaceID)
	}
	if _, ok := taskIDs[activeTaskID]; !ok {
		return fmt.Errorf("portable workspace %q active-task %q does not reference a valid portable task", workspaceID, activeTaskID)
	}
	return nil
}

// importPortable replaces portable-owned workspace state from the v2 export
// while preserving machine-local fields (LocalPaths, UpdatedAt).
func importPortable(s store.Store, src string) error {
	wsDir := filepath.Join(src, "workspaces")
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := importPortableWorkspace(s, filepath.Join(wsDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// importPortableWorkspace replaces the portable-owned workspace state for one
// workspace while preserving its existing machine-local fields.
func importPortableWorkspace(s store.Store, exportWsDir string) error {
	p, err := store.ReadPortableWorkspace(exportWsDir)
	if err != nil {
		return err
	}
	localDir := s.WorkspaceDir(p.ID)
	local, err := preserveLocalFields(s, p.ID)
	if err != nil {
		return err
	}
	// Replace only the known portable-owned paths.
	if err := os.RemoveAll(filepath.Join(localDir, "tasks")); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(localDir, "active-task"))
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		return err
	}
	if err := copyOptionalFile(filepath.Join(exportWsDir, "active-task"), filepath.Join(localDir, "active-task")); err != nil {
		return err
	}
	if err := copyTaskTree(exportWsDir, localDir); err != nil {
		return err
	}
	// Rebuild the full local workspace record: portable fields from remote,
	// machine-local fields preserved.
	w := store.Workspace{}.ApplyPortable(p)
	w.LocalPaths = local.LocalPaths
	w.UpdatedAt = local.UpdatedAt
	if w.UpdatedAt == "" {
		w.UpdatedAt = store.Now()
	}
	return s.WriteWorkspace(w)
}

// preserveLocalFields returns an empty record only when the workspace is
// genuinely absent. Existing metadata that cannot be read must stop import so
// machine-local fields are never discarded as though they did not exist.
func preserveLocalFields(s store.Store, id string) (store.Workspace, error) {
	w, err := s.ReadWorkspace(id)
	if err == nil {
		return w, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return store.Workspace{}, nil
	}
	return store.Workspace{}, fmt.Errorf("read local workspace %q before import: %w", id, err)
}
