package app

import (
	"errors"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// blockIfRecoveryPending refuses portable canonical mutators while an
// interrupted managed Pull has left canonical state possibly partial. It is
// deliberately destination-agnostic: a recovery record only ever exists for the
// managed GitHub destination, so its presence alone is the safety signal.
func blockIfRecoveryPending(s store.Store) error {
	exists, err := s.PullRecoveryExists()
	if err != nil {
		return err
	}
	if exists {
		return errors.New("managed pull recovery is required\nresolve the interrupted pull before continuing")
	}
	return nil
}
