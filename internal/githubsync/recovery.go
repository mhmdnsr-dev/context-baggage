package githubsync

import (
	"context"
	"errors"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

// ErrRecoveryNotPending reports a recovery attempt with no record to resolve.
var ErrRecoveryNotPending = errors.New("no managed pull recovery is pending")

// recoveryRuntime supplies deterministic seams for the recovery orchestration.
type recoveryRuntime struct {
	materialize func(context.Context, GitRunner, Locator, string) (RepositorySnapshot, string, func(), error)
	apply       func(store.Store, string) error
	eligible    func(store.Store) (string, error)
	persist     func(store.Store, store.SyncState, string) (string, error)
}

func (r recoveryRuntime) materializer() func(context.Context, GitRunner, Locator, string) (RepositorySnapshot, string, func(), error) {
	if r.materialize != nil {
		return r.materialize
	}
	return materializeExactCommit
}

func (r recoveryRuntime) applier() func(store.Store, string) error {
	if r.apply != nil {
		return r.apply
	}
	return portablesync.ApplyPortableSnapshot
}

func (r recoveryRuntime) hasher() func(store.Store) (string, error) {
	if r.eligible != nil {
		return r.eligible
	}
	return portablesync.EligibleHash
}

func (r recoveryRuntime) persister() func(store.Store, store.SyncState, string) (string, error) {
	if r.persist != nil {
		return r.persist
	}
	return persistManagedPull
}

// RecoverManagedPull resolves an interrupted managed Pull. It is deliberately
// conservative: only the recorded pre-state and the recorded target state are
// recoverable; any other LOCAL state is refused.
func RecoverManagedPull(ctx context.Context, s store.Store, git GitRunner) (string, error) {
	unlock, err := s.AcquireSyncExclusive(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	return recoverManagedPull(ctx, s, git, recoveryRuntime{})
}

func recoverManagedPull(ctx context.Context, s store.Store, git GitRunner, runtime recoveryRuntime) (string, error) {
	record, err := s.ReadPullRecovery()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrRecoveryNotPending
		}
		return "", err
	}
	state, locator, err := readPublicationState(s)
	if err != nil {
		return "", err
	}
	if err := validateRecoveryBinding(state, record); err != nil {
		return "", err
	}
	canonicalUnlock, err := s.AcquireCanonicalExclusive(ctx)
	if err != nil {
		return "", err
	}
	localHash, err := runtime.hasher()(s)
	if err != nil {
		_ = canonicalUnlock()
		return "", err
	}
	switch localHash {
	case record.PreLocalHash:
		if err := canonicalUnlock(); err != nil {
			return "", err
		}
		return recoverRetryExact(ctx, s, git, runtime, locator, state, record)
	case record.TargetPortableHash:
		defer func() { _ = canonicalUnlock() }()
		return runtime.persister()(s, state, record.TargetPortableHash)
	default:
		_ = canonicalUnlock()
		return "", ErrRecoveryAmbiguous
	}
}

// recoverRetryExact reapplies the recorded immutable target commit. It never
// substitutes the remote's current head, which may have moved. It releases
// canonical ownership during the remote refetch, then re-acquires it under a
// fresh LOCAL verification before any canonical mutation.
func recoverRetryExact(ctx context.Context, s store.Store, git GitRunner, runtime recoveryRuntime, locator Locator, state store.SyncState, record store.PullRecovery) (string, error) {
	snapshot, portableDir, cleanup, err := runtime.materializer()(ctx, git, locator, record.TargetCommitID)
	if err != nil {
		return "", err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if snapshot.State != RepositoryInitialized || snapshot.ManagedDestinationID != record.DestinationID {
		return "", ErrRepositoryIncompatible
	}
	if snapshot.PortableHash != record.TargetPortableHash {
		return "", ErrRepositoryIncompatible
	}
	canonicalUnlock, err := s.AcquireCanonicalExclusive(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = canonicalUnlock() }()
	if err := portablesync.PreflightPortable(s, portableDir); err != nil {
		return "", err
	}
	currentHash, err := runtime.hasher()(s)
	if err != nil {
		return "", err
	}
	if currentHash != record.PreLocalHash {
		return "", ErrRecoveryAmbiguous
	}
	if err := runtime.applier()(s, portableDir); err != nil {
		return "", err
	}
	appliedHash, err := runtime.hasher()(s)
	if err != nil {
		return "", err
	}
	if appliedHash != record.TargetPortableHash {
		return "", ErrApplyMismatch
	}
	return runtime.persister()(s, state, record.TargetPortableHash)
}

// validateRecoveryBinding confirms the recovery record still belongs to the
// active managed destination.
func validateRecoveryBinding(state store.SyncState, record store.PullRecovery) error {
	if state.DestinationType != store.DestinationGitHub ||
		state.ManagedDestinationID != record.DestinationID ||
		state.GitHubRepository != record.RepositoryIdentity {
		return ErrRecoveryBindingMismatch
	}
	return nil
}

// materializeExactCommit fetches and materializes one exact recorded commit. It
// is the recovery-only path that pins the target rather than the remote ref.
func materializeExactCommit(ctx context.Context, git GitRunner, locator Locator, commitID string) (RepositorySnapshot, string, func(), error) {
	if !locator.valid() {
		return RepositorySnapshot{}, "", nil, ErrInvalidLocator
	}
	if err := git.VerifyTargetBinding(ctx, locator); err != nil {
		return RepositorySnapshot{}, "", nil, err
	}
	root, err := os.MkdirTemp("", "ctx-bag-managed-recover-*")
	if err != nil {
		return RepositorySnapshot{}, "", nil, ErrTransportUnavailable
	}
	snapshot, portableDir, err := git.inspectAndMaterialize(ctx, locator.url, locator.identity, commitID, root)
	if err != nil {
		_ = os.RemoveAll(root)
		return RepositorySnapshot{}, "", nil, err
	}
	return snapshot, portableDir, func() { _ = os.RemoveAll(root) }, nil
}
