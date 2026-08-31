package store

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockSharedAndExclusiveMatrix(t *testing.T) {
	s := initializedLockStore(t)
	firstShared := acquireTestLock(t, s.AcquireCanonicalShared)
	secondShared := acquireTestLock(t, s.AcquireCanonicalShared)
	assertLockBusy(t, s.AcquireCanonicalExclusive)
	if err := secondShared(); err != nil {
		t.Fatal(err)
	}
	if err := firstShared(); err != nil {
		t.Fatal(err)
	}

	exclusive := acquireTestLock(t, s.AcquireCanonicalExclusive)
	assertLockBusy(t, s.AcquireCanonicalShared)
	assertLockBusy(t, s.AcquireCanonicalExclusive)
	if err := exclusive(); err != nil {
		t.Fatal(err)
	}
}

func TestLockPathsAreIndependent(t *testing.T) {
	s := initializedLockStore(t)
	if got, want := s.SyncLockPath(), filepath.Join(s.Home, "sync", "operation.lock"); got != want {
		t.Fatalf("sync lock path = %q, want %q", got, want)
	}
	if got, want := s.CanonicalLockPath(), filepath.Join(s.Home, "canonical.lock"); got != want {
		t.Fatalf("canonical lock path = %q, want %q", got, want)
	}
	syncUnlock := acquireTestLock(t, s.AcquireSyncExclusive)
	canonicalUnlock := acquireTestLock(t, s.AcquireCanonicalExclusive)
	if err := canonicalUnlock(); err != nil {
		t.Fatal(err)
	}
	if err := syncUnlock(); err != nil {
		t.Fatal(err)
	}
}

func TestLockAcquisitionHonorsCancellation(t *testing.T) {
	s := initializedLockStore(t)
	unlock := acquireTestLock(t, s.AcquireSyncExclusive)
	defer func() { _ = unlock() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := s.AcquireSyncExclusive(ctx)
	if !errors.Is(err, ErrOperationBusy) {
		t.Fatalf("cancelled acquisition error = %v", err)
	}
	if time.Since(started) > 250*time.Millisecond {
		t.Fatal("cancelled acquisition did not stop promptly")
	}
}

func TestLockCrossProcessForcedExitReleases(t *testing.T) {
	s := initializedLockStore(t)
	child := startLockHolder(t, s.Home, "sync-exclusive")
	assertLockBusy(t, s.AcquireSyncShared)
	if err := child.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.cmd.Wait(); err == nil {
		t.Fatal("killed lock-holder process unexpectedly succeeded")
	}
	_ = child.stdin.Close()
	assertLockAvailable(t, s.AcquireSyncExclusive)
}

func TestLockCrossProcessNormalExitReleases(t *testing.T) {
	s := initializedLockStore(t)
	child := startLockHolder(t, s.Home, "sync-exclusive")
	assertLockBusy(t, s.AcquireSyncExclusive)
	if err := child.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.cmd.Wait(); err != nil {
		t.Fatalf("lock-holder process exit: %v", err)
	}
	assertLockAvailable(t, s.AcquireSyncExclusive)
}

func assertLockAvailable(t *testing.T, acquire lockAcquirer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	unlock, err := acquire(ctx)
	if err != nil {
		t.Fatalf("lock remained owned after process exit: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("CONTEXT_BAGGAGE_LOCK_HELPER") != "1" {
		return
	}
	s := New(os.Getenv("CONTEXT_BAGGAGE_LOCK_HOME"))
	var acquire func(context.Context) (func() error, error)
	switch os.Getenv("CONTEXT_BAGGAGE_LOCK_MODE") {
	case "sync-exclusive":
		acquire = s.AcquireSyncExclusive
	default:
		fmt.Fprintln(os.Stderr, "unknown lock helper mode")
		os.Exit(2)
	}
	unlock, err := acquire(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer func() { _ = unlock() }()
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		os.Exit(2)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

type lockAcquirer func(context.Context) (func() error, error)

func initializedLockStore(t *testing.T) Store {
	t.Helper()
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func acquireTestLock(t *testing.T, acquire lockAcquirer) func() error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	unlock, err := acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return unlock
}

func assertLockBusy(t *testing.T, acquire lockAcquirer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if _, err := acquire(ctx); !errors.Is(err, ErrOperationBusy) {
		t.Fatalf("contended acquisition error = %v", err)
	}
}

type lockHolder struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

func startLockHolder(t *testing.T, home, mode string) lockHolder {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(),
		"CONTEXT_BAGGAGE_LOCK_HELPER=1",
		"CONTEXT_BAGGAGE_LOCK_HOME="+home,
		"CONTEXT_BAGGAGE_LOCK_MODE="+mode,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("wait for lock helper: %v", err)
	}
	if ready != "ready\n" {
		t.Fatalf("lock helper readiness = %q", ready)
	}
	return lockHolder{cmd: cmd, stdin: stdin}
}
