package githubsync

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const validMarker = "format: 1\ndestinationId: " + neutralDestinationID + "\n"

func TestRepositoryRefClassification(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		files      map[string]string
		addMain    bool
		addTag     bool
		removeHead bool
		wantState  RepositoryState
		wantErr    error
	}{
		{name: "no refs", wantState: RepositoryEmpty},
		{name: "managed ref only", branch: "context-baggage", files: markerFiles(), wantState: RepositoryInitialized},
		{name: "main only", branch: "main", files: markerFiles(), wantErr: ErrRepositoryIncompatible},
		{name: "managed and main", branch: "context-baggage", files: markerFiles(), addMain: true, wantErr: ErrRepositoryIncompatible},
		{name: "managed and tag", branch: "context-baggage", files: markerFiles(), addTag: true, wantErr: ErrRepositoryIncompatible},
		{name: "tag only", branch: "context-baggage", files: markerFiles(), addTag: true, removeHead: true, wantErr: ErrRepositoryIncompatible},
		{name: "empty tree commit", branch: "context-baggage", files: map[string]string{}, wantErr: ErrRepositoryIncompatible},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRepositoryRefCase(t, test.branch, test.files, test.addMain, test.addTag, test.removeHead, test.wantState, test.wantErr)
		})
	}
}

func assertRepositoryRefCase(t *testing.T, branch string, files map[string]string, addMain, addTag, removeHead bool, wantState RepositoryState, wantErr error) {
	t.Helper()
	remote, commitID := createBareFixture(t, branch, files)
	if addMain {
		runFixtureGit(t, "--git-dir", remote, "update-ref", "refs/heads/main", commitID)
	}
	if addTag {
		runFixtureGit(t, "--git-dir", remote, "update-ref", "refs/tags/example", commitID)
	}
	if removeHead {
		runFixtureGit(t, "--git-dir", remote, "update-ref", "-d", managedRef)
	}
	snapshot, err := inspectLocalFixture(t, remote)
	if wantErr != nil && !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got snapshot %+v, error %v", wantErr, snapshot, err)
	}
	if wantErr == nil && (err != nil || snapshot.State != wantState) {
		t.Fatalf("state = %q, error = %v", snapshot.State, err)
	}
}

func TestRepositoryRootAndPortableSnapshotContracts(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantError bool
	}{
		{name: "marker only", files: markerFiles()},
		{name: "marker and valid v2", files: validPortableFiles()},
		{name: "missing marker", files: map[string]string{portableRootName + "/workspaces/w_one/workspace.yaml": portableWorkspaceYAML()}, wantError: true},
		{name: "readme", files: mergeFiles(markerFiles(), map[string]string{"README.md": "unrelated\n"}), wantError: true},
		{name: "legacy namespace", files: mergeFiles(markerFiles(), map[string]string{"context-baggage-state/workspaces/old": "legacy\n"}), wantError: true},
		{name: "unrelated directory", files: mergeFiles(markerFiles(), map[string]string{"other/file": "other\n"}), wantError: true},
		{name: "multiple unexpected files", files: mergeFiles(markerFiles(), map[string]string{"README.md": "x", "LICENSE": "y"}), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRepositoryContentCase(t, test.name, test.files, test.wantError)
		})
	}
}

func assertRepositoryContentCase(t *testing.T, name string, files map[string]string, wantError bool) {
	t.Helper()
	remote, _ := createBareFixture(t, "context-baggage", files)
	snapshot, err := inspectLocalFixture(t, remote)
	if wantError {
		if !errors.Is(err, ErrRepositoryIncompatible) {
			t.Fatalf("expected incompatible repository, got %+v, %v", snapshot, err)
		}
		return
	}
	if err != nil || snapshot.State != RepositoryInitialized || snapshot.ManagedDestinationID != neutralDestinationID {
		t.Fatalf("unexpected snapshot %+v, %v", snapshot, err)
	}
	if name == "marker only" && (snapshot.PortablePresent || snapshot.PortableHash != "") {
		t.Fatalf("marker-only portable identity = present %t hash %q", snapshot.PortablePresent, snapshot.PortableHash)
	}
	if name == "marker and valid v2" && (!snapshot.PortablePresent || snapshot.PortableHash == "") {
		t.Fatalf("portable snapshot not observed: %+v", snapshot)
	}
}

