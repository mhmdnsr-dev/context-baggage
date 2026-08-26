# ADR-0006: Prefer Obvious Code Over Clever Code

Status: Accepted

## Context

Context Baggage is intended to be a trustworthy open-source CLI that developers and AI coding agents can re-enter after time away. Maintenance, review, onboarding, and debugging matter more than showing cleverness or minimizing line count.

## Decision

Context Baggage optimizes for code that can be understood quickly after time away from the project.

Explicit and conventional implementations are preferred over clever abstractions even when clever implementations reduce code size.

## Consequences

Positive:

- easier maintenance;
- easier review;
- easier onboarding;
- easier debugging;
- lower cognitive load;
- better AI-agent generated-code reviewability.

Tradeoffs:

- occasional duplication may be acceptable;
- code may sometimes be slightly more verbose;
- premature abstraction is deliberately avoided.

Simplicity does not justify poor structure or duplicated business rules. The project should remain clear, cohesive, and testable.
