package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// Namespace describes which sync namespaces are present in a folder.
type Namespace int

const (
	NamespaceNone Namespace = iota
	NamespaceLegacyOnly
	NamespaceV2Only
	NamespaceBoth
)

var (
	errPortableNotFound = errors.New("portable workspace not found")
	errPortableCorrupt  = errors.New("portable workspace corrupt")
)

// NamespaceState detects the presence of the legacy and v2 namespaces.
func NamespaceState(folder string) (Namespace, error) {
	legacy, err := pathExists(filepath.Join(folder, exportDir))
	if err != nil {
		return NamespaceNone, err
	}
	v2, err := pathExists(filepath.Join(folder, exportDirV2))
	if err != nil {
		return NamespaceNone, err
	}
	switch {
	case legacy && v2:
		return NamespaceBoth, nil
	case legacy:
		return NamespaceLegacyOnly, nil
	case v2:
		return NamespaceV2Only, nil
	default:
		return NamespaceNone, nil
	}
}

// IsAttachable reports whether a portable workspace identity type is an
// explicit attachment target. Git-remote workspaces already have deterministic
// Git-derived identity and are never explicit attachment targets.
func IsAttachable(t store.WorkspaceIdentity) bool {
	return t.Type == "local-directory" || t.Type == "git-local"
}

// ListPortableWorkspaces returns the valid portable workspaces from the
// authoritative v2 namespace, plus entry-level warnings. It is strictly
// read-only. Directory IDs are validated against workspace.yaml IDs.
func ListPortableWorkspaces(folder string) ([]store.PortableWorkspace, []string, error) {
	wsDir := filepath.Join(folder, exportDirV2, "workspaces")
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var out []store.PortableWorkspace
	var warnings []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirID := e.Name()
		wsPath := filepath.Join(wsDir, dirID)
		p, err := store.ReadPortableWorkspace(wsPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping corrupt portable workspace %s: %v", dirID, err))
			continue
		}
		if p.ID == "" {
			warnings = append(warnings, fmt.Sprintf("skipping portable workspace %s: empty id", dirID))
			continue
		}
		if p.ID != dirID {
			warnings = append(warnings, fmt.Sprintf("skipping portable workspace %s: id does not match directory name", dirID))
			continue
		}
		out = append(out, p)
	}
	return out, warnings, nil
}

// FindPortableWorkspace returns the portable workspace for an exact ID from the
// authoritative v2 namespace. It distinguishes not-found, corrupt, and
// namespace-level read failures.
func FindPortableWorkspace(folder, id string) (store.PortableWorkspace, error) {
	wsPath := filepath.Join(folder, exportDirV2, "workspaces", id)
	has, err := pathExists(wsPath)
	if err != nil {
		return store.PortableWorkspace{}, err
	}
	if !has {
		return store.PortableWorkspace{}, fmt.Errorf("%w: %s", errPortableNotFound, id)
	}
	p, err := store.ReadPortableWorkspace(wsPath)
	if err != nil {
		return store.PortableWorkspace{}, fmt.Errorf("%w: %s (%v)", errPortableCorrupt, id, err)
	}
	if p.ID == "" {
		return store.PortableWorkspace{}, fmt.Errorf("%w: %s (empty id)", errPortableCorrupt, id)
	}
	if p.ID != id {
		return store.PortableWorkspace{}, fmt.Errorf("%w: %s (id does not match directory name)", errPortableCorrupt, id)
	}
	return p, nil
}