func TestPortableHashExcludesMarkerAndGitCommit(t *testing.T) {
	firstFiles := validPortableFiles()
	secondFiles := validPortableFiles()
	secondFiles[managedMarkerName] = "format: 1\ndestinationId: dst_ffffffffffffffffffffffffffffffff\n"
	firstRemote, _ := createBareFixture(t, "context-baggage", firstFiles)
	secondRemote, _ := createBareFixture(t, "context-baggage", secondFiles)
	first, err := inspectLocalFixture(t, firstRemote)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspectLocalFixture(t, secondRemote)
	if err != nil {
		t.Fatal(err)
	}
	if first.CommitID == second.CommitID || first.ManagedDestinationID == second.ManagedDestinationID || first.PortableHash != second.PortableHash {
		t.Fatalf("marker or commit contaminated portable identity: first %+v second %+v", first, second)
	}
}

func TestInspectionStaysPinnedToObservedCommitWhenRefMoves(t *testing.T) {
	remote, observed := createBareFixture(t, "context-baggage", markerFiles())
	work := filepath.Join(t.TempDir(), "work")
	runFixtureGit(t, "clone", "--quiet", remote, work)
	runFixtureGit(t, "-C", work, "config", "user.name", "Context Baggage Test")
	runFixtureGit(t, "-C", work, "config", "user.email", "test@example.invalid")
	updatedMarker := "format: 1\ndestinationId: dst_ffffffffffffffffffffffffffffffff\n"
	writeFixtureFile(t, work, managedMarkerName, updatedMarker)
	runFixtureGit(t, "-C", work, "add", managedMarkerName)
	runFixtureGit(t, "-C", work, "commit", "--quiet", "-m", "move managed ref")
	runFixtureGit(t, "--git-dir", remote, "fetch", "--quiet", work, "HEAD:"+managedRef)

	runner := localFixtureRunner(t)
	snapshot, err := runner.inspectObservedCommit(context.Background(), remote, "github.com/neutral/repository", observed)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CommitID != observed || snapshot.ManagedDestinationID != neutralDestinationID {
		t.Fatalf("inspection moved with remote ref: %+v", snapshot)
	}
}

func TestInspectionDoesNotModifyRemoteRefs(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	before := runFixtureGit(t, "--git-dir", remote, "show-ref")
	if _, err := inspectLocalFixture(t, remote); err != nil {
		t.Fatal(err)
	}
	after := runFixtureGit(t, "--git-dir", remote, "show-ref")
	if after != before {
		t.Fatalf("read-only inspection changed remote refs:\nbefore: %s\nafter: %s", before, after)
	}
}

func TestInitialFetchDoesNotEagerlyDownloadLargeUnneededBlob(t *testing.T) {
	files := markerFiles()
	files["unrelated.bin"] = string(bytes.Repeat([]byte("x"), 1024*1024))
	remote, commitID := createBareFixture(t, "context-baggage", files)
	blobID := runFixtureGit(t, "--git-dir", remote, "rev-parse", commitID+":unrelated.bin")
	root := t.TempDir()
	gitDir := filepath.Join(root, "snapshot.git")
	runner := localFixtureRunner(t)
	if _, err := runner.run(context.Background(), time.Second, "", "init", "--bare", "--quiet", gitDir); err != nil {
		t.Fatal(err)
	}
	if err := appendManagedReadRemote(gitDir, remote); err != nil {
		t.Fatal(err)
	}
	if err := runner.fetchObservedCommit(context.Background(), root, gitDir, commitID); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "--git-dir", gitDir, "cat-file", "-e", blobID)
	command.Env = append(gitEnvironment(), "GIT_NO_LAZY_FETCH=1")
	if err := command.Run(); err == nil {
		t.Fatal("blob:none initial fetch eagerly downloaded an unneeded blob")
	}
}

