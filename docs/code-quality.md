# Code Quality

Context Baggage is intentionally a small, boring Go CLI. The most important engineering rule is:

> Optimize first for the next reader of the code, not for the cleverness of the current writer.

> If a simpler implementation is equally correct, choose the simpler implementation.

> Smart code is code whose intent remains obvious to its author after time away from it.

This guide is the normative source of truth for Code Quality. It applies equally to human contributors, open-source contributors, AI coding agents, and future maintainers.

## Principles

- **Descriptive.** Names reveal intent.
- **Simple.** Straight-line code, small explicit branches, small focused helpers, the standard library, and existing project primitives are preferred.
- **Explicit.** Important product decisions are visible in control flow (`validate → decide → mutate`), not hidden behind generic machinery.
- **Boring.** Avoid speculative abstractions, reflection, unnecessary interfaces, and frameworks for hypothetical future needs.

## Naming

Prefer names that reveal intent. A reader should not have to read the implementation just to know what a variable represents.

```text
portableWorkspace      currentWorkspace      authoritativeWorkspace
observedGitID          sharedBaseline        hasSharedBaseline
normalizedRemote       workspaceRoot
```

Avoid vague names when scope is not trivially tiny:

```text
x  v  obj  data  info  tmp  val  helper  thing  result2
```

Short names remain acceptable for conventional tiny scopes such as `i`, `j`, `err` when their meaning is obvious. Do not create a rigid naming bureaucracy.

## Functions and Control Flow

Each function should have one clear responsibility. Prefer functions whose names and responsibilities align:

```text
FindPortableWorkspace(...)     IsWorkspaceEmpty(...)
NormalizeRemote(...)           NamespaceState(...)
```

Avoid large functions that mix validation, filesystem mutation, formatting, sync policy, and CLI output when natural seams already exist. But do not mechanically split every function into tiny helpers — a helper should improve clarity, reuse, testability, or safety reasoning, not merely reduce line count.

## Abstractions

Before introducing an interface, generic type, service layer, repository layer, manager, provider abstraction, state machine, or framework, ask:

> What current duplication, dependency, or testing problem does this solve?

If the honest answer is "we might need it later," do not add it. A small amount of duplication may be preferable to an abstraction whose purpose is not yet real.

## Go Documentation

Go uses standard Go doc comments (not JSDoc-style annotations). No external documentation library is required. Comments should work naturally with `go doc` and Go tooling.

### Exported API

Every newly-created exported type, function, method, constant, or variable should normally have an idiomatic doc comment that begins with the exported identifier when natural. Do not imitate JSDoc with `@param` / `@return` / `@throws` — the Go signature already communicates parameter and return types.

```go
// NormalizeRemote removes credentials and normalizes a Git remote value
// into the stable representation used for workspace identity.
func NormalizeRemote(remote string) string {
	// ...
}
```

```go
// PortableWorkspace contains only workspace metadata that is allowed
// to participate in portable shared state.
type PortableWorkspace struct {
	// ...
}
```

### Helpers / Utils

A private helper MUST normally have a Go doc-style comment when it:

- normalizes data or transforms identity;
- changes filesystem, workspace, or sync state;
- makes a sync decision or enforces a safety invariant;
- performs conflict detection or portable/local ownership decisions;
- has important side effects, intentionally conservative behavior, or surprising edge-case semantics.

```go
// IsWorkspaceEmpty reports whether the workspace can be safely discarded
// during local attachment adoption.
//
// A workspace is empty only when its directory contains workspace.yaml and
// no other entries. The check is intentionally conservative so unknown or
// malformed local state is never silently deleted.
func (s Store) IsWorkspaceEmpty(id string) (bool, error) {
	// ...
}
```

### Do Not Comment Obvious Code

Do not require comments that merely restate the code or signature.

```go
// Increment i.
i++
```

```go
// FindPortableWorkspace finds a portable workspace.
func FindPortableWorkspace(...)
```

Prefer documentation that adds useful contract information:

```go
// FindPortableWorkspace reads one exact workspace ID from authoritative v2
// shared state and rejects inconsistent directory/metadata identities.
func FindPortableWorkspace(...)
```

## Comments

Inline comments are most valuable for **WHY**: safety reasons, non-obvious ordering, invariants, compatibility constraints, privacy decisions, and intentional conservative behavior.

```go
// Remove the old binding first so a crash can leave the path temporarily
// unattached, but never owned by two workspace IDs.
```

A comment describing the next obvious line is usually not useful. Names come first; comments complement clear naming rather than compensate for bad naming.

## Error Handling

Prefer explicit errors with enough context to understand what failed, which resource was involved, and what the user/developer can safely do next. Do not swallow errors unless intentionally documented. Avoid `panic` for expected runtime/user conditions. Use project-consistent wrapping. Do not add a custom error framework without a demonstrated need.

