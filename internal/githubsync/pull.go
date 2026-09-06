package githubsync

import (
	"context"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

// pullRuntime supplies deterministic seams for the managed Pull orchestration.
type pullRuntime struct {
	materialize   func(context.Context, GitRunner, Locator) (RepositorySnapshot, string, func(), error)
	writeRecovery func(store.Store, store.PullRecovery) error
	apply         func(store.Store, string) error
	eligibleHash  func(store.Store) (string, error)
	persist       func(store.Store, store.SyncState, string) (string, error)
}

func (r pullRuntime) materializer() func(context.Context, GitRunner, Locator) (RepositorySnapshot, string, func(), error) {
	if r.materialize != nil {
		return r.materialize
	}
	return materializeManagedSnapshot
}

func (r pullRuntime) recoveryWriter() func(store.Store, store.PullRecovery) error {
	if r.writeRecovery != nil {
		return r.writeRecovery
	}
	return func(s store.Store, record store.PullRecovery) error { return s.WritePullRecovery(record) }
}

func (r pullRuntime) applier() func(store.Store, string) error {
	if r.apply != nil {
		return r.apply
	}
	return portablesync.ApplyPortableSnapshot
}

func (r pullRuntime) hasher() func(store.Store) (string, error) {
	if r.eligibleHash != nil {
		return r.eligibleHash
	}
	return portablesync.EligibleHash
}

func (r pullRuntime) persister() func(store.Store, store.SyncState, string) (string, error) {
	if r.persist != nil {
		return r.persist
	}
	return persistManagedPull
}

// PullManaged pulls one immutable managed REMOTE snapshot into the canonical
// store while preserving the global sync-before-canonical lock order.
func PullManaged(ctx context.Context, s store.Store, git GitRunner) (string, error) {
	unlock, err := s.AcquireSyncExclusive(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	return pullManaged(ctx, s, git, pullRuntime{})
}

func pullManaged(ctx context.Context, s store.Store, git GitRunner, runtime pullRuntime) (string, error) {
	if exists, err := s.PullRecoveryExists(); err != nil {
		return "", err
	} else if exists {
		return "", ErrRecoveryRequired
	}
	state, locator, err := readPublicationState(s)
	if err != nil {
		return "", err
	}
	snapshot, portableDir, cleanup, err := runtime.materializer()(ctx, git, locator)
	if err != nil {
		return "", err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err := validateManagedDestinationRead(state, snapshot); err != nil {
		return "", err
	}
	canonicalUnlock, err := s.AcquireCanonicalExclusive(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = canonicalUnlock() }()
	localHash, err := runtime.hasher()(s)
	if err != nil {
		return "", err
	}
	localNonEmpty, err := portablesync.HasEligibleWorkspaces(s)
	if err != nil {
		return "", err
	}
	if portablesync.PullWouldConflict(state, localHash, snapshot.PortableHash, localNonEmpty) {
		return "", ErrPullConflict
	}
	if err := portablesync.PreflightPortable(s, portableDir); err != nil {
		return "", err
	}
	if localHash == snapshot.PortableHash {
		return runtime.persister()(s, state, snapshot.PortableHash)
	}
	record := store.PullRecovery{
		Format:             store.PullRecoveryFormatVersion,
		DestinationType:    store.DestinationGitHub,
		DestinationID:      state.ManagedDestinationID,
		RepositoryIdentity: state.GitHubRepository,
		TargetCommitID:     snapshot.CommitID,
		PreLocalHash:       localHash,
		TargetPortableHash: snapshot.PortableHash,
	}
	if err := runtime.recoveryWriter()(s, record); err != nil {
		return "", err
	}
	if err := runtime.applier()(s, portableDir); err != nil {
		return "", err
	}
	appliedHash, err := runtime.hasher()(s)
	if err != nil {
		return "", err
	}
	if appliedHash != snapshot.PortableHash {
		return "", ErrApplyMismatch
	}
	return runtime.persister()(s, state, snapshot.PortableHash)
}

// persistManagedPull advances BASE and bookkeeping, then clears the recovery
// record only after BASE has been successfully persisted.
func persistManagedPull(s store.Store, state store.SyncState, hash string) (string, error) {
	state.LastPull, state.LastPullHash = store.Now(), hash
	state.LastObservedRemoteHash = hash
	state.LastRefresh = store.Now()
	if err := portablesync.BindBaseToActiveDestination(&state, hash); err != nil {
		return "", err
	}
	if err := s.WriteSync(state); err != nil {
		return "", err
	}
	if err := s.RemovePullRecovery(); err != nil {
		return "", err
	}
	return hash, nil
}

// validateManagedDestinationRead enforces the managed destination identity
// contract for a read-only operation. It never adopts or re-claims.
func validateManagedDestinationRead(state store.SyncState, snapshot RepositorySnapshot) error {
	switch snapshot.State {
	case RepositoryEmpty:
		if state.ManagedDestinationID != "" {
			return ErrManagedDestinationLost
		}
		return ErrRepositoryIncompatible
	case RepositoryInitialized:
		if state.ManagedDestinationID == "" {
			return ErrManagedDestinationAdoptionRequired
		}
		if state.ManagedDestinationID != snapshot.ManagedDestinationID {
			return ErrManagedDestinationMismatch
		}
		return nil
	default:
		return ErrRepositoryIncompatible
	}
}

// materializeManagedSnapshot observes the managed ref once, fetches the exact
// observed commit, and materializes its portable snapshot. It returns the
// snapshot, the materialized portable directory, and a cleanup function.
func materializeManagedSnapshot(ctx context.Context, git GitRunner, locator Locator) (RepositorySnapshot, string, func(), error) {
	if !locator.valid() {
		return RepositorySnapshot{}, "", nil, ErrInvalidLocator
	}
	if err := git.VerifyTargetBinding(ctx, locator); err != nil {
		return RepositorySnapshot{}, "", nil, err
	}
	commitID, empty, err := git.observeManagedRef(ctx, locator.url)
	if err != nil {
		return RepositorySnapshot{}, "", nil, err
	}
	if empty {
		return RepositorySnapshot{RepositoryIdentity: locator.identity, State: RepositoryEmpty}, "", nil, nil
	}
	root, err := os.MkdirTemp("", "ctx-bag-managed-pull-*")
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
