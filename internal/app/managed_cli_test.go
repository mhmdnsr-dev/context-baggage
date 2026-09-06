package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func preserveManagedRuntime(t *testing.T) {
	t.Helper()
	discover, configure := managedGitDiscovery, managedConfigure
	publish, pull, recover := managedPublish, managedPull, managedRecover
	discovery := managedDiscovery
	classify, inspect := managedClassify, managedInspect
	t.Cleanup(func() {
		managedGitDiscovery, managedConfigure = discover, configure
		managedPublish, managedPull, managedRecover = publish, pull, recover
		managedDiscovery = discovery
		managedClassify, managedInspect = classify, inspect
	})
}

func managedCLIState(id string) store.SyncState {
	state := store.SyncState{
		FormatVersion: store.SyncStateFormatVersion, DestinationType: store.DestinationGitHub,
		GitHubLocator: "https://github.com/neutral/repository.git", GitHubRepository: "github.com/neutral/repository",
		ManagedDestinationID: id,
	}
	if id != "" {
		state.BasePresent = true
		state.BaseHash = strings.Repeat("a", 64)
		state.BaseDestinationType = store.DestinationGitHub
		state.BaseDestinationIdentity = id
	}
	return state
}

func TestParseSyncInitPreservesLiteralGitHub(t *testing.T) {
	tests := []struct {
		args        []string
		github      bool
		target      string
		replacement bool
		wantErr     bool
	}{
		{args: []string{"github"}, target: "github"},
		{args: []string{"github", "--replace"}, target: "github", replacement: true},
		{args: []string{"github", "https://github.com/owner/repo"}, github: true, target: "https://github.com/owner/repo"},
		{args: []string{"github", "https://github.com/owner/repo", "--replace"}, github: true, target: "https://github.com/owner/repo", replacement: true},
		{args: []string{"github", "git@github.com:owner/repo.git"}, github: true, target: "git@github.com:owner/repo.git"},
		{args: []string{"folder", "--unknown"}, wantErr: true},
		{args: []string{"github", "url", "extra"}, wantErr: true},
		{args: []string{"folder", "--replace", "--replace"}, wantErr: true},
	}
	for _, test := range tests {
		got, err := parseSyncInit(test.args)
		if test.wantErr {
			if err == nil {
				t.Fatalf("parseSyncInit(%v) succeeded", test.args)
			}
			continue
		}
		if err != nil || got.github != test.github || got.target != test.target || got.replacement != test.replacement {
			t.Fatalf("parseSyncInit(%v) = %+v, %v", test.args, got, err)
		}
	}
}

func TestManagedSyncDispatch(t *testing.T) {
	preserveManagedRuntime(t)
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSync(managedCLIState("dst_0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatal(err)
	}
	managedGitDiscovery = func() (githubsync.GitRunner, error) { return githubsync.GitRunner{}, nil }
	called := ""
	managedPublish = func(context.Context, store.Store, githubsync.GitRunner) (string, error) {
		called = "push"
		return strings.Repeat("b", 64), nil
	}
	managedPull = func(context.Context, store.Store, githubsync.GitRunner) (string, error) {
		called = "pull"
		return strings.Repeat("c", 64), nil
	}
	for _, command := range []string{"push", "pull"} {
		called = ""
		if err := runSync(s, []string{command}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if called != command {
			t.Fatalf("%s dispatched to %q", command, called)
		}
	}
}

func TestSyncStatusIsOfflineAndObservationBound(t *testing.T) {
	preserveManagedRuntime(t)
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	state := managedCLIState("dst_0123456789abcdef0123456789abcdef")
	state.LastObservedRemoteHash = state.BaseHash
	state.LastRefresh = "2026-09-06T00:00:00Z"
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	remoteCalls := 0
	managedGitDiscovery = func() (githubsync.GitRunner, error) {
		remoteCalls++
		return githubsync.GitRunner{}, errors.New("must remain offline")
	}
	managedClassify = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.PrivacyClassification, error) {
		remoteCalls++
		return githubsync.Unverifiable, errors.New("must remain offline")
	}
	managedInspect = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.RepositorySnapshot, error) {
		remoteCalls++
		return githubsync.RepositorySnapshot{}, errors.New("must remain offline")
	}
	var out bytes.Buffer
	if err := runSyncStatus(s, &out); err != nil {
		t.Fatal(err)
	}
	if remoteCalls != 0 || !strings.Contains(out.String(), "Remote vs BASE (last observed): unchanged") || !strings.Contains(out.String(), "Remote knowledge: 2026-09-06") {
		t.Fatalf("offline status mismatch calls=%d:\n%s", remoteCalls, out.String())
	}
	state.BasePresent = false
	state.BaseHash, state.BaseDestinationType, state.BaseDestinationIdentity = "", "", ""
	state.LastObservedRemoteHash, state.LastRefresh = "", ""
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runSyncStatus(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "last observed): unknown") || !strings.Contains(out.String(), "Remote knowledge: never refreshed") {
		t.Fatalf("unknown observation wording missing:\n%s", out.String())
	}
}

