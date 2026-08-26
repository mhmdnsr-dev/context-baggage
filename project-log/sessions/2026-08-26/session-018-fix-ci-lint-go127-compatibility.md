# Session 018 — Fix CI Lint Go 1.27 Compatibility

## Objective

Fix the single remote CI blocker: the `Lint` job fails because the pinned `golangci-lint v2.12` cannot type-check the Go 1.27.0 standard library. Pin the latest stable compatible `golangci-lint`, validate locally, and revalidate the full remote CI. Change only the `golangci-lint` version variable. No product-code change, no lint-rule weakening, no tag, no release.

## Starting State

```text
HEAD        = 655bff70d34c26119dc0ad448cc090ffdf9301f6
origin/main = 655bff70d34c26119dc0ad448cc090ffdf9301f6
```

```text
## main...origin/main
 M project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
?? project-log/sessions/2026-08-26/session-017-github-actions-dispatch-diagnosis.md
```

The workflow is the current `main` version with triggers unchanged:

```yaml
on:
  push:
    branches:
      - main
  pull_request:
    branches:
      - main
  workflow_dispatch:
```

## Existing Remote Failure

### Push Run

Run `32988513195` (`event = push`, `headSha = f046dd1`) — the CI publication push — completed as a failure.

### Manual Run

Run `32987806489` (`event = workflow_dispatch`, `headSha = 5882c4d`) — completed as a failure.

### Passing Jobs

Both runs:
```text
Verify Go 1.22.x    PASS
Verify Go 1.27.x    PASS
```

### Failing Lint Job

Both runs: `Lint` FAILS on the `run golangci-lint` step.

## Root Cause

The `Lint` job uses `golangci/golangci-lint-action@v9` with `version: v2.12`, which resolved remotely to `golangci-lint v2.12.2`. That binary cannot type-check the Go `1.27.0` standard library used by the lint job. Errors originate in Go's standard library, not project source:

```text
$GOROOT/src/crypto/internal/randutil/randutil.go:11:2:
  could not import math/rand/v2
  ($GOROOT/src/math/rand/v2/rand.go:213:17:
    method must have no type parameters) (typecheck)
$GOROOT/src/crypto/internal/randutil/randutil.go:21:5:
  undefined: rand (typecheck)
2 issues: * typecheck: 2
```

This is a toolchain/tool mismatch, not a Context Baggage defect: `go vet`, `go test`, and `go build` all pass on Go 1.27.0, and local `golangci-lint v2.13.1` (built with Go 1.27.0) reports `0 issues`.

## Latest Stable Version Lookup

### Official Upstream Result

```bash
gh api repos/golangci/golangci-lint/releases/latest \
  --jq '{tag_name, published_at, prerelease, draft}'
```

```json
{"draft": false, "prerelease": false, "published_at": "2026-08-20T14:59:22Z", "tag_name": "v2.13.1"}
```

```text
draft      = false
prerelease = false
LATEST_STABLE_GOLANGCI_LINT = v2.13.1
```

### Local Version

```bash
golangci-lint --version
```

```text
golangci-lint has version 2.13.1 built with go1.27.0 from 6d2288e0 on 2026-08-20T14:28:34Z
```

### Selected Version

The locally installed version matches the official latest stable exactly, so the selected pin is:

```text
SELECTED_LINT_VERSION = v2.13.1
```

## Workflow Change

```diff
       - name: Lint
         uses: golangci/golangci-lint-action@v9
         with:
-          version: v2.12
+          version: v2.13.1
```

Triggers, Go lanes, `actions/checkout@v7`, `actions/setup-go@v7`, and `golangci/golangci-lint-action@v9` are unchanged. Only the pinned `golangci-lint` version changed.

## Local Validation

### Go 1.22

```text
GOTOOLCHAIN=go1.22.2 go vet ./...      PASS
GOTOOLCHAIN=go1.22.2 go test ./...      PASS
GOTOOLCHAIN=go1.22.2 go build ./cmd/ctx-bag  PASS
```

### Go 1.27

```text
GOTOOLCHAIN=go1.27.0 go vet ./...      PASS
GOTOOLCHAIN=go1.27.0 go test ./...      PASS
GOTOOLCHAIN=go1.27.0 go build ./cmd/ctx-bag  PASS
```

### Lint

```text
golangci-lint config verify    PASS
golangci-lint run              0 issues.
```

The exact selected version `v2.13.1` is the installed local version, so local lint evidence is at the same version as the new remote pin.

## Privacy Normalization

Personal Go-home paths in recent session logs (sessions 013, 014, 015, 016) were normalized from a hard-coded absolute home path to the portable `$HOME/go` form:

```text
$HOME/go/bin/golangci-lint   (golangci-lint command locations — sessions 013, 015, 016)
$HOME/go/pkg/mod/...         (Go module-cache path — session 014)
```

Audit grep-pattern lines (`rg '<home>|/media/...'`, used to search for personal paths) were intentionally left untouched as documented audit commands.

## Staged Snapshot

### Files

```text
M .github/workflows/ci.yml
M project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
M project-log/sessions/2026-08-26/session-014-public-go-module-consumer-revalidation.md
M project-log/sessions/2026-08-26/session-015-minimal-github-ci.md
M project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
A project-log/sessions/2026-08-26/session-017-github-actions-dispatch-diagnosis.md
A project-log/sessions/2026-08-26/session-018-fix-ci-lint-go127-compatibility.md
```

No `cmd/` or `internal/` change. No binary, no runtime state, no secrets.

## Commit

```bash
git commit -m "ci: update golangci-lint for Go 1.27"
```

