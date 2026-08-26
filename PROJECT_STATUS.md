# Context Baggage — Current Project Status

Date: 2026-08-26

Phase:
v0.1.0 release preparation.

## Locked Decisions

- Product name: Context Baggage
- CLI: `ctx-bag`
- Core implementation language: Go
- Canonical v0.1 store: human-readable files
- Target repository remains untouched
- Workspace identity survives different paths/operating systems
- Initial adapters: Claude Code + Codex
- Agent discovery read-only in v0.1
- Sync v0.1: manual folder push/pull
- Advanced synchronization deferred
- MCP/vector memory/automatic AI memory deferred

## Current Status

| Area                        | Status               |
| --------------------------- | -------------------- |
| v0.1 implementation         | COMPLETE             |
| Local verification          | PASS                 |
| Manual sync validation      | PASS                 |
| Real conflict validation    | PASS                 |
| Workspace identity validation | PASS               |
| Privacy / secret audit      | PASS                 |
| Public module validation    | PASS                 |
| GitHub Actions CI           | PASS                 |

## Release State

```text
v0.1.0    RELEASE PREPARATION
```

The first public release is being prepared. An annotated `v0.1.0` tag has not yet been created; this file will not claim a release until the tag and GitHub Release actually exist.

## Recent Validation

- Remote CI (`Verify Go 1.22.x`, `Verify Go 1.27.x`, `Lint`) passes.
- Remote `golangci-lint` fixed to `v2.13.1` for Go 1.27 compatibility.
- Public `go install github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag@latest` verified.

## Next Action

Publish `v0.1.0`: tag release commit, push tag, create GitHub Release, then validate the public Go module and `go install @v0.1.0`.
