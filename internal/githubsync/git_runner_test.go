package githubsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const gitHelperEnvironment = "CTX_BAG_GIT_TEST_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(gitHelperEnvironment); mode != "" {
		runGitHelper(mode)
		return
	}
	os.Exit(m.Run())
}

func TestDiscoverGitReportsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := DiscoverGit()
	if !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("expected Git unavailable, got %v", err)
	}
}

func TestGitRunnerRejectsUnparsedLocatorBeforeExecution(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "secret-error")
	err := helperGitRunner().VerifyTargetBinding(context.Background(), Locator{})
	if !errors.Is(err, ErrInvalidLocator) {
		t.Fatalf("expected invalid locator, got %v", err)
	}
}

func TestGitRunnerVerifyReadableUsesBoundNonInteractiveGit(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "success")
	runner := helperGitRunner()
	if err := runner.VerifyReadable(context.Background(), mustLocator(t, "https://github.com/owner/repo")); err != nil {
		t.Fatal(err)
	}
}

func TestGitRunnerNetworkOperationDoesNotFollowHTTPSRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/redirected/") {
			redirectedRequests.Add(1)
			response.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			_, _ = response.Write([]byte("001e# service=git-upload-pack\n00000000"))
			return
		}
		http.Redirect(response, request, "/redirected/info/refs?"+request.URL.RawQuery, http.StatusFound)
	}))
	defer server.Close()

	t.Setenv("GIT_SSL_NO_VERIFY", "true")
	runner, err := DiscoverGit()
	if err != nil {
		t.Skipf("system Git unavailable: %v", err)
	}
	_, err = runner.runNetwork(context.Background(), 5*time.Second, "", "ls-remote", "--quiet", server.URL+"/configured/repo.git", "HEAD")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected redirected Git read to fail, got %v", err)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatal("Git followed an HTTPS redirect")
	}
}

func TestGitRunnerSanitizesExternalError(t *testing.T) {
	const fakeSecret = "password-example"
	t.Setenv(gitHelperEnvironment, "secret-error")
	runner := helperGitRunner()
	_, err := runner.run(context.Background(), time.Second, "", "test")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected transport failure, got %v", err)
	}
	if strings.Contains(err.Error(), fakeSecret) {
		t.Fatalf("Git stderr leaked in error: %q", err)
	}
}

func TestGitRunnerBoundsStdoutAndStderr(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "overflow")
	runner := helperGitRunner()
	_, err := runner.run(context.Background(), time.Second, "", "test")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected bounded-output failure, got %v", err)
	}
}

func TestBoundedBufferNeverRetainsMoreThanLimit(t *testing.T) {
	buffer := newBoundedBuffer(maxGitOutput)
	payload := []byte(strings.Repeat("x", maxGitOutput+1024))
	written, err := buffer.Write(payload)
	if err != nil || written != len(payload) {
		t.Fatalf("unexpected bounded write result: %d, %v", written, err)
	}
	if !buffer.overflow || len(buffer.String()) != maxGitOutput {
		t.Fatalf("output was not capped: overflow=%t bytes=%d", buffer.overflow, len(buffer.String()))
	}
}

func TestGitRunnerReturnsPromptlyWhenContextExpires(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "block")
	runner := helperGitRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runner.run(ctx, time.Second, "", "test")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected cancelled transport failure, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cancelled Git process returned too slowly: %s", elapsed)
	}
}

func TestGitRunnerPreservesAuthEnvironmentAndDisablesPrompts(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "check-environment")
	t.Setenv("SSH_AUTH_SOCK", "example-agent-socket")
	t.Setenv("GIT_DIR", "example-untrusted-repository")
	runner := helperGitRunner()
	if _, err := runner.run(context.Background(), 5*time.Second, "", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestOperationRemotePreservesWindowsStylePath(t *testing.T) {
	runner, err := DiscoverGit()
	if err != nil {
		t.Skipf("system Git unavailable: %v", err)
	}
	gitDir := filepath.Join(t.TempDir(), "repository.git")
	if _, err := runner.run(context.Background(), time.Second, "", "init", "--bare", "--quiet", gitDir); err != nil {
		t.Fatal(err)
	}
	const remote = `C:\neutral\managed.git`
	if err := appendOperationRemote(gitDir, remote); err != nil {
		t.Fatal(err)
	}
	result, err := runner.run(context.Background(), time.Second, gitDir, "remote", "get-url", targetRemoteName)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result.stdout) != remote {
		t.Fatalf("remote path was not preserved: %q", result.stdout)
	}
}

