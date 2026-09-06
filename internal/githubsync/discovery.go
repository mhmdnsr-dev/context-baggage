package githubsync

import (
	"context"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

// discoveryRuntime supplies deterministic seams for managed discovery.
type discoveryRuntime struct {
	materialize func(context.Context, GitRunner, Locator) (RepositorySnapshot, string, func(), error)
}

func (r discoveryRuntime) materializer() func(context.Context, GitRunner, Locator) (RepositorySnapshot, string, func(), error) {
	if r.materialize != nil {
		return r.materialize
	}
	return materializeManagedSnapshot
}

// DiscoverManagedWorkspaces lists attachable portable workspaces from a fresh
// immutable REMOTE snapshot. It never mutates canonical state or BASE and never
// adopts or re-claims a destination identity.
func DiscoverManagedWorkspaces(ctx context.Context, s store.Store, git GitRunner) ([]store.PortableWorkspace, error) {
	unlock, err := s.AcquireSyncExclusive(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()
	return discoverManagedWorkspaces(ctx, s, git, discoveryRuntime{})
}

func discoverManagedWorkspaces(ctx context.Context, s store.Store, git GitRunner, runtime discoveryRuntime) ([]store.PortableWorkspace, error) {
	if exists, err := s.PullRecoveryExists(); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrRecoveryRequired
	}
	state, locator, err := readPublicationState(s)
	if err != nil {
		return nil, err
	}
	snapshot, portableDir, cleanup, err := runtime.materializer()(ctx, git, locator)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err := validateManagedDestinationRead(state, snapshot); err != nil {
		return nil, err
	}
	workspaces, _, err := portablesync.ListPortableWorkspacesFromRoot(portableDir)
	if err != nil {
		return nil, err
	}
	state.LastObservedRemoteHash = snapshot.PortableHash
	state.LastRefresh = store.Now()
	if err := s.WriteSync(state); err != nil {
		return nil, err
	}
	return workspaces, nil
}
