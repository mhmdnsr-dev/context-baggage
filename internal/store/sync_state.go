package store

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// SyncPath returns the machine-local sync state file.
func (s Store) SyncPath() string {
	return filepath.Join(s.Home, "sync", "state.yaml")
}

// NormalizeFilesystemDestination applies the filesystem identity normalization
// used by both legacy decoding and current destination configuration.
func NormalizeFilesystemDestination(folder string) (string, error) {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// ReadSync decodes legacy v0.2 filesystem state or the current typed state.
// Legacy decoding is an in-memory interpretation and never rewrites the file.
func (s Store) ReadSync() (SyncState, error) {
	kv, err := readSyncFields(s.SyncPath())
	if err != nil {
		return SyncState{}, err
	}
	_, hasVersion := kv["formatVersion"]
	_, hasDestinationType := kv["destinationType"]
	if !hasVersion && !hasDestinationType {
		if err := validateSyncFieldNames(kv, false); err != nil {
			return SyncState{}, err
		}
		return decodeLegacySyncState(kv)
	}
	if !hasVersion || !hasDestinationType {
		return SyncState{}, fmt.Errorf("malformed typed sync state: formatVersion and destinationType must both be present")
	}
	if err := validateSyncFieldNames(kv, true); err != nil {
		return SyncState{}, err
	}
	return decodeCurrentSyncState(kv)
}

// WriteSync validates and writes the current typed sync state representation.
func (s Store) WriteSync(state SyncState) error {
	if err := validateCurrentSyncState(state); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("formatVersion: %d", state.FormatVersion),
		"destinationType: " + string(state.DestinationType),
		"folder: " + state.Folder,
		"githubLocator: " + state.GitHubLocator,
		"githubRepository: " + state.GitHubRepository,
		"managedDestinationId: " + state.ManagedDestinationID,
		"lastPush: " + state.LastPush,
		"lastPull: " + state.LastPull,
		"lastPushHash: " + state.LastPushHash,
		"lastPullHash: " + state.LastPullHash,
		"lastObservedRemoteHash: " + state.LastObservedRemoteHash,
		"lastRefresh: " + state.LastRefresh,
		"privacyClassification: " + string(state.PrivacyClassification),
		"privacyCheckedAt: " + state.PrivacyCheckedAt,
		"privacyRepositoryIdentity: " + state.PrivacyRepositoryIdentity,
		"basePresent: " + strconv.FormatBool(state.BasePresent),
		"baseHash: " + state.BaseHash,
		"baseDestinationType: " + string(state.BaseDestinationType),
		"baseDestinationIdentity: " + state.BaseDestinationIdentity,
	}
	return AtomicWrite(s.SyncPath(), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// decodeLegacySyncState maps the released v0.2 flat fields into the current
// destination-bound model while preserving historical BASE fallback order.
func decodeLegacySyncState(kv map[string]string) (SyncState, error) {
	if kv["folder"] == "" {
		return SyncState{}, fmt.Errorf("malformed legacy sync state: folder is required")
	}
	folder, err := NormalizeFilesystemDestination(kv["folder"])
	if err != nil {
		return SyncState{}, fmt.Errorf("normalize legacy sync folder: %w", err)
	}
	state := SyncState{
		FormatVersion:   SyncStateFormatVersion,
		DestinationType: DestinationFilesystem,
		Folder:          folder,
		LastPush:        kv["lastPush"],
		LastPull:        kv["lastPull"],
		LastPushHash:    kv["lastPushHash"],
		LastPullHash:    kv["lastPullHash"],
	}
	baseHash := firstNonEmpty(kv["baseHash"], state.LastPullHash, state.LastPushHash)
	if baseHash != "" {
		state.BasePresent = true
		state.BaseHash = baseHash
		state.BaseDestinationType = DestinationFilesystem
		state.BaseDestinationIdentity = folder
	}
	return state, nil
}

func decodeCurrentSyncState(kv map[string]string) (SyncState, error) {
	version, err := strconv.Atoi(kv["formatVersion"])
	if err != nil || version != SyncStateFormatVersion {
		return SyncState{}, fmt.Errorf("unsupported sync state format version %q", kv["formatVersion"])
	}
	basePresent, err := strconv.ParseBool(kv["basePresent"])
	if err != nil {
		return SyncState{}, fmt.Errorf("malformed typed sync state: invalid basePresent %q", kv["basePresent"])
	}
	state := SyncState{
		FormatVersion:             version,
		DestinationType:           DestinationType(kv["destinationType"]),
		Folder:                    kv["folder"],
		GitHubLocator:             kv["githubLocator"],
		GitHubRepository:          kv["githubRepository"],
		ManagedDestinationID:      kv["managedDestinationId"],
		LastPush:                  kv["lastPush"],
		LastPull:                  kv["lastPull"],
		LastPushHash:              kv["lastPushHash"],
		LastPullHash:              kv["lastPullHash"],
		LastObservedRemoteHash:    kv["lastObservedRemoteHash"],
		LastRefresh:               kv["lastRefresh"],
		PrivacyClassification:     PrivacyClassification(kv["privacyClassification"]),
		PrivacyCheckedAt:          kv["privacyCheckedAt"],
		PrivacyRepositoryIdentity: kv["privacyRepositoryIdentity"],
		BasePresent:               basePresent,
		BaseHash:                  kv["baseHash"],
		BaseDestinationType:       DestinationType(kv["baseDestinationType"]),
		BaseDestinationIdentity:   kv["baseDestinationIdentity"],
	}
	if state.DestinationType == DestinationFilesystem && state.Folder != "" {
		state.Folder, err = NormalizeFilesystemDestination(state.Folder)
		if err != nil {
			return SyncState{}, fmt.Errorf("normalize sync folder: %w", err)
		}
	}
	if err := validateCurrentSyncState(state); err != nil {
		return SyncState{}, err
	}
	return state, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func readSyncFields(path string) (map[string]string, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]string, len(lines))
	for lineNumber, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("malformed sync state at line %d", lineNumber+1)
		}
		key := strings.TrimSpace(parts[0])
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("malformed sync state: duplicate field %q", key)
		}
		fields[key] = strings.TrimSpace(parts[1])
	}
	return fields, nil
}
