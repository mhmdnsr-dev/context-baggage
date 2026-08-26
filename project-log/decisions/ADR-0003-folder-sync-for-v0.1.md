# ADR-0003: Use Folder Sync for v0.1

Status: Accepted

## Context

There are many possible synchronization systems. Building provider integrations early creates unnecessary scope and couples the MVP to cloud-specific behavior before the core workflow is proven.

## Decision

`v0.1` supports an explicit filesystem sync folder only. External tools provide network or cloud transport.

## Consequences

Advantages:

- provider-independent;
- simple;
- testable;
- works with many existing solutions.

Limitations:

- explicit push/pull;
- no background synchronization;
- no remote-provider awareness.