```text
[main 7cfd14b] ci: update golangci-lint for Go 1.27
 7 files changed, 635 insertions(+), 21 deletions(-)
 create mode 100644 project-log/sessions/2026-08-26/session-017-github-actions-dispatch-diagnosis.md
 create mode 100644 project-log/sessions/2026-08-26/session-018-fix-ci-lint-go127-compatibility.md
```

```text
LINT_FIX_COMMIT = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

Created normally, no `--amend`, no history rewrite.

## Push

```bash
git push origin main
```

```text
To github.com:mhmdnsr-dev/context-baggage.git
   655bff7..7cfd14b  main -> main
```

No `--force`, no `--force-with-lease`.

## Remote Run

### Run ID

```text
RUN_ID   = 32990928587
RUN_URL  = https://github.com/mhmdnsr-dev/context-baggage/actions/runs/32990928587
```

### Event

```text
EVENT = push
```

### SHA

```text
HEAD_SHA = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

`HEAD_SHA == LINT_FIX_COMMIT` and `EVENT == push` — this is the push-triggered run for the exact fix commit, not a manual dispatch and not an older run. It was dispatched starting `2026-08-26T16:52:49Z`, consistent with the repository's ~16–18 minute push-dispatch latency.

## Remote Jobs

| Remote Job       | Result   | Evidence       |
| ---------------- | -------- | -------------- |
| Verify Go 1.22.x | PASS     | job 98248065427 |
| Verify Go 1.27.x | PASS     | job 98248065477 |
| Lint             | PASS     | job 98248065232 |

### Verify Go 1.22.x

```text
result           PASS
actual Go version  go1.22.12 linux/amd64
Setup log        "Successfully set up Go version 1.22.x" / "go version go1.22.12 linux/amd64"
```

### Verify Go 1.27.x

```text
result           PASS
actual Go version  go1.27.0 linux/amd64
Setup log        "Successfully set up Go version 1.27.x" / "go version go1.27.0 linux/amd64"
```

### Lint

```text
result            PASS
action version    golangci/golangci-lint-action@v9
resolved lint     golangci-lint 2.13.1 (installed golangci-lint-2.13.1-linux-amd64)
result line       "0 issues." / "golangci-lint found no issues"
```

The resolved version is `v2.13.1`, not the previous `v2.12.2`. The Go 1.27 standard-library type-check error no longer occurs.

## Resolved Remote Tool Versions

```text
Go minimum lane            go1.22.12
Go current lane            go1.27.0
golangci-lint-action       v9
golangci-lint              v2.13.1
```

## Assertions

| Assertion                                  | Result    | Evidence   |
| ------------------------------------------ | --------- | ---------- |
| Push dispatch diagnosis corrected          | PASS      | session 016/017 |
| Official latest stable golangci-lint checked | PASS      | GitHub API |
| Selected lint version is stable            | PASS      | v2.13.1 (draft=false, prerelease=false) |
| Selected lint version compatible with Go 1.27 | PASS    | local 0 issues on v2.13.1/go1.27 |
| Workflow triggers unchanged                | PASS      | diff       |
| Only lint version changed behaviorally     | PASS      | workflow diff |
| No lint rules weakened                     | PASS      | config/diff |
| No product code changed                    | PASS      | diff       |
| Go 1.22 local validation passes            | PASS      | output     |
| Go 1.27 local validation passes            | PASS      | output     |
| Local lint passes                          | PASS      | 0 issues   |
| Privacy paths normalized                   | PASS      | sweep      |
| Commit created normally                            | PASS      | 7cfd14b (no amend) |
| Push used no force                                 | PASS      | push output |
| Push-triggered run found for exact commit          | PASS      | run 32990928587, event=push, SHA=7cfd14b |
| Remote Go 1.22 job passes                          | PASS      | run 32990928587 |
| Remote Go 1.27 job passes                          | PASS      | run 32990928587 |
| Remote Lint job passes                             | PASS      | run 32990928587, 0 issues |
| Actual remote lint version recorded                | PASS      | v2.13.1 |
| Entire remote workflow passes                      | PASS      | run 32990928587 |
| No tag created                                     | PASS      | state      |
| No release created                                 | PASS      | state      |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| LOW | The previous CI publication verified Go lanes passed but the Lint job failed due to a `golangci-lint v2.12` / Go 1.27 stdlib mismatch. | runs 32988513195, 32987806489 | Resolved here by pinning `v2.13.1`. |

## Final Git State

```text
7cfd14b (HEAD -> main, origin/main) ci: update golangci-lint for Go 1.27
655bff7 Rename workflow from CII to CI
5882c4d Add workflow_dispatch trigger to CI workflow
95e5fc5 Rename CI workflow to CII
f046dd1 ci: add GitHub Actions validation
95b3dcd feat: implement Context Baggage v0.1
```

```text
HEAD        = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
origin/main = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

`HEAD == origin/main`. This work log (session 018) was committed in its pre-run form and updated after the remote run with the evidence above; that post-commit edit is left uncommitted and will be included in the release-preparation commit. Everything else is clean. No tag, no release, no force push.

## Conclusion

The single CI blocker — `golangci-lint v2.12` being unable to type-check the Go 1.27.0 standard library — is resolved by pinning the officially latest stable `golangci-lint v2.13.1`. The workflow change was exactly one line; triggers, Go lanes, and Action majors were untouched, and no lint rules were weakened. The same remote push-triggered run (`32990928587`, `event = push`, `headSha = 7cfd14b`) passed all three jobs. Dispatch latency still applies to this repository (~16–18 minutes), but dispatch is confirmed functional.

FIRST FULL REMOTE CI VALIDATION: PASS
