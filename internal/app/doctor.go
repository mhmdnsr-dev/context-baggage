package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/config"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

func runDoctor(s store.Store, out io.Writer) error {
	if config.EnsureInitialized(s) == nil {
		syncUnlock, err := acquireSyncShared(s)
		if err != nil {
			return err
		}
		defer func() { _ = syncUnlock() }()
		canonicalUnlock, err := acquireCanonicalShared(s)
		if err != nil {
			return err
		}
		defer func() { _ = canonicalUnlock() }()
	}
	problems, warnings := doctorDiagnostics(s)
	if len(problems) > 0 {
		if err := writeOutput(out, "Doctor: problems found\n"); err != nil {
			return err
		}
		for _, p := range problems {
			if err := writeOutput(out, "- %s\n", p); err != nil {
				return err
			}
		}
		return errors.New("doctor found problems")
	}
	if len(warnings) > 0 {
		for _, w := range warnings {
			if err := writeOutput(out, "%s\n", w); err != nil {
				return err
			}
		}
		return writeOutput(out, "Doctor: OK (warnings)\n")
	}
	return writeOutput(out, "Doctor: OK\n")
}

// doctorDiagnostics gathers health problems and warnings for the current state.
// Problems are real integrity failures; warnings are observable discrepancies
// that do not by themselves indicate broken state.
func doctorDiagnostics(s store.Store) (problems, warnings []string) {
	if _, err := os.Stat(s.Home); err != nil {
		problems = append(problems, "application home is inaccessible: "+err.Error())
	}
	if err := config.EnsureInitialized(s); err != nil {
		problems = append(problems, strings.ReplaceAll(err.Error(), "\n", " "))
	}
	if _, err := workspace.Resolve(mustCwd()); err != nil {
		problems = append(problems, "workspace unresolved: "+strings.Split(err.Error(), "\n")[0])
	}
	if st, err := s.ReadSync(); err == nil && st.Folder != "" {
		if _, err := os.Stat(st.Folder); err != nil {
			problems = append(problems, "configured sync folder is unavailable: "+st.Folder)
		}
	}
	// Duplicate LocalPath ownership is a real resolver-integrity error: the
	// workspace resolver returns the first LocalPath match, so ambiguity makes
	// resolution order-dependent.
	for _, dup := range duplicateLocalPaths(s) {
		problems = append(problems, "duplicate LocalPath ownership: "+dup)
	}
	// An observed Git identity that differs (or collides) is reported, never
	// reconciled: the established local binding remains authoritative.
	if w, r, err := workspace.Current(s, mustCwd()); err == nil {
		if warn := gitIdentityWarning(s, w, r); warn != "" {
			warnings = append(warnings, warn)
		}
	}
	return problems, warnings
}

// gitIdentityWarning reports a live observed Git identity that differs from the
// established workspace binding. It returns an empty string when there is
// nothing to report. It never reconciles or re-keys; it is diagnostic only.
func gitIdentityWarning(s store.Store, w store.Workspace, r workspace.Resolved) string {
	if r.ID == "" || r.ID == w.ID {
		return ""
	}
	if canonicalWorkspaceExists(s, r.ID, w.ID) {
		return "Warning: observed Git identity conflicts with another canonical workspace\n" +
			"Established workspace: " + w.ID + "\n" +
			"Observed Git workspace: " + r.ID + "\n" +
			"The established local binding remains authoritative.\n" +
			"No automatic reconciliation was performed.\n"
	}
	return "Warning: observed Git identity differs from established workspace\n" +
		"Established workspace: " + w.ID + "\n" +
		"Observed Git workspace: " + r.ID + "\n"
}

// canonicalWorkspaceExists reports whether another canonical workspace with the
// given ID is known, either in the local store or authoritative v2. Legacy is
// never inspected as canonical evidence, per the v2 namespace authority rule.
func canonicalWorkspaceExists(s store.Store, id, exclude string) bool {
	if ws, err := s.ListWorkspaces(); err == nil {
		for _, w := range ws {
			if w.ID == id && w.ID != exclude {
				return true
			}
		}
	}
	if st, err := s.ReadSync(); err == nil && st.Folder != "" {
		if state, err := syncer.NamespaceState(st.Folder); err == nil && (state == syncer.NamespaceV2Only || state == syncer.NamespaceBoth) {
			if p, err := syncer.FindPortableWorkspace(st.Folder, id); err == nil && p.ID == id {
				return true
			}
		}
	}
	return false
}

// duplicateLocalPaths returns human-readable lines for any LocalPath that is
// owned by more than one workspace record. Paths are compared using the same
// absolute+clean semantics the resolver and attachment use.
func duplicateLocalPaths(s store.Store) []string {
	workspaces, err := s.ListWorkspaces()
	if err != nil {
		return nil
	}
	seen := map[string]string{}
	var out []string
	for _, w := range workspaces {
		for _, p := range w.LocalPaths {
			key := normalizeLocalPath(p)
			if key == "" {
				continue
			}
			if first, ok := seen[key]; ok {
				out = append(out, fmt.Sprintf("path %s is owned by multiple workspaces: %s, %s", p, first, w.ID))
			} else {
				seen[key] = w.ID
			}
		}
	}
	return out
}

// normalizeLocalPath returns the absolute, cleaned form of a local path using
// the same comparison semantics the workspace resolver and attachment use.
// It intentionally does not resolve symlinks.
func normalizeLocalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}
