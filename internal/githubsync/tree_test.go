package githubsync

import (
	"errors"
	"strings"
	"testing"
)

const neutralObjectID = "0123456789abcdef0123456789abcdef01234567"

func TestTreeValidationRejectsUnsafeObjectsAndPaths(t *testing.T) {
	tests := map[string]gitTreeEntry{
		"symlink":    {mode: "120000", objectType: "blob", objectID: neutralObjectID, size: -1, path: managedMarkerName},
		"gitlink":    {mode: "160000", objectType: "commit", objectID: neutralObjectID, size: -1, path: portableRootName},
		"executable": {mode: "100755", objectType: "blob", objectID: neutralObjectID, size: -1, path: managedMarkerName},
		"dot segment": {mode: "100644", objectType: "blob", objectID: neutralObjectID, size: -1,
			path: portableRootName + "/workspaces/../workspace.yaml"},
		"backslash": {mode: "100644", objectType: "blob", objectID: neutralObjectID, size: -1,
			path: portableRootName + `/workspaces/w_bad\workspace.yaml`},
		"drive-like segment": {mode: "100644", objectType: "blob", objectID: neutralObjectID, size: -1,
			path: portableRootName + "/workspaces/C:/workspace.yaml"},
		"over depth": {mode: "100644", objectType: "blob", objectID: neutralObjectID, size: -1,
			path: portableRootName + "/workspaces/w/tasks/t/extra/task.yaml"},
		"over segment": {mode: "040000", objectType: "tree", objectID: neutralObjectID, size: -1,
			path: portableRootName + "/workspaces/" + strings.Repeat("x", maxPathSegmentBytes+1)},
		"over path": {mode: "040000", objectType: "tree", objectID: neutralObjectID, size: -1,
			path: strings.Repeat("x", maxRelativePathBytes+1)},
	}
	for name, entry := range tests {
		t.Run(name, func(t *testing.T) {
			if err := validateTreeEntry(entry); !errors.Is(err, ErrRepositoryIncompatible) {
				t.Fatalf("expected incompatible tree entry, got %v", err)
			}
		})
	}
}

func TestTreeValidationEnforcesSemanticFileLimits(t *testing.T) {
	tests := map[string]int64{
		portableRootName + "/workspaces/w/workspace.yaml":            maxWorkspaceYAMLBytes,
		portableRootName + "/workspaces/w/active-task":               maxActiveTaskBytes,
		portableRootName + "/workspaces/w/tasks/t/task.yaml":         maxTaskYAMLBytes,
		portableRootName + "/workspaces/w/tasks/t/handoff.md":        maxHandoffBytes,
		portableRootName + "/workspaces/w/tasks/t/checkpoints.jsonl": maxCheckpointsBytes,
	}
	for entryPath, want := range tests {
		if got, err := effectiveBlobLimit(entryPath, 0); err != nil || got != want {
			t.Fatalf("limit for %s = %d, %v; want %d", entryPath, got, err, want)
		}
	}
}

func TestBlobWriterAcceptsBoundaryAndRejectsOverflow(t *testing.T) {
	boundary := &limitedBlobWriter{writer: &strings.Builder{}, remaining: 4}
	if _, err := boundary.Write([]byte("1234")); err != nil || boundary.written != 4 || boundary.overflow {
		t.Fatalf("blob boundary rejected: %+v, %v", boundary, err)
	}
	overflow := &limitedBlobWriter{writer: &strings.Builder{}, remaining: 4}
	if _, err := overflow.Write([]byte("12345")); !errors.Is(err, ErrResourceLimitExceeded) || !overflow.overflow {
		t.Fatalf("blob overflow accepted: %+v, %v", overflow, err)
	}
}

func TestTreeCollectorRejectsEntryOverflow(t *testing.T) {
	collector := &treeCollector{entries: make([]gitTreeEntry, maxTreeEntries)}
	record := "100644 blob " + neutralObjectID + "\t" + managedMarkerName
	if err := collector.add(record); !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("expected tree entry limit, got %v", err)
	}
}

func TestTreeCollectorAcceptsEntryLimitBoundary(t *testing.T) {
	collector := &treeCollector{entries: make([]gitTreeEntry, maxTreeEntries-1)}
	record := "100644 blob " + neutralObjectID + "\t" + managedMarkerName
	if err := collector.add(record); err != nil || len(collector.entries) != maxTreeEntries {
		t.Fatalf("tree entry boundary rejected: entries=%d error=%v", len(collector.entries), err)
	}
}

func TestTemporaryWorkingLimitBoundaries(t *testing.T) {
	if !temporarySizeAllowed(maxTemporaryBytes, 0) || !temporarySizeAllowed(maxTemporaryBytes-1, 1) {
		t.Fatal("temporary working boundary was rejected")
	}
	if temporarySizeAllowed(maxTemporaryBytes, 1) || temporarySizeAllowed(maxTemporaryBytes+1, 0) {
		t.Fatal("temporary working overflow was accepted")
	}
}

func TestEffectiveBlobLimitEnforcesMaterializedTotal(t *testing.T) {
	entryPath := portableRootName + "/workspaces/w/tasks/t/checkpoints.jsonl"
	if got, err := effectiveBlobLimit(entryPath, maxMaterializedBlobBytes-1); err != nil || got != 1 {
		t.Fatalf("remaining logical limit = %d, %v", got, err)
	}
	if _, err := effectiveBlobLimit(entryPath, maxMaterializedBlobBytes+1); !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("expected logical total refusal, got %v", err)
	}
}

func TestRootAllowlistRejectsUnrelatedAndLegacyPaths(t *testing.T) {
	for _, name := range []string{"README.md", "LICENSE", ".gitignore", "notes.txt", "context-baggage-state", "other"} {
		entry := gitTreeEntry{mode: "100644", objectType: "blob", objectID: neutralObjectID, size: -1, path: name}
		if name == "context-baggage-state" || name == "other" {
			entry.mode, entry.objectType, entry.size = "040000", "tree", -1
		}
		if err := validateTreeEntry(entry); !errors.Is(err, ErrRepositoryIncompatible) {
			t.Fatalf("unexpected root path %q accepted: %v", name, err)
		}
	}
}
