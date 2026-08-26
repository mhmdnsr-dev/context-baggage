# ADR-0002: Store Canonical State Outside Target Repositories

Status: Accepted

## Context

Context Baggage preserves private agent and task continuity for workspaces that may belong to company repositories or other repositories where local metadata should not appear in Git status.

## Decision

Context Baggage canonical state resides outside target Git repositories.

## Consequences

Reasons include:

- company repository restrictions;
- avoiding dirty Git status;
- preventing accidental commits;
- separating agent continuity from application source.

The target repository remains application source. Context Baggage state belongs to the user-level application-data store.
