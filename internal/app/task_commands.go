package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	taskpkg "github.com/mhmdnsr-dev/context-baggage/internal/task"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

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
		return runTaskStart(s, w, args[1:], out)
	case "resume":
		return runTaskResume(s, w, args[1:], out)
	case "status":
		return runTaskStatus(s, w, out)
	default:
		return fmt.Errorf("unknown task subcommand: %s", args[0])
	}
}

func runTaskStart(s store.Store, w store.Workspace, args []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("task name is required\nrun: ctx-bag task start <name>")
	}
	t, err := taskpkg.Start(s, w, args[0])
	if err != nil {
		return err
	}
	return writeOutput(out, "Task started\nID: %s\nWorkspace: %s\n", t.ID, w.ID)
}

func runTaskResume(s store.Store, w store.Workspace, args []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("task name is required\nrun: ctx-bag task resume <name>")
	}
	t, err := taskpkg.Resume(s, w, args[0])
	if err != nil {
		return err
	}
	return writeOutput(out, "Task resumed\nID: %s\n", t.ID)
}

func runTaskStatus(s store.Store, w store.Workspace, out io.Writer) error {
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
		return writeOutput(out, "  none\n")
	}
	return nil
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
