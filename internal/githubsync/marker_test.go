package githubsync

import (
	"errors"
	"strings"
	"testing"
)

const neutralDestinationID = "dst_0123456789abcdef0123456789abcdef"

func TestParseManagedMarkerAcceptsLockedFormat(t *testing.T) {
	marker, err := parseManagedMarker([]byte("format: 1\ndestinationId: " + neutralDestinationID + "\n"))
	if err != nil || marker.destinationID != neutralDestinationID {
		t.Fatalf("valid marker rejected: %+v, %v", marker, err)
	}
}

func TestParseManagedMarkerRejectsInvalidForms(t *testing.T) {
	tests := map[string]string{
		"missing format":        "destinationId: " + neutralDestinationID + "\n",
		"missing destination":   "format: 1\n",
		"wrong format":          "format: 2\ndestinationId: " + neutralDestinationID + "\n",
		"empty destination":     "format: 1\ndestinationId:\n",
		"uppercase destination": "format: 1\ndestinationId: dst_0123456789ABCDEF0123456789ABCDEF\n",
		"short destination":     "format: 1\ndestinationId: dst_0123\n",
		"long destination":      "format: 1\ndestinationId: dst_0123456789abcdef0123456789abcdef00\n",
		"non hex destination":   "format: 1\ndestinationId: dst_0123456789abcdef0123456789abcdeg\n",
		"malformed yaml":        "format 1\ndestinationId: " + neutralDestinationID + "\n",
		"duplicate field":       "format: 1\nformat: 1\ndestinationId: " + neutralDestinationID + "\n",
		"unknown field":         "format: 1\ndestinationId: " + neutralDestinationID + "\nfuture: value\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseManagedMarker([]byte(input))
			if !errors.Is(err, ErrManagedMarkerInvalid) {
				t.Fatalf("expected invalid marker, got %v", err)
			}
		})
	}
}

func TestParseManagedMarkerRejectsOversize(t *testing.T) {
	_, err := parseManagedMarker([]byte(strings.Repeat("x", maxMarkerBytes+1)))
	if !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("expected marker limit error, got %v", err)
	}
}

func TestParseManagedMarkerAcceptsSizeBoundary(t *testing.T) {
	data := validMarker + strings.Repeat("\n", maxMarkerBytes-len(validMarker))
	if _, err := parseManagedMarker([]byte(data)); err != nil {
		t.Fatalf("marker size boundary rejected: %v", err)
	}
}

func TestValidateExpectedDestinationRequiresMatchOrExplicitAdoption(t *testing.T) {
	snapshot := RepositorySnapshot{State: RepositoryInitialized, ManagedDestinationID: neutralDestinationID}
	if err := ValidateExpectedDestination(snapshot, neutralDestinationID); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExpectedDestination(snapshot, ""); !errors.Is(err, ErrManagedDestinationAdoptionRequired) {
		t.Fatalf("expected adoption requirement, got %v", err)
	}
	if err := ValidateExpectedDestination(snapshot, "dst_ffffffffffffffffffffffffffffffff"); !errors.Is(err, ErrManagedDestinationMismatch) {
		t.Fatalf("expected destination mismatch, got %v", err)
	}
}
