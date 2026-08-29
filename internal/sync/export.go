package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

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

// copyProjectedWorkspace writes the portable projection of one local workspace.
func copyProjectedWorkspace(srcDir, dstDir string, w store.Workspace) error {
	return copyPortableWorkspace(srcDir, dstDir, w.Portable())
}

// copyPortableWorkspace writes the portable projection of one workspace. Only
// allowlisted portable paths are copied; unknown files are ignored.
func copyPortableWorkspace(srcDir, dstDir string, p store.PortableWorkspace) error {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	if err := store.WritePortableWorkspace(dstDir, p); err != nil {
		return err
	}
	if err := copyOptionalFile(filepath.Join(srcDir, "active-task"), filepath.Join(dstDir, "active-task")); err != nil {
		return err
	}
	return copyTaskTree(srcDir, dstDir)
}

// copyOptionalFile copies src to dst when src exists. A missing src is a no-op.
func copyOptionalFile(src, dst string) error {
	has, err := pathExists(src)
	if err != nil || !has {
		return err
	}
	return copyFile(src, dst)
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
			if err := copyOptionalFile(filepath.Join(srcTask, name), filepath.Join(dstTask, name)); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
