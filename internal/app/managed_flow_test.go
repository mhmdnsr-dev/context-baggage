package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

const (
	flowDestinationID = "dst_0123456789abcdef0123456789abcdef"
	flowRepository    = "github.com/neutral/repository"
	flowLocator       = "https://github.com/neutral/repository.git"
	flowEmptyHash     = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func TestManagedCLIFlowInitClaimDiscoverAdoptPullRecover(t *testing.T) {
	preserveManagedRuntime(t)
	installManagedFlowRuntime()
	exerciseManagedClaimAndDiscovery(t, initializedStore(t))
	exerciseManagedAdoptionPullAndRecovery(t, initializedStore(t))
}

func installManagedFlowRuntime() {
	managedGitDiscovery = func() (githubsync.GitRunner, error) { return githubsync.GitRunner{}, nil }
	managedConfigure = func(_ context.Context, s store.Store, _ githubsync.GitRunner, raw string, _ bool) (store.SyncState, error) {
		state := store.SyncState{FormatVersion: store.SyncStateFormatVersion, DestinationType: store.DestinationGitHub, GitHubLocator: flowLocator, GitHubRepository: flowRepository}
		if old, err := s.ReadSync(); err == nil && old.GitHubRepository == flowRepository {
			state.ManagedDestinationID = flowDestinationID
		}
		if err := s.WriteSync(state); err != nil {
			return store.SyncState{}, err
		}
		return state, nil
	}
	managedPublish = func(_ context.Context, s store.Store, _ githubsync.GitRunner) (string, error) {
		if err := bindFlowBase(s); err != nil {
			return "", err
		}
		return flowEmptyHash, nil
	}
	managedDiscovery = func(context.Context, store.Store, githubsync.GitRunner) ([]store.PortableWorkspace, error) {
		return []store.PortableWorkspace{{ID: "w_portable", Name: "portable", Identity: store.WorkspaceIdentity{Type: "local-directory", Value: "portable"}}}, nil
	}
	managedPull = func(_ context.Context, s store.Store, _ githubsync.GitRunner) (string, error) {
		if err := bindFlowBase(s); err != nil {
			return "", err
		}
		return flowEmptyHash, nil
	}
	managedRecover = func(_ context.Context, s store.Store, _ githubsync.GitRunner) (string, error) {
		return flowEmptyHash, s.RemovePullRecovery()
	}
}

func bindFlowBase(s store.Store) error {
	state, _ := s.ReadSync()
	state.ManagedDestinationID = flowDestinationID
	state.BasePresent, state.BaseHash = true, flowEmptyHash
	state.BaseDestinationType, state.BaseDestinationIdentity = store.DestinationGitHub, flowDestinationID
	return s.WriteSync(state)
}

func exerciseManagedClaimAndDiscovery(t *testing.T, machine store.Store) {
	t.Helper()
	var out bytes.Buffer
	if err := runSync(machine, []string{"init", "github", flowLocator}, &out); err != nil {
		t.Fatal(err)
	}
	state, _ := machine.ReadSync()
	if state.ManagedDestinationID != "" || state.BasePresent {
		t.Fatalf("initial Empty configuration was claimed: %+v", state)
	}
	if err := runSync(machine, []string{"push"}, &out); err != nil {
		t.Fatal(err)
	}
	state, _ = machine.ReadSync()
	if state.ManagedDestinationID != flowDestinationID || !state.BasePresent {
		t.Fatalf("first claim not reflected in CLI state: %+v", state)
	}
	out.Reset()
	if err := runSync(machine, []string{"status"}, &out); err != nil || !strings.Contains(out.String(), "Destination: github") {
		t.Fatalf("managed status failed: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := runWorkspaceAvailable(machine, &out); err != nil || !strings.Contains(out.String(), "w_portable") {
		t.Fatalf("managed discovery failed: %v\n%s", err, out.String())
	}
}

func exerciseManagedAdoptionPullAndRecovery(t *testing.T, machine store.Store) {
	t.Helper()
	for range 2 {
		if _, err := managedConfigure(context.Background(), machine, githubsync.GitRunner{}, flowLocator, false); err != nil {
			t.Fatal(err)
		}
	}
	state, _ := machine.ReadSync()
	if state.ManagedDestinationID != flowDestinationID || state.BasePresent {
		t.Fatalf("explicit adoption established BASE: %+v", state)
	}
	if err := runSync(machine, []string{"pull"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	record := store.PullRecovery{
		Format: store.PullRecoveryFormatVersion, DestinationType: store.DestinationGitHub,
		DestinationID: flowDestinationID, RepositoryIdentity: flowRepository,
		TargetCommitID: strings.Repeat("a", 40), PreLocalHash: strings.Repeat("b", 64), TargetPortableHash: flowEmptyHash,
	}
	if err := machine.WritePullRecovery(record); err != nil {
		t.Fatal(err)
	}
	if err := runSync(machine, []string{"recover"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if exists, _ := machine.PullRecoveryExists(); exists {
		t.Fatal("CLI recovery did not finish the pending operation")
	}
}

func initializedStore(t *testing.T) store.Store {
	t.Helper()
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}
