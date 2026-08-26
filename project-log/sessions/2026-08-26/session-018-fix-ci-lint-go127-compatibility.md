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

Pending — recorded after commit.

## Push

Pending — recorded after push.

## Remote Run

### Run ID

Pending — recorded after push.

### Event

Pending — expected `push`.

### SHA

Pending — expected to equal the fix commit SHA.

## Remote Jobs

### Verify Go 1.22.x

Pending — recorded after run.

### Verify Go 1.27.x

Pending — recorded after run.

### Lint

Pending — recorded after run.

## Resolved Remote Tool Versions

Pending — recorded after run.

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
| Commit created normally                    | pending   | after commit |
| Push used no force                         | pending   | after push |
| Push-triggered run found for exact commit  | pending   | after run  |
| Remote Go 1.22 job passes                  | pending   | after run  |
| Remote Go 1.27 job passes                  | pending   | after run  |
| Remote Lint job passes                     | pending   | after run  |
| Actual remote lint version recorded        | pending   | after run  |
| Entire remote workflow passes              | pending   | after run  |
| No tag created                             | PASS      | state      |
| No release created                         | PASS      | state      |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| LOW | The previous CI publication verified Go lanes passed but the Lint job failed due to a `golangci-lint v2.12` / Go 1.27 stdlib mismatch. | runs 32988513195, 32987806489 | Resolved here by pinning `v2.13.1`. |

## Final Git State

Pending — recorded after push and run inspection.

## Conclusion

Pending — recorded after the remote run completes.
