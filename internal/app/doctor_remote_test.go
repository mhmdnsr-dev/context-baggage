package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestDoctorDefaultDoesNotCallManagedNetwork(t *testing.T) {
	preserveManagedRuntime(t)
	dir := t.TempDir()
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	remoteCalls := 0
	managedGitDiscovery = func() (githubsync.GitRunner, error) {
		remoteCalls++
		return githubsync.GitRunner{}, nil
	}
	if out := runCLI(t, home, dir, "doctor"); !strings.Contains(out, "Doctor: OK") {
		t.Fatalf("default doctor failed:\n%s", out)
	}
	if remoteCalls != 0 {
		t.Fatalf("default doctor performed %d remote calls", remoteCalls)
	}
}

func TestDoctorRemoteManagedDiagnostics(t *testing.T) {
	const destinationID = "dst_0123456789abcdef0123456789abcdef"
	tests := []struct {
		name       string
		localID    string
		privacy    githubsync.PrivacyClassification
		privacyErr error
		snapshot   githubsync.RepositorySnapshot
		inspectErr error
		want       string
	}{
		{name: "healthy", localID: destinationID, privacy: githubsync.VerifiedNonPublic, snapshot: githubsync.RepositorySnapshot{State: githubsync.RepositoryInitialized, ManagedDestinationID: destinationID}, want: "Remote Doctor: OK"},
		{name: "public", localID: destinationID, privacy: githubsync.VerifiedPublic, snapshot: githubsync.RepositorySnapshot{State: githubsync.RepositoryInitialized, ManagedDestinationID: destinationID}, want: "repository is public"},
		{name: "unverifiable", localID: destinationID, privacy: githubsync.Unverifiable, privacyErr: githubsync.ErrPrivacyUnverifiable, want: "privacy could not be verified"},
		{name: "missing", localID: destinationID, privacy: githubsync.VerifiedNonPublic, inspectErr: githubsync.ErrTransportUnavailable, want: "missing or inaccessible"},
		{name: "adoption", privacy: githubsync.VerifiedNonPublic, snapshot: githubsync.RepositorySnapshot{State: githubsync.RepositoryInitialized, ManagedDestinationID: destinationID}, want: "explicit adoption is required"},
		{name: "mismatch", localID: destinationID, privacy: githubsync.VerifiedNonPublic, snapshot: githubsync.RepositorySnapshot{State: githubsync.RepositoryInitialized, ManagedDestinationID: "dst_abcdef0123456789abcdef0123456789"}, want: "identity mismatch"},
		{name: "empty unclaimed", privacy: githubsync.VerifiedNonPublic, snapshot: githubsync.RepositorySnapshot{State: githubsync.RepositoryEmpty}, want: "Repository state: Empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preserveManagedRuntime(t)
			s := store.New(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			state := managedCLIState(test.localID)
			if err := s.WriteSync(state); err != nil {
				t.Fatal(err)
			}
			managedGitDiscovery = func() (githubsync.GitRunner, error) { return githubsync.GitRunner{}, nil }
			managedClassify = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.PrivacyClassification, error) {
				return test.privacy, test.privacyErr
			}
			managedInspect = func(context.Context, githubsync.GitRunner, githubsync.Locator) (githubsync.RepositorySnapshot, error) {
				return test.snapshot, test.inspectErr
			}
			var out bytes.Buffer
			err := runRemoteDoctor(s, &out)
			combined := out.String()
			if err != nil {
				combined += err.Error()
			}
			if !strings.Contains(combined, test.want) {
				t.Fatalf("diagnostic missing %q, err=%v out=%s", test.want, err, out.String())
			}
		})
	}
}

func TestDoctorRejectsUnknownOptions(t *testing.T) {
	s := store.New(t.TempDir())
	if err := runDoctor(s, []string{"--write"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("unknown doctor option accepted: %v", err)
	}
}
