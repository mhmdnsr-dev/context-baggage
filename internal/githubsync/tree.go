package githubsync

import (
	"bytes"
	"fmt"
	"path"
	"strings"
)

const (
	portableRootName         = "context-baggage-state-v2"
	maxTreeEntries           = 50_000
	maxMaterializedBlobBytes = int64(512 * 1024 * 1024)
	maxTemporaryBytes        = int64(640 * 1024 * 1024)
	maxRelativePathBytes     = 1024
	maxPathSegmentBytes      = 255
	maxPathDepth             = 6
	maxWorkspaceYAMLBytes    = int64(256 * 1024)
	maxTaskYAMLBytes         = int64(256 * 1024)
	maxActiveTaskBytes       = int64(4 * 1024)
	maxHandoffBytes          = int64(8 * 1024 * 1024)
	maxCheckpointsBytes      = int64(64 * 1024 * 1024)
)

type gitTreeEntry struct {
	mode       string
	objectType string
	objectID   string
	size       int64
	path       string
}

type treeCollector struct {
	pending bytes.Buffer
	entries []gitTreeEntry
	err     error
}

// Write incrementally parses NUL-delimited ls-tree records so repository-size
// output never enters PR3's diagnostic capture or unbounded memory.
func (collector *treeCollector) Write(data []byte) (int, error) {
	written := len(data)
	if collector.err != nil {
		return written, nil
	}
	_, _ = collector.pending.Write(data)
	for {
		separator := bytes.IndexByte(collector.pending.Bytes(), 0)
		if separator < 0 {
			break
		}
		record := collector.pending.Next(separator + 1)
		if err := collector.add(string(record[:len(record)-1])); err != nil {
			collector.err = err
			break
		}
	}
	if collector.pending.Len() > maxRelativePathBytes+256 {
		collector.pending.Reset()
		collector.err = ErrResourceLimitExceeded
	}
	return written, nil
}

func (collector *treeCollector) finish() error {
	if collector.err != nil {
		return collector.err
	}
	if collector.pending.Len() != 0 {
		return ErrRepositoryIncompatible
	}
	return nil
}

func (collector *treeCollector) add(record string) error {
	if len(collector.entries) >= maxTreeEntries {
		return ErrResourceLimitExceeded
	}
	header, entryPath, ok := strings.Cut(record, "\t")
	fields := strings.Fields(header)
	if !ok || len(fields) != 3 {
		return ErrRepositoryIncompatible
	}
	entry := gitTreeEntry{mode: fields[0], objectType: fields[1], objectID: fields[2], path: entryPath, size: -1}
	if err := validateTreeEntry(entry); err != nil {
		return err
	}
	collector.entries = append(collector.entries, entry)
	return nil
}

// validateTreeEntry rejects object and path hazards before any blob is written
// to the operation-local materialization directory.
func validateTreeEntry(entry gitTreeEntry) error {
	if !validGitObjectID(entry.objectID) || !safeRelativePath(entry.path) {
		return ErrRepositoryIncompatible
	}
	switch {
	case entry.mode == "040000" && entry.objectType == "tree" && entry.size == -1:
	case entry.mode == "100644" && entry.objectType == "blob" && entry.size == -1:
	default:
		return ErrRepositoryIncompatible
	}
	if !allowedManagedPath(entry) {
		return ErrRepositoryIncompatible
	}
	return nil
}

func safeRelativePath(value string) bool {
	if value == "" || len(value) > maxRelativePathBytes || path.IsAbs(value) || strings.Contains(value, `\`) {
		return false
	}
	segments := strings.Split(value, "/")
	if len(segments) > maxPathDepth {
		return false
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || len(segment) > maxPathSegmentBytes || !safePathSegment(segment) {
			return false
		}
	}
	return true
}

func safePathSegment(segment string) bool {
	for _, character := range []byte(segment) {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if !letter && !digit && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func allowedManagedPath(entry gitTreeEntry) bool {
	segments := strings.Split(entry.path, "/")
	if len(segments) == 1 {
		return (segments[0] == managedMarkerName && entry.objectType == "blob") ||
			(segments[0] == portableRootName && entry.objectType == "tree")
	}
	if segments[0] != portableRootName {
		return false
	}
	switch len(segments) {
	case 2:
		return segments[1] == "workspaces" && entry.objectType == "tree"
	case 3:
		return segments[1] == "workspaces" && entry.objectType == "tree"
	case 4:
		if segments[1] != "workspaces" {
			return false
		}
		return (segments[3] == "workspace.yaml" && entry.objectType == "blob") ||
			(segments[3] == "active-task" && entry.objectType == "blob") ||
			(segments[3] == "tasks" && entry.objectType == "tree")
	case 5:
		return segments[1] == "workspaces" && segments[3] == "tasks" && entry.objectType == "tree"
	case 6:
		if segments[1] != "workspaces" || segments[3] != "tasks" {
			return false
		}
		switch segments[5] {
		case "task.yaml":
			return entry.objectType == "blob"
		case "handoff.md":
			return entry.objectType == "blob"
		case "checkpoints.jsonl":
			return entry.objectType == "blob"
		}
	}
	return false
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func validateCompleteTree(entries []gitTreeEntry) error {
	seen := make(map[string]struct{}, len(entries))
	markerFound := false
	for _, entry := range entries {
		if _, duplicate := seen[entry.path]; duplicate {
			return ErrRepositoryIncompatible
		}
		seen[entry.path] = struct{}{}
		if entry.path == managedMarkerName {
			markerFound = true
		}
	}
	if !markerFound {
		return ErrRepositoryIncompatible
	}
	return nil
}

func blobLimit(entryPath string) (int64, error) {
	if entryPath == managedMarkerName {
		return maxMarkerBytes, nil
	}
	segments := strings.Split(entryPath, "/")
	if len(segments) == 4 {
		switch segments[3] {
		case "workspace.yaml":
			return maxWorkspaceYAMLBytes, nil
		case "active-task":
			return maxActiveTaskBytes, nil
		}
	}
	if len(segments) == 6 {
		switch segments[5] {
		case "task.yaml":
			return maxTaskYAMLBytes, nil
		case "handoff.md":
			return maxHandoffBytes, nil
		case "checkpoints.jsonl":
			return maxCheckpointsBytes, nil
		}
	}
	return 0, ErrRepositoryIncompatible
}

func treeEntryByPath(entries []gitTreeEntry, wanted string) (gitTreeEntry, error) {
	for _, entry := range entries {
		if entry.path == wanted {
			return entry, nil
		}
	}
	return gitTreeEntry{}, fmt.Errorf("%w: required file missing", ErrRepositoryIncompatible)
}
