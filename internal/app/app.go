package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/agents"
	"github.com/mhmdnsr-dev/context-baggage/internal/agents/claude"
	"github.com/mhmdnsr-dev/context-baggage/internal/agents/codex"
	"github.com/mhmdnsr-dev/context-baggage/internal/config"
	"github.com/mhmdnsr-dev/context-baggage/internal/platform"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return printHelp(stdout)
	}
	if args[0] == "man" {
		return runMan(args[1:], stdout)
	}
	s, err := config.OpenStore()
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return runInit(s, stdout)
	case "status":
		return runStatus(s, stdout)
	case "doctor":
		return runDoctor(s, stdout)
	case "discover":
		return withInit(s, func() error { return runDiscover(s, stdout) })
	case "workspace":
		return withInit(s, func() error { return runWorkspace(s, args[1:], stdout) })
	case "task":
		return withInit(s, func() error { return runTask(s, args[1:], stdout) })
	case "checkpoint":
		return withInit(s, func() error { return runCheckpoint(s, args[1:], stdout) })
	case "handoff":
		return withInit(s, func() error { return runHandoff(s, stdout) })
	case "sync":
		return withInit(s, func() error { return runSync(s, args[1:], stdout) })
	default:
		return fmt.Errorf("unknown command: %s\nrun: ctx-bag help", args[0])
	}
}

func withInit(s store.Store, fn func() error) error {
	if err := config.EnsureInitialized(s); err != nil {
		return err
	}
	return fn()
}

func runInit(s store.Store, out io.Writer) error {
	if err := s.Init(); err != nil {
		return err
	}
	d, err := s.ReadDevice()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// Device identity is Context Baggage-specific and random. Hardware
		// identifiers would create avoidable privacy risk and are not needed.
		id, err := store.NewID("d")
		if err != nil {
			return err
		}
		info := platform.CurrentInfo()
		d = store.Device{ID: id, Name: info.Name, OS: info.OS, Arch: info.Arch, CreatedAt: store.Now()}
		if err := s.WriteDevice(d); err != nil {
			return err
		}
	}
	return writeOutput(out, "Context Baggage initialized\nHome: %s\nDevice: %s\n", s.Home, d.ID)
}

func runDiscover(s store.Store, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// Discovery is read-only. The override keeps tests on fixtures instead of
	// inspecting a developer's real Claude/Codex configuration.
	if override := os.Getenv("CONTEXT_BAGGAGE_AGENT_HOME"); override != "" {
		home = override
	}
	adapters := []agents.Adapter{claude.Adapter{}, codex.Adapter{}}
	for _, adapter := range adapters {
		inv := adapter.Discover(home)
		if err := s.WriteInventory(adapter.Key(), inv); err != nil {
			return err
		}
		state := "not detected"
		if inv.Detected {
			state = "detected"
		}
		if err := writeOutput(out, "%s: %s\n", adapter.Name(), state); err != nil {
			return err
		}
		for _, p := range inv.ConfigPaths {
			if err := writeOutput(out, "  config: %s\n", p); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeOutput(out io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func parseWorkspaceSync(args []string) (*bool, error) {
	var out *bool
	for _, arg := range args {
		switch arg {
		case "--sync":
			// Workspace synchronization is opt-in because work repositories may
			// contain context that must not leave the current machine.
			v := true
			out = &v
		case "--no-sync":
			v := false
			out = &v
		default:
			return nil, fmt.Errorf("unknown workspace init option: %s", arg)
		}
	}
	return out, nil
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return filepath.Clean(cwd)
}

func empty(v string) string {
	if v == "" {
		return "never"
	}
	return v
}
