package githubsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyTargetBindingAcceptsUnchangedTarget(t *testing.T) {
	runner := isolatedGitRunner(t, "")
	locator := mustLocator(t, "https://github.com/owner/repo.git")
	if err := runner.VerifyTargetBinding(context.Background(), locator); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTargetBindingAcceptsSameIdentityTransportRewrite(t *testing.T) {
	runner := isolatedGitRunner(t, `[url "git@github.com:"]
	insteadOf = https://github.com/
`)
	locator := mustLocator(t, "https://github.com/owner/repo.git")
	if err := runner.VerifyTargetBinding(context.Background(), locator); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTargetBindingRejectsInsteadOfRewrite(t *testing.T) {
	runner := isolatedGitRunner(t, `[url "https://github.com/other/"]
	insteadOf = https://github.com/owner/
`)
	assertDestinationMismatch(t, runner, mustLocator(t, "https://github.com/owner/repo.git"))
}

func TestVerifyTargetBindingRejectsPushInsteadOfRewrite(t *testing.T) {
	runner := isolatedGitRunner(t, `[url "https://github.com/other/"]
	pushInsteadOf = https://github.com/owner/
`)
	assertDestinationMismatch(t, runner, mustLocator(t, "https://github.com/owner/repo.git"))
}

func TestVerifyTargetBindingRejectsExplicitPushURL(t *testing.T) {
	runner := isolatedGitRunner(t, `[remote "ctx-bag-security"]
	pushurl = https://github.com/other/repo.git
`)
	assertDestinationMismatch(t, runner, mustLocator(t, "https://github.com/owner/repo.git"))
}

func TestVerifyTargetBindingRejectsMultiplePushURLs(t *testing.T) {
	runner := isolatedGitRunner(t, `[remote "ctx-bag-security"]
	pushurl = https://github.com/owner/repo.git
	pushurl = git@github.com:owner/repo.git
`)
	assertDestinationMismatch(t, runner, mustLocator(t, "https://github.com/owner/repo.git"))
}

func TestVerifyTargetBindingIgnoresRepositoryRedirectingEnvironment(t *testing.T) {
	runner := isolatedGitRunner(t, "")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "different-repository"))
	if err := runner.VerifyTargetBinding(context.Background(), mustLocator(t, "https://github.com/owner/repo.git")); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTargetBindingRejectsCredentialRewriteWithoutEcho(t *testing.T) {
	const fakeSecret = "password-example"
	runner := isolatedGitRunner(t, `[url "https://user-example:`+fakeSecret+`@github.com/owner/"]
	insteadOf = https://github.com/owner/
`)
	err := runner.VerifyTargetBinding(context.Background(), mustLocator(t, "https://github.com/owner/repo.git"))
	if !errors.Is(err, ErrDestinationMismatch) {
		t.Fatalf("expected destination mismatch, got %v", err)
	}
	if strings.Contains(err.Error(), fakeSecret) {
		t.Fatalf("rewritten credential leaked in error: %q", err)
	}
}

func isolatedGitRunner(t *testing.T, globalConfig string) GitRunner {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(configPath, []byte(globalConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runner, err := DiscoverGit()
	if err != nil {
		t.Skipf("system Git unavailable: %v", err)
	}
	return runner
}

func assertDestinationMismatch(t *testing.T, runner GitRunner, locator Locator) {
	t.Helper()
	err := runner.VerifyTargetBinding(context.Background(), locator)
	if !errors.Is(err, ErrDestinationMismatch) {
		t.Fatalf("expected destination mismatch, got %v", err)
	}
}

func mustLocator(t *testing.T, raw string) Locator {
	t.Helper()
	locator, err := ParseLocator(raw)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}
