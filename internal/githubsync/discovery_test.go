package githubsync

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func discoveryFixtureRuntime(t *testing.T, runner GitRunner, remote string) discoveryRuntime {
	t.Helper()
	return discoveryRuntime{
		materialize: func(ctx context.Context, git GitRunner, locator Locator) (RepositorySnapshot, string, func(), error) {
			return materializeLocalRemote(ctx, git, locator, remote)
		},
	}
}

func TestManagedDiscoveryListsWorkspaces(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	workspaces, err := discoverManagedWorkspaces(context.Background(), s, runner, discoveryFixtureRuntime(t, runner, remote))
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != "w_one" {
		t.Fatalf("unexpected workspaces: %+v", workspaces)
	}
	state, _ := s.ReadSync()
	if state.BasePresent {
		t.Fatalf("discovery mutated BASE: %+v", state)
	}
	if state.LastObservedRemoteHash == "" || state.LastRefresh == "" {
		t.Fatalf("discovery did not persist observation bookkeeping: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); exists {
		t.Fatal("discovery created a recovery record")
	}
}

func TestManagedDiscoveryPendingRecoveryRefuses(t *testing.T) {
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())
	s := managedPullStore(t)
	runner := localFixtureRunner(t)
	record := recoveryRecordFixture(t, strings.Repeat("a", 40), strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err := s.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	_, err := discoverManagedWorkspaces(context.Background(), s, runner, discoveryFixtureRuntime(t, runner, remote))
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("expected recovery required, got %v", err)
	}
	state, _ := s.ReadSync()
	if state.LastObservedRemoteHash != "" {
		t.Fatalf("discovery mutated observation bookkeeping: %+v", state)
	}
	if exists, _ := s.PullRecoveryExists(); !exists {
		t.Fatal("recovery record dropped on pending-recovery refusal")
	}
}

func TestManagedDiscoveryDestinationRules(t *testing.T) {
	runner := localFixtureRunner(t)
	remote, _ := createBareFixture(t, "context-baggage", validPortableFiles())

	t.Run("missing id", func(t *testing.T) {
		s := managedPullStore(t)
		state, _ := s.ReadSync()
		state.ManagedDestinationID = ""
		if err := s.WriteSync(state); err != nil {
			t.Fatal(err)
		}
		_, err := discoverManagedWorkspaces(context.Background(), s, runner, discoveryFixtureRuntime(t, runner, remote))
		if !errors.Is(err, ErrManagedDestinationAdoptionRequired) {
			t.Fatalf("expected adoption required, got %v", err)
		}
	})
	t.Run("mismatching id", func(t *testing.T) {
		s := managedPullStore(t)
		state, _ := s.ReadSync()
		state.ManagedDestinationID = "dst_ffffffffffffffffffffffffffffffff"
		if err := s.WriteSync(state); err != nil {
			t.Fatal(err)
		}
		_, err := discoverManagedWorkspaces(context.Background(), s, runner, discoveryFixtureRuntime(t, runner, remote))
		if !errors.Is(err, ErrManagedDestinationMismatch) {
			t.Fatalf("expected mismatch, got %v", err)
		}
	})
	t.Run("remote empty after claim", func(t *testing.T) {
		emptyRemote, _ := createBareFixture(t, "", nil)
		s := managedPullStore(t)
		_, err := discoverManagedWorkspaces(context.Background(), s, runner, discoveryFixtureRuntime(t, runner, emptyRemote))
		if !errors.Is(err, ErrManagedDestinationLost) {
			t.Fatalf("expected identity loss, got %v", err)
		}
	})
}
