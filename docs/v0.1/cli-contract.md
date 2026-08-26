# CLI Contract

## Command Tree

```text
ctx-bag
├── init
├── status
├── doctor
├── discover
│
├── workspace
│   ├── init
│   └── status
│
├── task
│   ├── start <name>
│   ├── status
│   └── resume <name>
│
├── checkpoint
│
├── handoff
│
└── sync
    ├── init <folder>
    ├── status
    ├── push
    └── pull
```

## `ctx-bag init`

Purpose: initialize the Context Baggage application home and device identity.

Syntax:

```bash
ctx-bag init
```

Arguments: none.

Options: none.

Prerequisites: application-data location must be writable.

State changes: creates required directories, `config.yaml`, and `device.yaml` when absent.

Output concept: show application home and device ID.

Expected errors: app-data directory unavailable or not writable.

Exit behavior: exits non-zero on initialization failure.

Example:

```bash
ctx-bag init
```

## `ctx-bag status`

Purpose: show concise device, workspace, task, agent, and sync state.

Syntax:

```bash
ctx-bag status
```

Arguments: none.

Options: none.

Prerequisites: initialization is recommended.

State changes: none.

Output concept: relevant sections only.

Expected errors: malformed canonical files.

Exit behavior: exits non-zero when required state cannot be read.

Example:

```bash
ctx-bag status
```

## `ctx-bag doctor`

Purpose: diagnose store, Git, workspace, task, sync, and inventory health.

Syntax:

```bash
ctx-bag doctor
```

Arguments: none.

Options: none.

Prerequisites: none.

State changes: none.

Output concept: checks with actionable problems.

Expected errors: inaccessible app home or corrupt state.

Exit behavior: exits non-zero when serious problems are found.

Example:

```bash
ctx-bag doctor
```

## `ctx-bag discover`

Purpose: perform read-only Claude Code and Codex discovery.

Syntax:

```bash
ctx-bag discover
```

Arguments: none.

Options: none.

Prerequisites: initialized application home.

State changes: writes sanitized inventory into Context Baggage state.

Output concept: detected agents and high-level sanitized configuration metadata.

Expected errors: inventory directory unavailable or malformed readable files.

Exit behavior: exits non-zero on unsafe persistence failure.

Example:

```bash
ctx-bag discover
```

## `ctx-bag workspace init`

Purpose: register the current directory or Git repository as a Context Baggage workspace.

Syntax:

```bash
ctx-bag workspace init
ctx-bag workspace init --sync
ctx-bag workspace init --no-sync
```

Arguments: none.

Options:

- `--sync`: opt this workspace into folder-sync export.
- `--no-sync`: keep or set this workspace excluded from folder-sync export.

Prerequisites: initialized app home and a readable current directory.

State changes: creates or updates workspace metadata and local path registration.

Output concept: workspace name, identity, workspace ID, and sync policy.

Expected errors: current directory unavailable, app home unavailable, or remote identity cannot be safely resolved.

Exit behavior: exits non-zero on resolution or write failure.

Example:

```bash
ctx-bag workspace init --sync
```

## `ctx-bag workspace status`

Purpose: show the current directory's Context Baggage workspace status.

Syntax:

```bash
ctx-bag workspace status
```

Arguments: none.

Options: none.

Prerequisites: initialized app home and an initialized workspace for the current directory.

State changes: none.

Output concept: current workspace root, normalized identity where available, workspace ID, local path registration, and sync policy.

Expected errors: workspace not initialized for current directory.

Exit behavior: exits non-zero when the workspace cannot be resolved or is not registered.

Example:

```bash
ctx-bag workspace status
```

## `ctx-bag task start <name>`

Purpose: create and activate a task in the current workspace.

Syntax:

```bash
ctx-bag task start <name>
```

Arguments: `<name>` is the task name and source for the filesystem-safe task ID.

Options: none.

Prerequisites: initialized workspace for the current repository.

State changes: creates `task.yaml`, initializes `checkpoints.jsonl`, and marks the task active.

Output concept: task ID, workspace ID, and active status.

