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
$HOME/go/bin/golangci-lint config verify
$HOME/go/bin/golangci-lint run
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

### Files

```text
A	.github/workflows/ci.yml
M	docs/development.md
A	project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
A	project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
A	project-log/sessions/2026-08-26/session-015-minimal-github-ci.md
A	project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
```

Stat: `6 files changed, 2013 insertions(+), 1 deletion(-)`.

### Modes

All newly staged files were recorded as mode `100644`:

```text
git ls-files --stage
```

No `100755` mode was introduced.

### Diff Review

- No product source (`cmd/`, `internal/`, `go.mod`) changed.
- No generated binary staged; `bin/` is ignored (`.gitignore:1:/bin/`).
- No secrets or credentials found in the staged content.
- Session 013's personal repository absolute path was replaced with the neutral `<repo-root>` placeholder and captured by re-staging.
- The staged session logs document local-verifier Go toolchain paths under `$HOME/go/...`. These are not credentials; they are recorded as a low-severity finding below and left untouched to preserve the authored evidence.
- `git diff --cached --check` passed.

## Commit

### Commit Message

```text
ci: add GitHub Actions validation
```

### Commit SHA

```text
f046dd11539247a54752d9d45007127214115947
```

Created as a normal commit (no `--amend`, no `--force`). `git show --stat --summary HEAD` confirms 6 files, 2013 insertions, 1 deletion, all session logs and the workflow created at mode `100644`.

## Push

```text
To github.com:mhmdnsr-dev/context-baggage.git
   95b3dcd..f046dd1  main -> main
```

Normal push to `main`, no `--force` and no `--force-with-lease`.

## Remote Commit Verification

After `git fetch origin`:

```text
HEAD        = f046dd11539247a54752d9d45007127214115947
origin/main = f046dd11539247a54752d9d45007127214115947
```

`HEAD == origin/main`. The CI publication commit is the tip of `main` on the remote.

## GitHub Actions Run

### Inspection method

GitHub CLI is installed and authenticated (`gh version 2.93.0`, account `mhmdnsr-dev`). Read-only mechanisms were used and cross-checked: `gh run list` / `gh api .../actions/runs`, the Actions workflow object, per-commit `status` and `check-runs`, and the public Actions web page.

### Workflow registration

The workflow was registered and is valid:

```text
github.com/mhmdnsr-dev/context-baggage/actions/workflows/343065117
  name: CI
  path: .github/workflows/ci.yml
  state: active
```

`gh workflow view ci.yml --yaml` returns the same YAML that was committed, so GitHub parsed it exactly as intended. Referenced action majors all resolve to real releases:

```text
actions/checkout              -> v7 (latest v7.0.1)
actions/setup-go              -> v7 (latest v7.0.0)
golangci/golangci-lint-action -> v9 (latest v9.3.0)
```

### Run dispatch result

```text
actions/runs?per_page=20 (no filter)     total_count: 0
actions/workflows/343065117/runs         total_count: 0
commits/f046dd1/check-runs               total_count: 0
commits/f046dd1/status                   state: "pending", total_count: 0
```

The public Actions page (`https://github.com/mhmdnsr-dev/context-baggage/actions`) displayed `0 workflow runs`.

No run (and therefore no job) was dispatched for the CI publication commit `f046dd1`. The commit is the tip of `origin/main`, the workflow is valid and `active`, Actions are enabled (`actions/permissions` -> `enabled: true`), and every referenced action resolves. No runner was created and no checks were posted. This was re-confirmed across repeated polls spanning roughly 11 minutes after the push.

## Remote Jobs

| Remote Job       | Result    | Evidence                                       |
| ---------------- | --------- | ---------------------------------------------- |
| Verify Go 1.22.x | NOT RUN   | no run dispatched for f046dd1; GitHub run      |
| Verify Go 1.27.x | NOT RUN   | no run dispatched for f046dd1; GitHub run      |
| Lint             | NOT RUN   | no run dispatched for f046dd1; GitHub run      |

None of the three jobs executed remotely because GitHub did not create a workflow run for the pushed commit. There is therefore no remote `go version`, no resolved Go patch versions, and no remote `golangci-lint` version to record. Local equivalents (Go 1.22.2, Go 1.27.0, golangci-lint 2.13.1) were verified above but are local evidence, not remote evidence.

## Assertions

