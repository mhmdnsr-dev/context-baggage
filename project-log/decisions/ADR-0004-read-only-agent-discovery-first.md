# ADR-0004: Read-Only Agent Discovery First

Status: Accepted

## Context

Developers using several coding agents often cannot remember which agent has which configuration, instructions, skills, or MCP servers. Mutating those configurations too early risks data loss, secrets exposure, and unclear ownership.

## Decision

`v0.1` discovers Claude Code and Codex configuration but does not mutate it.

## Consequences

The immediate pain is understanding what is configured where. Safe inspection should precede synchronization or rewriting of agent configuration.

Future features may add:

```text
diff
apply
backup
restore
```

but they are not part of `v0.1`.
