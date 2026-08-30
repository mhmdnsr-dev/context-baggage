package sync

import (
	"os"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestInitFilesystemSameDestinationPreservesBookkeeping(t *testing.T) {
	s := newInitializedStore(t)
	folder := t.TempDir()
	want := populatedFilesystemState(t, folder)
	if err := s.WriteSync(want); err != nil {
		t.Fatal(err)
	}
	candidate := folder + string(os.PathSeparator) + "."

	got, err := initFilesystem(s, candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("same destination changed bookkeeping:\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestInitFilesystemReplacePermissionDoesNotClearSameDestination(t *testing.T) {
	s := newInitializedStore(t)
	folder := t.TempDir()
	want := populatedFilesystemState(t, folder)
	if err := s.WriteSync(want); err != nil {
		t.Fatal(err)
	}

	got, err := initFilesystem(s, folder, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("replace permission cleared same destination:\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestInitFilesystemDifferentDestinationRequiresAuthorization(t *testing.T) {
	s := newInitializedStore(t)
	oldFolder := t.TempDir()
	want := populatedFilesystemState(t, oldFolder)
	if err := s.WriteSync(want); err != nil {
		t.Fatal(err)
	}

	if _, err := initFilesystem(s, t.TempDir(), false); err == nil {
		t.Fatal("expected replacement authorization error")
	}
	got, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("refused replacement changed state:\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestInitFilesystemDifferentAuthorizedDestinationClearsBookkeeping(t *testing.T) {
	s := newInitializedStore(t)
	oldFolder := t.TempDir()
	if err := s.WriteSync(populatedFilesystemState(t, oldFolder)); err != nil {
		t.Fatal(err)
	}
	newFolder := t.TempDir()

	got, err := initFilesystem(s, newFolder, true)
	if err != nil {
		t.Fatal(err)
	}
	wantFolder, err := store.NormalizeFilesystemDestination(newFolder)
	if err != nil {
		t.Fatal(err)
	}
	want := store.SyncState{
		FormatVersion:   store.SyncStateFormatVersion,
		DestinationType: store.DestinationFilesystem,
		Folder:          wantFolder,
	}
	if got != want {
		t.Fatalf("replacement retained destination bookkeeping:\nwant: %+v\ngot:  %+v", want, got)
	}
	persisted, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if persisted != want {
		t.Fatalf("persisted replacement mismatch:\nwant: %+v\ngot:  %+v", want, persisted)
	}
}

func TestInitFilesystemReplacingGitHubClearsManagedAndPrivacyState(t *testing.T) {
	s := newInitializedStore(t)
	old := store.SyncState{
		FormatVersion:             store.SyncStateFormatVersion,
		DestinationType:           store.DestinationGitHub,
		GitHubLocator:             "transport-locator",
		GitHubRepository:          "github.com/owner/repo",
		ManagedDestinationID:      "managed-id",
		LastPush:                  "2026-01-01T00:00:00Z",
		LastPull:                  "2026-01-02T00:00:00Z",
		LastObservedRemoteHash:    "remote",
		LastRefresh:               "2026-01-03T00:00:00Z",
		PrivacyClassification:     store.PrivacyVerifiedNonPublic,
		PrivacyCheckedAt:          "2026-01-04T00:00:00Z",
		PrivacyRepositoryIdentity: "github.com/owner/repo",
		BasePresent:               true,
		BaseHash:                  "base",
		BaseDestinationType:       store.DestinationGitHub,
		BaseDestinationIdentity:   "managed-id",
	}
	if err := s.WriteSync(old); err != nil {
		t.Fatal(err)
	}

	got, err := initFilesystem(s, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if got.ManagedDestinationID != "" || got.PrivacyClassification != "" || got.LastObservedRemoteHash != "" || got.LastPush != "" || got.LastPull != "" || got.BasePresent {
		t.Fatalf("replacement retained old destination bookkeeping: %+v", got)
	}
}

func TestInitWritesTypedFilesystemState(t *testing.T) {
	s := newInitializedStore(t)
	folder := t.TempDir()

	got, err := Init(s, folder)
	if err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != store.SyncStateFormatVersion || got.DestinationType != store.DestinationFilesystem {
		t.Fatalf("typed destination not written: %+v", got)
	}
	if got.BasePresent {
		t.Fatalf("new destination unexpectedly has BASE: %+v", got)
	}
}

func TestSharedBasePreservesExplicitEmptyValue(t *testing.T) {
	hash, present := sharedBase(store.SyncState{BasePresent: true, BaseHash: ""})
	if !present || hash != "" {
		t.Fatalf("explicit empty BASE collapsed to no BASE: present %t hash %q", present, hash)
	}
}

func newInitializedStore(t *testing.T) store.Store {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func populatedFilesystemState(t *testing.T, folder string) store.SyncState {
	t.Helper()
	normalized, err := store.NormalizeFilesystemDestination(folder)
	if err != nil {
		t.Fatal(err)
	}
	return store.SyncState{
		FormatVersion:           store.SyncStateFormatVersion,
		DestinationType:         store.DestinationFilesystem,
		Folder:                  normalized,
		ManagedDestinationID:    "",
		LastPush:                "2026-01-01T00:00:00Z",
		LastPull:                "2026-01-02T00:00:00Z",
		LastPushHash:            "push",
		LastPullHash:            "pull",
		LastObservedRemoteHash:  "remote",
		LastRefresh:             "2026-01-03T00:00:00Z",
		BasePresent:             true,
		BaseHash:                "base",
		BaseDestinationType:     store.DestinationFilesystem,
		BaseDestinationIdentity: normalized,
	}
}
