# Session 015 — Minimal GitHub CI

## Objective

Add one small GitHub Actions workflow for `v0.1` that verifies:

```text
formatting
vet
tests
build
lint
minimum Go compatibility
current Go compatibility
```

The workflow is CI-only. No product behavior, release automation, tags, commits, or pushes were added.

## Starting Git State

### Command

```bash
git status --short
git status
git branch --show-current
git remote -v
find .github -maxdepth 3 -type f -print 2>/dev/null || true
cat .gitattributes
cat .golangci.yml
cat go.mod
```

### Output

```text
?? project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
?? project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
On branch main
Your branch is up to date with 'origin/main'.

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
	project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md

nothing added to commit but untracked files present (use "git add" to track)
main
origin	git@github.com:mhmdnsr-dev/context-baggage.git (fetch)
origin	git@github.com:mhmdnsr-dev/context-baggage.git (push)
* text=auto

*.go text eol=lf
*.md text eol=lf
*.yml text eol=lf
*.yaml text eol=lf
*.mod text eol=lf
*.sum text eol=lf
*.jsonl text eol=lf
*.toml text eol=lf
version: "2"

linters:
  default: standard
  enable:
    - ineffassign
    - misspell
    - unconvert
    - unused

formatters:
  enable:
    - gofmt
module github.com/mhmdnsr-dev/context-baggage

go 1.22
```

Existing state:

- No `.github/workflows` workflow existed.
- `.gitattributes` already enforces LF for Go, Markdown, YAML, module, checksum, JSONL, and TOML files.
- `.golangci.yml` already uses v2 schema and was not modified.
- `go.mod` keeps `go 1.22` as the minimum compatibility contract.
- Session logs 013 and 014 were already untracked and were preserved.

## CI Design

### Trigger Policy

Run on:

```yaml
push:
  branches:
    - main

pull_request:
  branches:
    - main
```

No schedule, manual dispatch, release, or tag trigger was added.

### Permissions

The workflow uses:

```yaml
permissions:
  contents: read
```

No write permissions are granted.

### Go Compatibility

The `verify` job uses:

```yaml
strategy:
  matrix:
    go-version:
      - '1.22.x'
      - '1.27.x'
```

Reason:

- `1.22.x` validates the declared minimum in `go.mod`.
- `1.27.x` validates the current release/toolchain used during pre-release checks.

### Lint Strategy

Lint runs in a separate `lint` job on Go `1.27.x` only.

Reason:

- The verify matrix already checks compilation, vet, tests, and build under both Go versions.
- Lint should represent the current development environment and should not double the lint runtime for the minimum compatibility check.

## Action Versions

References used:

- Context7 `/websites/github_en_actions`: workflow syntax, branch triggers, permissions, matrix strategy.
- Context7 `/golangci/golangci-lint-action`: official golangci-lint action usage, `golangci/golangci-lint-action@v9`, `version: v2.12`, and default config verification behavior.
- Task requirement supplied the current major versions `actions/checkout@v7`, `actions/setup-go@v7`, and `golangci/golangci-lint-action@v9`.

Selected versions:

```text
actions/checkout@v7
actions/setup-go@v7
golangci/golangci-lint-action@v9
golangci-lint version: v2.12
```

Context7's indexed examples for checkout/setup-go still showed older action majors, so the workflow follows the explicit current-version requirement for those two official actions.

## Workflow

Created:

```text
.github/workflows/ci.yml
```

Final contents:

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

## Local Validation — Go 1.22

### Go Version

```bash
GOTOOLCHAIN=go1.22.2 go version
```

Output:

```text
go version go1.22.2 linux/amd64
```

### Formatting

```bash
files="$(gofmt -l .)"
if [ -n "$files" ]; then
  echo "$files"
  exit 1
fi
```

Output:

```text
```

Result: PASS.

### Vet

```bash
GOTOOLCHAIN=go1.22.2 go vet ./...
```

Output:

```text
```

Result: PASS.

### Tests

```bash
GOTOOLCHAIN=go1.22.2 go test ./...
```

Output:

```text
?   	github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag	[no test files]
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/claude	[no test files]
?   	github.com/mhmdnsr-dev/context-baggage/internal/agents/codex	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/agents	0.002s
?   	github.com/mhmdnsr-dev/context-baggage/internal/config	[no test files]
ok  	github.com/mhmdnsr-dev/context-baggage/internal/app	0.062s
ok  	github.com/mhmdnsr-dev/context-baggage/internal/platform	0.002s
ok  	github.com/mhmdnsr-dev/context-baggage/internal/store	0.005s
ok  	github.com/mhmdnsr-dev/context-baggage/internal/sync	0.022s
ok  	github.com/mhmdnsr-dev/context-baggage/internal/task	0.008s
ok  	github.com/mhmdnsr-dev/context-baggage/internal/workspace	0.055s
```

Result: PASS.

### Build

Local validation used an explicit output path to avoid leaving `./ctx-bag` in the repository root. CI uses `go build ./cmd/ctx-bag` because the runner workspace is disposable.

