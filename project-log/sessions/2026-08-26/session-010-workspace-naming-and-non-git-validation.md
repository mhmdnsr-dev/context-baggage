# Session 010 — Workspace Naming And Non-Git Validation

## Objective

Fix and validate the pre-release workspace metadata issue where a Git workspace display name came from the first machine's local checkout directory instead of the portable Git repository identity.

Also validate that Git repositories without remotes and plain non-Git directories can still initialize as workspaces without treating absolute paths or matching basenames as portable cross-machine identity.

## Existing Behavior Inspection

Source inspected:

- `internal/workspace/workspace.go`
- `internal/workspace/workspace_test.go`
- `internal/app/app.go`
- `docs/v0.1/data-model.md`
- `docs/v0.1/architecture.md`
- `docs/v0.1/requirements.md`
- `PROJECT_STATUS.md`

Existing workspace name creation:

```go
base := filepath.Base(root)
id := store.StableID("w", typ+":"+value)
return Resolved{Root: root, Name: base, Identity: store.WorkspaceIdentity{Type: typ, Value: value}, ID: id}, nil
```

Existing workspace ID creation:

- Git remote workspaces used `store.StableID("w", "git-remote:"+NormalizeRemote(remote))`.
- Git repositories without a remote used `store.StableID("w", "git-local:"+root)`, which tied the ID to the absolute local Git root.
- Non-Git directories were not supported because `git rev-parse --show-toplevel` failure returned an error.

Existing Git remote normalization:

- SSH form such as `git@example.com:org/example-repo.git` normalizes to `example.com/org/example-repo`.
- HTTPS form such as `https://example.com/org/example-repo.git` normalizes to `example.com/org/example-repo`.
- Credentials are stripped before persisting or displaying remote identity.

Existing non-Git workspace identity:

- No non-Git workspace identity existed.
- `ctx-bag workspace init` failed outside a Git repository.

## Root Cause

The workspace resolver always set:

```go
Name: filepath.Base(root)
```

For a Git workspace with a usable remote, the workspace ID correctly came from normalized remote identity, but the display name still came from the local Git root basename.

During sync, `workspace.yaml` is portable state. Machine A initialized:

```text
Git root: /tmp/ctx-bag-machine-a
Name: ctx-bag-machine-a
Identity: git-remote:example.com/org/example-repo
```

Machine B then pulled that portable workspace metadata and displayed Machine A's local folder basename as the workspace name even though Machine B's local Git root was different.

## Naming Rule Implemented

The implemented decision tree is:

```text
IF usable Git remote exists:
    identity.type = git-remote
    identity.value = normalized remote identity
    id = stable ID from git-remote:<normalized identity>
    name = final path segment from normalized remote identity

ELSE IF Git repository exists without usable remote:
    identity.type = git-local
    identity.value = empty
    id = generated Context Baggage workspace ID at initialization
    name = Git root basename

ELSE:
    identity.type = local-directory
    identity.value = empty
    id = generated Context Baggage workspace ID at initialization
    name = current directory basename
```

## Identity Behavior

### Git with remote

Git remote identity remains the portable identity input. Equivalent SSH/HTTPS remotes normalize to the same value and produce the same workspace ID.

Example:

```text
Remote: https://example.com/org/example-repo.git
Identity: git-remote:example.com/org/example-repo
Name: example-repo
ID: w_a49d7498d9efa7f4
```

### Git without remote

There is no portable repository identity. Context Baggage now generates a workspace ID at initialization and resolves that workspace locally by recorded local path.

The Git root basename is display metadata only.

### Non-Git folder

There is no Git repository identity. Context Baggage generates a workspace ID at initialization and resolves that workspace locally by recorded local path.

The folder basename is display metadata only.

Two non-Git directories with the same basename are not automatically linked.

## Code Changes

- `internal/workspace/workspace.go`
  - Derived Git workspace display names from normalized remote identity.
  - Added local-only resolution for Git repositories without remotes.
  - Added non-Git directory resolution.
  - Added generated workspace IDs for local-only workspaces instead of deriving IDs from absolute paths.
  - Added local-path lookup for current local-only workspaces.

- `internal/workspace/workspace_test.go`
  - Added focused tests for HTTPS remote naming.
  - Added focused tests for SSH remote naming.
  - Extended same-remote/different-path coverage to verify same ID, same name, and different local roots.
  - Added Git-without-remote coverage.
  - Added non-Git folder coverage.
  - Added no basename-only auto-link coverage.

- `internal/app/app.go`
  - Changed `workspace status` to print `Workspace root` for non-Git workspaces instead of incorrectly labeling the path as `Git root`.

- `docs/v0.1/data-model.md`
  - Documented workspace ID, display name, repository identity, and local path as separate concepts.
  - Documented `git-remote`, `git-local`, and `local-directory` behavior.

