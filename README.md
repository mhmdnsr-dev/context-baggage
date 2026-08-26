# Context Baggage

> Carry your agent context wherever you work.

Context Baggage is an experimental open-source developer tool for carrying coding-agent context across machines, operating systems, and agent tools without writing private state into source repositories.

## Problem

Developers increasingly use several AI coding agents such as Claude Code, Codex, Gemini CLI, OpenCode, and others. Each agent keeps its own configuration, instructions, skills, MCP setup, and local state. That state also varies across Linux, macOS, Windows, work machines, and personal machines.

The result is fragmented context: it is hard to remember what was configured where, what was happening on a long-running task, and what needs to move safely to another device.

## Core idea

Context Baggage stores portable developer and agent context outside the source repository. It is a local-first continuity layer for workspace identity, task state, checkpoints, handoffs, read-only agent discovery, and explicit folder-based synchronization.

## v0.1 scope

The `v0.1` scope is intentionally small:

- workspace identity that survives different local paths and operating systems;
- task state for one workspace;
- append-only checkpoints;
- Markdown handoffs;
- read-only Claude Code discovery;
- read-only Codex discovery;
- explicit push/pull through a filesystem sync folder.

Context Baggage does not provide MCP, embeddings, vector memory, semantic search, background sync, cloud-provider APIs, or automatic AI memory extraction in `v0.1`.

## Example workflow

On Machine A:

```bash
ctx-bag init
ctx-bag workspace init --sync
ctx-bag task start <task-name>
ctx-bag checkpoint -m "<checkpoint-message>"
ctx-bag handoff
ctx-bag sync init <sync-folder>
ctx-bag sync push
```

On Machine B:

```bash
ctx-bag init
ctx-bag sync init <sync-folder>
ctx-bag sync pull
ctx-bag workspace status
ctx-bag task resume <task-name>
```

## Privacy model

Context Baggage is local-first. It does not automatically upload anything, and it does not write application-specific state into the target project's Git repository. Synchronization is explicit and uses a user-configured filesystem folder. New workspaces default to `sync: false`; use `ctx-bag workspace init --sync` to opt a workspace into export.

Cloud-provider integrations are not part of `v0.1`.

## Status

Context Baggage is early and experimental. The `v0.1.0` implementation is complete enough for first manual real-world validation.

## Development

See [docs/development.md](docs/development.md) for the local Go workflow, linting, and build commands.

## Roadmap

- M0: foundation, initialization, device identity, baseline status;
- M1: workspace identity;
- M2: task, checkpoint, and handoff;
- M3: read-only agent discovery;
- M4: folder sync;
- M5: release readiness.
