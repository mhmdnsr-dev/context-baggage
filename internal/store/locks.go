package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

const (
	stateLockWaitTimeout = time.Second
	stateLockRetryDelay  = 10 * time.Millisecond
)

// ErrOperationBusy reports that another Context Baggage process retained a
// conflicting state lock for the full bounded acquisition period.
var ErrOperationBusy = errors.New("another Context Baggage operation is in progress")

// SyncLockPath returns the machine-local sync-operation lock path.
func (s Store) SyncLockPath() string {
	return filepath.Join(s.Home, "sync", "operation.lock")
}

// CanonicalLockPath returns the machine-local canonical-state lock path.
func (s Store) CanonicalLockPath() string {
	return filepath.Join(s.Home, "canonical.lock")
}

// AcquireSyncExclusive obtains exclusive ownership of sync configuration,
// bookkeeping, and destination operations.
func (s Store) AcquireSyncExclusive(ctx context.Context) (func() error, error) {
	return acquireStateLock(ctx, s.SyncLockPath(), false)
}

// AcquireSyncShared obtains shared ownership for a coherent read-only sync
// configuration or filesystem-destination observation.
func (s Store) AcquireSyncShared(ctx context.Context) (func() error, error) {
	return acquireStateLock(ctx, s.SyncLockPath(), true)
}

// AcquireCanonicalExclusive obtains exclusive ownership of canonical workspace
// and task state for one complete mutation use case.
func (s Store) AcquireCanonicalExclusive(ctx context.Context) (func() error, error) {
	return acquireStateLock(ctx, s.CanonicalLockPath(), false)
}

// AcquireCanonicalShared obtains shared ownership for a coherent canonical
// snapshot such as a portable Push export.
func (s Store) AcquireCanonicalShared(ctx context.Context) (func() error, error) {
	return acquireStateLock(ctx, s.CanonicalLockPath(), true)
}

// acquireStateLock waits for at most the internal bound and returns an unlock
// function only after ownership is established. Lock files may persist; OS
// ownership, not file existence, is the synchronization authority.
func acquireStateLock(ctx context.Context, path string, shared bool) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("prepare Context Baggage state lock: %w", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, stateLockWaitTimeout)
	defer cancel()

	fileLock := flock.New(path)
	var (
		locked bool
		err    error
	)
	if shared {
		locked, err = fileLock.TryRLockContext(waitCtx, stateLockRetryDelay)
	} else {
		locked, err = fileLock.TryLockContext(waitCtx, stateLockRetryDelay)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, ErrOperationBusy
	}
	if err != nil {
		return nil, fmt.Errorf("acquire Context Baggage state lock: %w", err)
	}
	if !locked {
		return nil, ErrOperationBusy
	}
	return fileLock.Unlock, nil
}
