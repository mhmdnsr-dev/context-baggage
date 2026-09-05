package githubsync

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

func generateManagedDestinationID(random io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", ErrTransportUnavailable
	}
	return "dst_" + hex.EncodeToString(value), nil
}

func managedMarkerContents(destinationID string) []byte {
	return []byte("format: 1\ndestinationId: " + destinationID + "\n")
}

func secureRandom() io.Reader {
	return rand.Reader
}