- `docs/v0.1/architecture.md`
  - Documented workspace identity/display-name separation and non-Git limitations.

- `docs/v0.1/requirements.md`
  - Added testable requirements for Git remote-derived names, no-remote Git workspaces, non-Git directories, and no basename-only auto-linking.

- `PROJECT_STATUS.md`
  - Updated current project status after successful validation.

## Automated Tests

### Focused workspace tests

Command:

```bash
go test ./internal/workspace -run 'TestGitHTTPSRemoteNameAndIdentity|TestGitSSHRemoteNameAndIdentity|TestWorkspaceIDIgnoresLocalPathForRemote|TestGitRepositoryWithoutRemoteUsesLocalDisplayNameAndLocalOnlyIdentity|TestNonGitFolderInitializesAndUsesBasename|TestNonGitSameBasenameDoesNotAutoLink' -v
```

Output:

```text
=== RUN   TestGitHTTPSRemoteNameAndIdentity
--- PASS: TestGitHTTPSRemoteNameAndIdentity (0.01s)
=== RUN   TestGitSSHRemoteNameAndIdentity
--- PASS: TestGitSSHRemoteNameAndIdentity (0.01s)
=== RUN   TestWorkspaceIDIgnoresLocalPathForRemote
--- PASS: TestWorkspaceIDIgnoresLocalPathForRemote (0.01s)
=== RUN   TestGitRepositoryWithoutRemoteUsesLocalDisplayNameAndLocalOnlyIdentity
--- PASS: TestGitRepositoryWithoutRemoteUsesLocalDisplayNameAndLocalOnlyIdentity (0.01s)
=== RUN   TestNonGitFolderInitializesAndUsesBasename
--- PASS: TestNonGitFolderInitializesAndUsesBasename (0.00s)
=== RUN   TestNonGitSameBasenameDoesNotAutoLink
--- PASS: TestNonGitSameBasenameDoesNotAutoLink (0.00s)
PASS
ok  	github.com/context-baggage/context-baggage/internal/workspace	0.045s
```

### Full test suite

Command:

```bash
go test ./...
```

Output:

```text
?   	github.com/context-baggage/context-baggage/cmd/ctx-bag	[no test files]
?   	github.com/context-baggage/context-baggage/internal/agents/claude	[no test files]
?   	github.com/context-baggage/context-baggage/internal/agents/codex	[no test files]
ok  	github.com/context-baggage/context-baggage/internal/agents	(cached)
?   	github.com/context-baggage/context-baggage/internal/config	[no test files]
ok  	github.com/context-baggage/context-baggage/internal/app	(cached)
ok  	github.com/context-baggage/context-baggage/internal/platform	(cached)
ok  	github.com/context-baggage/context-baggage/internal/store	(cached)
ok  	github.com/context-baggage/context-baggage/internal/sync	(cached)
ok  	github.com/context-baggage/context-baggage/internal/task	(cached)
ok  	github.com/context-baggage/context-baggage/internal/workspace	(cached)
```

## Manual Validation — Git Workspace

Validation used the rebuilt binary:

```bash
<repo-root>/bin/ctx-bag --help
```

Output:

```text
ctx-bag

Commands:
  init
  status
  doctor
  discover
  workspace init [--sync|--no-sync]
  workspace status
  task start <name>
  task status
  task resume <name>
  checkpoint -m <message>
  handoff
  sync init <folder>
  sync status
  sync push
  sync pull
```

Clean validation state:

```bash
rm -rf /tmp/ctx-bag-name-test-a /tmp/ctx-bag-name-test-b /tmp/ctx-bag-name-test-home-a /tmp/ctx-bag-name-test-home-b /tmp/ctx-bag-local-folder /tmp/ctx-bag-local-folder-home /tmp/ctx-bag-local-git /tmp/ctx-bag-local-git-home
mkdir -p /tmp/ctx-bag-name-test-a /tmp/ctx-bag-name-test-b /tmp/ctx-bag-local-folder /tmp/ctx-bag-local-git
```

### Git workspace A

Command:

```bash
git -C /tmp/ctx-bag-name-test-a init
```

Output:

```text
Initialized empty Git repository in /tmp/ctx-bag-name-test-a/.git/
```

Command:

```bash
git -C /tmp/ctx-bag-name-test-a remote add origin https://example.com/org/example-repo.git
```

Output:

```text
```

Command:

```bash
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-a <repo-root>/bin/ctx-bag init
```

Output:

```text
Context Baggage initialized
Home: /tmp/ctx-bag-name-test-home-a
Device: d_5ff3091e619c682d83c185e27568d7da
```

Command:

