package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSyncDecodesLegacyFilesystemState(t *testing.T) {
	tests := []struct {
		name         string
		baseHash     string
		lastPullHash string
		lastPushHash string
		wantBase     bool
		wantHash     string
	}{
		{name: "explicit BaseHash", baseHash: "base", lastPullHash: "pull", lastPushHash: "push", wantBase: true, wantHash: "base"},
		{name: "LastPullHash fallback", lastPullHash: "pull", lastPushHash: "push", wantBase: true, wantHash: "pull"},
		{name: "LastPushHash fallback", lastPushHash: "push", wantBase: true, wantHash: "push"},
		{name: "no historical hash", wantBase: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			folder := t.TempDir()
			writeLegacySyncState(t, s, folder, tt.baseHash, tt.lastPullHash, tt.lastPushHash)

			got, err := s.ReadSync()
			if err != nil {
				t.Fatal(err)
			}
			assertLegacySyncState(t, got, folder, tt.wantBase, tt.wantHash)
		})
	}
}

func assertLegacySyncState(t *testing.T, got SyncState, folder string, wantBase bool, wantHash string) {
	t.Helper()
	wantFolder, err := NormalizeFilesystemDestination(folder)
	if err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != SyncStateFormatVersion || got.DestinationType != DestinationFilesystem || got.Folder != wantFolder {
		t.Fatalf("legacy destination = version %d type %q folder %q", got.FormatVersion, got.DestinationType, got.Folder)
	}
	if got.LastPush != "2026-01-01T00:00:00Z" || got.LastPull != "2026-01-02T00:00:00Z" {
		t.Fatalf("legacy operation timestamps were not preserved: %+v", got)
	}
	if got.LastRefresh != "" || got.LastObservedRemoteHash != "" {
		t.Fatalf("legacy state fabricated remote observation: %+v", got)
	}
	if got.BasePresent != wantBase || got.BaseHash != wantHash {
		t.Fatalf("legacy BASE = present %t hash %q, want present %t hash %q", got.BasePresent, got.BaseHash, wantBase, wantHash)
	}
	if wantBase && (got.BaseDestinationType != DestinationFilesystem || got.BaseDestinationIdentity != wantFolder) {
		t.Fatalf("legacy BASE binding = %q/%q", got.BaseDestinationType, got.BaseDestinationIdentity)
	}
	if !wantBase && (got.BaseDestinationType != "" || got.BaseDestinationIdentity != "") {
		t.Fatalf("legacy no-BASE has binding: %+v", got)
	}
}

func TestReadSyncLegacyDoesNotRewriteFile(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	writeLegacySyncState(t, s, t.TempDir(), "base", "pull", "push")
	want, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ReadSync(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("read rewrote legacy state:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestSyncStateRoundTripDistinguishesNoBaseAndEmptyBase(t *testing.T) {
	folder := normalizedTestFolder(t)
	tests := []struct {
		name string
		base SyncState
	}{
		{
			name: "no BASE",
			base: SyncState{FormatVersion: SyncStateFormatVersion, DestinationType: DestinationFilesystem, Folder: folder},
		},
		{
			name: "present empty portable BASE",
			base: SyncState{
				FormatVersion:           SyncStateFormatVersion,
				DestinationType:         DestinationFilesystem,
				Folder:                  folder,
				BasePresent:             true,
				BaseHash:                "",
				BaseDestinationType:     DestinationFilesystem,
				BaseDestinationIdentity: folder,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			if err := s.WriteSync(tt.base); err != nil {
				t.Fatal(err)
			}
			got, err := s.ReadSync()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.base {
				t.Fatalf("round trip mismatch:\nwant: %+v\ngot:  %+v", tt.base, got)
			}
		})
	}
}

func TestSyncStateRoundTripPreservesV03Bookkeeping(t *testing.T) {
	state := SyncState{
		FormatVersion:             SyncStateFormatVersion,
		DestinationType:           DestinationGitHub,
		GitHubLocator:             "transport-locator",
		GitHubRepository:          "github.com/owner/repo",
		ManagedDestinationID:      "managed-id",
		LastPush:                  "2026-01-01T00:00:00Z",
		LastPull:                  "2026-01-02T00:00:00Z",
		LastPushHash:              "push",
		LastPullHash:              "pull",
		LastObservedRemoteHash:    "remote",
		LastRefresh:               "2026-01-03T00:00:00Z",
		PrivacyClassification:     PrivacyVerifiedNonPublic,
		PrivacyCheckedAt:          "2026-01-04T00:00:00Z",
		PrivacyRepositoryIdentity: "github.com/owner/repo",
		BasePresent:               true,
		BaseHash:                  "base",
		BaseDestinationType:       DestinationGitHub,
		BaseDestinationIdentity:   "managed-id",
	}
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if got != state {
		t.Fatalf("round trip mismatch:\nwant: %+v\ngot:  %+v", state, got)
	}
}

func TestReadSyncRefusesUnsupportedOrMalformedTypedState(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "unsupported version", yaml: "formatVersion: 99\ndestinationType: filesystem\nfolder: /tmp/shared\nbasePresent: false\n"},
		{name: "typed state without version", yaml: "destinationType: filesystem\nfolder: /tmp/shared\n"},
		{name: "unknown destination", yaml: "formatVersion: 1\ndestinationType: unknown\nbasePresent: false\n"},
		{name: "filesystem without folder", yaml: "formatVersion: 1\ndestinationType: filesystem\nbasePresent: false\n"},
		{name: "filesystem with managed id", yaml: "formatVersion: 1\ndestinationType: filesystem\nfolder: /tmp/shared\nmanagedDestinationId: unexpected\nbasePresent: false\n"},
		{name: "BASE without binding", yaml: "formatVersion: 1\ndestinationType: filesystem\nfolder: /tmp/shared\nbasePresent: true\nbaseHash: base\n"},
		{name: "unknown legacy field", yaml: "folder: /tmp/shared\ngithubLocator: unexpected\n"},
		{name: "duplicate typed field", yaml: "formatVersion: 1\nformatVersion: 1\ndestinationType: filesystem\nfolder: /tmp/shared\nbasePresent: false\n"},
		{name: "incomplete remote observation", yaml: "formatVersion: 1\ndestinationType: filesystem\nfolder: /tmp/shared\nlastObservedRemoteHash: remote\nbasePresent: false\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			if err := AtomicWrite(s.SyncPath(), []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ReadSync(); err == nil {
				t.Fatal("expected malformed sync state to be refused")
			}
		})
	}
}

func writeLegacySyncState(t *testing.T, s Store, folder, baseHash, lastPullHash, lastPushHash string) {
	t.Helper()
	legacy := strings.Join([]string{
		"folder: " + folder,
		"lastPush: 2026-01-01T00:00:00Z",
		"lastPull: 2026-01-02T00:00:00Z",
		"lastPushHash: " + lastPushHash,
		"lastPullHash: " + lastPullHash,
		"baseHash: " + baseHash,
		"",
	}, "\n")
	if err := AtomicWrite(s.SyncPath(), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
}

func normalizedTestFolder(t *testing.T) string {
	t.Helper()
	folder, err := NormalizeFilesystemDestination(filepath.Join(t.TempDir(), "."))
	if err != nil {
		t.Fatal(err)
	}
	return folder
}