func TestLocalObjectInspectionNeverUsesUnexpectedPromisor(t *testing.T) {
	files := markerFiles()
	files["unrelated.bin"] = string(bytes.Repeat([]byte("x"), 1024))
	remote, commitID := createBareFixture(t, "context-baggage", files)
	blobID := runFixtureGit(t, "--git-dir", remote, "rev-parse", commitID+":unrelated.bin")
	root := t.TempDir()
	gitDir := filepath.Join(root, "snapshot.git")
	runner := localFixtureRunner(t)
	if _, err := runner.run(context.Background(), time.Second, "", "init", "--bare", "--quiet", gitDir); err != nil {
		t.Fatal(err)
	}
	if err := appendManagedReadRemote(gitDir, remote); err != nil {
		t.Fatal(err)
	}
	if err := runner.fetchObservedCommit(context.Background(), root, gitDir, commitID); err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int32
	hostile := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer hostile.Close()
	t.Setenv("GIT_SSL_NO_VERIFY", "true")
	runFixtureGit(t, "--git-dir", gitDir, "config", "remote."+targetRemoteName+".promisor", "false")
	runFixtureGit(t, "--git-dir", gitDir, "config", "remote.hostile.url", hostile.URL+"/objects.git")
	runFixtureGit(t, "--git-dir", gitDir, "config", "remote.hostile.promisor", "true")

	if _, err := runner.run(context.Background(), time.Second, gitDir, "cat-file", "-e", blobID); err == nil {
		t.Fatal("missing promised blob unexpectedly became available")
	}
	if requests.Load() != 0 {
		t.Fatal("local object inspection contacted an unexpected promisor remote")
	}
}

func inspectLocalFixture(t *testing.T, remote string) (RepositorySnapshot, error) {
	t.Helper()
	runner := localFixtureRunner(t)
	commitID, empty, err := runner.observeManagedRef(context.Background(), remote)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	if empty {
		return RepositorySnapshot{RepositoryIdentity: "github.com/neutral/repository", State: RepositoryEmpty}, nil
	}
	return runner.inspectObservedCommit(context.Background(), remote, "github.com/neutral/repository", commitID)
}

func localFixtureRunner(t *testing.T) GitRunner {
	t.Helper()
	runner, err := DiscoverGit()
	if err != nil {
		t.Skipf("system Git unavailable: %v", err)
	}
	runner.testFileTransport = true
	runner.inspectionTimeout = 10 * time.Second
	runner.readTimeout = 30 * time.Second
	return runner
}

func createBareFixture(t *testing.T, branch string, files map[string]string) (string, string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "managed.git")
	if branch == "" {
		runFixtureGit(t, "init", "--bare", "--quiet", remote)
		return remote, ""
	}
	work := createWorkFixture(t, branch, files)
	commitID := runFixtureGit(t, "-C", work, "rev-parse", "HEAD")
	runFixtureGit(t, "clone", "--bare", "--quiet", work, remote)
	runFixtureGit(t, "--git-dir", remote, "config", "uploadpack.allowFilter", "true")
	runFixtureGit(t, "--git-dir", remote, "config", "uploadpack.allowAnySHA1InWant", "true")
	return remote, commitID
}

func createWorkFixture(t *testing.T, branch string, files map[string]string) string {
	t.Helper()
	work := t.TempDir()
	runFixtureGit(t, "init", "--quiet", work)
	runFixtureGit(t, "-C", work, "config", "user.name", "Context Baggage Test")
	runFixtureGit(t, "-C", work, "config", "user.email", "test@example.invalid")
	for name, content := range files {
		writeFixtureFile(t, work, name, content)
	}
	runFixtureGit(t, "-C", work, "add", "--all")
	runFixtureGit(t, "-C", work, "commit", "--quiet", "--allow-empty", "-m", "fixture")
	runFixtureGit(t, "-C", work, "branch", "-M", branch)
	return work
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runFixtureGit(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Env = append(gitEnvironment(), "HOME="+t.TempDir(), "USERPROFILE="+t.TempDir())
	output, err := command.Output()
	if err != nil {
		t.Fatalf("fixture Git failed: %v", err)
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(value []byte) []byte {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\n' || value[start] == '\r' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\r' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}

func markerFiles() map[string]string { return map[string]string{managedMarkerName: validMarker} }

func validPortableFiles() map[string]string {
	return map[string]string{
		managedMarkerName: validMarker,
		portableRootName + "/workspaces/w_one/workspace.yaml": portableWorkspaceYAML(),
	}
}

func portableWorkspaceYAML() string {
	return "id: w_one\nname: example\nidentity:\n  type: local-directory\n  value: neutral\nsync: true\ncreatedAt: 2026-01-01T00:00:00Z\n"
}

func mergeFiles(groups ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, group := range groups {
		for name, content := range group {
			merged[name] = content
		}
	}
	return merged
}
