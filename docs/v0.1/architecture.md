# Architecture

## MVP Architecture

```text
                  ctx-bag CLI
                       │
       ┌───────────────┼────────────────┐
       │               │                │
       ▼               ▼                ▼
 Workspace         Task/Handoff      Agent Discovery
 Resolver             Manager
       │               │                │
       └───────────────┼────────────────┘
                       │
                       ▼
                 Canonical Store
                       │
                       ▼
                  Sync Engine
                       │
                       ▼
               Filesystem Folder
```

## Package Responsibilities

| Package | Responsibility |
| --- | --- |
| `app` | Command orchestration, dependency wiring, user-facing flows, and exit behavior. |
| `platform` | OS-specific paths, hostname, runtime metadata, and filesystem-safe platform helpers. |
| `config` | Application configuration loading and persistence. |
| `store` | Canonical file layout, safe reads/writes, directory creation, and schema validation helpers. |
| `workspace` | Workspace root detection, Git remote normalization, workspace IDs, display names, local path registration, and workspace metadata. |
| `task` | Task lifecycle, active task state, checkpoints, and handoff files. |
| `sync` | Filesystem sync-folder configuration, push, pull, status, workspace exclusion, and conflict detection. |
| `agents/claude` | Read-only Claude Code detection and sanitized inventory collection. |
| `agents/codex` | Read-only Codex detection and sanitized inventory collection. |

## Canonical state is not agent state

Claude Code and Codex configuration files are external sources discovered by adapters. They are not Context Baggage's canonical task or workspace state.

## Agent-specific files are not the source of truth for task memory

Task memory, checkpoints, and handoffs live in Context Baggage's canonical store. Agent configuration may influence discovery reports, but it does not own task continuity.

## Sync providers are external

Context Baggage only knows the configured filesystem folder in `v0.1`. Syncthing, Dropbox folders, OneDrive folders, network shares, USB drives, NAS mounts, and similar mechanisms provide external transport.

## No database

Human-readable files are canonical. Future derived indexes may be considered later, but they do not replace canonical files without a new ADR.

## No background service

All actions occur through explicit CLI operations. There is no daemon, watcher, or automatic sync loop in `v0.1`.

## Target repository remains untouched

Context Baggage state is stored in the application-data directory. It does not create `.agent/`, `.ctx-bag/`, or other hidden project state inside target repositories.

## Workspace identity and display names

Workspace ID, display name, repository identity, and local path are separate concepts.

- Git workspaces with a usable remote derive the workspace ID from the normalized remote identity and derive the display name from the repository name.
- Git workspaces without a usable remote use the Git root basename as display metadata and receive a Context Baggage workspace ID at initialization.
- Non-Git folders use the folder basename as display metadata and receive a Context Baggage workspace ID at initialization.

Local paths remain machine-local metadata. Context Baggage does not infer that two non-Git folders on different machines are the same workspace merely because their basenames match.
