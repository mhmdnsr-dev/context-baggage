package sync

import (
	"errors"
	"os"
	"path/filepath"
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

func TestInitRefusesWhileRecoveryPending(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	first := t.TempDir()
	if _, err := Init(s, first); err != nil {
		t.Fatal(err)
	}
	before, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WritePullRecovery(testRecoveryRecord()); err != nil {
		t.Fatal(err)
	}
	_, err = Init(s, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "recovery is pending") {
		t.Fatalf("expected recovery pending refusal, got %v", err)
	}
	after, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("active destination changed despite refusal: %+v -> %+v", before, after)
	}
	if exists, err := s.PullRecoveryExists(); err != nil || !exists {
		t.Fatalf("recovery record changed on refusal: exists=%t err=%v", exists, err)
	}
}

func TestEmptyPortableIdentityConsistent(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	eligible, err := EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	push, err := BuildPushSnapshot(s, dest)
	if err != nil {
		t.Fatal(err)
	}
	bounded, err := BuildPushSnapshotBounded(s, t.TempDir(), 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	emptyDir := t.TempDir()
	emptyHash, err := store.HashDir(emptyDir)
	if err != nil {
		t.Fatal(err)
	}
	if eligible != push || bounded != emptyHash || eligible != emptyHash {
		t.Fatalf("empty identity mismatch: eligible=%s push=%s bounded=%s empty=%s", eligible, push, bounded, emptyHash)
	}
}

func TestEmptyPortableIdentityBaseBinding(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	emptyHash, err := EligibleHash(s)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if err := BindBaseToActiveDestination(&state, emptyHash); err != nil {
		t.Fatal(err)
	}
	if !state.BasePresent || state.BaseHash != emptyHash {
		t.Fatalf("BASE not bound to empty identity: %+v", state)
	}
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if !got.BasePresent || got.BaseHash != emptyHash {
		t.Fatalf("BASE lost after write: %+v", got)
	}
	state.BasePresent = false
	state.BaseHash = ""
	state.BaseDestinationType = ""
	state.BaseDestinationIdentity = ""
	if err := s.WriteSync(state); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSync(); err != nil {
		t.Fatal(err)
	}
}

func TestFilesystemEmptyPushPullSemantics(t *testing.T) {
	folder := t.TempDir()
	a, _ := newStore(t)
	if _, err := Init(a, folder); err != nil {
		t.Fatal(err)
	}
	hash, err := Push(a)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("empty filesystem push produced an empty hash")
	}
	b, _ := newStore(t)
	if _, err := Init(b, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(b); err != nil {
		t.Fatal(err)
	}
	state, err := b.ReadSync()
	if err != nil {
		t.Fatal(err)
	}
	if !state.BasePresent || state.BaseHash != hash {
		t.Fatalf("filesystem empty pull did not bind BASE: %+v", state)
	}
}

func TestRecoveryRecordPathIsMachineLocal(t *testing.T) {
	s, folder := newStore(t)
	if _, err := Init(s, folder); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePullRecovery(testRecoveryRecord()); err != nil {
		t.Fatal(err)
	}
	path := s.PullRecoveryPath()
	if filepath.Dir(path) != filepath.Join(s.Home, "sync") {
		t.Fatalf("recovery record path not machine-local sync dir: %s", path)
	}
	if _, err := os.Stat(filepath.Join(folder, exportDirV2, "pull-recovery.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery record leaked into portable export: %v", err)
	}
}
