package githubsync

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxGitOutput         = 64 * 1024
	gitInspectionTimeout = 5 * time.Second
	gitReadTimeout       = 30 * time.Second
	targetRemoteName     = "ctx-bag-security"
)

// GitRunner is the concrete system-Git boundary used by GitHub synchronization.
// It preserves normal external authentication while bounding execution and
// preventing raw subprocess diagnostics from reaching callers.
type GitRunner struct {
	executable         string
	inspectionTimeout  time.Duration
	readTimeout        time.Duration
	testFileTransport  bool
	testTemporaryLimit int64
}

// DiscoverGit locates the system Git executable only when a GitHub operation
// explicitly asks for it. Filesystem synchronization never calls this function.
func DiscoverGit() (GitRunner, error) {
	executable, err := exec.LookPath("git")
	if err != nil {
		return GitRunner{}, ErrGitUnavailable
	}
	return GitRunner{
		executable:        executable,
		inspectionTimeout: gitInspectionTimeout,
		readTimeout:       gitReadTimeout,
	}, nil
}

// VerifyTargetBinding proves that Git's effective fetch and push destinations
// still identify the same repository selected by the strict locator parser.
// This rejects insteadOf, pushInsteadOf, and explicit push URL rebinding.
func (g GitRunner) VerifyTargetBinding(ctx context.Context, configured Locator) error {
	if !configured.valid() {
		return ErrInvalidLocator
	}
	fetch, push, err := g.effectiveTargets(ctx, configured)
	if err != nil {
		return err
	}
	if fetch.identity != configured.identity || push.identity != configured.identity {
		return ErrDestinationMismatch
	}
	return nil
}

// VerifyReadable proves target binding before asking authenticated system Git
// to read the repository. It does not inspect refs or managed repository state.
func (g GitRunner) VerifyReadable(ctx context.Context, configured Locator) error {
	if err := g.VerifyTargetBinding(ctx, configured); err != nil {
		return err
	}
	return g.readRepository(ctx, configured)
}

func (g GitRunner) readRepository(ctx context.Context, configured Locator) error {
	_, err := g.runNetwork(ctx, g.readTimeout, "", "ls-remote", "--quiet", configured.url, "HEAD")
	return err
}

// runNetwork executes a managed Git network operation with HTTP redirects
// disabled so authenticated access cannot silently move to another target.
func (g GitRunner) runNetwork(ctx context.Context, timeout time.Duration, dir string, args ...string) (gitCommandResult, error) {
	networkArgs := []string{"-c", "http.followRedirects=false"}
	networkArgs = append(networkArgs, args...)
	return g.run(ctx, timeout, dir, networkArgs...)
}

// runNetworkStream keeps network stdout out of the fixed diagnostic capture
// so a trusted incremental parser can enforce its own semantic limits.
func (g GitRunner) runNetworkStream(ctx context.Context, timeout time.Duration, dir string, stdout io.Writer, args ...string) error {
	networkArgs := []string{"-c", "http.followRedirects=false"}
	networkArgs = append(networkArgs, args...)
	return g.runStream(ctx, timeout, dir, stdout, nil, networkArgs...)
}

// effectiveTargets asks Git itself to expand URL rewriting for one isolated
// operation-local remote. Requiring exactly one fetch and one push URL avoids
// accidental multi-target publication semantics.
func (g GitRunner) effectiveTargets(ctx context.Context, configured Locator) (Locator, Locator, error) {
	dir, err := os.MkdirTemp("", "ctx-bag-git-target-*")
	if err != nil {
		return Locator{}, Locator{}, ErrTransportUnavailable
	}
	defer func() { _ = os.RemoveAll(dir) }()
	gitDir := dir
	if _, err := g.run(ctx, g.inspectionTimeout, gitDir, "init", "--bare", "--quiet", "."); err != nil {
		return Locator{}, Locator{}, err
	}
	if err := appendOperationRemote(gitDir, configured.url); err != nil {
		return Locator{}, Locator{}, err
	}
	fetchOutput, err := g.run(ctx, g.inspectionTimeout, gitDir, "remote", "get-url", "--all", targetRemoteName)
	if err != nil {
		return Locator{}, Locator{}, err
	}
	pushOutput, err := g.run(ctx, g.inspectionTimeout, gitDir, "remote", "get-url", "--push", "--all", targetRemoteName)
	if err != nil {
		return Locator{}, Locator{}, err
	}
	fetch, err := parseSingleEffectiveTarget(fetchOutput.stdout)
	if err != nil {
		return Locator{}, Locator{}, err
	}
	push, err := parseSingleEffectiveTarget(pushOutput.stdout)
	if err != nil {
		return Locator{}, Locator{}, err
	}
	return fetch, push, nil
}

