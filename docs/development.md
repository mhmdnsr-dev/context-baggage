# Development

This document covers the project-specific Go workflow for Context Baggage. It is intentionally short; it is not a general Go tutorial.

## Prerequisites

- Go
- Git
- Optional: `golangci-lint`

## Check Go

```bash
go version
```

This checks the installed Go toolchain version, similar to checking a runtime/toolchain version in a TypeScript project.

## Format

```bash
go fmt ./...
```

This is the canonical formatting step. Conceptually it plays the role that Prettier often plays in TypeScript projects, but Go formatting is standardized by the Go toolchain.

## Vet

```bash
go vet ./...
```

`go vet` checks for suspicious Go constructs that compile but are likely mistakes.

## Test

```bash
go test ./...
```

Go tests also compile the tested packages, so this catches both test failures and many package-level build problems.

## Lint

```bash
golangci-lint run
```

`golangci-lint` is the repository's additional lint layer. It is not an application runtime dependency and should be installed as a developer or CI tool.

The initial configuration targets the `golangci-lint` v2 series; CI pins the `v2.13` minor line.

## Tool version policy

Use the latest stable compatible versions of Go and development tools by default.

The current development and verification toolchain may be newer than the minimum Go version declared in `go.mod`. For example, Context Baggage currently verifies the minimum compatibility lane with Go 1.22.x and the current development lane with Go 1.27.x.

The minimum supported Go version is changed intentionally, not automatically during routine tool upgrades.

## Task completion and work logs

Normal completed tasks should leave the Git working tree clean:

```bash
git status --short
```

should produce no output.

Session logs document the work that produced the commit or Pull Request and should normally be complete before the task's final commit. Do not modify committed logs merely to copy GitHub Actions run IDs, timestamps, or job results — GitHub Actions is the authoritative source for remote CI execution.

If post-push evidence is important enough to preserve in the repository, record it later through a separate explicit documentation or validation change using the normal branch → PR → CI → squash-merge workflow.

Normal development happens on a short-lived branch and enters `main` only through a Pull Request; direct commits to `main` are not the default.

## Build

```bash
mkdir -p bin

go build \
  -o ./bin/ctx-bag \
  ./cmd/ctx-bag
```

Command parts:

```text
go build
    compile

-o
    output path

./cmd/ctx-bag
    package containing the CLI entry point
```

## Run Built CLI

```bash
./bin/ctx-bag --help
```

Manual testing should use the real built executable rather than relying only on `go run ...`.

## Module Cleanup

```bash
go mod tidy
```

`go.mod` declares the module and dependencies. `go.sum` records dependency checksums when external modules are used. They are useful TypeScript learning anchors, but they are not exactly equivalent to `package.json` and a lock file.

## Sync format v2

The `v0.2` portable sync representation lives in a sibling namespace:

```text
<sync-folder>/context-baggage-state/       # legacy v0.1 namespace
<sync-folder>/context-baggage-state-v2/    # v0.2 namespace
```

Once the v2 namespace exists it is authoritative; legacy state is ignored. `v0.2` never modifies the legacy namespace.

To transition a legacy-only sync folder to v2:

```bash
ctx-bag sync upgrade
```

This converts the legacy shared representation to a sanitized v2 projection. It is a format conversion only: the local canonical store and the sync BASE bookkeeping are not changed, and the legacy namespace is preserved. After upgrading, run `ctx-bag sync pull` to import the shared state.

`ctx-bag workspace available` lists attachable portable workspaces (local-directory / git-local) from the authoritative v2 shared state. It is read-only; `ctx-bag workspace attach` is not yet implemented.