Expected errors: missing workspace, invalid task name, or duplicate task.

Exit behavior: exits non-zero on validation or write failure.

Example:

```bash
ctx-bag task start <task-name>
```

## `ctx-bag task status`

Purpose: show active task state for the current workspace.

Syntax:

```bash
ctx-bag task status
```

Arguments: none.

Options: none.

Prerequisites: initialized workspace.

State changes: none.

Output concept: active task and known tasks.

Expected errors: no active task or malformed task metadata.

Exit behavior: exits non-zero for invalid task state.

Example:

```bash
ctx-bag task status
```

## `ctx-bag task resume <name>`

Purpose: make an existing task active.

Syntax:

```bash
ctx-bag task resume <name>
```

Arguments: `<name>` identifies an existing task by name or task ID.

Options: none.

Prerequisites: initialized workspace and existing task.

State changes: updates active task state.

Output concept: resumed task ID.

Expected errors: unknown task or ambiguous task reference.

Exit behavior: exits non-zero when the task cannot be resumed.

Example:

```bash
ctx-bag task resume <task-name>
```

## `ctx-bag checkpoint`

Purpose: append a checkpoint to the active task.

Syntax:

```bash
ctx-bag checkpoint -m "<checkpoint-message>"
ctx-bag checkpoint --message "<checkpoint-message>"
```

Arguments: none.

Options: `-m`, `--message`.

Prerequisites: initialized workspace and active task.

State changes: appends one JSONL checkpoint record.

Output concept: timestamp and task ID.

Expected errors: missing active task or empty message.

Exit behavior: exits non-zero when no checkpoint is written.

Example:

```bash
ctx-bag checkpoint -m "<checkpoint-message>"
```

## `ctx-bag handoff`

Purpose: create or update the active task's Markdown handoff.

Syntax:

```bash
ctx-bag handoff
```

Arguments: none.

Options: none in `v0.1`.

Prerequisites: initialized workspace and active task.

State changes: creates `handoff.md` with the standard sections if absent.

Output concept: handoff file path.

Expected errors: missing active task or write failure.

Exit behavior: exits non-zero when handoff cannot be created or found.

Example:

```bash
ctx-bag handoff
```

## `ctx-bag sync init <folder>`

Purpose: configure a filesystem folder as the sync target.

Syntax:

```bash
ctx-bag sync init <folder>
```

Arguments: `<folder>` is a local filesystem path.

Options: none.

Prerequisites: initialized app home and reachable folder.

State changes: writes sync configuration and state metadata.

Output concept: configured sync folder.

Expected errors: folder missing or not writable.

Exit behavior: exits non-zero on validation or write failure.

Example:

```bash
ctx-bag sync init <sync-folder>
```

## `ctx-bag sync status`

Purpose: show sync configuration and reachability.

Syntax:

```bash
ctx-bag sync status
```

Arguments: none.

Options: none.

Prerequisites: initialized app home.

State changes: none.

Output concept: folder, availability, last push, and last pull.

Expected errors: malformed sync state.

Exit behavior: exits non-zero when sync state cannot be read.

Example:

```bash
ctx-bag sync status
```

## `ctx-bag sync push`

Purpose: export eligible local state to the configured sync folder.

Syntax:

```bash
ctx-bag sync push
```

Arguments: none.

Options: none in `v0.1`.

Prerequisites: initialized sync folder.

State changes: writes exported state and updates last push metadata.

Output concept: copied resources and skipped `sync: false` workspaces.

Expected errors: missing folder or detected conflict.

Exit behavior: exits non-zero on failure or conflict.

Example:

```bash
ctx-bag sync push
```

## `ctx-bag sync pull`

Purpose: import state from the configured sync folder.

Syntax:

```bash
ctx-bag sync pull
```

Arguments: none.

Options: none in `v0.1`.

Prerequisites: initialized sync folder.

State changes: imports remote state and updates last pull metadata.

Output concept: imported resources or conflict details.

Expected errors: missing folder or detected conflict.

Exit behavior: exits non-zero on failure or conflict.

Example:

```bash
ctx-bag sync pull
```
