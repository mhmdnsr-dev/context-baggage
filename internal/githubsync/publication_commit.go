package githubsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const publicationCommitMessage = "Context Baggage sync"

type preparedPublication struct {
	workDir       string
	commitID      string
	destinationID string
	portableHash  string
	expectedID    string
}

func (g GitRunner) preparePublication(ctx context.Context, root, localDir, remoteURL, parentID, destinationID, portableHash string) (preparedPublication, error) {
	workDir := filepath.Join(root, "publication")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		return preparedPublication{}, ErrTransportUnavailable
	}
	if _, err := g.runLocalGuarded(ctx, g.inspectionTimeout, "", root, "init", "--quiet", workDir); err != nil {
		return preparedPublication{}, err
	}
	if err := appendManagedReadRemote(filepath.Join(workDir, ".git"), remoteURL); err != nil {
		return preparedPublication{}, err
	}
	if parentID != "" {
		if err := g.fetchPublicationParent(ctx, root, workDir, parentID); err != nil {
			return preparedPublication{}, err
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, managedMarkerName), managedMarkerContents(destinationID), 0o600); err != nil {
		return preparedPublication{}, ErrTransportUnavailable
	}
	present, err := portableSnapshotPresent(localDir)
	if err != nil {
		return preparedPublication{}, ErrTransportUnavailable
	}
	if present {
		if err := os.Rename(localDir, filepath.Join(workDir, portableRootName)); err != nil {
			return preparedPublication{}, ErrTransportUnavailable
		}
	}
	if _, err := g.runLocalGuarded(ctx, g.readTimeout, workDir, root, "add", "--all", "--", "."); err != nil {
		return preparedPublication{}, err
	}
	tree, err := g.runLocalGuarded(ctx, g.inspectionTimeout, workDir, root, "write-tree")
	if err != nil {
		return preparedPublication{}, err
	}
	treeID := strings.TrimSpace(tree.stdout)
	args := []string{"-c", "commit.gpgSign=false", "commit-tree", treeID, "-m", publicationCommitMessage}
	if parentID != "" {
		args = append(args, "-p", parentID)
	}
	commit, err := g.runLocalGuarded(ctx, g.inspectionTimeout, workDir, root, args...)
	if err != nil {
		return preparedPublication{}, err
	}
	commitID := strings.TrimSpace(commit.stdout)
	if !validGitObjectID(commitID) {
		return preparedPublication{}, ErrTransportUnavailable
	}
	return preparedPublication{
		workDir: workDir, commitID: commitID, destinationID: destinationID,
		portableHash: portableHash, expectedID: parentID,
	}, nil
}

func (g GitRunner) fetchPublicationParent(ctx context.Context, root, workDir, parentID string) error {
	return g.runNetworkGuarded(ctx, g.readTimeout, workDir, root, g.temporaryByteLimit(),
		"fetch", "--no-tags", "--depth=1", "--filter=blob:none", "--quiet", targetRemoteName, parentID)
}

func (g GitRunner) pushPrepared(ctx context.Context, root string, prepared preparedPublication) error {
	lease := "--force-with-lease=" + managedRef + ":" + prepared.expectedID
	refspec := prepared.commitID + ":" + managedRef
	return g.runNetworkGuarded(ctx, g.readTimeout, prepared.workDir, root, g.temporaryByteLimit(),
		"push", "--no-verify", "--porcelain", lease, targetRemoteName, refspec)
}
