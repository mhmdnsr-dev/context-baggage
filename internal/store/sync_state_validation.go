package store

import "fmt"

// validateCurrentSyncState rejects typed state whose destination-bound fields
// are incomplete or internally inconsistent.
func validateCurrentSyncState(state SyncState) error {
	if state.FormatVersion != SyncStateFormatVersion {
		return fmt.Errorf("unsupported sync state format version %d", state.FormatVersion)
	}
	switch state.DestinationType {
	case DestinationFilesystem:
		if err := validateFilesystemSyncState(state); err != nil {
			return err
		}
	case DestinationGitHub:
		if err := validateGitHubSyncState(state); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported sync destination type %q", state.DestinationType)
	}
	if err := validateBaseBinding(state); err != nil {
		return err
	}
	if (state.LastObservedRemoteHash == "") != (state.LastRefresh == "") {
		return fmt.Errorf("malformed remote observation: identity and refresh timestamp must both be present")
	}
	return validatePrivacyObservation(state)
}

func validateFilesystemSyncState(state SyncState) error {
	if state.Folder == "" {
		return fmt.Errorf("malformed filesystem sync destination: folder is required")
	}
	normalized, err := NormalizeFilesystemDestination(state.Folder)
	if err != nil {
		return fmt.Errorf("normalize filesystem sync destination: %w", err)
	}
	if state.Folder != normalized {
		return fmt.Errorf("malformed filesystem sync destination: folder is not normalized")
	}
	if state.GitHubLocator != "" || state.GitHubRepository != "" || state.ManagedDestinationID != "" {
		return fmt.Errorf("malformed filesystem sync destination: managed destination fields must be empty")
	}
	if state.PrivacyClassification != "" || state.PrivacyCheckedAt != "" || state.PrivacyRepositoryIdentity != "" {
		return fmt.Errorf("malformed filesystem sync destination: privacy observation must be empty")
	}
	return nil
}

func validateGitHubSyncState(state SyncState) error {
	if state.Folder != "" {
		return fmt.Errorf("malformed github sync destination: filesystem folder must be empty")
	}
	if state.GitHubLocator == "" || state.GitHubRepository == "" {
		return fmt.Errorf("malformed github sync destination: locator and repository identity are required")
	}
	return nil
}

func validateBaseBinding(state SyncState) error {
	if !state.BasePresent {
		if state.BaseHash != "" || state.BaseDestinationType != "" || state.BaseDestinationIdentity != "" {
			return fmt.Errorf("malformed sync BASE: absent BASE must not contain a value or binding")
		}
		return nil
	}
	if state.BaseDestinationType != state.DestinationType {
		return fmt.Errorf("malformed sync BASE: destination type does not match active destination")
	}
	wantIdentity := state.Folder
	if state.DestinationType == DestinationGitHub {
		wantIdentity = state.ManagedDestinationID
	}
	if wantIdentity == "" || state.BaseDestinationIdentity != wantIdentity {
		return fmt.Errorf("malformed sync BASE: destination identity does not match active destination")
	}
	return nil
}

func validatePrivacyObservation(state SyncState) error {
	if state.PrivacyClassification == "" {
		if state.PrivacyCheckedAt != "" || state.PrivacyRepositoryIdentity != "" {
			return fmt.Errorf("malformed privacy observation: classification is required")
		}
		return nil
	}
	switch state.PrivacyClassification {
	case PrivacyVerifiedPublic, PrivacyVerifiedNonPublic, PrivacyUnverifiable:
	default:
		return fmt.Errorf("malformed privacy observation: unsupported classification %q", state.PrivacyClassification)
	}
	if state.PrivacyCheckedAt == "" || state.PrivacyRepositoryIdentity == "" {
		return fmt.Errorf("malformed privacy observation: timestamp and repository identity are required")
	}
	return nil
}

func validateSyncFieldNames(fields map[string]string, current bool) error {
	for key := range fields {
		if current && isCurrentSyncField(key) {
			continue
		}
		if !current && isLegacySyncField(key) {
			continue
		}
		return fmt.Errorf("malformed sync state: unexpected field %q", key)
	}
	return nil
}

func isLegacySyncField(key string) bool {
	switch key {
	case "folder", "lastPush", "lastPull", "lastPushHash", "lastPullHash", "baseHash":
		return true
	default:
		return false
	}
}

func isCurrentSyncField(key string) bool {
	if isLegacySyncField(key) {
		return true
	}
	switch key {
	case "formatVersion", "destinationType", "githubLocator", "githubRepository", "managedDestinationId",
		"lastObservedRemoteHash", "lastRefresh", "privacyClassification", "privacyCheckedAt",
		"privacyRepositoryIdentity", "basePresent", "baseDestinationType", "baseDestinationIdentity":
		return true
	default:
		return false
	}
}
