package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

func runWorkspace(s store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("workspace subcommand required\nrun: ctx-bag workspace init")
	}
	switch args[0] {
	case "init":
		syncPreference, err := parseWorkspaceSync(args[1:])
		if err != nil {
			return err
		}
		w, err := workspace.Init(s, mustCwd(), syncPreference)
		if err != nil {
			return err
		}
		return writeOutput(out, "Workspace initialized\nName: %s\nID: %s\nIdentity: %s:%s\nSync: %t\n", w.Name, w.ID, w.Identity.Type, w.Identity.Value, w.Sync)
	case "status":
		w, r, err := workspace.Current(s, mustCwd())
		if err != nil {
			return err
		}
		rootLabel := "Git root"
		if w.Identity.Type == "local-directory" {
			rootLabel = "Workspace root"
		}
		if err := writeOutput(out, "Workspace\nName: %s\nID: %s\n%s: %s\nIdentity: %s:%s\nSync: %t\n", w.Name, w.ID, rootLabel, r.Root, w.Identity.Type, w.Identity.Value, w.Sync); err != nil {
			return err
		}
		if r.ID != "" && r.ID != w.ID {
			if err := writeOutput(out, "Observed Git ID: %s\nGit identity: differs from established workspace\n", r.ID); err != nil {
				return err
			}
		}
		return nil
	case "available":
		return runWorkspaceAvailable(s, out)
	case "attach":
		if len(args) < 2 {
			return errors.New("workspace id is required\nrun: ctx-bag workspace attach <id>")
		}
		return runWorkspaceAttach(s, args[1], out)
	default:
		return fmt.Errorf("unknown workspace subcommand: %s", args[0])
	}
}

// runWorkspaceAttach binds the current directory to an existing canonical
// portable workspace. It is an identity/local-attachment operation only.
func runWorkspaceAttach(s store.Store, arg string, out io.Writer) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	return attachFromSync(s, st.Folder, arg, out)
}

// attachFromSync performs the authoritative-v2 attach check and delegation.
func attachFromSync(s store.Store, folder, arg string, out io.Writer) error {
	switch state, err := syncer.NamespaceState(folder); {
	case err != nil:
		return err
	case state == syncer.NamespaceLegacyOnly:
		return errors.New("legacy sync state detected\nrun: ctx-bag sync upgrade")
	case state == syncer.NamespaceNone:
		return fmt.Errorf("portable workspace %s not found", arg)
	}
	target, err := syncer.FindPortableWorkspace(folder, arg)
	if err != nil {
		return err
	}
	if !syncer.IsAttachable(target.Identity) {
		return fmt.Errorf("workspace %s is not attachable (identity: %s)", target.ID, target.Identity.Type)
	}
	w, changed, err := workspace.Attach(s, mustCwd(), target)
	if err != nil {
		return err
	}
	if changed {
		return writeOutput(out, "Attached workspace: %s\nPath: %s\nrun: ctx-bag sync pull\n", w.ID, mustCwd())
	}
	return writeOutput(out, "already attached to workspace %s\n", w.ID)
}

// runWorkspaceAvailable lists attachable portable workspaces from the
// authoritative v2 shared state. It is strictly read-only.
func runWorkspaceAvailable(s store.Store, out io.Writer) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	workspaces, warnings, err := listAttachable(s, st.Folder)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if err := writeOutput(out, "%s\n", w); err != nil {
			return err
		}
	}
	if len(workspaces) == 0 {
		return writeOutput(out, "Available portable workspaces: none\n")
	}
	if err := writeOutput(out, "Available portable workspaces\n"); err != nil {
		return err
	}
	for _, p := range workspaces {
		if err := writeOutput(out, "%s   %s   %s\n", p.ID, p.Name, p.Identity.Type); err != nil {
			return err
		}
	}
	return nil
}

// listAttachable returns the attachable portable workspaces and any entry-level
// warnings from authoritative v2 shared state, or an upgrade hint for legacy
// state.
func listAttachable(s store.Store, folder string) ([]store.PortableWorkspace, []string, error) {
	if state, err := syncer.NamespaceState(folder); err != nil {
		return nil, nil, err
	} else if state == syncer.NamespaceLegacyOnly {
		return nil, []string{"Legacy sync state detected.\nrun: ctx-bag sync upgrade\n"}, nil
	}
	workspaces, warnings, err := syncer.ListPortableWorkspaces(folder)
	if err != nil {
		return nil, nil, err
	}
	var attachable []store.PortableWorkspace
	for _, p := range workspaces {
		if syncer.IsAttachable(p.Identity) {
			attachable = append(attachable, p)
		}
	}
	return attachable, warnings, nil
}
