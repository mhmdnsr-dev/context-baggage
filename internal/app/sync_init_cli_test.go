package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/githubsync"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestSyncInitLiteralGitHubIsFilesystem(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "github"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	runCLI(t, home, root, "init")
	runCLI(t, home, root, "sync", "init", "github")
	state, err := store.New(home).ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if state.DestinationType != store.DestinationFilesystem || state.Folder != filepath.Join(root, "github") {
		t.Fatalf("literal github parsed as managed destination: %+v", state)
	}
}

func TestSyncInitGitHubLocatorDispatch(t *testing.T) {
	preserveManagedRuntime(t)
	managedGitDiscovery = func() (githubsync.GitRunner, error) { return githubsync.GitRunner{}, nil }
	tests := []struct {
		raw     string
		replace bool
	}{
		{raw: "https://github.com/owner/repo"},
		{raw: "https://github.com/owner/repo.git", replace: true},
		{raw: "git@github.com:owner/repo.git"},
	}
	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			gotRaw, gotReplace := "", false
			managedConfigure = func(_ context.Context, _ store.Store, _ githubsync.GitRunner, raw string, replace bool) (store.SyncState, error) {
				gotRaw, gotReplace = raw, replace
				return store.SyncState{GitHubRepository: "github.com/owner/repo"}, nil
			}
			args := []string{"init", "github", test.raw}
			if test.replace {
				args = append(args, "--replace")
			}
			if err := runSync(initializedStore(t), args, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if gotRaw != test.raw || gotReplace != test.replace {
				t.Fatalf("managed init dispatch = %q/%t", gotRaw, gotReplace)
			}
		})
	}
}

func TestFilesystemCLIReplacementSemantics(t *testing.T) {
	s := initializedStore(t)
	first, second := t.TempDir(), t.TempDir()
	if err := runSync(s, []string{"init", first}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	state, _ := s.ReadSync()
	state.BasePresent = true
	state.BaseHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	state.BaseDestinationType = store.DestinationFilesystem
	state.BaseDestinationIdentity = state.Folder
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	if err := runSync(s, []string{"init", first, "--replace"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	state, _ = s.ReadSync()
	if !state.BasePresent {
		t.Fatal("same filesystem destination with --replace cleared BASE")
	}
	if err := runSync(s, []string{"init", second}, &bytes.Buffer{}); err == nil {
		t.Fatal("different filesystem destination accepted without --replace")
	}
	state, _ = s.ReadSync()
	if state.Folder != first || !state.BasePresent {
		t.Fatalf("refused replacement changed state: %+v", state)
	}
	if err := runSync(s, []string{"init", second, "--replace"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	state, _ = s.ReadSync()
	if state.Folder != second || state.BasePresent {
		t.Fatalf("authorized replacement retained old binding: %+v", state)
	}
}
