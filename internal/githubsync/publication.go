package githubsync

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	portablesync "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

type publicationRuntime struct {
	random        io.Reader
	inspect       func(context.Context, GitRunner, Locator) (RepositorySnapshot, error)
	classify      func(context.Context, GitRunner, Locator) (PrivacyClassification, error)
	remoteURL     string
	beforePush    func()
	afterPrepare  func()
	push          func(context.Context, string, preparedPublication) error
	afterSnapshot func()
	observe       func(context.Context, GitRunner, string) (string, bool, error)
}

// PublishManaged publishes one immutable LOCAL snapshot while preserving the
// global sync-before-canonical lock order. It never holds canonical ownership
// during remote Git or HTTP operations.
func PublishManaged(ctx context.Context, s store.Store, git GitRunner) (string, error) {
	unlock, err := s.AcquireSyncExclusive(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = unlock() }()
	return publishManaged(ctx, s, git, publicationRuntime{})
}

func publishManaged(ctx context.Context, s store.Store, git GitRunner, runtime publicationRuntime) (string, error) {
	state, locator, err := readPublicationState(s)
	if err != nil {
		return "", err
	}
	root, err := os.MkdirTemp("", "ctx-bag-managed-publish-*")
	if err != nil {
		return "", ErrTransportUnavailable
	}
	defer func() { _ = os.RemoveAll(root) }()
	localDir := filepath.Join(root, "local")
	localHash, err := portablesync.BuildPushSnapshotBounded(s, localDir, maxMaterializedBlobBytes)
	if err != nil {
		if errors.Is(err, portablesync.ErrPortableExportLimit) {
			return "", ErrResourceLimitExceeded
		}
		return "", err
	}
	if runtime.afterSnapshot != nil {
		runtime.afterSnapshot()
	}
	if err := validateLocalPublication(root, localDir, localHash, git.temporaryByteLimit()); err != nil {
		return "", err
	}
	inspect := runtime.inspector()
	remote, err := inspect(ctx, git, locator)
	if err != nil {
		return "", err
	}
	prepared, remoteURL, err := prepareManagedPublication(ctx, git, runtime, locator, state, remote, root, localDir, localHash)
	if err != nil {
		return "", err
	}
	if prepared.expectedID == "" {
		if err := runtime.requireStillEmpty(ctx, git, remoteURL); err != nil {
			return "", err
		}
	}
	if runtime.beforePush != nil {
		runtime.beforePush()
	}
	push := runtime.pusher(git)
	_ = push(ctx, root, prepared)
	confirmed, err := inspect(ctx, git, locator)
	if err := confirmPublication(prepared, confirmed, err); err != nil {
		if errors.Is(err, ErrTransportUnavailable) {
			return "", ErrPublicationAmbiguous
		}
		return "", err
	}
	return persistConfirmedPublication(s, state, prepared, locator.identity)
}

func prepareManagedPublication(ctx context.Context, git GitRunner, runtime publicationRuntime, locator Locator, state store.SyncState, remote RepositorySnapshot, root, localDir, localHash string) (preparedPublication, string, error) {
	destinationID, parentID, err := publicationIdentity(state, remote, runtime.random)
	if err != nil {
		return preparedPublication{}, "", err
	}
	if portablesync.PushWouldConflict(state, localHash, remote.PortableHash) {
		return preparedPublication{}, "", ErrPublicationConflict
	}
	remoteURL := runtime.remoteURL
	if remoteURL == "" {
		remoteURL = locator.url
	}
	prepared, err := git.preparePublication(ctx, root, localDir, remoteURL, parentID, destinationID, localHash)
	if err != nil {
		return preparedPublication{}, "", err
	}
	if runtime.afterPrepare != nil {
		runtime.afterPrepare()
	}
	if err := runtime.authorize(ctx, git, locator); err != nil {
		return preparedPublication{}, "", err
	}
	return prepared, remoteURL, nil
}

