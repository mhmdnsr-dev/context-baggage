package config

import (
	"errors"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/platform"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func OpenStore() (store.Store, error) {
	home, err := platform.AppHome()
	if err != nil {
		return store.Store{}, err
	}
	return store.New(home), nil
}

func EnsureInitialized(s store.Store) error {
	if _, err := os.Stat(s.DevicePath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("context baggage is not initialized; run: ctx-bag init")
		}
		return err
	}
	return nil
}
