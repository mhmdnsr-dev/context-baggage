package store

type Device struct {
	ID        string
	Name      string
	OS        string
	Arch      string
	CreatedAt string
}

type Config struct {
	Version string
}

type WorkspaceIdentity struct {
	Type  string
	Value string
}

type Workspace struct {
	ID         string
	Name       string
	Identity   WorkspaceIdentity
	LocalPaths []string
	Sync       bool
	CreatedAt  string
	UpdatedAt  string
}

// PortableWorkspace is the explicit allowlist of workspace fields that
// participate in portable shared state. Machine-local fields such as
// LocalPaths and UpdatedAt are intentionally absent.
type PortableWorkspace struct {
	ID        string
	Name      string
	Identity  WorkspaceIdentity
	Sync      bool
	CreatedAt string
}

type Task struct {
	ID          string
	Name        string
	WorkspaceID string
	Status      string
	CreatedAt   string
	UpdatedAt   string
}

type SyncState struct {
	Folder       string
	LastPush     string
	LastPull     string
	LastPushHash string
	LastPullHash string
	BaseHash     string
}

type AgentInventory struct {
	Name        string
	Detected    bool
	ConfigPaths []string
	Metadata    map[string]string
	Warnings    []string
	UpdatedAt   string
}
