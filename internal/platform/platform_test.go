package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppHomeOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONTEXT_BAGGAGE_HOME", filepath.Join(dir, "bag"))
	got, err := AppHome()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "bag") {
		t.Fatalf("AppHome() = %q", got)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatalf("AppHome should not create directories")
	}
}
