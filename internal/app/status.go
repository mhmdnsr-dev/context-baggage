package app

import (
	"io"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	taskpkg "github.com/mhmdnsr-dev/context-baggage/internal/task"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

func runStatus(s store.Store, out io.Writer) error {
	if err := writeOutput(out, "Context Baggage\n"); err != nil {
		return err
	}
	if err := printStatusDevice(s, out); err != nil {
		return err
	}
	if err := printStatusWorkspace(s, out); err != nil {
		return err
	}
	if err := printStatusAgents(s, out); err != nil {
		return err
	}
	return printStatusSync(s, out)
}

func printStatusDevice(s store.Store, out io.Writer) error {
	d, err := s.ReadDevice()
	if err != nil {
		return nil
	}
	return writeOutput(out, "\nDevice\n  ID: %s\n  OS: %s\n", d.ID, d.OS)
}

func printStatusWorkspace(s store.Store, out io.Writer) error {
	w, _, err := workspace.Current(s, mustCwd())
	if err != nil {
		return nil
	}
	if err := writeOutput(out, "\nWorkspace\n  %s\n  %s\n", w.Name, w.ID); err != nil {
		return err
	}
	t, err := taskpkg.Active(s, w)
	if err != nil {
		return nil
	}
	return writeOutput(out, "\nActive Task\n  %s\n", t.ID)
}

func printStatusAgents(s store.Store, out io.Writer) error {
	for _, key := range []string{"claude", "codex"} {
		if _, err := os.Stat(s.InventoryPath(key)); err == nil {
			if err := writeOutput(out, "\nAgents\n  Inventory available: %s\n", key); err != nil {
				return err
			}
		}
	}
	return nil
}

func printStatusSync(s store.Store, out io.Writer) error {
	st, err := s.ReadSync()
	if err != nil || st.Folder == "" {
		return nil
	}
	if err := writeOutput(out, "\nSync\n  configured\n  folder: %s\n", st.Folder); err != nil {
		return err
	}
	if st.LastPush != "" {
		return writeOutput(out, "  last push: %s\n", st.LastPush)
	}
	return nil
}
