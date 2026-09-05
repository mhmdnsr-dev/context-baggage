package githubsync

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func validatePublicationSnapshot(root, expectedHash string) error {
	return validatePublicationSnapshotWithLimit(root, expectedHash, maxMaterializedBlobBytes)
}

func validatePublicationSnapshotWithLimit(root, expectedHash string, totalLimit int64) error {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		if expectedHash == "" {
			return nil
		}
		return ErrRepositoryIncompatible
	} else if err != nil {
		return ErrTransportUnavailable
	}
	hash, err := portablesync.ValidatePortableSnapshot(root)
	if err != nil || hash != expectedHash {
		return ErrRepositoryIncompatible
	}
	validator := publicationLimitValidator{root: root, totalLimit: totalLimit}
	err = filepath.WalkDir(root, validator.visit)
	if errors.Is(err, ErrResourceLimitExceeded) || errors.Is(err, ErrRepositoryIncompatible) {
		return err
	}
	if err != nil {
		return ErrTransportUnavailable
	}
	return nil
}

type publicationLimitValidator struct {
	root       string
	entries    int
	total      int64
	totalLimit int64
}

func (validator *publicationLimitValidator) visit(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return ErrTransportUnavailable
	}
	if path == validator.root {
		return nil
	}
	validator.entries++
	if validator.entries > maxTreeEntries {
		return ErrResourceLimitExceeded
	}
	relative, err := filepath.Rel(validator.root, path)
	if err != nil {
		return ErrRepositoryIncompatible
	}
	managedPath := portableRootName + "/" + filepath.ToSlash(relative)
	if !safeRelativePath(managedPath) || entry.Type()&fs.ModeSymlink != 0 {
		return ErrRepositoryIncompatible
	}
	if entry.IsDir() {
		return nil
	}
	return validator.addFile(entry, managedPath)
}

func (validator *publicationLimitValidator) addFile(entry fs.DirEntry, managedPath string) error {
	if !entry.Type().IsRegular() {
		return ErrRepositoryIncompatible
	}
	limit, err := blobLimit(managedPath)
	if err != nil {
		return err
	}
	info, err := entry.Info()
	if err != nil {
		return ErrTransportUnavailable
	}
	if info.Size() > limit || validator.total > validator.totalLimit-info.Size() {
		return ErrResourceLimitExceeded
	}
	validator.total += info.Size()
	return nil
}

func portableSnapshotPresent(root string) (bool, error) {
	entries, err := filepath.Glob(filepath.Join(root, "*"))
	return len(entries) != 0, err
}
