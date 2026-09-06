package app

import (
	"context"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

var (
	managedGitDiscovery = githubsync.DiscoverGit
	managedConfigure    = githubsync.ConfigureManaged
	managedPublish      = githubsync.PublishManaged
	managedPull         = githubsync.PullManaged
	managedRecover      = githubsync.RecoverManagedPull
	managedDiscovery    = githubsync.DiscoverManagedWorkspaces
	managedClassify     = githubsync.ClassifyPrivacy
	managedInspect      = githubsync.InspectRepository
)

type managedDoctorObservation struct {
	privacy  githubsync.PrivacyClassification
	snapshot githubsync.RepositorySnapshot
}

func inspectManagedDoctor(ctx context.Context, git githubsync.GitRunner, locator githubsync.Locator) (managedDoctorObservation, error) {
	privacy, err := managedClassify(ctx, git, locator)
	if err != nil && privacy == githubsync.Unverifiable {
		return managedDoctorObservation{privacy: privacy}, err
	}
	snapshot, err := managedInspect(ctx, git, locator)
	return managedDoctorObservation{privacy: privacy, snapshot: snapshot}, err
}

func configuredManagedLocator(state store.SyncState) (githubsync.Locator, error) {
	locator, err := githubsync.ParseLocator(state.GitHubLocator)
	if err != nil || locator.Identity() != state.GitHubRepository {
		return githubsync.Locator{}, githubsync.ErrDestinationMismatch
	}
	return locator, nil
}
