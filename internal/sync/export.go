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
	return buildPortableExportBounded(s, dest, nil)
}

func buildPortableExportBounded(s store.Store, dest string, budget *exportBudget) error {
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
		if err := copyProjectedWorkspaceBounded(s.WorkspaceDir(w.ID), dst, w, budget); err != nil {
			return err
		}
	}
	return nil
}

func copyProjectedWorkspaceBounded(srcDir, dstDir string, w store.Workspace, budget *exportBudget) error {
	return copyPortableWorkspaceBounded(srcDir, dstDir, w.Portable(), budget)
}

// copyPortableWorkspace writes the portable projection of one workspace. Only
// allowlisted portable paths are copied; unknown files are ignored.
func copyPortableWorkspace(srcDir, dstDir string, p store.PortableWorkspace) error {
	return copyPortableWorkspaceBounded(srcDir, dstDir, p, nil)
}

func copyPortableWorkspaceBounded(srcDir, dstDir string, p store.PortableWorkspace, budget *exportBudget) error {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	if err := store.WritePortableWorkspace(dstDir, p); err != nil {
		return err
	}
	if err := accountExportFile(filepath.Join(dstDir, "workspace.yaml"), budget); err != nil {
		return err
	}
	if err := copyOptionalFileBounded(filepath.Join(srcDir, "active-task"), filepath.Join(dstDir, "active-task"), budget); err != nil {
		return err
	}
	return copyTaskTreeBounded(srcDir, dstDir, budget)
}

// copyOptionalFile copies src to dst when src exists. A missing src is a no-op.
func copyOptionalFile(src, dst string) error {
	return copyOptionalFileBounded(src, dst, nil)
}

func copyOptionalFileBounded(src, dst string, budget *exportBudget) error {
	has, err := pathExists(src)
	if err != nil || !has {
		return err
	}
	return copyFileBounded(src, dst, budget)
}

// copyTaskTree copies only the allowlisted task filenames from the source
// tasks directory to the destination tasks directory.
func copyTaskTree(srcWsDir, dstWsDir string) error {
	return copyTaskTreeBounded(srcWsDir, dstWsDir, nil)
}

func copyTaskTreeBounded(srcWsDir, dstWsDir string, budget *exportBudget) error {
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
			if err := copyOptionalFileBounded(filepath.Join(srcTask, name), filepath.Join(dstTask, name), budget); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileBounded(src, dst string, budget *exportBudget) error {
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
	limit := int64(-1)
	if budget != nil {
		limit = budget.remaining
	}
	written, err := copyWithLimit(tmp, in, limit)
	if err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return fmt.Errorf("copy file: %w; close temporary file: %v", err, closeErr)
		}
		return err
	}
	if budget != nil {
		budget.remaining -= written
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

type exportBudget struct {
	remaining int64
}

func accountExportFile(path string, budget *exportBudget) error {
	if budget == nil {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > budget.remaining {
		return ErrPortableExportLimit
	}
	budget.remaining -= info.Size()
	return nil
}

func copyWithLimit(destination io.Writer, source io.Reader, limit int64) (int64, error) {
	if limit < 0 {
		return io.Copy(destination, source)
	}
	written, err := io.CopyN(destination, source, limit+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return written, err
	}
	if written > limit {
		return written, ErrPortableExportLimit
	}
	return written, nil
}
