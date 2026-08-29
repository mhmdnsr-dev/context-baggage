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
	if remote != "" {
		cmd = exec.Command("git", "remote", "add", "origin", remote)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote add failed: %v\n%s", err, out)
		}
	}
	return dir
}

func gitRepoNoRemote(t *testing.T) string {
	t.Helper()
	return gitRepo(t, "")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func runCLIErr(t *testing.T, home, cwd string, args ...string) (string, error) {
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
	err = Run(args, &out, &stderr)
	return out.String() + stderr.String(), err
}

func hasLocalPath(paths []string, want string) bool {
	for _, p := range paths {
		if filepath.Clean(p) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

func seedLegacyWorkspace(t *testing.T, folder, id, yaml string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(folder, "context-baggage-state", "workspaces", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workspace.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDoctorNoGitIdentityOK(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	if out := runCLI(t, home, dir, "doctor"); !strings.Contains(out, "Doctor: OK") {
		t.Fatalf("expected doctor OK:\n%s", out)
	}
}

func TestDoctorGitMismatchWarning(t *testing.T) {
	dir := gitRepoNoRemote(t)
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	runGit(t, dir, "git", "remote", "add", "origin", "https://example.com/org/other-repo.git")
	out := runCLI(t, home, dir, "doctor")
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "differs") {
		t.Fatalf("expected mismatch warning:\n%s", out)
	}
	if strings.Contains(out, "problems found") {
		t.Fatalf("mismatch must not be a problem:\n%s", out)
	}
}

func TestDoctorGitCollisionLocal(t *testing.T) {
	dir := gitRepoNoRemote(t)
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	remote := "https://example.com/org/conflict-repo.git"
	derived := store.StableID("w", "git-remote:"+workspace.NormalizeRemote(remote))
	s := store.New(home)
	if err := s.WriteWorkspace(store.Workspace{ID: derived, Name: "conflict", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "remote", "add", "origin", remote)
	out := runCLI(t, home, dir, "doctor")
	if !strings.Contains(out, "conflicts with another canonical workspace") {
		t.Fatalf("expected collision warning:\n%s", out)
	}
	if strings.Contains(out, "problems found") {
		t.Fatalf("collision must be a warning:\n%s", out)
	}
}

func TestDoctorDuplicateLocalPathError(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	s := store.New(home)
	if err := s.WriteWorkspace(store.Workspace{ID: "w_a", Name: "a", Sync: false, LocalPaths: []string{dir}, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteWorkspace(store.Workspace{ID: "w_b", Name: "b", Sync: false, LocalPaths: []string{dir}, CreatedAt: store.Now(), UpdatedAt: store.Now()}); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIErr(t, home, dir, "doctor")
	if err == nil || !strings.Contains(out, "duplicate LocalPath ownership") {
		t.Fatalf("expected duplicate-path error, err=%v out=%s", err, out)
	}
}

func TestWorkspaceStatusObservedGitID(t *testing.T) {
	dir := gitRepoNoRemote(t)
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	runGit(t, dir, "git", "remote", "add", "origin", "https://example.com/org/status-repo.git")
	if out := runCLI(t, home, dir, "workspace", "status"); !strings.Contains(out, "Observed Git ID") {
		t.Fatalf("expected observed git id in status:\n%s", out)
	}
}

func TestWorkspaceStatusNoObservedGitWhenAgreeing(t *testing.T) {
	dir := gitRepo(t, "https://example.com/org/agree-repo.git")
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	if out := runCLI(t, home, dir, "workspace", "status"); strings.Contains(out, "Observed Git ID") {
		t.Fatalf("agreement must not show observed git id:\n%s", out)
	}
}

func TestIntegrationNonGitAToB(t *testing.T) {
	syncFolder := t.TempDir()
	dirA, dirB := t.TempDir(), t.TempDir()
	homeA, homeB := t.TempDir(), t.TempDir()

	runCLI(t, homeA, dirA, "init")
	runCLI(t, homeA, dirA, "sync", "init", syncFolder)
	runCLI(t, homeA, dirA, "workspace", "init", "--sync")
	runCLI(t, homeA, dirA, "task", "start", "task-a")
	runCLI(t, homeA, dirA, "checkpoint", "-m", "checkpoint-a")
	runCLI(t, homeA, dirA, "sync", "push")

	storeA := store.New(homeA)
	list, err := storeA.ListWorkspaces()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected one workspace on A, got %d: %v", len(list), err)
	}
	idA := list[0].ID

	runCLI(t, homeB, dirB, "init")
	runCLI(t, homeB, dirB, "sync", "init", syncFolder)
	if out := runCLI(t, homeB, dirB, "workspace", "available"); !strings.Contains(out, idA) {
		t.Fatalf("available does not list %s:\n%s", idA, out)
	}
	runCLI(t, homeB, dirB, "workspace", "attach", idA)
	runCLI(t, homeB, dirB, "sync", "pull")
	if out := runCLI(t, homeB, dirB, "task", "resume", "task-a"); !strings.Contains(out, "task-a") {
		t.Fatalf("resume failed:\n%s", out)
	}

	storeB := store.New(homeB)
	wb, err := storeB.ReadWorkspace(idA)
	if err != nil {
		t.Fatal(err)
	}
	if wb.ID != idA {
		t.Fatalf("B canonical ID differs: %s != %s", wb.ID, idA)
	}
	if !hasLocalPath(wb.LocalPaths, dirB) {
		t.Fatalf("B local path missing: %v", wb.LocalPaths)
	}
	if hasLocalPath(wb.LocalPaths, dirA) {
		t.Fatalf("A path leaked into B: %v", wb.LocalPaths)
	}
}

func TestIntegrationGitNoRemoteAToB(t *testing.T) {
	syncFolder := t.TempDir()
	dirA, dirB := gitRepoNoRemote(t), gitRepoNoRemote(t)
	homeA, homeB := t.TempDir(), t.TempDir()

	runCLI(t, homeA, dirA, "init")
	runCLI(t, homeA, dirA, "sync", "init", syncFolder)
	runCLI(t, homeA, dirA, "workspace", "init", "--sync")
	runCLI(t, homeA, dirA, "task", "start", "task-a")
	runCLI(t, homeA, dirA, "sync", "push")

	storeA := store.New(homeA)
	list, err := storeA.ListWorkspaces()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected one workspace on A, got %d: %v", len(list), err)
	}
	idA := list[0].ID

	runCLI(t, homeB, dirB, "init")
	runCLI(t, homeB, dirB, "sync", "init", syncFolder)
	runCLI(t, homeB, dirB, "workspace", "attach", idA)
	runCLI(t, homeB, dirB, "sync", "pull")
	if out := runCLI(t, homeB, dirB, "task", "resume", "task-a"); !strings.Contains(out, "task-a") {
		t.Fatalf("resume failed:\n%s", out)
	}
	storeB := store.New(homeB)
	wb, err := storeB.ReadWorkspace(idA)
	if err != nil {
		t.Fatal(err)
	}
	if wb.ID != idA {
		t.Fatalf("B canonical ID differs: %s != %s", wb.ID, idA)
	}
}

func TestIntegrationLegacyUpgradeToAttachToPull(t *testing.T) {
	syncFolder := t.TempDir()
	// Legacy-only shared folder with one non-Git workspace + continuity.
	seedLegacyWorkspace(t, syncFolder, "w_legacy", "id: w_legacy\nname: legacy-notes\nidentity:\n  type: local-directory\n  value: \nlocalPaths:\n  - /home/alice/private/foo\nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\nupdatedAt: 2026-01-02T00:00:00Z\n", map[string]string{
		"tasks/legacy-task/task.yaml":         "id: legacy-task\nname: legacy-task\nworkspaceId: w_legacy\nstatus: active\ncreatedAt: 2026-01-01T00:00:00Z\nupdatedAt: 2026-01-01T00:00:00Z\n",
		"tasks/legacy-task/checkpoints.jsonl": "",
		"active-task":                         "legacy-task\n",
	})

	dirB := t.TempDir()
	homeB := t.TempDir()
	runCLI(t, homeB, dirB, "init")
	runCLI(t, homeB, dirB, "sync", "init", syncFolder)
	runCLI(t, homeB, dirB, "sync", "upgrade")
	// Legacy namespace preserved, v2 authoritative.
	if _, err := os.Stat(filepath.Join(syncFolder, "context-baggage-state", "workspaces", "w_legacy", "workspace.yaml")); err != nil {
		t.Fatalf("legacy namespace modified: %v", err)
	}
	if out := runCLI(t, homeB, dirB, "workspace", "available"); !strings.Contains(out, "w_legacy") {
		t.Fatalf("available does not list w_legacy:\n%s", out)
	}
	runCLI(t, homeB, dirB, "workspace", "attach", "w_legacy")
	runCLI(t, homeB, dirB, "sync", "pull")
	resumeOut := runCLI(t, homeB, dirB, "task", "resume", "legacy-task")
	if !strings.Contains(resumeOut, "legacy-task") {
		t.Fatalf("resume after legacy transition failed:\n%s", resumeOut)
	}
	storeB := store.New(homeB)
	wb, err := storeB.ReadWorkspace("w_legacy")
	if err != nil {
		t.Fatal(err)
	}
	if !hasLocalPath(wb.LocalPaths, dirB) {
		t.Fatalf("B LocalPaths missing: %v", wb.LocalPaths)
	}
	if hasLocalPath(wb.LocalPaths, "/home/alice/private/foo") {
		t.Fatalf("legacy LocalPaths leaked into B: %v", wb.LocalPaths)
	}
}
