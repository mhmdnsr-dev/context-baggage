# Requirements

## Functional Requirements

| ID | Requirement |
| --- | --- |
| FR-001 | `ctx-bag init` initializes the application state directory and required subdirectories. |
| FR-002 | Initialization creates a stable random device identity if one does not already exist. |
| FR-003 | Device identity must include non-sensitive friendly metadata such as hostname, OS, architecture, and creation time where available. |
| FR-004 | Workspace discovery resolves the current Git root when run inside a Git repository. |
| FR-005 | Workspace identity is derived from normalized repository identity, not absolute local path alone. |
| FR-006 | Workspace initialization records workspace metadata outside the target repository. |
| FR-007 | Workspace initialization records local paths as metadata and preserves existing compatible state. |
| FR-008 | Workspace status displays the resolved workspace and local registration state. |
| FR-009 | Task creation creates a task under the current workspace using a filesystem-safe task ID. |
| FR-010 | Task creation prevents ambiguous duplicate behavior. |
| FR-011 | Task resume switches the active task for the current workspace to an existing task. |
| FR-012 | Task status displays the active task and available workspace task state. |
| FR-013 | Checkpoints append timestamped JSONL records to the active task. |
| FR-014 | `ctx-bag checkpoint -m` and `ctx-bag checkpoint --message` are both supported. |
| FR-015 | Handoff creates or updates `handoff.md` for the active task using the defined Markdown sections. |
| FR-016 | Claude Code discovery detects safely available configuration metadata without mutating Claude files. |
| FR-017 | Codex discovery detects safely available configuration metadata without mutating Codex files. |
| FR-018 | Discovery redacts secrets before printing or persisting any metadata. |
| FR-019 | Sync initialization configures a filesystem folder used for explicit push/pull. |
| FR-020 | Sync status reports configured folder state and last push/pull metadata where available. |
| FR-021 | Sync push exports local eligible state to the configured sync folder. |
| FR-022 | Sync pull imports state from the configured sync folder. |
| FR-023 | Sync detects ambiguous conflicts and stops without silent last-write-wins. |
| FR-024 | Workspace state with `sync: false` is excluded from sync push. |
| FR-025 | Doctor diagnostics report initialization, store, Git, workspace, task, sync, and inventory issues safely. |
| FR-026 | Status provides a concise overview of initialized device, workspace, task, agents, and sync state where available. |
| FR-027 | Git workspaces with a usable remote derive their display name from the normalized repository identity's final path segment. |
| FR-028 | Git repositories without a usable remote initialize successfully using the Git root basename as display metadata only. |
| FR-029 | Non-Git directories initialize successfully using the folder basename as display metadata only. |
| FR-030 | Non-Git and no-remote Git workspaces must not be automatically linked across machines by matching basenames. |

## Non-Functional Requirements

| ID | Requirement |
| --- | --- |
| NFR-001 | The CLI must target Linux, macOS, and Windows. |
| NFR-002 | Canonical state must be human-readable. |
| NFR-003 | Operations should be deterministic for the same inputs and state. |
| NFR-004 | Filesystem writes should use temporary files and atomic replacement where practical. |
| NFR-005 | Tests must use isolated temporary directories and fixtures, not the developer's actual home or real agent config. |
| NFR-006 | Privacy-preserving behavior must be conservative by default. |
| NFR-007 | Dependencies must remain minimal and justified. |
| NFR-008 | Errors must be actionable and include the next useful command where appropriate. |
| NFR-009 | Distribution should ultimately be a single executable named `ctx-bag`. |

## Constraints

- State must live outside target repositories.
- No database is used in `v0.1`.
- No automatic cloud upload is performed.
- No AI is required for checkpoints or handoffs.
- Agent configuration is never mutated in `v0.1`.

## Explicit Non-Goals

MCP server support, vector databases, embeddings, semantic memory, automatic memory extraction, AI-generated summaries, background synchronization, cloud-provider APIs, real-time collaboration, CRDTs, UI, plugins, authentication, user accounts, and custom encryption are out of scope for `v0.1`.
