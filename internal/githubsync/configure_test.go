package githubsync

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func configureFixture(t *testing.T, raw string, snapshot RepositorySnapshot) (store.Store, GitRunner, configureRuntime) {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	runtime := configureRuntime{
		classify: func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
			return VerifiedNonPublic, nil
		},
		inspect: func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
			return snapshot, nil
		},
	}
	return s, GitRunner{}, runtime
}

func TestConfigureManagedEmptyAndInitialized(t *testing.T) {
	const raw = "https://github.com/neutral/repository"
	t.Run("empty remains unclaimed", func(t *testing.T) {
		s, git, runtime := configureFixture(t, raw, RepositorySnapshot{State: RepositoryEmpty})
		state, err := configureManaged(context.Background(), s, git, raw, false, runtime)
		if err != nil {
			t.Fatal(err)
		}
		if state.ManagedDestinationID != "" || state.BasePresent {
			t.Fatalf("empty candidate was claimed or bound: %+v", state)
		}
	})
	t.Run("initialized is explicitly adopted without base", func(t *testing.T) {
		s, git, runtime := configureFixture(t, raw, RepositorySnapshot{State: RepositoryInitialized, ManagedDestinationID: neutralDestinationID})
		state, err := configureManaged(context.Background(), s, git, raw, false, runtime)
		if err != nil {
			t.Fatal(err)
		}
		if state.ManagedDestinationID != neutralDestinationID || state.BasePresent {
			t.Fatalf("initialized candidate not adopted cleanly: %+v", state)
		}
	})
}

func TestConfigureManagedExplicitlyAdoptsExternalClaim(t *testing.T) {
	const raw = "https://github.com/neutral/repository"
	locator, _ := ParseLocator(raw)
	s, git, runtime := configureFixture(t, raw, RepositorySnapshot{State: RepositoryInitialized, ManagedDestinationID: neutralDestinationID})
	old := newManagedState(locator, "")
	if err := s.WriteSync(old); err != nil {
		t.Fatal(err)
	}
	state, err := configureManaged(context.Background(), s, git, raw, false, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if state.ManagedDestinationID != neutralDestinationID || state.BasePresent {
		t.Fatalf("explicit adoption did not preserve absent BASE: %+v", state)
	}
}

func claimedManagedState() store.SyncState {
	const oldRaw = "https://github.com/neutral/old"
	oldLocator, _ := ParseLocator(oldRaw)
	return store.SyncState{
		FormatVersion: store.SyncStateFormatVersion, DestinationType: store.DestinationGitHub,
		GitHubLocator: oldLocator.TransportURL(), GitHubRepository: oldLocator.Identity(), ManagedDestinationID: neutralDestinationID,
		BasePresent: true, BaseHash: strings.Repeat("a", 64), BaseDestinationType: store.DestinationGitHub, BaseDestinationIdentity: neutralDestinationID,
		LastPush: "preserved", LastPushHash: strings.Repeat("a", 64),
	}
}

func TestConfigureManagedSameMarkerAtNewURLPreservesBase(t *testing.T) {
	const newRaw = "https://github.com/neutral/new"
	s, git, runtime := configureFixture(t, newRaw, RepositorySnapshot{State: RepositoryInitialized, ManagedDestinationID: neutralDestinationID})
	if err := s.WriteSync(claimedManagedState()); err != nil {
		t.Fatal(err)
	}
	state, err := configureManaged(context.Background(), s, git, newRaw, false, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !state.BasePresent || state.LastPush != "preserved" || state.GitHubRepository != "github.com/neutral/new" {
		t.Fatalf("same logical destination lost history: %+v", state)
	}
}

func TestConfigureManagedDifferentMarkerRequiresReplace(t *testing.T) {
	const newRaw = "https://github.com/neutral/new"
	const otherID = "dst_abcdef0123456789abcdef0123456789"
	s, git, runtime := configureFixture(t, newRaw, RepositorySnapshot{State: RepositoryInitialized, ManagedDestinationID: otherID})
	if err := s.WriteSync(claimedManagedState()); err != nil {
		t.Fatal(err)
	}
	if _, err := configureManaged(context.Background(), s, git, newRaw, false, runtime); !errors.Is(err, ErrManagedDestinationMismatch) {
		t.Fatalf("different marker accepted without replace: %v", err)
	}
	state, err := configureManaged(context.Background(), s, git, newRaw, true, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if state.ManagedDestinationID != otherID || state.BasePresent || state.LastPush != "" {
		t.Fatalf("replacement retained destination-bound state: %+v", state)
	}
}

func TestConfigureManagedClaimedToEmptyRequiresReplace(t *testing.T) {
	const newRaw = "https://github.com/neutral/new"
	s, git, runtime := configureFixture(t, newRaw, RepositorySnapshot{State: RepositoryEmpty})
	if err := s.WriteSync(claimedManagedState()); err != nil {
		t.Fatal(err)
	}
	if _, err := configureManaged(context.Background(), s, git, newRaw, false, runtime); !errors.Is(err, ErrManagedDestinationLost) {
		t.Fatalf("identity loss accepted: %v", err)
	}
	state, err := configureManaged(context.Background(), s, git, newRaw, true, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if state.ManagedDestinationID != "" || state.BasePresent {
		t.Fatalf("empty replacement was claimed or bound: %+v", state)
	}
}

func TestConfigureManagedFailureDoesNotMutateCurrentDestination(t *testing.T) {
	const raw = "https://github.com/neutral/repository"
	s, git, runtime := configureFixture(t, raw, RepositorySnapshot{State: RepositoryEmpty})
	folder := t.TempDir()
	old := store.SyncState{FormatVersion: store.SyncStateFormatVersion, DestinationType: store.DestinationFilesystem, Folder: folder}
	if err := s.WriteSync(old); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.SyncPath())
	if err != nil {
		t.Fatal(err)
	}
	runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
		return VerifiedPublic, nil
	}
	if _, err := configureManaged(context.Background(), s, git, raw, true, runtime); !errors.Is(err, ErrRepositoryPublic) {
		t.Fatalf("privacy refusal = %v", err)
	}
	after, _ := os.ReadFile(s.SyncPath())
	if string(before) != string(after) {
		t.Fatal("candidate verification failure changed active destination")
	}
}

func TestConfigureManagedRefusesUnverifiableAndIncompatibleCandidates(t *testing.T) {
	const raw = "https://github.com/neutral/repository"
	for _, test := range []struct {
		name       string
		privacy    PrivacyClassification
		privacyErr error
		snapshot   RepositorySnapshot
		inspectErr error
		want       error
	}{
		{name: "unverifiable privacy", privacy: Unverifiable, privacyErr: ErrNetworkUnavailable, want: ErrPrivacyUnverifiable},
		{name: "incompatible repository", privacy: VerifiedNonPublic, inspectErr: ErrRepositoryIncompatible, want: ErrRepositoryIncompatible},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, git, runtime := configureFixture(t, raw, test.snapshot)
			runtime.classify = func(context.Context, GitRunner, Locator) (PrivacyClassification, error) {
				return test.privacy, test.privacyErr
			}
			runtime.inspect = func(context.Context, GitRunner, Locator) (RepositorySnapshot, error) {
				return test.snapshot, test.inspectErr
			}
			if _, err := configureManaged(context.Background(), s, git, raw, true, runtime); !errors.Is(err, test.want) {
				t.Fatalf("configure error = %v, want %v", err, test.want)
			}
			if _, err := os.Stat(s.SyncPath()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed candidate created sync state: %v", err)
			}
		})
	}
}
