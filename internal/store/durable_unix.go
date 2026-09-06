//go:build !windows

package store

import (
	"os"
	"path/filepath"
)

// durableInstall atomically installs the flushed temporary file at targetPath
// and synchronizes the containing directory so the rename itself survives a
// crash. On macOS Go's File.Sync uses F_FULLFSYNC where supported and falls
// back appropriately; on Linux it fsyncs the directory. The guarantee is
// bounded by the OS/filesystem's crash semantics.
func durableInstall(tempPath, targetPath string) error {
	if err := os.Rename(tempPath, targetPath); err != nil {
		return err
	}
	return syncDir(filepath.Dir(targetPath))
}

// syncDir flushes the directory entry for a newly installed file so the rename
// is durable. On POSIX this fsyncs the directory file descriptor.
func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()
	return handle.Sync()
}
