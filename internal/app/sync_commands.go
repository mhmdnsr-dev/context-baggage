package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

type syncInitRequest struct {
	github      bool
	target      string
	replacement bool
}

func runSync(s store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("sync subcommand required\nrun: ctx-bag sync status")
	}
	switch args[0] {
	case "init":
		return runSyncInit(s, args[1:], out)
	case "status":
		if len(args) != 1 {
			return errors.New("sync status accepts no arguments")
		}
		return runSyncStatus(s, out)
	case "upgrade":
		return runSyncUpgrade(s, args[1:], out)
	case "push":
		return runSyncPush(s, args[1:], out)
	case "pull":
		return runSyncPull(s, args[1:], out)
	case "recover":
		return runSyncRecover(s, args[1:], out)
	default:
		return fmt.Errorf("unknown sync subcommand: %s", args[0])
	}
}

func runSyncInit(s store.Store, args []string, out io.Writer) error {
	request, err := parseSyncInit(args)
	if err != nil {
		return err
	}
	if !request.github {
		state, err := syncer.InitWithReplacement(s, request.target, request.replacement)
		if err != nil {
			return mapManagedError(err)
		}
		return writeOutput(out, "Sync configured\nDestination: filesystem\nFolder: %s\n", state.Folder)
	}
	git, err := managedGitDiscovery()
	if err != nil {
		return mapManagedError(err)
	}
	state, err := managedConfigure(context.Background(), s, git, request.target, request.replacement)
	if err != nil {
		return mapManagedError(err)
	}
	managed := state.ManagedDestinationID
	if managed == "" {
		managed = "unclaimed"
	}
	return writeOutput(out, "Sync configured\nDestination: github\nRepository: %s\nManaged destination: %s\n", state.GitHubRepository, managed)
}

// parseSyncInit preserves the historical literal folder named "github". The
// managed form exists only when a second positional repository locator follows.
func parseSyncInit(args []string) (syncInitRequest, error) {
	if len(args) == 0 {
		return syncInitRequest{}, errors.New("sync destination is required\nrun: ctx-bag sync init <folder>")
	}
	request := syncInitRequest{}
	positionals := make([]string, 0, 2)
	for _, arg := range args {
		if arg == "--replace" {
			if request.replacement {
				return syncInitRequest{}, errors.New("duplicate --replace option")
			}
			request.replacement = true
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			return syncInitRequest{}, fmt.Errorf("unknown sync init option: %s", arg)
		}
		positionals = append(positionals, arg)
	}
	if len(positionals) == 1 {
		request.target = positionals[0]
		return request, nil
	}
	if len(positionals) == 2 && positionals[0] == "github" {
		request.github, request.target = true, positionals[1]
		return request, nil
	}
	return syncInitRequest{}, errors.New("invalid sync init arguments\nrun: ctx-bag man sync init")
}

func runSyncPush(s store.Store, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("sync push accepts no arguments")
	}
	state, err := readSyncConfig(s)
	if err != nil {
		return err
	}
	var hash string
	if state.DestinationType == store.DestinationGitHub {
		git, discoverErr := managedGitDiscovery()
		if discoverErr != nil {
			return mapManagedError(discoverErr)
		}
		hash, err = managedPublish(context.Background(), s, git)
	} else {
		hash, err = syncer.Push(s)
	}
	if err != nil {
		return mapManagedError(err)
	}
	return writeOutput(out, "Sync push complete\nHash: %s\n", hash)
}

func runSyncPull(s store.Store, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("sync pull accepts no arguments")
	}
	state, err := readSyncConfig(s)
	if err != nil {
		return err
	}
	var hash string
	if state.DestinationType == store.DestinationGitHub {
		git, discoverErr := managedGitDiscovery()
		if discoverErr != nil {
			return mapManagedError(discoverErr)
		}
		hash, err = managedPull(context.Background(), s, git)
	} else {
		hash, err = syncer.Pull(s)
	}
	if err != nil {
		return mapManagedError(err)
	}
	return writeOutput(out, "Sync pull complete\nHash: %s\n", hash)
}