// appendOperationRemote writes only the already-sanitized locator into the
// temporary repository. Avoiding `git config` prevents environment such as
// GIT_CONFIG from redirecting this setup write outside the temporary state.
func appendOperationRemote(gitDir, transportURL string) error {
	config, err := os.OpenFile(filepath.Join(gitDir, "config"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return ErrTransportUnavailable
	}
	_, writeErr := io.WriteString(config, "\n[remote \""+targetRemoteName+"\"]\n\turl = "+quoteGitConfigValue(transportURL)+"\n")
	closeErr := config.Close()
	if writeErr != nil || closeErr != nil {
		return ErrTransportUnavailable
	}
	return nil
}

func quoteGitConfigValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

// parseSingleEffectiveTarget reuses the strict parser so rewritten custom
// protocols, embedded credentials, extra targets, and non-GitHub hosts fail
// closed without their values appearing in an error.
func parseSingleEffectiveTarget(output string) (Locator, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 || strings.TrimSpace(lines[0]) == "" {
		return Locator{}, ErrDestinationMismatch
	}
	target, err := ParseLocator(strings.TrimSuffix(lines[0], "\r"))
	if err != nil {
		return Locator{}, ErrDestinationMismatch
	}
	return target, nil
}

type gitCommandResult struct {
	stdout string
}

// run executes Git without a shell, with non-interactive authentication and
// bounded output. Raw argv, stdout, and stderr are intentionally omitted from
// all returned errors because credential helpers and SSH may emit secrets.
func (g GitRunner) run(ctx context.Context, timeout time.Duration, dir string, args ...string) (gitCommandResult, error) {
	if g.executable == "" {
		return gitCommandResult{}, ErrGitUnavailable
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	commandArgs := g.commandArgs(args)
	command := exec.CommandContext(runCtx, g.executable, commandArgs...)
	command.Dir = dir
	command.Env = gitEnvironment()
	stdout := newBoundedBuffer(maxGitOutput)
	stderr := newBoundedBuffer(maxGitOutput)
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if runCtx.Err() != nil || err != nil || stdout.overflow || stderr.overflow {
		return gitCommandResult{}, ErrTransportUnavailable
	}
	return gitCommandResult{stdout: stdout.String()}, nil
}

// runStream directs stdout only to a purpose-built bounded parser or writer;
// stderr remains bounded and neither stream is included in returned errors.
func (g GitRunner) runStream(ctx context.Context, timeout time.Duration, dir string, stdout io.Writer, stdin io.Reader, args ...string) error {
	if g.executable == "" {
		return ErrGitUnavailable
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, g.executable, g.commandArgs(args)...)
	command.Dir = dir
	command.Env = gitEnvironment()
	command.Stdout = stdout
	command.Stdin = stdin
	stderr := newBoundedBuffer(maxGitOutput)
	command.Stderr = stderr
	err := command.Run()
	if runCtx.Err() != nil || err != nil || stderr.overflow {
		return ErrTransportUnavailable
	}
	return nil
}

func (g GitRunner) commandArgs(args []string) []string {
	commandArgs := []string{
		"-c", "protocol.allow=never",
		"-c", "protocol.https.allow=always",
		"-c", "protocol.ssh.allow=always",
	}
	if g.testFileTransport {
		commandArgs = append(commandArgs, "-c", "protocol.file.allow=always")
	}
	commandArgs = append(commandArgs, args...)
	return commandArgs
}

// gitEnvironment preserves external authentication configuration while
// disabling terminal/credential-manager prompts and stabilizing diagnostics.
func gitEnvironment() []string {
	replacements := map[string]string{
		"GIT_AUTHOR_EMAIL":    "sync@context-baggage.invalid",
		"GIT_AUTHOR_NAME":     "Context Baggage",
		"GIT_COMMITTER_EMAIL": "sync@context-baggage.invalid",
		"GIT_COMMITTER_NAME":  "Context Baggage",
		"GIT_TERMINAL_PROMPT": "0",
		"GCM_INTERACTIVE":     "Never",
		"GIT_NO_LAZY_FETCH":   "1",
		"LC_ALL":              "C",
	}
	environment := make([]string, 0, len(os.Environ())+len(replacements))
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		if found {
			upperKey := strings.ToUpper(key)
			if _, replace := replacements[upperKey]; replace || redirectsGitRepository(upperKey) {
				continue
			}
		}
		environment = append(environment, entry)
	}
	for key, value := range replacements {
		environment = append(environment, key+"="+value)
	}
	return environment
}

// redirectsGitRepository identifies process environment that could make an
// operation-local command read or mutate a different repository. Authentication
// environment such as HOME and SSH_AUTH_SOCK remains untouched.
func redirectsGitRepository(key string) bool {
	switch key {
	case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE":
		return true
	default:
		return false
	}
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	originalLength := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining < len(p) {
		b.overflow = true
		p = p[:max(remaining, 0)]
	}
	_, _ = b.buffer.Write(p)
	return originalLength, nil
}

func (b *boundedBuffer) String() string {
	return b.buffer.String()
}
