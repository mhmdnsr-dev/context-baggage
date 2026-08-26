package agents

import "github.com/mhmdnsr-dev/context-baggage/internal/store"

type Adapter interface {
	Key() string
	Name() string
	Discover(home string) store.AgentInventory
}
