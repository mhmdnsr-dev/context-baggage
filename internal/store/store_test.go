package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndDeviceIdempotent(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	d := Device{ID: "d_test", Name: "host", OS: "linux", Arch: "amd64", CreatedAt: Now()}
	if err := s.WriteDevice(d); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadDevice()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != d.ID {
		t.Fatalf("device overwritten: %q", got.ID)
	}
	for _, rel := range []string{"config.yaml", "inventory/claude", "inventory/codex", "workspaces", "sync"} {
		if _, err := os.Stat(filepath.Join(s.Home, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestMalformedWorkspace(t *testing.T) {
	s := New(t.TempDir())
	path := s.WorkspacePath("w_bad")
	if err := AtomicWrite(path, []byte("name: bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.ReadWorkspace("w_bad")
	if err == nil || !strings.Contains(err.Error(), "workspace id missing") {
		t.Fatalf("expected malformed workspace error, got %v", err)
	}
}
