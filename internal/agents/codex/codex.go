package codex

import (
	"os"
	"path/filepath"

	"github.com/mhmdnsr-dev/context-baggage/internal/agents"
	"github.com/mhmdnsr-dev/context-baggage/internal/store"
)

type Adapter struct{}

func (Adapter) Key() string  { return "codex" }
func (Adapter) Name() string { return "Codex" }

func (Adapter) Discover(home string) store.AgentInventory {
	inv := store.AgentInventory{Name: "Codex", Metadata: map[string]string{}, UpdatedAt: store.Now()}
	candidates := []string{
		filepath.Join(home, ".codex", "config.toml"),
		filepath.Join(home, ".codex", "AGENTS.md"),
		filepath.Join(home, ".codex", "instructions.md"),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			inv.Detected = true
			inv.ConfigPaths = append(inv.ConfigPaths, p)
			inv.Metadata[filepath.Base(p)] = agents.RedactKeyValue(filepath.Base(p), agents.CountLikelyServers(string(data)))
		}
	}
	if st, err := os.Stat(filepath.Join(home, ".codex", "skills")); err == nil && st.IsDir() {
		inv.Detected = true
		inv.ConfigPaths = append(inv.ConfigPaths, filepath.Join(home, ".codex", "skills"))
		inv.Metadata["skills"] = "directory detected"
	}
	return inv
}