## State Mutation

Functions that mutate the filesystem, workspace records, sync state, or portable state should make that fact clear through their name, documentation, control flow, and tests. Avoid helpers whose names sound read-only but mutate state. For example `ListPortableWorkspaces` must remain read-only. A mutation should never be hidden inside a "getter".

## Testing

### Safety invariants

When code protects an invariant — never silently overwrite divergent no-BASE state, never export `LocalPaths`, never attach one path to two Workspace IDs, never silently re-key an established workspace — there must be a focused regression test. Tests should capture the contract, edge case, and failure behavior, not implementation trivia.

### Test naming

Use descriptive test names:

```go
func TestPullRefusesDivergentStateWithoutBaseline(t *testing.T)
func TestAttachPreservesUnrelatedLocalPaths(t *testing.T)
func TestResolverKeepsEstablishedWorkspaceWhenGitRemoteAppears(t *testing.T)
```

A failing test name should tell a maintainer which product behavior broke.

### Test structure

Keep tests readable with an obvious Arrange → Act → Assert flow. Use `t.TempDir()` and existing project helpers. Avoid large test frameworks. A test helper is justified when it removes distracting setup while keeping the tested behavior visible.

### Avoid brittle tests

Assert semantic values, file presence/absence, IDs, hash equality, state transitions, and errors rather than excessive whitespace or incidental implementation details. CLI-output tests may assert important phrases rather than every space unless formatting itself is the contract.

## Safety Invariants

These invariants are core to Context Baggage and must remain visible and tested:

```text
no silent last-write-wins
no automatic identity re-key
old LocalPath ownership removed before target attachment
v2 authoritative once it exists
never export LocalPaths / UpdatedAt
portable/local ownership boundaries
```

Do not obscure such rules behind generic frameworks.

## Maintainability Guardrails

These gates are guardrails, not design goals. A metric violation is a signal to improve the responsibility boundary or control flow — never a target to game. They run automatically:

- **File size** — enforced by a repository test under `go test ./...`.

```text
Preferred:   ≤ 300 physical lines
Review zone: 301–500 lines (question whether multiple responsibilities exist)
Hard limit:  > 500 lines → CI failure
```

- **Function length** — enabled via `funlen` in `golangci-lint`.

```text
lines: 100, statements: 60 (hard threshold)
Most functions should be substantially below the hard threshold.
```

- **Cognitive complexity** — enabled via `gocognit` in `golangci-lint`.

```text
min-complexity: 20
```

### Metric anti-gaming rule

> Do not split, compress, rename, or abstract code merely to satisfy a metric.

> When a metric reveals a violation, improve the responsibility boundary or control flow.

A metric-driven refactor must make the code easier to understand, not merely make the metric green. Never split a file into arbitrary `part1`/`part2`, extract meaningless one-use helpers, move complexity into poorly named abstractions, compress code to reduce physical lines, or remove useful comments to satisfy a line limit.

### Complexity budget

> A change should not add complexity without buying a concrete current capability, correctness property, or safety property.

## Refactoring

Do not refactor unrelated code while implementing a feature simply because it could look cleaner. Refactoring should be required for current correctness or small and directly enabling the current change. Large cleanup belongs in a separate task/PR, which keeps reviews small and makes regressions easier to locate.

## Review Checklist

Before requesting review:

```text
[ ] Are names descriptive?
[ ] Is there a simpler equally-correct implementation?
[ ] Does each new abstraction solve a current problem?
[ ] Are non-obvious helpers documented?
[ ] Do comments explain WHY rather than restate code?
[ ] Are mutation and side effects obvious?
[ ] Are safety invariants covered by tests?
[ ] Are tests named after behavior?
[ ] Is unrelated refactoring excluded?
[ ] Did gofmt/vet/test/build/lint pass?
[ ] Did I review the diff for secrets, personal paths, and runtime state?
[ ] Do changes stay within file-size, function-length, and complexity guardrails?
```

## AI Agent Expectations

The same engineering rules apply to AI-generated code. An AI coding agent must:

```text
read the current architecture and docs before editing
prefer existing project primitives
use descriptive names
avoid speculative abstractions
document non-obvious helpers
add WHY comments for safety-sensitive behavior
add focused regression tests
run the project validation commands
review its own diff
avoid unrelated refactoring
preserve privacy/security invariants
```

> Generated code is held to the same readability standard as human-written code.

> An AI agent must not perform mechanical metric gaming. If a file or function exceeds a maintainability gate, identify the actual responsibilities, split only at meaningful boundaries, preserve behavior and safety invariants, use descriptive names, and test the refactor.

There is no separate lower standard for AI-generated code.
