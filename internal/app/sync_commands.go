package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
	syncer "github.com/mhmdnsr-dev/context-baggage/internal/sync"
)

func runSync(s store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("sync subcommand required\nrun: ctx-bag sync status")
	}
	switch args[0] {
	case "init":
		if len(args) < 2 {
			return errors.New("sync folder is required\nrun: ctx-bag sync init <folder>")
		}
		st, err := syncer.Init(s, args[1])
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync configured\nFolder: %s\n", st.Folder)
	case "status":
		return runSyncStatus(s, out)
	case "upgrade":
		if err := syncer.SyncUpgrade(s); err != nil {
			return err
		}
		return writeOutput(out, "Shared state upgraded to v2\nLegacy state preserved\nrun: ctx-bag sync pull\nUpgrade other devices sharing this folder.\n")
	case "push":
		hash, err := syncer.Push(s)
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync push complete\nHash: %s\n", hash)
	case "pull":
		hash, err := syncer.Pull(s)
		if err != nil {
			return err
		}
		return writeOutput(out, "Sync pull complete\nHash: %s\n", hash)
	default:
		return fmt.Errorf("unknown sync subcommand: %s", args[0])
	}
}

func runSyncStatus(s store.Store, out io.Writer) error {
	st, err := s.ReadSync()
	if err != nil {
		return errors.New("sync is not configured\nrun: ctx-bag sync init <folder>")
	}
	if err := writeOutput(out, "Sync\nFolder: %s\nLast push: %s\nLast pull: %s\n", st.Folder, empty(st.LastPush), empty(st.LastPull)); err != nil {
		return err
	}
	state, err := syncer.NamespaceState(st.Folder)
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