```bash
cd /tmp/ctx-bag-name-test-a && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-a <repo-root>/bin/ctx-bag workspace init
```

Output:

```text
Workspace initialized
Name: example-repo
ID: w_a49d7498d9efa7f4
Identity: git-remote:example.com/org/example-repo
Sync: false
```

Command:

```bash
cd /tmp/ctx-bag-name-test-a && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-a <repo-root>/bin/ctx-bag workspace status
```

Output:

```text
Workspace
Name: example-repo
ID: w_a49d7498d9efa7f4
Git root: /tmp/ctx-bag-name-test-a
Identity: git-remote:example.com/org/example-repo
Sync: false
```

Workspace YAML:

```text
id: w_a49d7498d9efa7f4
name: example-repo
identity:
  type: git-remote
  value: example.com/org/example-repo
localPaths:
  - /tmp/ctx-bag-name-test-a
sync: false
createdAt: 2026-08-26T12:58:55Z
updatedAt: 2026-08-26T12:58:55Z
```

### Git workspace B

Command:

```bash
git -C /tmp/ctx-bag-name-test-b init
```

Output:

```text
Initialized empty Git repository in /tmp/ctx-bag-name-test-b/.git/
```

Command:

```bash
git -C /tmp/ctx-bag-name-test-b remote add origin https://example.com/org/example-repo.git
```

Output:

```text
```

Command:

```bash
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-b <repo-root>/bin/ctx-bag init
```

Output:

```text
Context Baggage initialized
Home: /tmp/ctx-bag-name-test-home-b
Device: d_5468651926be0c0fc89532d7676834c4
```

Command:

```bash
cd /tmp/ctx-bag-name-test-b && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-b <repo-root>/bin/ctx-bag workspace init
```

Output:

```text
Workspace initialized
Name: example-repo
ID: w_a49d7498d9efa7f4
Identity: git-remote:example.com/org/example-repo
Sync: false
```

Command:

```bash
cd /tmp/ctx-bag-name-test-b && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-name-test-home-b <repo-root>/bin/ctx-bag workspace status
```

Output:

```text
Workspace
Name: example-repo
ID: w_a49d7498d9efa7f4
Git root: /tmp/ctx-bag-name-test-b
Identity: git-remote:example.com/org/example-repo
Sync: false
```

Workspace YAML:

```text
id: w_a49d7498d9efa7f4
name: example-repo
identity:
  type: git-remote
  value: example.com/org/example-repo
localPaths:
  - /tmp/ctx-bag-name-test-b
sync: false
createdAt: 2026-08-26T12:59:15Z
updatedAt: 2026-08-26T12:59:15Z
```

Observation:

- Machine A and Machine B have the same workspace ID: `w_a49d7498d9efa7f4`.
- Machine A and Machine B have the same workspace name: `example-repo`.
- Machine A and Machine B retain different local Git roots.

## Manual Validation — Git Workspace Without Remote

Command:

```bash
git -C /tmp/ctx-bag-local-git init
```

Output:

```text
Initialized empty Git repository in /tmp/ctx-bag-local-git/.git/
```

Command:

```bash
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-git-home <repo-root>/bin/ctx-bag init
```

Output:

```text
Context Baggage initialized
Home: /tmp/ctx-bag-local-git-home
Device: d_272eee957a047e8c70c3bdb328cc999c
```

Command:

```bash
cd /tmp/ctx-bag-local-git && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-git-home <repo-root>/bin/ctx-bag workspace init
```

Output:

```text
Workspace initialized
Name: ctx-bag-local-git
ID: w_68574efc78c70c803d73aa74be64f7bc
Identity: git-local:
Sync: false
```

Command:

```bash
cd /tmp/ctx-bag-local-git && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-git-home <repo-root>/bin/ctx-bag workspace status
```

Output:

```text
Workspace
Name: ctx-bag-local-git
ID: w_68574efc78c70c803d73aa74be64f7bc
Git root: /tmp/ctx-bag-local-git
Identity: git-local:
Sync: false
```

Workspace YAML:

```text
id: w_68574efc78c70c803d73aa74be64f7bc
name: ctx-bag-local-git
identity:
  type: git-local
  value: 
localPaths:
  - /tmp/ctx-bag-local-git
sync: false
createdAt: 2026-08-26T12:59:33Z
updatedAt: 2026-08-26T12:59:33Z
```

Observation:

- Git without a remote initializes successfully.
- The name is the Git root basename.
- The identity is `git-local:` and does not contain the absolute local path.
- The workspace ID is generated by Context Baggage.

## Manual Validation — Non-Git Workspace

Command:

```bash
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-folder-home <repo-root>/bin/ctx-bag init
```

Output:

