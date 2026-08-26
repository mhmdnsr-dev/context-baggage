package task

import (
	"errors"
	"fmt"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

type Checkpoint struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	DeviceID  string `json:"deviceId"`
	Message   string `json:"message"`
}

func Start(s store.Store, w store.Workspace, name string) (store.Task, error) {
	id := workspace.Slug(name)
	if id == "" {
		return store.Task{}, errors.New("task name is empty")
	}
	if _, err := s.ReadTask(w.ID, id); err == nil {
		return store.Task{}, fmt.Errorf("task already exists: %s\nrun: ctx-bag task resume %s", id, id)
	}
	now := store.Now()
	t := store.Task{ID: id, Name: name, WorkspaceID: w.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.WriteTask(t); err != nil {
		return store.Task{}, err
	}
	if err := s.SetActiveTask(w.ID, id); err != nil {
		return store.Task{}, err
	}
	return t, nil
}

func Resume(s store.Store, w store.Workspace, name string) (store.Task, error) {
	id := workspace.Slug(name)
	t, err := s.ReadTask(w.ID, id)
	if err != nil {
		return store.Task{}, fmt.Errorf("task not found: %s\nrun: ctx-bag task start %s", id, id)
	}
	if err := s.SetActiveTask(w.ID, t.ID); err != nil {
		return store.Task{}, err
	}
	return t, nil
}

func Active(s store.Store, w store.Workspace) (store.Task, error) {
	id, err := s.ActiveTask(w.ID)
	if err != nil {
		return store.Task{}, fmt.Errorf("no active task for workspace\nrun: ctx-bag task start <name>")
	}
	t, err := s.ReadTask(w.ID, id)
	if err != nil {
		return store.Task{}, fmt.Errorf("active task state is invalid: %w", err)
	}
	return t, nil
}

func AddCheckpoint(s store.Store, w store.Workspace, device store.Device, message string) error {
	if message == "" {
		return errors.New("checkpoint message is required\nrun: ctx-bag checkpoint -m \"message\"")
	}
	t, err := Active(s, w)
	if err != nil {
		return err
	}
	return s.AppendCheckpoint(w.ID, t.ID, Checkpoint{Type: "checkpoint", Timestamp: store.Now(), DeviceID: device.ID, Message: message})
}

func EnsureHandoff(s store.Store, w store.Workspace) (string, error) {
	t, err := Active(s, w)
	if err != nil {
		return "", err
	}
	path := s.HandoffPath(w.ID, t.ID)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	template := "# Current Handoff\n\n## Completed\n\n## Current State\n\n## Decisions\n\n## Next Steps\n\n## Relevant Files\n\n## Blockers\n"
	return path, store.AtomicWrite(path, []byte(template), 0o600)
}
