package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

const (
	exportDir   = "context-baggage-state"
	exportDirV2 = "context-baggage-state-v2"
)

// Namespace describes which sync namespaces are present in a folder.
type Namespace int

const (
	NamespaceNone Namespace = iota
	NamespaceLegacyOnly
	NamespaceV2Only
	NamespaceBoth
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

var (
	errPortableNotFound = errors.New("portable workspace not found")
	errPortableCorrupt  = errors.New("portable workspace corrupt")
)

func Init(s store.Store, folder string) (store.SyncState, error) {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return store.SyncState{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return store.SyncState{}, fmt.Errorf("sync folder is unavailable: %s; check that the folder exists and is mounted", abs)
	}
	if !info.IsDir() {
		return store.SyncState{}, fmt.Errorf("sync target is not a directory: %s", abs)
	}
	st := store.SyncState{Folder: abs}
	if old, err := s.ReadSync(); err == nil {
		st.LastPush = old.LastPush
		st.LastPull = old.LastPull
		st.LastPushHash = old.LastPushHash
		st.LastPullHash = old.LastPullHash
		st.BaseHash = old.BaseHash
	}
	return st, s.WriteSync(st)
}

func Push(s store.Store) (string, error) {
	st, err := s.ReadSync()
	if err != nil {
		return "", errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return "", err
	}
	if state == NamespaceLegacyOnly {
		return "", errors.New("legacy sync state detected\nrun: ctx-bag sync upgrade")
	}
	dest := filepath.Join(st.Folder, exportDirV2)
	if err := os.MkdirAll(st.Folder, 0o700); err != nil {
		return "", err
	}
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	remoteHash, err := store.HashDir(dest)
	if err != nil {
		return "", err
	}
	base := sharedBase(st)
	if base == "" {
		// First-sync safety: with no shared baseline a push may only establish
		// v2 from an empty or already-equivalent remote state.
		if remoteHash != "" && localHash != remoteHash {
			return "", noBaseErr()
		}
	} else if hasConflict(base, localHash, remoteHash) {
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: sync folder\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pushing", localHash, remoteHash, dest)
	}
	tmp, err := os.MkdirTemp(st.Folder, ".ctx-bag-push-*")
	if err != nil {
		return "", err
	}
	defer func() {
		// Temporary-directory cleanup is best-effort; it does not change the
		// result of a completed or failed push.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExport(s, tmp); err != nil {
		return "", err
	}
	hash, err := replaceV2(tmp, dest)
	if err != nil {
		return "", err
	}
	st.LastPush, st.LastPushHash, st.BaseHash = store.Now(), hash, hash
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return hash, nil
}

func Pull(s store.Store) (string, error) {
	st, err := s.ReadSync()
	if err != nil {
		return "", errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return "", err
	}
	if state == NamespaceLegacyOnly {
		return "", errors.New("legacy sync state detected\nrun: ctx-bag sync upgrade")
	}
	src := filepath.Join(st.Folder, exportDirV2)
	incomingHash, err := store.HashDir(src)
	if err != nil {
		return "", err
	}
	if incomingHash == "" {
		return "", fmt.Errorf("sync folder has no exported state: %s", src)
	}
	// Sync bookkeeping is machine-local and must not affect the portable-state
	// hash. Otherwise a successful push would make the next pull look local.
	localHash, err := eligibleHash(s)
	if err != nil {
		return "", err
	}
	base := sharedBase(st)
	if base == "" {
		// First-sync safety: with no shared baseline a pull may only import when
		// the local side is empty or already equals the remote state.
		localNonEmpty, err := hasEligibleWorkspaces(s)
		if err != nil {
			return "", err
		}
		if localNonEmpty && localHash != incomingHash {
			return "", noBaseErr()
		}
	} else if hasConflict(base, localHash, incomingHash) {
		return "", fmt.Errorf("CONFLICT DETECTED\nresource: local store\nlocal hash: %s\nincoming hash: %s\nsafe next action: inspect %s before pulling", localHash, incomingHash, src)
	}
	if err := preflightPortable(s, src); err != nil {
		return "", err
	}
	if err := importPortable(s, src); err != nil {
		return "", err
	}
	st.LastPull, st.LastPullHash, st.BaseHash = store.Now(), incomingHash, incomingHash
	if err := s.WriteSync(st); err != nil {
		return "", err
	}
	return incomingHash, nil
}

func eligibleHash(s store.Store) (string, error) {
	tmp, err := os.MkdirTemp("", "ctx-bag-hash-*")
	if err != nil {
		return "", err
	}
	defer func() {
		// Temporary hash input cleanup is best-effort after the hash result is known.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExport(s, tmp); err != nil {
		return "", err
	}
	return store.HashDir(tmp)
}

func sharedBase(st store.SyncState) string {
	if st.BaseHash != "" {
		return st.BaseHash
	}
	if st.LastPullHash != "" {
		return st.LastPullHash
	}
	return st.LastPushHash
}

func hasConflict(base, localHash, remoteHash string) bool {
	if base == "" || localHash == remoteHash {
		return false
	}
	// A conflict exists only when both local and remote state changed from the
	// last shared baseline and the resulting portable states differ.
	return localHash != base && remoteHash != base
}

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
	if has, err := pathExists(filepath.Join(srcDir, "active-task")); err == nil && has {
		if err := copyFile(filepath.Join(srcDir, "active-task"), filepath.Join(dstDir, "active-task")); err != nil {
			return err
		}
	}
	return copyTaskTree(srcDir, dstDir)
}