```bash
GOTOOLCHAIN=go1.22.2 go build -o ./bin/ctx-bag ./cmd/ctx-bag
```

Output:

```text
```

Result: PASS.

## Local Validation — Go 1.27

### Go Version

```bash
GOTOOLCHAIN=go1.27.0 go version
```

Output:

```text
go version go1.27.0 linux/amd64
```

### Formatting

```bash
files="$(gofmt -l .)"
if [ -n "$files" ]; then
  echo "$files"
  exit 1
fi
```

Output:

```text
```

Result: PASS.

### Vet

```bash
GOTOOLCHAIN=go1.27.0 go vet ./...
```

Output:

```text
```

Result: PASS.

### Tests

```bash
GOTOOLCHAIN=go1.27.0 go test ./...
```

Output:

```text
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

### Build

```bash
GOTOOLCHAIN=go1.27.0 go build -o ./bin/ctx-bag ./cmd/ctx-bag
```

Output:

```text
```

Result: PASS.

## Local Lint Validation

### Command

```bash
$HOME/go/bin/golangci-lint config verify
$HOME/go/bin/golangci-lint run
```

### Output

```text
0 issues.
```

Result: PASS.

## Git Diff Review

### Command

```bash
git diff --check
git diff --stat
git diff -- .github/workflows/ci.yml
git diff -- .golangci.yml
git diff -- go.mod
git diff -- cmd internal
git check-ignore -v bin/ctx-bag
test ! -e ./ctx-bag && echo root-binary-absent
git status --short --ignored
```

### Output

```text
.gitignore:1:/bin/	bin/ctx-bag
root-binary-absent
?? .github/
?? project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
?? project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
!! bin/
```

`git diff --check`, `git diff --stat`, and product-source diffs produced no tracked diff output because the new workflow and logs are untracked.

Review:

- Product source under `cmd` and `internal` was not modified.
- `.golangci.yml` was not modified.
- `go.mod` was not modified.
- `bin/ctx-bag` remains ignored.
- No root `./ctx-bag` artifact exists.

## Assertions

| Assertion                                         | Result | Evidence |
| ------------------------------------------------- | ------ | -------- |
| Exactly one CI workflow added                     | PASS   | `.github/workflows/ci.yml` only |
| Workflow runs on push to main                     | PASS   | YAML `on.push.branches: main` |
| Workflow runs on PR to main                       | PASS   | YAML `on.pull_request.branches: main` |
| Workflow permissions are read-only                | PASS   | YAML `permissions: contents: read` |
| Go 1.22 compatibility is tested                   | PASS   | matrix includes `'1.22.x'`; local `go1.22.2` validation passed |
| Go 1.27 compatibility is tested                   | PASS   | matrix includes `'1.27.x'`; local `go1.27.0` validation passed |
| Formatting is check-only                          | PASS   | CI uses `gofmt -l` and exits non-zero when files are listed |
| `go vet ./...` runs                               | PASS   | YAML `run: go vet ./...` |
| `go test ./...` runs                              | PASS   | YAML `run: go test ./...` |
| CLI build runs                                    | PASS   | YAML `run: go build ./cmd/ctx-bag` |
| Lint uses separate job                            | PASS   | YAML has separate `lint` job |
| `golangci-lint-action@v9` used                    | PASS   | YAML `uses: golangci/golangci-lint-action@v9` |
| golangci-lint config verification remains enabled | PASS   | no `verify: false`; action default preserved |
| No lint rules weakened                            | PASS   | `.golangci.yml` unchanged |
| No write GitHub permissions added                 | PASS   | only `contents: read` |
| No release automation added                       | PASS   | only `.github/workflows/ci.yml` created |
| No product behavior changed                       | PASS   | no `cmd`/`internal` diffs |
| Local Go 1.22 validation passes                   | PASS   | format/vet/test/build passed with `go1.22.2` |
| Local Go 1.27 validation passes                   | PASS   | format/vet/test/build passed with `go1.27.0` |
| Local lint passes with 0 issues                   | PASS   | `golangci-lint run` reported `0 issues.` |
| No generated binary remains in durable changes    | PASS   | `bin/ctx-bag` ignored; root binary absent |
| No commit performed                               | PASS   | no commit command run |
| No push performed                                 | PASS   | no push command run |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| INFO | Remote GitHub Actions execution has not happened yet. | Workflow is local/uncommitted. | Human review, then commit/push CI and inspect the first Actions run. |
| INFO | Context7 examples for checkout/setup-go showed older action majors than the task-specified current versions. | Context7 GitHub Actions snippets referenced older majors; task specifies `actions/checkout@v7` and `actions/setup-go@v7`. | Use the explicit current versions from the task for CI. |

## Conclusion

MINIMAL CI IMPLEMENTATION: PASS

Local workflow implementation is complete and locally validated. Remote GitHub Actions execution is still pending because the workflow has not been committed or pushed.

No commit was created. No push was performed.
