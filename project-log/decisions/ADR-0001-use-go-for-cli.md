# ADR-0001: Use Go for the Context Baggage CLI

Status: Accepted

## Context

Context Baggage needs a cross-platform CLI for Linux, Windows, and macOS without requiring users to maintain Node.js, Python, Java, Docker, or another runtime environment.

## Decision

Use Go for the core CLI.

## Consequences

Positive:

- single binaries;
- strong cross-platform support;
- suitable filesystem and process APIs;
- easy deployment;
- good testing and tooling.

Tradeoffs:

- contributors need Go familiarity;
- some AI ecosystem libraries are richer in Python or TypeScript.

## Alternatives

- TypeScript/Node.js;
- Python;
- Rust.
