package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	"github.com/mhmdnsr-dev/context-baggage/internal/workspace"
)

func TestEndToEndTwoMachineWorkflow(t *testing.T) {
	syncFolder := t.TempDir()
	repoA := gitRepo(t, "git@example.com:example-org/example-repo.git")
	repoB := gitRepo(t, "https://example.com/example-org/example-repo.git")
	homeA := t.TempDir()
	homeB := t.TempDir()

	runCLI(t, homeA, repoA, "init")
	runCLI(t, homeA, repoA, "workspace", "init", "--sync")
	runCLI(t, homeA, repoA, "task", "start", "test-task")
	runCLI(t, homeA, repoA, "checkpoint", "-m", "checkpoint A")
	runCLI(t, homeA, repoA, "handoff")
	runCLI(t, homeA, repoA, "sync", "init", syncFolder)
	runCLI(t, homeA, repoA, "sync", "push")
	storeA := store.New(homeA)
	deviceA, err := storeA.ReadDevice()
	if err != nil {
		t.Fatal(err)
	}
	stateA, err := storeA.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	hashA1 := stateA.LastPushHash
	workspaceA, err := workspace.Resolve(repoA)
	if err != nil {
		t.Fatal(err)
	}

	runCLI(t, homeB, repoB, "init")
	runCLI(t, homeB, repoB, "sync", "init", syncFolder)
	runCLI(t, homeB, repoB, "sync", "pull")
	runCLI(t, homeB, repoB, "checkpoint", "-m", "checkpoint B")
	runCLI(t, homeB, repoB, "sync", "push")
	storeB := store.New(homeB)
	deviceB, err := storeB.ReadDevice()
	if err != nil {
		t.Fatal(err)
	}
	if deviceA.ID == deviceB.ID {
		t.Fatal("machine-local device IDs should differ")
	}
	stateB, err := storeB.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	hashB2 := stateB.LastPushHash
	if hashA1 == hashB2 {
		t.Fatal("B checkpoint should change the portable sync hash")
	}
	workspaceB, err := workspace.Resolve(repoB)
	if err != nil {
		t.Fatal(err)
	}
	if workspaceA.ID != workspaceB.ID {
		t.Fatalf("workspace IDs differ: %s != %s", workspaceA.ID, workspaceB.ID)
	}
	runCLI(t, homeA, repoA, "sync", "pull")

	out := runCLI(t, homeB, repoB, "workspace", "status")
	if !strings.Contains(out, "w_") {
		t.Fatalf("workspace not recognized after pull:\n%s", out)
	}
	out = runCLI(t, homeB, repoB, "task", "resume", "test-task")
	if !strings.Contains(out, "test-task") {
		t.Fatalf("task not available after pull:\n%s", out)
	}
	cp := filepath.Join(homeA, "workspaces", workspaceA.ID, "tasks", "test-task", "checkpoints.jsonl")
	data, err := os.ReadFile(cp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "checkpoint A") || !strings.Contains(string(data), "checkpoint B") {
		t.Fatalf("checkpoint missing after pull:\n%s", data)
	}
	checkpoints := readCheckpoints(t, data)
	if checkpoints[0].DeviceID != deviceA.ID {
		t.Fatalf("checkpoint A device = %s, want %s", checkpoints[0].DeviceID, deviceA.ID)
	}
	if checkpoints[1].DeviceID != deviceB.ID {
		t.Fatalf("checkpoint B device = %s, want %s", checkpoints[1].DeviceID, deviceB.ID)
	}
	deviceAAfterPull, err := storeA.ReadDevice()
	if err != nil {
		t.Fatal(err)
	}
	if deviceAAfterPull.ID != deviceA.ID {
		t.Fatalf("pull overwrote machine A device ID: %s != %s", deviceAAfterPull.ID, deviceA.ID)
	}
	handoff := filepath.Join(homeA, "workspaces", workspaceA.ID, "tasks", "test-task", "handoff.md")
	if _, err := os.Stat(handoff); err != nil {
		t.Fatalf("handoff missing after pull: %v", err)
	}
}

type checkpointRecord struct {
	DeviceID string `json:"deviceId"`
	Message  string `json:"message"`
}

func readCheckpoints(t *testing.T, data []byte) []checkpointRecord {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := make([]checkpointRecord, 0, len(lines))
	for _, line := range lines {
		var cp checkpointRecord
		if err := json.Unmarshal([]byte(line), &cp); err != nil {
			t.Fatal(err)
		}
		out = append(out, cp)
	}
	return out
}

func TestSyncFalsePreventsCLIExport(t *testing.T) {
	syncFolder := t.TempDir()
	repo := gitRepo(t, "git@example.com:example-org/private-repo.git")
	home := t.TempDir()
	runCLI(t, home, repo, "init")
	runCLI(t, home, repo, "workspace", "init")
	runCLI(t, home, repo, "task", "start", "private-task")
	runCLI(t, home, repo, "sync", "init", syncFolder)
	runCLI(t, home, repo, "sync", "push")
	if _, err := os.Stat(filepath.Join(syncFolder, "context-baggage-state", "workspaces")); !os.IsNotExist(err) {
		t.Fatalf("sync:false workspace state exported: %v", err)
	}
}

func runCLI(t *testing.T, home, cwd string, args ...string) string {
	t.Helper()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	t.Setenv("CONTEXT_BAGGAGE_HOME", home)
	var out, stderr bytes.Buffer
	if err := Run(args, &out, &stderr); err != nil {
		t.Fatalf("ctx-bag %v failed: %v\nstdout:\n%s\nstderr:\n%s", args, err, out.String(), stderr.String())
	}
	return out.String()
}

func gitRepo(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "remote", "add", "origin", remote)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add failed: %v\n%s", err, out)
	}
	return dir
}
