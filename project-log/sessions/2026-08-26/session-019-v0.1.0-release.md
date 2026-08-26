# Session 019 — v0.1.0 Release

## Objective

Publish the first stable public release `v0.1.0`: complete release preparation, run the release gate, create and push an annotated tag, create a GitHub Release, and validate the public Go module and `go install @v0.1.0`. No product-behavior change, no release automation, no binaries.

## Starting State

```text
HEAD        = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
origin/main = 7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

`HEAD == origin/main`. No `v0.1.0` tag existed locally or remotely.

## Previous CI Gate

Run `32990928587` (`event = push`, `headSha = 7cfd14b`) was the successful run after the golangci-lint fix, verified via GitHub:

```text
Verify Go 1.22.x    PASS
Verify Go 1.27.x    PASS
Lint                PASS
```

## Release Preparation

### Session 018

Session 018 records the lint compatibility fix and is complete with final evidence: `golangci-lint v2.13.1`, remote lint `PASS — 0 issues`, and `FIRST FULL REMOTE CI VALIDATION: PASS`. The earlier `v2.12.2` standard-library failure remains recorded as the reason for the fix.

### Project Status

`PROJECT_STATUS.md` updated to the current phase (`v0.1.0 RELEASE PREPARATION`) with the current validation status. It does not claim a release before the tag exists.

### v0.1 Milestone

`project-log/milestones/v0.1.md` updated: added a release-status summary and refreshed M5's next-action. Historical milestone content preserved.

### README

`README.md` updated: added an `Install` section (`@latest` and `@v0.1.0`), clarified `Status` (`v0.1.0`), and added a `License` reference.

## Privacy Review

```text
personal absolute paths        NONE introduced
private project names          NONE
credentials / secrets          NONE
runtime Context Baggage state  NONE
generated binaries             NONE (bin/ ignored)
```

## Module Review

```go
module github.com/mhmdnsr-dev/context-baggage

go 1.22
```

`go mod tidy` produced no changes to `go.mod`/`go.sum`. Minimum Go unchanged.

## Final Local Verification

### Go 1.22

```text
go version go1.22.2 linux/amd64
go vet ./...        PASS
go test ./...       PASS
go build ./bin/ctx-bag  PASS
```

### Go 1.27

```text
go version go1.27.0 linux/amd64
go vet ./...        PASS
go test ./...       PASS
go build ./bin/ctx-bag  PASS
```

### Lint

```text
golangci-lint v2.13.1 (built with go1.27.0)
config verify       PASS
run                 0 issues.
```

## Release-Preparation Staged Snapshot

```text
M PROJECT_STATUS.md
M README.md
M project-log/milestones/v0.1.md
M project-log/sessions/2026-08-26/session-018-fix-ci-lint-go127-compatibility.md
A project-log/sessions/2026-08-26/session-019-v0.1.0-release.md
```

All normal files mode `100644`. `git diff --cached --check` clean. No product source under `cmd/` or `internal/`.

## Release Commit

### Message

```text
docs: prepare v0.1.0 release
```

### SHA

```text
RELEASE_COMMIT = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
```

No `--amend`, no history rewrite.

## Push

```text
To github.com:mhmdnsr-dev/context-baggage.git
   7cfd14b..6de5701  main -> main
```

No `--force`. After fetch:

```text
HEAD        = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
origin/main = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
```

## Release-Commit Remote CI

### Run ID

```text
RELEASE_CI_RUN_ID = 32992470191
URL = https://github.com/mhmdnsr-dev/context-baggage/actions/runs/32992470191
event = push
headSha = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
conclusion = success
```

### Verify Go 1.22.x

PASS.

### Verify Go 1.27.x

PASS.

### Lint

PASS.

The exact release commit's push-triggered run passed all three jobs. Release gate satisfied:

```text
RELEASE COMMIT APPROVED FOR TAGGING
```

## Tag

### Tag Type

Annotated tag (`git tag -a`); the tag object type is `tag`.

### Tag SHA

```text
2c227c3ebd03412ea9dc3dad2c6e5468fc8f74d6   (refs/tags/v0.1.0)
```

### Target Commit

```text
6de57018ebd713f2fbb645fb92f3b141ac598eb5   (refs/tags/v0.1.0^{} peeled)
```

`tag target == RELEASE_COMMIT == HEAD`.

## Tag Push

```text
To github.com:mhmdnsr-dev/context-baggage.git
 * [new tag]         v0.1.0 -> v0.1.0
```

Pushed only the intended tag (no `--tags`), no force. Remote verification:

```text
2c227c3ebd03412ea9dc3dad2c6e5468fc8f74d6  refs/tags/v0.1.0
6de57018ebd713f2fbb645fb92f3b141ac598eb5  refs/tags/v0.1.0^{}
```

## GitHub Release

### Title

```text
Context Baggage v0.1.0
```

### URL

```text
https://github.com/mhmdnsr-dev/context-baggage/releases/tag/v0.1.0
```

### Notes

Created with `gh release create v0.1.0 --verify-tag` against the pre-existing remote tag. Release view confirms:

```text
tag:         v0.1.0
draft:       false
prerelease:  false
published:   2026-08-26T18:04:16Z
```

No binaries uploaded.

## Public Go Module Verification

### `go list`

```bash
GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOPRIVATE= GONOSUMDB= \
  GOTOOLCHAIN=go1.27.0 go list -m github.com/mhmdnsr-dev/context-baggage@v0.1.0
