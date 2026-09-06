package app

import (
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func testRecoveryRecord() store.PullRecovery {
	return store.PullRecovery{
		Format:             store.PullRecoveryFormatVersion,
		DestinationType:    store.DestinationGitHub,
		DestinationID:      "dst_0123456789abcdef0123456789abcdef",
		RepositoryIdentity: "github.com/neutral/repository",
		TargetCommitID:     strings.Repeat("a", 40),
		PreLocalHash:       strings.Repeat("b", 64),
		TargetPortableHash: strings.Repeat("c", 64),
	}
}

func TestBlockIfRecoveryPending(t *testing.T) {
	home := t.TempDir()
	s := store.New(home)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := blockIfRecoveryPending(s); err != nil {
		t.Fatalf("no recovery pending should be allowed: %v", err)
	}
	if err := s.WritePullRecovery(testRecoveryRecord()); err != nil {
		t.Fatal(err)
	}
	if err := blockIfRecoveryPending(s); err == nil || !strings.Contains(err.Error(), "recovery is required") {
		t.Fatalf("expected recovery pending refusal, got %v", err)
	}
}

func TestRecoveryPendingBlocksCanonicalMutators(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	runCLI(t, home, dir, "init")
	runCLI(t, home, dir, "workspace", "init", "--sync")
	runCLI(t, home, dir, "sync", "init", t.TempDir())
	if err := store.New(home).WritePullRecovery(testRecoveryRecord()); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLIErr(t, home, dir, "task", "start", "foo"); err == nil || !strings.Contains(err.Error(), "recovery is required") {
		t.Fatalf("task start did not refuse: %v", err)
	}
	if _, err := runCLIErr(t, home, dir, "checkpoint", "-m", "msg"); err == nil || !strings.Contains(err.Error(), "recovery is required") {
		t.Fatalf("checkpoint did not refuse: %v", err)
	}
	if out := runCLI(t, home, dir, "workspace", "available"); !strings.Contains(out, "Available portable workspaces") {
		t.Fatalf("filesystem workspace available unexpectedly blocked: %s", out)
	}
}
