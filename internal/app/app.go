package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhmdnsr-dev/context-baggage/internal/agents"
	"github.com/mhmdnsr-dev/context-baggage/internal/agents/claude"
	"github.com/mhmdnsr-dev/context-baggage/internal/agents/codex"
	"github.com/mhmdnsr-dev/context-baggage/internal/config"
	"github.com/mhmdnsr-dev/context-baggage/internal/platform"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
	taskpkg "github.com/mhmdnsr-dev/context-baggage/internal/task"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return printHelp(stdout)
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

func runStatus(s store.Store, out io.Writer) error {
	if err := writeOutput(out, "Context Baggage\n"); err != nil {
		return err
	}
	if d, err := s.ReadDevice(); err == nil {
		if err := writeOutput(out, "\nDevice\n  ID: %s\n  OS: %s\n", d.ID, d.OS); err != nil {
			return err
		}
	}
	if w, _, err := workspace.Current(s, mustCwd()); err == nil {
		if err := writeOutput(out, "\nWorkspace\n  %s\n  %s\n", w.Name, w.ID); err != nil {
			return err
		}
		if t, err := taskpkg.Active(s, w); err == nil {
			if err := writeOutput(out, "\nActive Task\n  %s\n", t.ID); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"claude", "codex"} {
		if _, err := os.Stat(s.InventoryPath(key)); err == nil {
			if err := writeOutput(out, "\nAgents\n  Inventory available: %s\n", key); err != nil {
				return err
			}
		}
	}
	if st, err := s.ReadSync(); err == nil && st.Folder != "" {
		if err := writeOutput(out, "\nSync\n  configured\n  folder: %s\n", st.Folder); err != nil {
			return err
		}
		if st.LastPush != "" {
			if err := writeOutput(out, "  last push: %s\n", st.LastPush); err != nil {
				return err
			}
		}
	}
	return nil
}

func runDoctor(s store.Store, out io.Writer) error {
	var problems []string
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
	if len(problems) == 0 {
		return writeOutput(out, "Doctor: OK\n")
	}
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
		return writeOutput(out, "Workspace\nName: %s\nID: %s\n%s: %s\nIdentity: %s:%s\nSync: %t\n", w.Name, w.ID, rootLabel, r.Root, w.Identity.Type, w.Identity.Value, w.Sync)
	case "available":
		return runWorkspaceAvailable(s, out)
	default:
		return fmt.Errorf("unknown workspace subcommand: %s", args[0])
	}
}

// runWorkspaceAvailable lists attachable portable workspaces from the
// authoritative v2 shared state. It is strictly read-only.
func runWorkspaceAvailable(s store.Store, out io.Writer) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	state, err := syncer.NamespaceState(st.Folder)
	if err != nil {
		return err
	}
	if state == syncer.NamespaceLegacyOnly {
		return writeOutput(out, "Legacy sync state detected.\nrun: ctx-bag sync upgrade\n")
	}
	workspaces, warnings, err := syncer.ListPortableWorkspaces(st.Folder)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		if err := writeOutput(out, "%s\n", w); err != nil {
			return err
		}
	}
	var attachable []store.PortableWorkspace
	for _, p := range workspaces {
		if p.Identity.Type == "local-directory" || p.Identity.Type == "git-local" {
			attachable = append(attachable, p)
		}
	}
	if len(attachable) == 0 {
		return writeOutput(out, "Available portable workspaces: none\n")
	}
	if err := writeOutput(out, "Available portable workspaces\n"); err != nil {
		return err
	}
	for _, p := range attachable {
		if err := writeOutput(out, "%s   %s   %s\n", p.ID, p.Name, p.Identity.Type); err != nil {
			return err
		}
	}
	return nil
}

