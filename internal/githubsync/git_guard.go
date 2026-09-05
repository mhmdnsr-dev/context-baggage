package githubsync

import (
	"context"
	"io"
	"os/exec"
	"time"
)

const temporaryGuardInterval = 5 * time.Millisecond

// runNetworkGuarded executes a quiet network operation while actively
// monitoring its operation-local disk footprint.
func (g GitRunner) runNetworkGuarded(ctx context.Context, timeout time.Duration, dir, guardRoot string, limit int64, args ...string) error {
	stdout := newBoundedBuffer(maxGitOutput)
	err := g.runNetworkStreamGuarded(ctx, timeout, dir, guardRoot, limit, stdout, args...)
	if err != nil {
		return err
	}
	if stdout.overflow {
		return ErrTransportUnavailable
	}
	return nil
}

func (g GitRunner) runLocalGuarded(ctx context.Context, timeout time.Duration, dir, guardRoot string, args ...string) (gitCommandResult, error) {
	stdout := newBoundedBuffer(maxGitOutput)
	err := g.runStreamGuarded(ctx, timeout, dir, guardRoot, g.temporaryByteLimit(), stdout, args...)
	if err != nil || stdout.overflow {
		return gitCommandResult{}, errOrTransport(err)
	}
	return gitCommandResult{stdout: stdout.String()}, nil
}

func errOrTransport(err error) error {
	if err != nil {
		return err
	}
	return ErrTransportUnavailable
}

// runNetworkStreamGuarded preserves redirect hardening while allowing a
// bounded consumer and active temporary-storage guard to observe the command.
func (g GitRunner) runNetworkStreamGuarded(ctx context.Context, timeout time.Duration, dir, guardRoot string, limit int64, stdout io.Writer, args ...string) error {
	networkArgs := []string{"-c", "http.followRedirects=false"}
	networkArgs = append(networkArgs, args...)
	return g.runStreamGuarded(ctx, timeout, dir, guardRoot, limit, stdout, networkArgs...)
}

func (g GitRunner) runStreamGuarded(ctx context.Context, timeout time.Duration, dir, guardRoot string, limit int64, stdout io.Writer, args ...string) error {
	if g.executable == "" {
		return ErrGitUnavailable
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, g.executable, g.commandArgs(args)...)
	command.Dir = dir
	command.Env = gitEnvironment()
	command.Stdout = stdout
	stderr := newBoundedBuffer(maxGitOutput)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return ErrTransportUnavailable
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(temporaryGuardInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if exceededTemporaryLimit(guardRoot, limit) {
				return ErrResourceLimitExceeded
			}
			if runCtx.Err() != nil || err != nil || stderr.overflow {
				return ErrTransportUnavailable
			}
			return nil
		case <-ticker.C:
			if exceededTemporaryLimit(guardRoot, limit) {
				_ = command.Process.Kill()
				<-done
				return ErrResourceLimitExceeded
			}
		case <-runCtx.Done():
			_ = command.Process.Kill()
			<-done
			return ErrTransportUnavailable
		}
	}
}

func exceededTemporaryLimit(root string, limit int64) bool {
	used, err := temporaryUsage(root)
	return err != nil || used > limit
}