```text
Context Baggage initialized
Home: /tmp/ctx-bag-local-folder-home
Device: d_0708d46490abf26c2197336f0b79c5e8
```

Command:

```bash
cd /tmp/ctx-bag-local-folder && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-folder-home <repo-root>/bin/ctx-bag workspace init
```

Output:

```text
Workspace initialized
Name: ctx-bag-local-folder
ID: w_39697cfb0fe61d1055d1fd44146bee11
Identity: local-directory:
Sync: false
```

Command:

```bash
cd /tmp/ctx-bag-local-folder && CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-local-folder-home <repo-root>/bin/ctx-bag workspace status
```

Output:

```text
Workspace
Name: ctx-bag-local-folder
ID: w_39697cfb0fe61d1055d1fd44146bee11
Workspace root: /tmp/ctx-bag-local-folder
Identity: local-directory:
Sync: false
```

Workspace YAML:

```text
id: w_39697cfb0fe61d1055d1fd44146bee11
name: ctx-bag-local-folder
identity:
  type: local-directory
  value: 
localPaths:
  - /tmp/ctx-bag-local-folder
sync: false
createdAt: 2026-08-26T12:59:45Z
updatedAt: 2026-08-26T12:59:45Z
```

Observation:

- Non-Git folder initialization succeeds.
- The display name is the folder basename.
- The status output correctly labels the path as `Workspace root`.
- The identity is `local-directory:` and does not contain the absolute local path.
- The workspace ID is generated by Context Baggage.

## Assertions

| Assertion | Result | Evidence |
| --- | --- | --- |
| Git remote-derived name works | PASS | `Name: example-repo` for `git-remote:example.com/org/example-repo` |
| Same Git repo gets same name across paths | PASS | A and B both display `Name: example-repo` |
| Same Git repo keeps same workspace ID | PASS | A and B both use `w_a49d7498d9efa7f4` |
| Local Git roots remain machine-specific | PASS | A root `/tmp/ctx-bag-name-test-a`; B root `/tmp/ctx-bag-name-test-b` |
| Git repo without remote still works | PASS | `workspace init` succeeded for `/tmp/ctx-bag-local-git` |
| Non-Git folder initializes successfully | PASS | `workspace init` succeeded for `/tmp/ctx-bag-local-folder` |
| Non-Git name uses folder basename | PASS | `Name: ctx-bag-local-folder` |
| Absolute path is not treated as portable Git identity | PASS | `git-local:` and `local-directory:` identity values are empty; local paths are stored under `localPaths` only |
| No basename-only cross-machine auto-linking added | PASS | `TestNonGitSameBasenameDoesNotAutoLink` passed |

## Limitations

Cross-machine matching for non-Git folders is not automatically inferable in `v0.1`.

Context Baggage deliberately does not infer that two folders are the same workspace from matching basenames. An explicit link/attach workflow may be considered later, but no such command was added in this session.

## Verification

Command:

```bash
go fmt ./...
```

Output:

```text
```

Command:

```bash
go vet ./...
```

Output:

```text
```

Command:

```bash
go test ./...
```

Output:

```text
?   	github.com/context-baggage/context-baggage/cmd/ctx-bag	[no test files]
?   	github.com/context-baggage/context-baggage/internal/agents/claude	[no test files]
?   	github.com/context-baggage/context-baggage/internal/agents/codex	[no test files]
ok  	github.com/context-baggage/context-baggage/internal/agents	(cached)
?   	github.com/context-baggage/context-baggage/internal/config	[no test files]
ok  	github.com/context-baggage/context-baggage/internal/app	(cached)
ok  	github.com/context-baggage/context-baggage/internal/platform	(cached)
ok  	github.com/context-baggage/context-baggage/internal/store	(cached)
ok  	github.com/context-baggage/context-baggage/internal/sync	(cached)
ok  	github.com/context-baggage/context-baggage/internal/task	(cached)
ok  	github.com/context-baggage/context-baggage/internal/workspace	(cached)
```

Command:

```bash
<golangci-lint-path> config verify
```

Output:

```text
```

Command:

```bash
<golangci-lint-path> run
```

Output:

```text
0 issues.
```

Command:

```bash
go build -o ./bin/ctx-bag ./cmd/ctx-bag
```

Output:

```text
```

Command:

```bash
./bin/ctx-bag --help
```

Output:

```text
ctx-bag

Commands:
  init
  status
  doctor
  discover
  workspace init [--sync|--no-sync]
  workspace status
  task start <name>
  task status
  task resume <name>
  checkpoint -m <message>
  handoff
  sync init <folder>
  sync status
  sync push
  sync pull
```

Command:

```bash
git diff --check
```

Output:

```text
```

## Conclusion

WORKSPACE NAMING VALIDATION: PASS
