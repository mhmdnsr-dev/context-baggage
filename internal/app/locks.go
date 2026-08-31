package app

import (
	"context"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

// When an operation needs both locks, callers must acquire sync first and
// canonical second. Local-only operations acquire canonical and never sync.
func acquireSyncShared(s store.Store) (func() error, error) {
	return s.AcquireSyncShared(context.Background())
}

// acquireCanonicalShared protects one coherent read-only canonical snapshot.
func acquireCanonicalShared(s store.Store) (func() error, error) {
	return s.AcquireCanonicalShared(context.Background())
}

// acquireCanonicalExclusive protects one complete canonical mutation use case.
func acquireCanonicalExclusive(s store.Store) (func() error, error) {
	return s.AcquireCanonicalExclusive(context.Background())
}