func runTask(s store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("task subcommand required\nrun: ctx-bag task status")
	}
	w, _, err := workspace.Current(s, mustCwd())
	if err != nil {
		return err
	}
	switch args[0] {
	case "start":
		if len(args) < 2 {
			return errors.New("task name is required\nrun: ctx-bag task start <name>")
		}
		t, err := taskpkg.Start(s, w, args[1])
		if err != nil {
			return err
		}
		return writeOutput(out, "Task started\nID: %s\nWorkspace: %s\n", t.ID, w.ID)
	case "resume":
		if len(args) < 2 {
			return errors.New("task name is required\nrun: ctx-bag task resume <name>")
		}
		t, err := taskpkg.Resume(s, w, args[1])
		if err != nil {
			return err
		}
		return writeOutput(out, "Task resumed\nID: %s\n", t.ID)
	case "status":
		tasks, err := s.ListTasks(w.ID)
		if err != nil {
			return err
		}
		active, _ := s.ActiveTask(w.ID)
		if err := writeOutput(out, "Tasks\n"); err != nil {
			return err
		}
		for _, t := range tasks {
			mark := " "
			if t.ID == active {
				mark = "*"
			}
			if err := writeOutput(out, "%s %s (%s)\n", mark, t.ID, t.Status); err != nil {
				return err
			}
		}
		if len(tasks) == 0 {
			if err := writeOutput(out, "  none\n"); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown task subcommand: %s", args[0])
	}
}

func runCheckpoint(s store.Store, args []string, out io.Writer) error {
	msg := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-m" || args[i] == "--message" {
			if i+1 >= len(args) {
				return errors.New("checkpoint message is required")
			}
			msg = args[i+1]
			i++
		}
	}
	w, _, err := workspace.Current(s, mustCwd())
	if err != nil {
		return err
	}
	d, err := s.ReadDevice()
	if err != nil {
		return err
	}
	if err := taskpkg.AddCheckpoint(s, w, d, msg); err != nil {
		return err
	}
	return writeOutput(out, "Checkpoint recorded\n")
}

func runHandoff(s store.Store, out io.Writer) error {
	w, _, err := workspace.Current(s, mustCwd())
	if err != nil {
		return err
	}
	path, err := taskpkg.EnsureHandoff(s, w)
	if err != nil {
		return err
	}
	return writeOutput(out, "Handoff: %s\n", path)
}

func runSync(s store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("sync subcommand required\nrun: ctx-bag sync status")
	}
	switch args[0] {
	case "init":
		if len(args) < 2 {
			return errors.New("sync folder is required\nrun: ctx-bag sync init <folder>")
		}
		st, err := syncer.Init(s, args[1])
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync configured\nFolder: %s\n", st.Folder)
	case "status":
		st, err := s.ReadSync()
		if err != nil {
			return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
		}
		if err := writeOutput(out, "Sync\nFolder: %s\nLast push: %s\nLast pull: %s\n", st.Folder, empty(st.LastPush), empty(st.LastPull)); err != nil {
			return err
		}
		state, err := syncer.NamespaceState(st.Folder)
		if err != nil {
			return err
		}
		switch state {
		case syncer.NamespaceLegacyOnly:
			return writeOutput(out, "Shared format: legacy\nTransition: required\nrun: ctx-bag sync upgrade\n")
		case syncer.NamespaceV2Only:
			return writeOutput(out, "Shared format: v2\n")
		case syncer.NamespaceBoth:
			return writeOutput(out, "Shared format: v2\nLegacy state: detected / ignored\nUpgrade other devices sharing this folder.\n")
		default:
			return writeOutput(out, "Shared format: none\n")
		}
	case "upgrade":
		if err := syncer.SyncUpgrade(s); err != nil {
			return err
		}
		return writeOutput(out, "Shared state upgraded to v2\nLegacy state preserved\nrun: ctx-bag sync pull\nUpgrade other devices sharing this folder.\n")
	case "push":
		hash, err := syncer.Push(s)
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync push complete\nHash: %s\n", hash)
	case "pull":
		hash, err := syncer.Pull(s)
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync pull complete\nHash: %s\n", hash)
	default:
		return fmt.Errorf("unknown sync subcommand: %s", args[0])
	}
}

func printHelp(out io.Writer) error {
	return writeOutput(out, `ctx-bag

Commands:
  init
  status
  doctor
  discover
  workspace init [--sync|--no-sync]
  workspace status
  task start <name>
  task status
  task resume <name>
  checkpoint -m <message>
  handoff
  sync init <folder>
  sync status
  sync push
  sync pull
`)
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
