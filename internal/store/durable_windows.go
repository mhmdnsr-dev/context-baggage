//go:build windows

package store

import "golang.org/x/sys/windows"

// durableInstall atomically installs the flushed temporary file at targetPath
// using a write-through move. MoveFileEx with MOVEFILE_WRITE_THROUGH ensures
// the move is written through to disk before returning, which is the Windows
// durability primitive for a rename. The source temporary file must already
// have been flushed before this call.
func durableInstall(tempPath, targetPath string) error {
	from, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