func validateLocalPublication(root, localDir, localHash string, temporaryLimit int64) error {
	if err := validatePublicationSnapshot(localDir, localHash); err != nil {
		return err
	}
	if exceededTemporaryLimit(root, temporaryLimit) {
		return ErrResourceLimitExceeded
	}
	return nil
}

func (runtime publicationRuntime) inspector() func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
	if runtime.inspect != nil {
		return runtime.inspect
	}
	return InspectRepository
}

func (runtime publicationRuntime) authorize(ctx context.Context, git GitRunner, locator Locator) error {
	classify := runtime.classify
	if classify == nil {
		classify = ClassifyPrivacy
	}
	privacy, err := classify(ctx, git, locator)
	if err != nil || privacy != VerifiedNonPublic {
		return ErrPrivacyRefused
	}
	return nil
}

func (runtime publicationRuntime) pusher(git GitRunner) func(context.Context, string, preparedPublication) error {
	if runtime.push != nil {
		return runtime.push
	}
	return git.pushPrepared
}

func (runtime publicationRuntime) requireStillEmpty(ctx context.Context, git GitRunner, remoteURL string) error {
	observe := runtime.observe
	if observe == nil {
		observe = func(ctx context.Context, git GitRunner, remoteURL string) (string, bool, error) {
			return git.observeManagedRef(ctx, remoteURL)
		}
	}
	_, empty, err := observe(ctx, git, remoteURL)
	if err != nil {
		return err
	}
	if !empty {
		return ErrPublicationConflict
	}
	return nil
}

func readPublicationState(s store.Store) (store.SyncState, Locator, error) {
	state, err := s.ReadSync()
	if err != nil || state.DestinationType != store.DestinationGitHub {
		return store.SyncState{}, Locator{}, ErrDestinationMismatch
	}
	locator, err := ParseLocator(state.GitHubLocator)
	if err != nil || state.GitHubRepository != locator.identity {
		return store.SyncState{}, Locator{}, ErrDestinationMismatch
	}
	return state, locator, nil
}

func publicationIdentity(state store.SyncState, remote RepositorySnapshot, random io.Reader) (string, string, error) {
	switch remote.State {
	case RepositoryEmpty:
		if state.ManagedDestinationID != "" {
			return "", "", ErrManagedDestinationLost
		}
		if random == nil {
			random = secureRandom()
		}
		destinationID, err := generateManagedDestinationID(random)
		return destinationID, "", err
	case RepositoryInitialized:
		if err := ValidateExpectedDestination(remote, state.ManagedDestinationID); err != nil {
			return "", "", err
		}
		return remote.ManagedDestinationID, remote.CommitID, nil
	default:
		return "", "", ErrRepositoryIncompatible
	}
}

func confirmPublication(prepared preparedPublication, remote RepositorySnapshot, observationErr error) error {
	if observationErr != nil {
		if errors.Is(observationErr, ErrRepositoryIncompatible) {
			return observationErr
		}
		return ErrTransportUnavailable
	}
	if remote.State != RepositoryInitialized || remote.CommitID != prepared.commitID {
		return ErrPublicationConflict
	}
	if remote.ManagedDestinationID != prepared.destinationID || remote.PortableHash != prepared.portableHash {
		return ErrRepositoryIncompatible
	}
	return nil
}

func persistConfirmedPublication(s store.Store, state store.SyncState, prepared preparedPublication, repositoryIdentity string) (string, error) {
	state.ManagedDestinationID = prepared.destinationID
	state.LastPush, state.LastPushHash = store.Now(), prepared.portableHash
	state.LastObservedRemoteHash, state.LastRefresh = prepared.portableHash, ""
	if prepared.portableHash != "" {
		state.LastRefresh = store.Now()
	}
	state.PrivacyClassification = store.PrivacyVerifiedNonPublic
	state.PrivacyCheckedAt, state.PrivacyRepositoryIdentity = store.Now(), repositoryIdentity
	if err := portablesync.BindBaseToActiveDestination(&state, prepared.portableHash); err != nil {
		return "", err
	}
	if err := s.WriteSync(state); err != nil {
		return "", err
	}
	return prepared.portableHash, nil
}