// buildPortableExportFromLegacy writes the sanitized v2 projection of a legacy
// workspace set into dest. The legacy source differs, but the v2 output
// contract is identical to normal push.
func buildPortableExportFromLegacy(legacyDir, dest string) error {
	wsEntries, err := os.ReadDir(filepath.Join(legacyDir, "workspaces"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range wsEntries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		srcWsDir := filepath.Join(legacyDir, "workspaces", id)
		p, err := store.ReadPortableWorkspace(srcWsDir)
		if err != nil {
			return err
		}
		dstWsDir := filepath.Join(dest, "workspaces", id)
		if err := copyPortableWorkspace(srcWsDir, dstWsDir, p); err != nil {
			return err
		}
	}
	return nil
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
			if has, err := pathExists(filepath.Join(srcTask, name)); err == nil && has {
				if err := copyFile(filepath.Join(srcTask, name), filepath.Join(dstTask, name)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// preflightPortable validates all incoming portable workspace IDs before any
// import mutation. It refuses to overwrite a locally staged (Sync:false)
// workspace that has accumulated unshared local context.
func preflightPortable(s store.Store, src string) error {
	wsDir := filepath.Join(src, "workspaces")
	entries, err := os.ReadDir(wsDir)
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
		id := e.Name()
		w, err := s.ReadWorkspace(id)
		if err != nil {
			continue // no local record; nothing stale to protect
		}
		if w.Sync {
			continue // sync-enabled: the normal conflict model governs
		}
		empty, err := s.IsWorkspaceEmpty(id)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("local workspace %s has unshared local context\nrefusing to overwrite it\nsafe next action: resolve the local context before pulling", id)
		}
	}
	return nil
}

// importPortable replaces portable-owned workspace state from the v2 export
// while preserving machine-local fields (LocalPaths, UpdatedAt).
func importPortable(s store.Store, src string) error {
	wsEntries, err := os.ReadDir(filepath.Join(src, "workspaces"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range wsEntries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		exportWsDir := filepath.Join(src, "workspaces", id)
		p, err := store.ReadPortableWorkspace(exportWsDir)
		if err != nil {
			return err
		}
		localDir := s.WorkspaceDir(p.ID)
		// Preserve machine-local fields from an existing local workspace.
		var localPaths []string
		var updatedAt string
		if w, err := s.ReadWorkspace(p.ID); err == nil {
			localPaths = w.LocalPaths
			updatedAt = w.UpdatedAt
		}
		// Replace only the known portable-owned paths.
		if err := os.RemoveAll(filepath.Join(localDir, "tasks")); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(localDir, "active-task"))
		if err := os.MkdirAll(localDir, 0o700); err != nil {
			return err
		}
		if has, err := pathExists(filepath.Join(exportWsDir, "active-task")); err == nil && has {
			if err := copyFile(filepath.Join(exportWsDir, "active-task"), filepath.Join(localDir, "active-task")); err != nil {
				return err
			}
		}
		// Import the remote allowlisted task state.
		if err := copyTaskTree(exportWsDir, localDir); err != nil {
			return err
		}
		// Rebuild the full local workspace record: portable fields from remote,
		// machine-local fields preserved.
		w := store.Workspace{}.ApplyPortable(p)
		w.LocalPaths = localPaths
		w.UpdatedAt = updatedAt
		if w.UpdatedAt == "" {
			w.UpdatedAt = store.Now()
		}
		if err := s.WriteWorkspace(w); err != nil {
			return err
		}
	}
	return nil
}

// SyncUpgrade converts a legacy-only shared namespace into a sanitized v2
// namespace. It is a format conversion only: it never mutates the local
// canonical store or the sync BASE bookkeeping.
func SyncUpgrade(s store.Store) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	state, err := NamespaceState(st.Folder)
	if err != nil {
		return err
	}
	switch state {
	case NamespaceV2Only, NamespaceBoth:
		return errors.New("sync state already uses the v2 format")
	case NamespaceNone:
		return errors.New("no shared sync state to upgrade\nrun: ctx-bag sync push")
	}
	tmp, err := os.MkdirTemp(st.Folder, ".ctx-bag-upgrade-*")
	if err != nil {
		return err
	}
	defer func() {
		// Temporary-directory cleanup is best-effort; it does not change the
		// result of a completed or failed upgrade.
		_ = os.RemoveAll(tmp)
	}()
	if err := buildPortableExportFromLegacy(filepath.Join(st.Folder, exportDir), tmp); err != nil {
		return err
	}
	dest := filepath.Join(st.Folder, exportDirV2)
	if _, err := replaceV2(tmp, dest); err != nil {
		return err
	}
	// Format conversion is not a synchronization: leave sync BASE bookkeeping
	// unchanged until a real sync operation establishes the relationship.
	return nil
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

// replaceV2 atomically replaces the v2 namespace with a prebuilt temporary
// tree. The hash is computed before replacement so a hash failure leaves the
// current authoritative v2 namespace untouched.
func replaceV2(tmp, dest string) (string, error) {
	hash, err := store.HashDir(tmp)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return hash, nil
}

func noBaseErr() error {
	return errors.New("no shared baseline exists\nlocal and shared portable state differ\nautomatic direction cannot be determined safely")
}

func hasEligibleWorkspaces(s store.Store) (bool, error) {
	ws, err := s.ListWorkspaces()
	if err != nil {
		return false, err
	}
	for _, w := range ws {
		if w.Sync {
			return true, nil
		}
	}
	return false, nil
}

// IsAttachable reports whether a portable workspace identity type is an
// explicit attachment target. Git-remote workspaces already have deterministic
// Git-derived identity and are never explicit attachment targets.
func IsAttachable(t store.WorkspaceIdentity) bool {
	return t.Type == "local-directory" || t.Type == "git-local"
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
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
	// Close must succeed before rename so a late write/flush error cannot look
	// like a successful copied file.
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