func TestRemoteVersusBaseWording(t *testing.T) {
	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tests := []struct {
		name  string
		state store.SyncState
		want  string
	}{
		{name: "base absent", state: store.SyncState{}, want: "unknown"},
		{name: "observation absent", state: store.SyncState{BasePresent: true, BaseHash: hashA}, want: "unknown"},
		{name: "unchanged", state: store.SyncState{BasePresent: true, BaseHash: hashA, LastObservedRemoteHash: hashA}, want: "unchanged"},
		{name: "changed", state: store.SyncState{BasePresent: true, BaseHash: hashA, LastObservedRemoteHash: hashB}, want: "changed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := printRemoteKnowledge(&out, test.state); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "last observed): "+test.want) {
				t.Fatalf("relation %q missing:\n%s", test.want, out.String())
			}
		})
	}
}

func TestDoctorRemoteDoesNotPersist(t *testing.T) {
	preserveManagedRuntime(t)
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	state := managedCLIState("dst_0123456789abcdef0123456789abcdef")
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	workspace := store.Workspace{ID: "w_doctor", Name: "doctor", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}
	if err := s.WriteWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	recovery := store.PullRecovery{
		Format: store.PullRecoveryFormatVersion, DestinationType: store.DestinationGitHub,
		DestinationID: state.ManagedDestinationID, RepositoryIdentity: state.GitHubRepository,
		TargetCommitID: strings.Repeat("a", 40), PreLocalHash: strings.Repeat("b", 64), TargetPortableHash: strings.Repeat("c", 64),
	}
	if err := s.WritePullRecovery(recovery); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.SyncPath())
	recoveryBefore, _ := os.ReadFile(s.PullRecoveryPath())
	workspaceBefore, _ := os.ReadFile(s.WorkspacePath(workspace.ID))
	managedGitDiscovery = func() (githubsync.GitRunner, error) { return githubsync.GitRunner{}, nil }
	managedClassify = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.PrivacyClassification, error) {
		return githubsync.VerifiedNonPublic, nil
	}
	managedInspect = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.RepositorySnapshot, error) {
		return githubsync.RepositorySnapshot{State: githubsync.RepositoryInitialized, ManagedDestinationID: state.ManagedDestinationID}, nil
	}
	var out bytes.Buffer
	if err := runRemoteDoctor(s, &out); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.SyncPath())
	recoveryAfter, _ := os.ReadFile(s.PullRecoveryPath())
	workspaceAfter, _ := os.ReadFile(s.WorkspacePath(workspace.ID))
	if !bytes.Equal(before, after) || !bytes.Equal(recoveryBefore, recoveryAfter) || !bytes.Equal(workspaceBefore, workspaceAfter) {
		t.Fatal("doctor --remote changed local state bytes")
	}
	if !strings.Contains(out.String(), "Privacy: verified non-public") {
		t.Fatalf("healthy remote diagnostics missing:\n%s", out.String())
	}
}

func TestSyncRecoverNoPendingIsSuccessfulNoop(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runSyncRecover(s, nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "No managed pull recovery is pending.\n" {
		t.Fatalf("unexpected no-op output: %q", out.String())
	}
}

func TestSyncRecoverMalformedRecordRefusesBeforeGit(t *testing.T) {
	preserveManagedRuntime(t)
	s := initializedStore(t)
	if err := os.MkdirAll(filepath.Dir(s.PullRecoveryPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.PullRecoveryPath(), []byte("not: a valid recovery\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCalls := 0
	managedGitDiscovery = func() (githubsync.GitRunner, error) {
		gitCalls++
		return githubsync.GitRunner{}, nil
	}
	err := runSyncRecover(s, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "recovery state is malformed") {
		t.Fatalf("malformed recovery error = %v", err)
	}
	if gitCalls != 0 {
		t.Fatalf("malformed recovery performed %d Git calls", gitCalls)
	}
}