func TestExplicitBlobAcquisitionUsesHardenedNetworkThenLocalRead(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "blob-acquisition")
	buffer := &strings.Builder{}
	entry := gitTreeEntry{objectID: strings.Repeat("a", 40)}
	written, err := helperGitRunner().acquireBlob(context.Background(), t.TempDir(), t.TempDir(), entry, 4, buffer)
	if err != nil {
		t.Fatal(err)
	}
	if written != 4 || buffer.String() != "blob" {
		t.Fatalf("unexpected materialized blob: bytes=%d value=%q", written, buffer.String())
	}
}

func TestExplicitBlobAcquisitionIsCancellable(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "block")
	runner := helperGitRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := runner.acquirePromisedBlob(ctx, t.TempDir(), t.TempDir(), strings.Repeat("a", 40))
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected cancelled acquisition, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelled acquisition returned too slowly: %s", elapsed)
	}
}

func TestExplicitBlobAcquisitionUsesTemporaryGuard(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "grow-temp")
	root := t.TempDir()
	runner := helperGitRunner()
	runner.testTemporaryLimit = 4 * 1024
	err := runner.acquirePromisedBlob(context.Background(), root, root, strings.Repeat("a", 40))
	if !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("expected guarded acquisition refusal, got %v", err)
	}
}

func TestGuardedGitOperationStopsTemporaryGrowth(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "grow-temp")
	root := t.TempDir()
	err := helperGitRunner().runNetworkGuarded(context.Background(), 5*time.Second, root, root, 4*1024, "grow")
	if !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("expected temporary guard refusal, got %v", err)
	}
	info, statErr := os.Stat(filepath.Join(root, "growing.pack"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() >= 64*1024 {
		t.Fatalf("temporary guard reacted too late: %d bytes", info.Size())
	}
}

func helperGitRunner() GitRunner {
	return GitRunner{
		executable:        os.Args[0],
		inspectionTimeout: 5 * time.Second,
		readTimeout:       5 * time.Second,
	}
}

func runGitHelper(mode string) {
	switch mode {
	case "success":
		if hasArguments("remote", "get-url") {
			_, _ = fmt.Fprintln(os.Stdout, "https://github.com/owner/repo.git")
		}
		if hasArguments("ls-remote") && !hasArguments("--quiet", "https://github.com/owner/repo.git", "HEAD") {
			os.Exit(2)
		}
		if hasArguments("ls-remote") && !hasArguments("http.followRedirects=false") {
			os.Exit(2)
		}
	case "secret-error":
		_, _ = fmt.Fprintln(os.Stderr, "credential helper exposed password-example")
		os.Exit(1)
	case "overflow":
		_, _ = os.Stdout.Write([]byte(strings.Repeat("o", maxGitOutput+1024)))
		_, _ = os.Stderr.Write([]byte(strings.Repeat("e", maxGitOutput+1024)))
		os.Exit(1)
	case "block":
		time.Sleep(10 * time.Second)
	case "check-environment":
		if os.Getenv("GIT_TERMINAL_PROMPT") != "0" || os.Getenv("GCM_INTERACTIVE") != "Never" ||
			os.Getenv("GIT_NO_LAZY_FETCH") != "1" || os.Getenv("LC_ALL") != "C" {
			os.Exit(1)
		}
		if os.Getenv("SSH_AUTH_SOCK") != "example-agent-socket" {
			os.Exit(1)
		}
		if os.Getenv("GIT_DIR") != "" {
			os.Exit(1)
		}
	case "grow-temp":
		growTemporaryFile()
	case "blob-acquisition":
		assertBlobAcquisitionCommand()
	default:
		os.Exit(2)
	}
}

func assertBlobAcquisitionCommand() {
	const objectID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if hasArguments("fetch") {
		if !hasArguments("http.followRedirects=false", "--no-tags", "--no-write-fetch-head", targetRemoteName, objectID) {
			os.Exit(1)
		}
		return
	}
	if hasArguments("cat-file", "blob", objectID) && os.Getenv("GIT_NO_LAZY_FETCH") == "1" {
		_, _ = fmt.Fprint(os.Stdout, "blob")
		return
	}
	os.Exit(1)
}

func growTemporaryFile() {
	file, err := os.Create("growing.pack")
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = file.Close() }()
	chunk := []byte(strings.Repeat("x", 1024))
	for {
		if _, err := file.Write(chunk); err != nil {
			os.Exit(1)
		}
		if err := file.Sync(); err != nil {
			os.Exit(1)
		}
		time.Sleep(time.Millisecond)
	}
}

func hasArguments(want ...string) bool {
	joined := strings.Join(os.Args[1:], " ")
	for _, value := range want {
		if !strings.Contains(joined, value) {
			return false
		}
	}
	return true
}
