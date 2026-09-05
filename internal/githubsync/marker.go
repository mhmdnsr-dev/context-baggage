package githubsync

import (
	"regexp"
	"strings"
)

const (
	managedMarkerName = "context-baggage-managed.yaml"
	maxMarkerBytes    = 4 * 1024
)

var managedDestinationIDPattern = regexp.MustCompile(`^dst_[0-9a-f]{32}$`)

type managedMarker struct {
	format        string
	destinationID string
}

// parseManagedMarker accepts only the two scalar fields in format version 1.
// A deliberately small parser makes duplicate and unknown keys unambiguous.
func parseManagedMarker(data []byte) (managedMarker, error) {
	if len(data) > maxMarkerBytes {
		return managedMarker{}, ErrResourceLimitExceeded
	}
	marker := managedMarker{}
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != key || strings.TrimSpace(value) == "" || seen[key] {
			return managedMarker{}, ErrManagedMarkerInvalid
		}
		seen[key] = true
		switch key {
		case "format":
			marker.format = strings.TrimSpace(value)
		case "destinationId":
			marker.destinationID = strings.TrimSpace(value)
		default:
			return managedMarker{}, ErrManagedMarkerInvalid
		}
	}
	if marker.format != "1" || !managedDestinationIDPattern.MatchString(marker.destinationID) || len(seen) != 2 {
		return managedMarker{}, ErrManagedMarkerInvalid
	}
	return marker, nil
}

// ValidateExpectedDestination distinguishes an existing binding from mismatch
// and explicit-adoption cases without persisting or adopting remote identity.
func ValidateExpectedDestination(snapshot RepositorySnapshot, expectedID string) error {
	if snapshot.State != RepositoryInitialized {
		return ErrRepositoryIncompatible
	}
	if expectedID == "" {
		return ErrManagedDestinationAdoptionRequired
	}
	if !managedDestinationIDPattern.MatchString(expectedID) || expectedID != snapshot.ManagedDestinationID {
		return ErrManagedDestinationMismatch
	}
	return nil
}
