package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrPullRecoveryMalformed reports a recovery record that cannot be trusted for
// deterministic recovery. A malformed record is refused, never guessed at, and
// never automatically deleted.
var ErrPullRecoveryMalformed = errors.New("malformed pull recovery record")

var managedDestinationIDPattern = regexp.MustCompile(`^dst_[0-9a-f]{32}$`)

// PullRecoveryFormatVersion is the current machine-local pull-recovery format.
const PullRecoveryFormatVersion = 1

// maxPullRecoveryBytes bounds the recovery record. It holds only the minimal
// evidence needed to finish or refuse an interrupted managed Pull, so a small
// hard limit rejects arbitrary large input without a general parser.
const maxPullRecoveryBytes = 64 * 1024

// PullRecovery records the minimal evidence required to deterministically
// finish or refuse an interrupted managed Pull. It is machine-local safety
// metadata: it never enters portable state, sync BASE, or the Git repository.
type PullRecovery struct {
	Format             int
	DestinationType    DestinationType
	DestinationID      string
	RepositoryIdentity string
	TargetCommitID     string
	PreLocalHash       string
	TargetPortableHash string
}

// PullRecoveryPath returns the machine-local pull-recovery record path. It is
// deliberately inside the sync directory but never inside the portable export.
func (s Store) PullRecoveryPath() string {
	return filepath.Join(s.Home, "sync", "pull-recovery.yaml")
}

// ReadPullRecovery decodes a strictly validated recovery record. A missing
// record returns os.ErrNotExist; a malformed record returns a refusal error.
func (s Store) ReadPullRecovery() (PullRecovery, error) {
	fields, err := readPullRecoveryFields(s.PullRecoveryPath())
	if err != nil {
		return PullRecovery{}, err
	}
	if _, absent := fields["format"]; !absent {
		return PullRecovery{}, ErrPullRecoveryMalformed
	}
	return decodePullRecovery(fields)
}

// WritePullRecovery validates and persists the recovery record atomically. It
// must succeed before the first canonical mutation of a managed Pull.
func (s Store) WritePullRecovery(record PullRecovery) error {
	if err := validatePullRecovery(record); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("format: %d", record.Format),
		"destinationType: " + string(record.DestinationType),
		"destinationId: " + record.DestinationID,
		"repositoryIdentity: " + record.RepositoryIdentity,
		"targetCommitId: " + record.TargetCommitID,
		"preLocalHash: " + record.PreLocalHash,
		"targetPortableHash: " + record.TargetPortableHash,
	}
	return AtomicWriteDurable(s.PullRecoveryPath(), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// RemovePullRecovery deletes the recovery record. It is called only after BASE
// and bookkeeping have been successfully persisted.
func (s Store) RemovePullRecovery() error {
	err := os.Remove(s.PullRecoveryPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// PullRecoveryExists reports whether a recovery record is present. Its existence
// means canonical state may be partial and BASE has not been finalized.
func (s Store) PullRecoveryExists() (bool, error) {
	_, err := os.Stat(s.PullRecoveryPath())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func decodePullRecovery(fields map[string]string) (PullRecovery, error) {
	record := PullRecovery{
		Format:             parseIntField(fields, "format"),
		DestinationType:    DestinationType(fields["destinationType"]),
		DestinationID:      fields["destinationId"],
		RepositoryIdentity: fields["repositoryIdentity"],
		TargetCommitID:     fields["targetCommitId"],
		PreLocalHash:       fields["preLocalHash"],
		TargetPortableHash: fields["targetPortableHash"],
	}
	if err := validatePullRecovery(record); err != nil {
		return PullRecovery{}, err
	}
	return record, nil
}

func validatePullRecovery(record PullRecovery) error {
	if record.Format != PullRecoveryFormatVersion {
		return fmt.Errorf("%w: unsupported format version %d", ErrPullRecoveryMalformed, record.Format)
	}
	if record.DestinationType != DestinationGitHub {
		return fmt.Errorf("%w: unsupported destination type %q", ErrPullRecoveryMalformed, record.DestinationType)
	}
	if !managedDestinationIDPattern.MatchString(record.DestinationID) {
		return fmt.Errorf("%w: malformed destination id", ErrPullRecoveryMalformed)
	}
	if record.RepositoryIdentity == "" {
		return fmt.Errorf("%w: repository identity is required", ErrPullRecoveryMalformed)
	}
	if !validGitObjectID(record.TargetCommitID) {
		return fmt.Errorf("%w: malformed target commit id", ErrPullRecoveryMalformed)
	}
	if !validPortableHash(record.PreLocalHash) || !validPortableHash(record.TargetPortableHash) {
		return fmt.Errorf("%w: malformed portable hash", ErrPullRecoveryMalformed)
	}
	return nil
}

func readPullRecoveryFields(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxPullRecoveryBytes {
		return nil, ErrPullRecoveryMalformed
	}
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]string, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, ErrPullRecoveryMalformed
		}
		key = strings.TrimSpace(key)
		if _, exists := fields[key]; exists {
			return nil, ErrPullRecoveryMalformed
		}
		fields[key] = strings.TrimSpace(value)
	}
	for _, required := range []string{"format", "destinationType", "destinationId", "repositoryIdentity", "targetCommitId", "preLocalHash", "targetPortableHash"} {
		if _, ok := fields[required]; !ok {
			return nil, ErrPullRecoveryMalformed
		}
	}
	for key := range fields {
		if !isPullRecoveryField(key) {
			return nil, ErrPullRecoveryMalformed
		}
	}
	return fields, nil
}

func isPullRecoveryField(key string) bool {
	switch key {
	case "format", "destinationType", "destinationId", "repositoryIdentity", "targetCommitId", "preLocalHash", "targetPortableHash":
		return true
	default:
		return false
	}
}

func parseIntField(fields map[string]string, key string) int {
	var value int
	_, _ = fmt.Sscanf(fields[key], "%d", &value)
	return value
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func validPortableHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
