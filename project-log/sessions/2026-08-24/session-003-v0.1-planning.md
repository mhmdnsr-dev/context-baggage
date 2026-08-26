# Session 003: v0.1 Planning

Date: 2026-08-24

## v0.1 Objective

Three core questions:

```text
1. What did I configure?
2. What was I doing on this project/task?
3. How do I take that state to another machine?
```

## MVP Workflow

Machine A → task → checkpoint → handoff → sync push → Machine B → sync pull → resume.

## Initial Technology

Go.

## Initial Agents

Claude Code and Codex only.

## Initial Storage

Human-readable files.

## Initial Sync

Explicit folder-based push/pull.

## Deferred

- MCP;
- semantic memory;
- vector DB;
- AI memory extraction;
- cloud-specific providers;
- daemon;
- real-time sync;
- advanced conflict resolution;
- UI.

## Milestone Order

```text
M0 Foundation
M1 Workspace Identity
M2 Task / Checkpoint / Handoff
M3 Agent Discovery
M4 Folder Sync
M5 Release Readiness
v0.1.0
```

Reaching M2 enables early dogfooding of Context Baggage while developing Context Baggage itself.