func runSyncRecover(s store.Store, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("sync recover accepts no arguments or options")
	}
	exists, err := s.PullRecoveryExists()
	if err != nil {
		return mapManagedError(err)
	}
	if !exists {
		return writeOutput(out, "No managed pull recovery is pending.\n")
	}
	if _, err := s.ReadPullRecovery(); err != nil {
		return mapManagedError(err)
	}
	git, err := managedGitDiscovery()
	if err != nil {
		return mapManagedError(err)
	}
	hash, err := managedRecover(context.Background(), s, git)
	if errors.Is(err, githubsync.ErrRecoveryNotPending) {
		return writeOutput(out, "No managed pull recovery is pending.\n")
	}
	if err != nil {
		return mapManagedError(err)
	}
	return writeOutput(out, "Managed pull recovery complete\nHash: %s\n", hash)
}

func runSyncUpgrade(s store.Store, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("sync upgrade accepts no arguments")
	}
	state, err := readSyncConfig(s)
	if err != nil {
		return err
	}
	if state.DestinationType == store.DestinationGitHub {
		return errors.New("sync upgrade is not applicable to managed GitHub destinations")
	}
	if err := syncer.SyncUpgrade(s); err != nil {
		return err
	}
	return writeOutput(out, "Shared state upgraded to v2\nLegacy state preserved\nrun: ctx-bag sync pull\nUpgrade other devices sharing this folder.\n")
}

// mapManagedError renders security-sensitive outcomes without Git output,
// temporary paths, or credential-helper diagnostics.
func mapManagedError(err error) error {
	switch {
	case errors.Is(err, githubsync.ErrGitUnavailable):
		return errors.New("git is not installed")
	case errors.Is(err, githubsync.ErrTransportUnavailable):
		return errors.New("repository missing or inaccessible")
	case errors.Is(err, githubsync.ErrPrivacyRefused):
		return errors.New("repository privacy could not be verified; managed sync requires a non-public repository")
	case errors.Is(err, githubsync.ErrRepositoryPublic):
		return errors.New("repository is public; managed sync requires a non-public repository")
	case errors.Is(err, githubsync.ErrPrivacyUnverifiable), errors.Is(err, githubsync.ErrNetworkUnavailable):
		return errors.New("repository privacy could not be verified")
	case errors.Is(err, githubsync.ErrDestinationMismatch):
		return errors.New("repository target does not match the configured destination")
	case errors.Is(err, githubsync.ErrRepositoryIncompatible), errors.Is(err, githubsync.ErrManagedMarkerInvalid):
		return errors.New("repository is incompatible with Context Baggage managed sync")
	case errors.Is(err, githubsync.ErrManagedDestinationAdoptionRequired):
		return errors.New("explicit adoption is required; run ctx-bag sync init github <repository-url>")
	case errors.Is(err, githubsync.ErrManagedDestinationMismatch):
		return errors.New("a different sync destination is already configured; use --replace to change it")
	case errors.Is(err, githubsync.ErrManagedDestinationLost):
		return errors.New("managed destination identity was lost")
	case errors.Is(err, githubsync.ErrRecoveryRequired):
		return errors.New("managed pull recovery is pending; run ctx-bag sync recover")
	case errors.Is(err, githubsync.ErrPullConflict):
		return errors.New("pull conflict; local state was not changed")
	case errors.Is(err, githubsync.ErrPublicationConflict):
		return errors.New("publication conflict; remote changes were preserved")
	case errors.Is(err, githubsync.ErrPublicationAmbiguous):
		return errors.New("publication outcome is ambiguous; BASE was not changed")
	case errors.Is(err, githubsync.ErrRecoveryAmbiguous):
		return errors.New("recovery cannot safely continue because local state is ambiguous")
	case errors.Is(err, githubsync.ErrRecoveryBindingMismatch):
		return errors.New("active destination does not match the recovery record")
	case errors.Is(err, store.ErrPullRecoveryMalformed):
		return errors.New("managed pull recovery state is malformed")
	case errors.Is(err, os.ErrNotExist):
		return errors.New("repository missing or inaccessible")
	case errors.Is(err, syncer.ErrDestinationAlreadyConfigured):
		return errors.New("a different sync destination is already configured; use --replace to change it")
	default:
		return err
	}
}
