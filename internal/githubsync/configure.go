package githubsync

import (
	"context"
	"errors"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

type configureRuntime struct {
	classify func(context.Context, GitRunner, Locator) (PrivacyClassification, error)
	inspect  func(context.Context, GitRunner, Locator) (RepositorySnapshot, error)
}

// ConfigureManaged validates a GitHub candidate completely before changing
// local configuration. An initialized marker is adopted only through this
// explicit operation; ordinary remote reads never adopt it.
func ConfigureManaged(ctx context.Context, s store.Store, git GitRunner, raw string, allowReplacement bool) (store.SyncState, error) {
	unlock, err := s.AcquireSyncExclusive(ctx)
	if err != nil {
		return store.SyncState{}, err
	}
	defer func() { _ = unlock() }()
	return configureManaged(ctx, s, git, raw, allowReplacement, configureRuntime{})
}

func configureManaged(ctx context.Context, s store.Store, git GitRunner, raw string, allowReplacement bool, runtime configureRuntime) (store.SyncState, error) {
	if exists, err := s.PullRecoveryExists(); err != nil {
		return store.SyncState{}, err
	} else if exists {
		return store.SyncState{}, ErrRecoveryRequired
	}
	locator, err := ParseLocator(raw)
	if err != nil {
		return store.SyncState{}, err
	}
	classify := runtime.classify
	if classify == nil {
		classify = ClassifyPrivacy
	}
	privacy, err := classify(ctx, git, locator)
	if err != nil || privacy == Unverifiable {
		return store.SyncState{}, ErrPrivacyUnverifiable
	}
	if privacy == VerifiedPublic {
		return store.SyncState{}, ErrRepositoryPublic
	}
	if privacy != VerifiedNonPublic {
		return store.SyncState{}, ErrPrivacyUnverifiable
	}
	inspect := runtime.inspect
	if inspect == nil {
		inspect = InspectRepository
	}
	snapshot, err := inspect(ctx, git, locator)
	if err != nil {
		return store.SyncState{}, err
	}
	old, err := s.ReadSync()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return store.SyncState{}, err
	}
	next, err := managedTransition(old, err == nil, locator, snapshot, allowReplacement)
	if err != nil {
		return store.SyncState{}, err
	}
	next.PrivacyClassification = store.PrivacyVerifiedNonPublic
	next.PrivacyCheckedAt = store.Now()
	next.PrivacyRepositoryIdentity = locator.Identity()
	if err := s.WriteSync(next); err != nil {
		return store.SyncState{}, err
	}
	return next, nil
}

// managedTransition decides logical sameness before mutation. Once claimed,
// marker identity outranks repository slug so an explicit rename can preserve
// BASE while a reused slug with a different marker cannot.
func managedTransition(old store.SyncState, hasOld bool, locator Locator, remote RepositorySnapshot, allowReplacement bool) (store.SyncState, error) {
	candidateID := ""
	if remote.State == RepositoryInitialized {
		candidateID = remote.ManagedDestinationID
	} else if remote.State != RepositoryEmpty {
		return store.SyncState{}, ErrRepositoryIncompatible
	}
	if !hasOld {
		return newManagedState(locator, candidateID), nil
	}
	if sameManagedDestination(old, locator.Identity(), candidateID, remote.State) {
		old.FormatVersion = store.SyncStateFormatVersion
		old.DestinationType = store.DestinationGitHub
		old.Folder = ""
		old.GitHubLocator = locator.TransportURL()
		old.GitHubRepository = locator.Identity()
		old.ManagedDestinationID = candidateID
		return old, nil
	}
	if !allowReplacement {
		if old.DestinationType == store.DestinationGitHub && old.ManagedDestinationID != "" && remote.State == RepositoryEmpty {
			return store.SyncState{}, ErrManagedDestinationLost
		}
		return store.SyncState{}, ErrManagedDestinationMismatch
	}
	return newManagedState(locator, candidateID), nil
}

func sameManagedDestination(old store.SyncState, repositoryIdentity, candidateID string, state RepositoryState) bool {
	if old.DestinationType != store.DestinationGitHub {
		return false
	}
	if old.ManagedDestinationID != "" {
		return state == RepositoryInitialized && old.ManagedDestinationID == candidateID
	}
	return old.GitHubRepository == repositoryIdentity
}

func newManagedState(locator Locator, destinationID string) store.SyncState {
	return store.SyncState{
		FormatVersion:        store.SyncStateFormatVersion,
		DestinationType:      store.DestinationGitHub,
		GitHubLocator:        locator.TransportURL(),
		GitHubRepository:     locator.Identity(),
		ManagedDestinationID: destinationID,
	}
}