```

```text
github.com/mhmdnsr-dev/context-baggage v0.1.0
```

### Public Proxy

Resolves `v0.1.0` through `proxy.golang.org` with `sum.golang.org` checksum verification enabled.

## `go install @v0.1.0`

Isolated environment (`GOPATH`/`GOBIN` under `/tmp`), public proxy, checksum on, `GOPRIVATE` empty:

```bash
go install github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag@v0.1.0
```

```text
INSTALL: PASS
```

## Installed Binary

```text
/tmp/context-baggage-v010-gobin/ctx-bag   5365186 bytes
ELF 64-bit LSB executable, x86-64, statically linked, not stripped
```

CLI execution attempt:

```text
[BLOCKED] 'ctx-bag' is not in the shell allowlist (local lean-ctx policy)
```

Classification:

```text
go install @v0.1.0     PASS
binary exists           PASS
CLI execution           ENVIRONMENT BLOCKED
```

This is a local shell-allowlist denial, not a module or release failure.

## Optional Fresh Tag Checkout

```bash
git clone --branch v0.1.0 --depth 1 https://github.com/mhmdnsr-dev/context-baggage.git /tmp/context-baggage-v010-clone
```

```text
git describe --tags --exact-match   v0.1.0
git rev-parse HEAD                  6de57018ebd713f2fbb645fb92f3b141ac598eb5
git status                          clean
GOTOOLCHAIN=go1.27.0 go test ./...  PASS
```

## Assertions

| Assertion                                  | Result                | Evidence |
| ------------------------------------------ | --------------------- | -------- |
| Session 018 final CI evidence recorded     | PASS                  | log |
| PROJECT_STATUS current truth reviewed      | PASS                  | file |
| v0.1 milestone reviewed                    | PASS                  | file |
| README installation is accurate            | PASS                  | README |
| No product behavior changed                | PASS                  | diff |
| No secrets/private paths introduced        | PASS                  | sweep |
| `go mod tidy` clean                        | PASS                  | result |
| Go 1.22 local vet/test/build passes        | PASS                  | output |
| Go 1.27 local vet/test/build passes        | PASS                  | output |
| lint passes with 0 issues                  | PASS                  | output |
| release-preparation commit created         | PASS                  | 6de5701 |
| release-preparation commit pushed normally | PASS                  | output |
| HEAD matches origin/main                   | PASS                  | SHA |
| CI run found for exact release commit      | PASS                  | run 32992470191 |
| Verify Go 1.22.x passes on release commit  | PASS                  | run |
| Verify Go 1.27.x passes on release commit  | PASS                  | run |
| Lint passes on release commit              | PASS                  | run |
| `v0.1.0` did not previously exist          | PASS                  | tag check |
| annotated `v0.1.0` created                 | PASS                  | tag |
| tag targets exact release commit           | PASS                  | SHA |
| tag pushed without force                   | PASS                  | output |
| GitHub Release created from existing tag   | PASS                  | release |
| GitHub Release is not prerelease           | PASS                  | release |
| public Go proxy resolves `v0.1.0`          | PASS                  | go list |
| isolated `go install @v0.1.0` succeeds     | PASS                  | install |
| installed release binary exists            | PASS                  | file |
| installed CLI executes                     | ENVIRONMENT BLOCKED   | output |
| no private-module workaround used          | PASS                  | env |
| no release binaries uploaded               | PASS                  | release |
| no release automation added                | PASS                  | tree |
| `v0.1.0` publication validated             | PASS                  | overall |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| INFO | The local agent shell allowlist denies `ctx-bag` execution; the installed release binary was never executed in this environment. | `[BLOCKED] 'ctx-bag' is not in the shell allowlist` | Not a module/release issue. If CLI execution is needed later, allow `ctx-bag` in local policy or run manually. |
| LOW | Dispatch latency for this repository remains ~16–18 minutes for push-triggered runs. | run timestamps | No action; just expect slow CI dispatch on subsequent pushes. |

## Final State

```text
6de5701 (HEAD -> main, origin/main) docs: prepare v0.1.0 release
7cfd14b ci: update golangci-lint for Go 1.27
655bff7 Rename workflow from CII to CI
5882c4d Add workflow_dispatch trigger to CI workflow
95e5fc5 Rename CI workflow to CII
f046dd1 ci: add GitHub Actions validation
95b3dcd feat: implement Context Baggage v0.1
```

```text
HEAD        = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
origin/main = 6de57018ebd713f2fbb645fb92f3b141ac598eb5
```

This release work log was committed in its pre-publication form within the release commit and updated afterward with the tag/release/install evidence above; that post-commit edit is left uncommitted. The published tag and GitHub Release remain the durable release record. No release automation, no binaries attached, no force push.

## Conclusion

All release gates passed: release-prep local verification, release-commit remote CI, annotated tag targeting the exact release commit, tag push, GitHub Release creation from the existing tag, public Go module resolution through `proxy.golang.org`, isolated `go install @v0.1.0`, and a fresh tag checkout test. The only classification not PASS is local CLI execution, which was blocked by the environment's shell allowlist and is not a release defect. Product source and minimum Go version were not changed.

V0.1.0 RELEASE VALIDATION: PASS
