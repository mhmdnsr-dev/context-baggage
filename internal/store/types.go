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

// DestinationType identifies the configured sync destination implementation.
type DestinationType string

const (
	// DestinationFilesystem stores portable state in a configured local folder.
	DestinationFilesystem DestinationType = "filesystem"
	// DestinationGitHub reserves the managed GitHub destination state shape.
	DestinationGitHub DestinationType = "github"
)

// PrivacyClassification records the last managed-destination privacy result.
type PrivacyClassification string

const (
	// PrivacyVerifiedPublic records a destination proven publicly accessible.
	PrivacyVerifiedPublic PrivacyClassification = "verified-public"
	// PrivacyVerifiedNonPublic records a destination proven non-public.
	PrivacyVerifiedNonPublic PrivacyClassification = "verified-non-public"
	// PrivacyUnverifiable records a privacy result that could not be proven.
	PrivacyUnverifiable PrivacyClassification = "unverifiable"
)

// SyncStateFormatVersion is the current machine-local sync state format.
const SyncStateFormatVersion = 1

// SyncState contains the active destination configuration and machine-local
// synchronization bookkeeping. It is never part of portable state.
type SyncState struct {
	FormatVersion             int
	DestinationType           DestinationType
	Folder                    string
	GitHubLocator             string
	GitHubRepository          string
	ManagedDestinationID      string
	LastPush                  string
	LastPull                  string
	LastPushHash              string
	LastPullHash              string
	LastObservedRemoteHash    string
	LastRefresh               string
	PrivacyClassification     PrivacyClassification
	PrivacyCheckedAt          string
	PrivacyRepositoryIdentity string
	BasePresent               bool
	BaseHash                  string
	BaseDestinationType       DestinationType
	BaseDestinationIdentity   string
}

type AgentInventory struct {
	Name        string
	Detected    bool
	ConfigPaths []string
	Metadata    map[string]string
	Warnings    []string
	UpdatedAt   string
}
