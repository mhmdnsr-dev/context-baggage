package githubsync

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
)

const managedRef = "refs/heads/context-baggage"

// RepositoryState is the complete successful read-only classification.
type RepositoryState string

const (
	// RepositoryEmpty means the remote advertised no refs and therefore no
	// reachable commit. An empty-tree commit still has a ref and is not Empty.
	RepositoryEmpty RepositoryState = "Empty"
	// RepositoryInitialized means the sole ref and its immutable commit satisfy
	// the complete managed marker and portable-state contract.
	RepositoryInitialized RepositoryState = "Initialized"
)

// RepositorySnapshot records one immutable remote observation. Commit identity
// pins transport history; PortableHash identifies only provider-independent v2.
type RepositorySnapshot struct {
	RepositoryIdentity   string
	State                RepositoryState
	CommitID             string
	ManagedDestinationID string
	PortableHash         string
	PortablePresent      bool
}

// InspectRepository validates target binding, observes refs once, and inspects
// only the exact managed commit observed in that operation.
func InspectRepository(ctx context.Context, git GitRunner, locator Locator) (RepositorySnapshot, error) {
	if !locator.valid() {
		return RepositorySnapshot{}, ErrInvalidLocator
	}
	if err := git.VerifyTargetBinding(ctx, locator); err != nil {
		return RepositorySnapshot{}, err
	}
	commitID, empty, err := git.observeManagedRef(ctx, locator.url)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	if empty {
		return RepositorySnapshot{RepositoryIdentity: locator.identity, State: RepositoryEmpty}, nil
	}
	return git.inspectObservedCommit(ctx, locator.url, locator.identity, commitID)
}

func (g GitRunner) observeManagedRef(ctx context.Context, remoteURL string) (string, bool, error) {
	// --refs excludes symbolic HEAD transport metadata and peeled tag pseudo-refs.
	// Every advertised ordinary ref is still parsed, and only the fixed managed
	// branch is accepted because this repository must remain dedicated.
	collector := &remoteRefCollector{}
	err := g.runNetworkStream(ctx, g.readTimeout, "", collector, "ls-remote", "--refs", "--quiet", remoteURL)
	if err != nil {
		return "", false, err
	}
	if err := collector.finish(); err != nil {
		return "", false, err
	}
	if collector.refName == "" {
		return "", true, nil
	}
	return collector.objectID, false, nil
}

type remoteRefCollector struct {
	pending  bytes.Buffer
	refName  string
	objectID string
	err      error
}

func (collector *remoteRefCollector) Write(data []byte) (int, error) {
	written := len(data)
	if collector.err != nil {
		return written, nil
	}
	_, _ = collector.pending.Write(data)
	for {
		separator := bytes.IndexByte(collector.pending.Bytes(), '\n')
		if separator < 0 {
			break
		}
		line := collector.pending.Next(separator + 1)
		if err := collector.add(strings.TrimSuffix(string(line), "\n")); err != nil {
			collector.err = err
			break
		}
	}
	if collector.pending.Len() > maxRelativePathBytes+256 {
		collector.pending.Reset()
		collector.err = ErrResourceLimitExceeded
	}
	return written, nil
}

func (collector *remoteRefCollector) add(line string) error {
	objectID, refName, ok := strings.Cut(strings.TrimSuffix(line, "\r"), "\t")
	if !ok || !validGitObjectID(objectID) || refName != managedRef || collector.refName != "" {
		return ErrRepositoryIncompatible
	}
	collector.objectID, collector.refName = objectID, refName
	return nil
}

func (collector *remoteRefCollector) finish() error {
	if collector.err != nil {
		return collector.err
	}
	if collector.pending.Len() != 0 {
		return ErrRepositoryIncompatible
	}
	return nil
}

// inspectObservedCommit fetches the observed object ID rather than the ref, so
// later remote movement cannot silently change the snapshot under inspection.
func (g GitRunner) inspectObservedCommit(ctx context.Context, remoteURL, repositoryIdentity, commitID string) (RepositorySnapshot, error) {
	root, err := os.MkdirTemp("", "ctx-bag-managed-read-*")
	if err != nil {
		return RepositorySnapshot{}, ErrTransportUnavailable
	}
	defer func() { _ = os.RemoveAll(root) }()
	gitDir := filepath.Join(root, "repository.git")
	if _, err := g.run(ctx, g.inspectionTimeout, "", "init", "--bare", "--quiet", gitDir); err != nil {
		return RepositorySnapshot{}, err
	}
	if err := appendManagedReadRemote(gitDir, remoteURL); err != nil {
		return RepositorySnapshot{}, err
	}
	if err := g.fetchObservedCommit(ctx, root, gitDir, commitID); err != nil {
		return RepositorySnapshot{}, err
	}
	temporaryBytes, err := temporaryUsage(root)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	if !temporarySizeAllowed(temporaryBytes, 0) {
		return RepositorySnapshot{}, ErrResourceLimitExceeded
	}
	if _, err := g.run(ctx, g.inspectionTimeout, gitDir, "cat-file", "-e", commitID+"^{commit}"); err != nil {
		return RepositorySnapshot{}, ErrRepositoryIncompatible
	}
	entries, err := g.readManagedTree(ctx, gitDir, commitID)
	if err != nil {
		return RepositorySnapshot{}, err
	}
	if err := validateCompleteTree(entries); err != nil {
		return RepositorySnapshot{}, err
	}
	return g.materializeAndValidate(ctx, root, gitDir, repositoryIdentity, commitID, entries)
}

// fetchObservedCommit obtains commit and tree objects without eagerly receiving
// repository blobs. The active guard bounds even malicious metadata transfers.
func (g GitRunner) fetchObservedCommit(ctx context.Context, root, gitDir, commitID string) error {
	return g.runNetworkGuarded(ctx, g.readTimeout, gitDir, root, g.temporaryByteLimit(),
		"fetch", "--no-tags", "--depth=1", "--filter=blob:none", "--quiet", targetRemoteName, commitID)
}

func (g GitRunner) readManagedTree(ctx context.Context, gitDir, commitID string) ([]gitTreeEntry, error) {
	collector := &treeCollector{}
	if err := g.runStream(ctx, g.readTimeout, gitDir, collector, nil, "ls-tree", "-r", "-t", "-z", "--full-tree", commitID); err != nil {
		return nil, err
	}
	if err := collector.finish(); err != nil {
		return nil, err
	}
	return collector.entries, nil
}
