package githubsync

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func (g GitRunner) materializeAndValidate(ctx context.Context, root, gitDir, repositoryIdentity, commitID string, entries []gitTreeEntry) (RepositorySnapshot, error) {
	markerEntry, err := treeEntryByPath(entries, managedMarkerName)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	markerBuffer := &bytes.Buffer{}
	markerBytes, err := g.acquireBlob(ctx, root, gitDir, markerEntry, maxMarkerBytes, markerBuffer)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	marker, err := parseManagedMarker(markerBuffer.Bytes())
	if err != nil {
		return RepositorySnapshot{}, err
	}
	portableDir := filepath.Join(root, "portable")
	portablePresent := false
	materializedBytes := markerBytes
	for _, entry := range entries {
		if entry.objectType != "blob" || !strings.HasPrefix(entry.path, portableRootName+"/") {
			continue
		}
		portablePresent = true
		relative := strings.TrimPrefix(entry.path, portableRootName+"/")
		limit, err := effectiveBlobLimit(entry.path, materializedBytes)
		if err != nil {
			return RepositorySnapshot{}, err
		}
		written, err := g.materializeBlob(ctx, root, gitDir, entry, limit, filepath.Join(portableDir, filepath.FromSlash(relative)))
		if err != nil {
			return RepositorySnapshot{}, err
		}
		materializedBytes += written
	}
	portableHash, err := portablesync.ValidatePortableSnapshot(portableDir)
	if err != nil {
		return RepositorySnapshot{}, ErrRepositoryIncompatible
	}
	return RepositorySnapshot{
		// The marker stays outside portableDir, so destination identity, commit
		// identity, and Git history cannot contaminate portable REMOTE identity.
		RepositoryIdentity: repositoryIdentity, State: RepositoryInitialized, CommitID: commitID,
		ManagedDestinationID: marker.destinationID, PortableHash: portableHash, PortablePresent: portablePresent,
	}, nil
}

func effectiveBlobLimit(entryPath string, materializedBytes int64) (int64, error) {
	limit, err := blobLimit(entryPath)
	if err != nil {
		return 0, err
	}
	remaining := maxMaterializedBlobBytes - materializedBytes
	if remaining < 0 {
		return 0, ErrResourceLimitExceeded
	}
	if remaining < limit {
		limit = remaining
	}
	return limit, nil
}

func (g GitRunner) materializeBlob(ctx context.Context, root, gitDir string, entry gitTreeEntry, limit int64, destination string) (int64, error) {
	if limit < 0 {
		return 0, ErrResourceLimitExceeded
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return 0, ErrTransportUnavailable
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, ErrTransportUnavailable
	}
	written, runErr := g.acquireBlob(ctx, root, gitDir, entry, limit, file)
	closeErr := file.Close()
	if runErr != nil || closeErr != nil {
		return 0, errors.Join(runErr, closeErr)
	}
	return written, nil
}

func (g GitRunner) acquireBlob(ctx context.Context, root, gitDir string, entry gitTreeEntry, limit int64, destination io.Writer) (int64, error) {
	if err := g.acquirePromisedBlob(ctx, root, gitDir, entry.objectID); err != nil {
		return 0, err
	}
	limited := &limitedBlobWriter{writer: destination, remaining: limit}
	// Object inspection is deliberately local: GIT_NO_LAZY_FETCH prevents
	// cat-file from turning a missing object into an implicit network request.
	err := g.runStreamGuarded(ctx, g.readTimeout, gitDir, root, g.temporaryByteLimit(), limited,
		"cat-file", "blob", entry.objectID)
	if limited.overflow || errors.Is(err, ErrResourceLimitExceeded) {
		return 0, ErrResourceLimitExceeded
	}
	if err != nil {
		return 0, err
	}
	return limited.written, nil
}

// acquirePromisedBlob makes network ownership explicit. The fixed, locally
// written remote is the only source, while the hardened boundary supplies
// redirect prevention, cancellation, bounded diagnostics, and the disk guard.
func (g GitRunner) acquirePromisedBlob(ctx context.Context, root, gitDir, objectID string) error {
	return g.runNetworkGuarded(ctx, g.readTimeout, gitDir, root, g.temporaryByteLimit(),
		"fetch", "--no-tags", "--no-write-fetch-head", "--quiet", targetRemoteName, objectID)
}

type limitedBlobWriter struct {
	writer    io.Writer
	remaining int64
	written   int64
	overflow  bool
}

func (writer *limitedBlobWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		writer.overflow = true
		allowed := int(writer.remaining)
		if allowed > 0 {
			written, _ := writer.writer.Write(data[:allowed])
			writer.written += int64(written)
			writer.remaining -= int64(written)
		}
		return 0, ErrResourceLimitExceeded
	}
	written, err := writer.writer.Write(data)
	writer.remaining -= int64(written)
	writer.written += int64(written)
	return written, err
}

func temporaryUsage(root string) (int64, error) {
	var used int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			used += info.Size()
			if !temporarySizeAllowed(used, 0) {
				return ErrResourceLimitExceeded
			}
		}
		return nil
	})
	if errors.Is(err, ErrResourceLimitExceeded) {
		return 0, err
	}
	if err != nil {
		return 0, ErrTransportUnavailable
	}
	return used, nil
}

func temporarySizeAllowed(used, additional int64) bool {
	return used >= 0 && additional >= 0 && used <= maxTemporaryBytes-additional
}

func (g GitRunner) temporaryByteLimit() int64 {
	if g.testTemporaryLimit > 0 {
		return g.testTemporaryLimit
	}
	return maxTemporaryBytes
}

func appendManagedReadRemote(gitDir, remoteURL string) error {
	if err := appendOperationRemote(gitDir, remoteURL); err != nil {
		return err
	}
	config, err := os.OpenFile(filepath.Join(gitDir, "config"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ErrTransportUnavailable
	}
	_, writeErr := io.WriteString(config, "\n[remote \""+targetRemoteName+"\"]\n\tpromisor = true\n\tpartialclonefilter = blob:none\n")
	closeErr := config.Close()
	if writeErr != nil || closeErr != nil {
		return ErrTransportUnavailable
	}
	return nil
}
