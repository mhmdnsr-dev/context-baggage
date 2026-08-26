# Product Contract

## Identity

```text
Product:
Context Baggage

CLI:
ctx-bag
```

## Problem Statement

### Fragmented agent configuration

Different coding agents maintain different:

- configs;
- MCP servers;
- skills;
- instructions;
- prompts;
- preferences.

Developers often forget what was configured where.

### OS differences

Agent configuration paths and representation may differ between:

- Linux;
- Windows;
- macOS.

### Private project context

Some context related to work projects should not be committed to the source repository.

### Task continuity

Developers need to preserve context for large tasks/features across:

- agent sessions;
- coding agents;
- computers;
- operating systems.

### Temporary vs persistent context

Not all session state should become permanent memory.

### Cross-device continuity

A developer should be able to stop on Machine A and continue meaningfully on Machine B.

## Target User

Initially, Context Baggage targets developers actively using multiple AI coding agents.

## v0.1 Promise

`v0.1` proves this workflow:

```text
Machine A
   ↓
enter project
   ↓
Context Baggage identifies workspace
   ↓
start task
   ↓
record checkpoints
   ↓
create handoff
   ↓
sync push
   ↓
shared filesystem folder
   ↓
Machine B
   ↓
sync pull
   ↓
same workspace recognized
   ↓
same task available
   ↓
handoff available
   ↓
continue work
```

## Scope

- local initialization;
- device identity;
- workspace identity;
- tasks;
- checkpoints;
- handoffs;
- Claude Code discovery;
- Codex discovery;
- folder sync;
- basic conflict detection;
- workspace sync policy.

New workspaces default to `sync: false`. Users can opt in during initialization with `ctx-bag workspace init --sync` or preserve an existing `sync: true` workspace state.

## Non-goals

`v0.1` does not include MCP, vector databases, embeddings, semantic memory, semantic search, AI-generated memory, automatic memory extraction, automatic conversation summarization, background daemon behavior, background synchronization, real-time synchronization, custom cloud services, Dropbox API, Google Drive API, OneDrive API, WebDAV, S3, CRDTs, automatic intelligent merge, web UI, desktop UI, TUI, plugin marketplace, secrets manager, custom encryption, SaaS backend, authentication, or user accounts.

## Success Criteria

1. The same Git project is recognized across different filesystem paths.
2. Task state can be persisted outside the project.
3. Checkpoints survive restart.
4. A handoff can be read by another machine.
5. State can be explicitly pushed and pulled through a filesystem directory.
6. `sync: false` prevents project context export.
7. Claude/Codex discovery does not leak secrets.
8. No application-specific state is written into the target project's Git repository.
