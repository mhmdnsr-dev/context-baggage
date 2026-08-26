package claude

import (
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/agents"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

type Adapter struct{}

func (Adapter) Key() string  { return "claude" }
func (Adapter) Name() string { return "Claude Code" }

func (Adapter) Discover(home string) store.AgentInventory {
	inv := store.AgentInventory{Name: "Claude Code", Metadata: map[string]string{}, UpdatedAt: store.Now()}
	candidates := []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "CLAUDE.md"),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			inv.Detected = true
			inv.ConfigPaths = append(inv.ConfigPaths, p)
			inv.Metadata[filepath.Base(p)] = agents.RedactKeyValue(filepath.Base(p), agents.CountLikelyServers(string(data)))
		}
	}
	if st, err := os.Stat(filepath.Join(home, ".claude", "skills")); err == nil && st.IsDir() {
		inv.Detected = true
		inv.ConfigPaths = append(inv.ConfigPaths, filepath.Join(home, ".claude", "skills"))
		inv.Metadata["skills"] = "directory detected"
	}
	return inv
}
