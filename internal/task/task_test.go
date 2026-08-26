package task

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

func TestTaskCheckpointAndHandoff(t *testing.T) {
	s := store.New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	w := store.Workspace{ID: "w_test", Name: "repo", Sync: true, CreatedAt: store.Now(), UpdatedAt: store.Now()}
	if err := s.WriteWorkspace(w); err != nil {
		t.Fatal(err)
	}
	got, err := Start(s, w, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "test-task" {
		t.Fatalf("unexpected task ID: %s", got.ID)
	}
	if _, err := Start(s, w, "Test Task"); err == nil || !strings.Contains(err.Error(), "task already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	device := store.Device{ID: "d_test"}
	if err := AddCheckpoint(s, w, device, "first"); err != nil {
		t.Fatal(err)
	}
	if err := AddCheckpoint(s, w, device, "second"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.CheckpointsPath(w.ID, got.ID))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("checkpoint count = %d", len(lines))
	}
	var cp Checkpoint
	if err := json.Unmarshal([]byte(lines[0]), &cp); err != nil {
		t.Fatal(err)
	}
	if cp.Message != "first" || cp.DeviceID != "d_test" {
		t.Fatalf("bad checkpoint: %#v", cp)
	}
	path, err := EnsureHandoff(s, w)
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(handoff), "## Next Steps") {
		t.Fatalf("handoff template missing section:\n%s", handoff)
	}
}
