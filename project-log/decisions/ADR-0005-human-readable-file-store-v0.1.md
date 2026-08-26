# ADR-0005: Use a Human-Readable File Store for v0.1

Status: Accepted

## Context

The `v0.1` schema is expected to evolve while the core workflow is proven. Users and contributors need to inspect state easily, debug failures, and understand what is being synchronized.

## Decision

Use human-readable canonical files rather than SQLite or database storage in `v0.1`.

## Consequences

Reasons:

- inspectable;
- debuggable;
- easy to sync;
- easy to test;
- easy to understand while schemas evolve.

Potential future derived indexes or databases do not replace canonical files unless a later ADR changes this decision.