| Assertion                                         | Result    | Evidence        |
| ------------------------------------------------- | --------- | --------------- |
| Latest-stable tooling policy documented           | PASS      | development.md  |
| CI workflow unchanged except reviewed publication | PASS      | diff            |
| No product source changed                         | PASS      | staged diff     |
| Session 013 included                              | PASS      | commit          |
| Session 014 included                              | PASS      | commit          |
| Session 015 included                              | PASS      | commit          |
| CI workflow committed                             | PASS      | commit          |
| Commit created without amend/force                | PASS      | Git history     |
| Push to main succeeded                            | PASS      | output          |
| Local HEAD matches origin/main after push         | PASS      | SHAs            |
| GitHub Actions run found for exact commit         | FAIL      | no run dispatched |
| Verify Go 1.22.x passes remotely                  | FAIL      | no run dispatched |
| Verify Go 1.27.x passes remotely                  | FAIL      | no run dispatched |
| Lint passes remotely                              | FAIL      | no run dispatched |
| Actual remote Go versions recorded                | FAIL      | no run dispatched |
| Remote lint version recorded                      | FAIL      | no run dispatched |
| No release created                                | PASS      | state           |
| No tag created                                    | PASS      | state           |
| No force push used                                | PASS      | command history |
| First remote CI validation passes                 | FAIL      | blocked / no run |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| HIGH (blocking) | GitHub did not dispatch any workflow run for the CI publication commit. Workflow is valid and `active`, Actions are enabled, all action refs resolve, commit is `origin/main` tip, yet 0 runs, 0 check-runs, 0 statuses after ~11 minutes. | `gh api actions/runs` = 0; `gh workflow view ci.yml --yaml` valid; public Actions page shows `0 workflow runs` | Confirm GitHub Actions activation for this new repository, then re-check `gh run list --workflow CI`. If it remains undispatched, investigate GitHub-side first-run activation before proceeding to the v0.1.0 release. Do not blame the workflow or change product code. |
| LOW | The new session logs document local-verifier toolchain paths under `$HOME/go/...` (e.g. `golangci-lint` commands [sessions 013, 015, 016] and one `go list -m -json` module-cache `Dir` [session 014]). These are not secrets or credentials and are consistent with the already-committed grep-pattern references in sessions 011/012. | staged diff; `git grep /home/nasr` | Treated as acceptable local-verifier evidence and not rewritten to preserve authored logs. Before the v0.1.0 release, consider normalizing these to `$HOME`/environment variables and scrubbing the single module-cache `Dir` line to avoid publishing a personal home path in a public repo. |

## Final Git State

```text
f046dd1 (HEAD -> main, origin/main) ci: add GitHub Actions validation
95b3dcd feat: implement Context Baggage v0.1
```

Working tree after the remote inspection:

```text
modified:
project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
```

Session 016 was committed before remote execution (with the workflow/local-verification content) and was updated afterward with the remote inspection evidence above. That post-commit edit is intentional and left uncommitted, per the task: the final log update is deferred to the release-preparation commit. Everything else is clean. No release, no tag, no force push.

## Conclusion

The commit and push are complete and verified (`HEAD == origin/main`). The workflow is valid and registered, but GitHub has NOT dispatched any workflow run for the pushed commit, so no remote job executed and no remote CI evidence exists. The §23 pass condition — all three jobs succeeding — is therefore unmet; this is a dispatch/activation blocker rather than a code or workflow failure. No changes were made to product code, no rerun was attempted, and no second commit was created to chase it.

FIRST REMOTE CI VALIDATION: FAIL

---

### Addendum — delayed dispatch (supersedes the "no run dispatched" reading above)

The absence of a run at inspection time was dispatch latency, not a block. A `push` run for this commit (ID `32988513195`, `event = push`, `headSha = f046dd1`) was dispatched later, starting `2026-08-26T16:27:46Z` — roughly 18 minutes after the push. It completed as a failure: `Verify Go 1.22.x` and `Verify Go 1.27.x` both PASS, while `Lint` FAILS because the pinned `golangci-lint v2.12` cannot typecheck the Go 1.27.0 standard library.

The `FIRST REMOTE CI VALIDATION: FAIL` conclusion remains correct, but the reason is the `Lint` version incompatibility, not missing dispatch. See `session-017-github-actions-dispatch-diagnosis.md` for the full diagnosis and the recommended `golangci-lint` version bump.
