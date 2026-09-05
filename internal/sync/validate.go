package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// ValidatePortableSnapshot validates portable workspace/task relationships and
// returns the same provider-independent identity used by filesystem sync.
// It is read-only and does not inspect or mutate canonical local state.
func ValidatePortableSnapshot(root string) (string, error) {
	workspacesDir := filepath.Join(root, "workspaces")
	portableStore := store.New(root)
	entries, err := os.ReadDir(workspacesDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := validatePortableWorkspace(portableStore, workspacesDir, entry.Name()); err != nil {
			return "", err
		}
	}
	return store.HashDir(root)
}

func validatePortableWorkspace(portableStore store.Store, workspacesDir, directoryID string) error {
	workspaceDir := filepath.Join(workspacesDir, directoryID)
	portable, err := store.ReadPortableWorkspace(workspaceDir)
	if err != nil {
		return fmt.Errorf("read portable workspace %q: %w", directoryID, err)
	}
	if portable.ID != directoryID {
		return fmt.Errorf("portable workspace directory ID %q does not match workspace metadata ID %q", directoryID, portable.ID)
	}
	return validatePortableTasks(portableStore, workspaceDir, directoryID)
}

func validatePortableTasks(portableStore store.Store, workspaceDir, workspaceID string) error {
	taskIDs := make(map[string]struct{})
	entries, err := os.ReadDir(filepath.Join(workspaceDir, "tasks"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read portable tasks for workspace %q: %w", workspaceID, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directoryID := entry.Name()
		task, err := portableStore.ReadTask(workspaceID, directoryID)
		if err != nil {
			return fmt.Errorf("read portable task %q for workspace %q: %w", directoryID, workspaceID, err)
		}
		if task.ID == "" {
			return fmt.Errorf("portable task %q for workspace %q has an empty metadata ID", directoryID, workspaceID)
		}
		if task.ID != directoryID {
			return fmt.Errorf("portable task directory ID %q does not match task metadata ID %q in workspace %q", directoryID, task.ID, workspaceID)
		}
		if task.WorkspaceID != "" && task.WorkspaceID != workspaceID {
			return fmt.Errorf("portable task %q workspace ID %q does not match workspace %q", directoryID, task.WorkspaceID, workspaceID)
		}
		taskIDs[directoryID] = struct{}{}
	}
	return validatePortableActiveTask(workspaceDir, workspaceID, taskIDs)
}

func validatePortableActiveTask(workspaceDir, workspaceID string, taskIDs map[string]struct{}) error {
	data, err := os.ReadFile(filepath.Join(workspaceDir, "active-task"))
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
