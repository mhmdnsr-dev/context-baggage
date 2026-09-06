package app

import (
	"errors"
	"io"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func runSyncStatus(s store.Store, out io.Writer) error {
	state, err := readSyncConfig(s)
	if err != nil {
		return err
	}
	if err := printSyncDestination(out, state); err != nil {
		return err
	}
	if err := printRemoteKnowledge(out, state); err != nil {
		return err
	}
	if exists, recoveryErr := s.PullRecoveryExists(); recoveryErr != nil {
		return errors.New("recovery state is malformed or inaccessible")
	} else if exists {
		if _, err := s.ReadPullRecovery(); err != nil {
			return errors.New("recovery state is malformed or inaccessible")
		}
		if err := writeOutput(out, "Recovery: pending\nRun: ctx-bag sync recover\n"); err != nil {
			return err
		}
	}
	if state.DestinationType == store.DestinationGitHub {
		return nil
	}
	return printFilesystemNamespace(out, state.Folder)
}

func printSyncDestination(out io.Writer, state store.SyncState) error {
	if state.DestinationType == store.DestinationGitHub {
		managed := state.ManagedDestinationID
		if managed == "" {
			managed = "unclaimed"
		}
		return writeOutput(out, "Sync\nDestination: github\nRepository: %s\nManaged destination: %s\nLast push: %s\nLast pull: %s\n", state.GitHubRepository, managed, empty(state.LastPush), empty(state.LastPull))
	}
	return writeOutput(out, "Sync\nDestination: filesystem\nFolder: %s\nLast push: %s\nLast pull: %s\n", state.Folder, empty(state.LastPush), empty(state.LastPull))
}

// printRemoteKnowledge labels persisted observations explicitly so offline
// status never implies that it contacted or describes the current remote.
func printRemoteKnowledge(out io.Writer, state store.SyncState) error {
	relation := "unknown"
	if state.BasePresent && state.LastObservedRemoteHash != "" {
		relation = "changed"
		if state.BaseHash == state.LastObservedRemoteHash {
			relation = "unchanged"
		}
	}
	refresh := state.LastRefresh
	if refresh == "" {
		refresh = "never refreshed"
	}
	return writeOutput(out, "Remote vs BASE (last observed): %s\nRemote knowledge: %s\n", relation, refresh)
}

func printFilesystemNamespace(out io.Writer, folder string) error {
	state, err := syncer.NamespaceState(folder)
	if err != nil {
		return err
	}
	switch state {
	case syncer.NamespaceLegacyOnly:
		return writeOutput(out, "Shared format: legacy\nTransition: required\nrun: ctx-bag sync upgrade\n")
	case syncer.NamespaceV2Only:
		return writeOutput(out, "Shared format: v2\n")
	case syncer.NamespaceBoth:
		return writeOutput(out, "Shared format: v2\nLegacy state: detected / ignored\nUpgrade other devices sharing this folder.\n")
	default:
		return writeOutput(out, "Shared format: none\n")
	}
}
