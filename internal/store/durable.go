package store

import (
	"os"
	"path/filepath"
)

// durableInstaller installs a flushed temporary file at its target path with
// the platform's durable atomic-rename semantics. It is a test seam so the
// durability step can be injected with a failure.
var durableInstaller = durableInstall

// AtomicWriteDurable installs a file atomically and durably. The sequence is:
// write a temporary file, flush it, then perform the durable atomic install
// (rename + parent-directory flush on Unix, write-through move on Windows).
// It is the durability guarantee required for safety metadata such as the
// pull-recovery record, so a durable install failure aborts before any
// dependent canonical mutation.
func AtomicWriteDurable(path string, data []byte, perm os.FileMode) error {
	tmpName, err := writeTempAtomic(path, data, perm)
	if err != nil {
		return err
	}
	if err := durableInstaller(tmpName, path); err != nil {
		// Best-effort temporary-file cleanup. On Windows the temp is not renamed
		// when the move fails; on Unix a successful rename already removed it.
		// No rollback is attempted on a partially installed target file.
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// writeTempAtomic writes data beside path, flushes it, and returns the
// temporary file name without installing it. The temporary file is removed on
// any failure so a half-written file is never left behind.
func writeTempAtomic(path string, data []byte, perm os.FileMode) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return tmpName, nil
}
