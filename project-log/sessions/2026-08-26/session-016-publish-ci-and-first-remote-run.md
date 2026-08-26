# Session 016 — Publish CI And First Remote Run

## Objective

Document the tooling version policy, commit and push the minimal GitHub Actions workflow, then inspect the first real remote GitHub Actions run for the exact pushed commit.

Scope restrictions:

- no product behavior changes;
- no release;
- no tag;
- no release automation;
- no force push.

## Tool Version Policy

Default development policy:

```text
Use the latest stable compatible versions of Go and development tools by default.
```

The policy intentionally separates:

```text
LATEST DEVELOPMENT TOOLCHAIN
    current stable compatible toolchain used for normal development and CI current lane

MINIMUM SUPPORTED VERSION
    explicit compatibility baseline declared by the project
```

For the current repository state:

```text
Go development/current lane: Go 1.27.x
Go minimum lane: Go 1.22.x
go.mod directive: go 1.22
```

Official Go release history records Go 1.27.0 as released on 2026-08-19:

```text
https://go.dev/doc/devel/release
```

Tool updates should check official upstream sources, choose the latest stable compatible version, avoid obsolete tutorial versions, and document intentional pins or exceptions.

No version-management automation was added.

## Starting State

### Command

```bash
git status --short
git log -3 --oneline --decorate
git branch --show-current
git remote -v
git rev-parse HEAD
```

### Output

```text
A  .github/workflows/ci.yml
M  docs/development.md
AM project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
A  project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
A  project-log/sessions/2026-08-26/session-015-minimal-github-ci.md
A  project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
main
origin	git@github.com:mhmdnsr-dev/context-baggage.git (fetch)
origin	git@github.com:mhmdnsr-dev/context-baggage.git (push)
```

The files were already partially staged when this session began. Session 013 (`AM`) carried an unstaged privacy improvement that replaced the repository's personal absolute path with the neutral `<repo-root>` placeholder. That fix was re-staged during this session so the committed history contains no personal repository absolute path.

Earlier state before this session showed:

```text
?? .github/
?? project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
?? project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
?? project-log/sessions/2026-08-26/session-015-minimal-github-ci.md
```

The current base commit before the CI publication was:

```text
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
```

## Files Changed

Changed/added for this CI publication:

```text
.github/workflows/ci.yml
docs/development.md
project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
project-log/sessions/2026-08-26/session-015-minimal-github-ci.md
project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
```

Product source under `cmd/` and `internal/` was not changed.

## Local Verification

### Workflow Review

### Command

```bash
cat .github/workflows/ci.yml
```

### Output

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:
    branches:
      - main

permissions:
  contents: read

jobs:
  verify:
    name: Verify Go ${{ matrix.go-version }}
    runs-on: ubuntu-latest

    strategy:
      matrix:
        go-version:
          - '1.22.x'
          - '1.27.x'

    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ matrix.go-version }}

      - name: Record Go version
        run: go version

      - name: Check formatting
        run: |
          files="$(gofmt -l .)"
          if [ -n "$files" ]; then
            echo "$files"
            exit 1
          fi

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test ./...

      - name: Build
        run: go build ./cmd/ctx-bag

  lint:
    name: Lint
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: '1.27.x'

      - name: Lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12
```

### Go 1.22

Commands:

```bash
GOTOOLCHAIN=go1.22.2 go version
files="$(gofmt -l .)"
if [ -n "$files" ]; then
  echo "$files"
  exit 1
fi
GOTOOLCHAIN=go1.22.2 go vet ./...
GOTOOLCHAIN=go1.22.2 go test ./...
GOTOOLCHAIN=go1.22.2 go build -o ./bin/ctx-bag ./cmd/ctx-bag
```

Output:

```text
go version go1.22.2 linux/amd64
?   	github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag	[no test files]
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/claude	[no test files]
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/codex	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/agents	(cached)
?   	github.com/mhmdnsr-dev/context-baggage/internal/config	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/app	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/platform	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/store	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/sync	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/task	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/workspace	(cached)
```

Result: PASS.

### Go 1.27

Commands:

```bash
GOTOOLCHAIN=go1.27.0 go version
files="$(gofmt -l .)"
if [ -n "$files" ]; then
  echo "$files"
  exit 1
fi
GOTOOLCHAIN=go1.27.0 go vet ./...
GOTOOLCHAIN=go1.27.0 go test ./...
GOTOOLCHAIN=go1.27.0 go build -o ./bin/ctx-bag ./cmd/ctx-bag
```

Output:

```text
go version go1.27.0 linux/amd64
?   	github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/agents	(cached)
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/claude	[no test files]
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/codex	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/app	(cached)
?   	github.com/mhmdnsr-dev/context-baggage/internal/config	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/platform	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/store	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/sync	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/task	(cached)
ok  	github.com/mhmdnsr-dev/context-baggage/internal/workspace	(cached)
```

Result: PASS.

### Lint

Commands:

```bash
/home/nasr/go/bin/golangci-lint config verify
/home/nasr/go/bin/golangci-lint run
```

Output:

```text
golangci-lint has version 2.13.1 built with go1.27.0
0 issues.
```

Result: PASS.

The local `golangci-lint` is `v2.13.1`; the CI workflow pins the `v2.12` minor line. Local and CI may differ slightly in resolved patch, so the remote lint job is the authoritative evidence: the resolved remote version is captured in the Remote Jobs section.

### Diff Check / Artifact Check

Commands:

```bash
git diff --check
git check-ignore -v bin/ctx-bag
test ! -e ./ctx-bag && echo root-binary-absent
```

Output:

```text
.gitignore:1:/bin/	bin/ctx-bag
root-binary-absent
```

Result: PASS.

## Staged Snapshot

Pending at time this initial log section was created.

## Commit

Pending at time this initial log section was created.

## Push

Pending at time this initial log section was created.

## Remote Commit Verification

Pending at time this initial log section was created.

## GitHub Actions Run

Pending at time this initial log section was created.

## Remote Jobs

Pending at time this initial log section was created.

## Assertions

Pending final remote CI result.

## Findings

Pending final remote CI result.

## Final Git State

Pending final remote CI result.

## Conclusion

Pending final remote CI result.
