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

func TestPortableWorkspaceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := PortableWorkspace{
		ID: "w_shared", Name: "example",
		Identity:  WorkspaceIdentity{Type: "git-remote", Value: "example.com/org/repo"},
		Sync:      true,
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	if err := WritePortableWorkspace(dir, p); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPortableWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("round-trip mismatch: %+v != %+v", got, p)
	}
}

func TestApplyPortablePreservesLocalFields(t *testing.T) {
	w := Workspace{ID: "w_old", Name: "old", LocalPaths: []string{"/machine/foo"}, Sync: false, CreatedAt: "old", UpdatedAt: "u1"}
	p := PortableWorkspace{ID: "w_target", Name: "target", Identity: WorkspaceIdentity{Type: "local-directory"}, Sync: true, CreatedAt: "2026-01-01T00:00:00Z"}
	got := w.ApplyPortable(p)
	if got.ID != "w_target" || got.Name != "target" || !got.Sync || got.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("portable fields not applied: %+v", got)
	}
	if len(got.LocalPaths) != 1 || got.LocalPaths[0] != "/machine/foo" {
		t.Fatalf("LocalPaths not preserved: %+v", got.LocalPaths)
	}
	if got.UpdatedAt != "u1" {
		t.Fatalf("UpdatedAt not preserved: %q", got.UpdatedAt)
	}
}
