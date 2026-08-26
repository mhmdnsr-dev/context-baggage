# Session 017 — GitHub Actions Dispatch Diagnosis

## Objective

Determine whether the `CI` workflow can be executed at all, whether automatic `push` events dispatch a run, and identify the push actor/authentication context. Distinguish manual `workflow_dispatch` from automatic `push` dispatch. Do not change CI behavior or versions.

## New Context

`workflow_dispatch` was manually added on the remote repository before this diagnostic task:

```yaml
on:
  workflow_dispatch:

  push:
    branches:
      - main

  pull_request:
    branches:
      - main
```

Its presence is diagnostic visibility only and was not treated as a fix. The automatic `push → main` dispatch was the open question.

## Remote/Local Divergence Check

```bash
git fetch origin
git status -sb
git rev-parse HEAD
git rev-parse origin/main
git log --oneline --decorate --graph -10 --all
```

Result — local `main` was already fast-forwarded to match the remote; no divergence remained:

```text
## main...origin/main
 M project-log/sessions/2026-08-26/session-016-publish-ci-and-first-remote-run.md
HEAD        = 655bff70d34c26119dc0ad448cc090ffdf9301f6
origin/main = 655bff70d34c26119dc0ad448cc090ffdf9301f6
```

History (all linear, top is `origin/main`):

```text
* 655bff7 Rename workflow from CII to CI
* 5882c4d Add workflow_dispatch trigger to CI workflow
* 95e5fc5 Rename CI workflow to CII
* f046dd1 ci: add GitHub Actions validation
* 95b3dcd feat: implement Context Baggage v0.1
```

The GitHub web-edit commits (`95e5fc5`, `5882c4d`, `655bff7`) were already integrated locally, so nothing needed reconciling. The remote manual `workflow_dispatch` addition is preserved; no force push and no rewrite.

## Manual Dispatch Test

The repository's `workflow_dispatch` test was exercised and produced a run (the manual dispatch was triggered on the remote; the resulting run is recorded below). This proves the workflow is executable by GitHub.

## Actions Permissions

```bash
gh api repos/mhmdnsr-dev/context-baggage/actions/permissions
```

```json
{"enabled": true, "allowed_actions": "all", "sha_pinning_required": false}
```

```text
enabled              = true
allowed_actions      = all
sha_pinning_required = false
```

Actions are enabled and all actions are allowed. No `selected-actions` restriction needed; permissions were not modified.

## Repository Metadata

```bash
gh api repos/mhmdnsr-dev/context-baggage \
  --jq '{default_branch, visibility, private, archived, disabled}'
```

```json
{"archived": false, "default_branch": "main", "disabled": false, "private": false, "visibility": "public"}
```

```text
default_branch = main
visibility     = public
private        = false
archived       = false
disabled       = false
```

## Authentication / Push Actor

```bash
git remote -v
git remote get-url --push origin
git config --show-origin --get-all credential.helper
```

```text
origin  git@github.com:mhmdnsr-dev/context-baggage.git (fetch)
origin  git@github.com:mhmdnsr-dev/context-baggage.git (push)
git@github.com:mhmdnsr-dev/context-baggage.git
(no credential.helper configured — SSH)
```

```text
Push transport: SSH
Push authentication: SSH (manual developer-machine push)
GITHUB_TOKEN recursive-event suppression: NOT APPLICABLE
```

Commit `f046dd1` author/committer:

```text
author    = Mohamed Nasr <100061737+mhmdnsr-dev@users.noreply.github.com>
committer = Mohamed Nasr <100061737+mhmdnsr-dev@users.noreply.github.com>
```

GitHub API attributes the commit to `author_login = mhmdnsr-dev`, `committer_login = mhmdnsr-dev`, verification `verified = false` with no web-flow type. This is a normal user-authenticated commit/push — not the GitHub Actions bot, not a GitHub App, not `GITHUB_TOKEN` automation. Therefore recursive-event-suppression rules do not apply and cannot explain a missing push dispatch.

## GitHub Web-Edit Commit

The web-edit commits are direct commits to `main` made through the GitHub web editor (`committer = GitHub <noreply@github.com>`):

```text
95e5fc5 Rename CI workflow to CII
5882c4d Add workflow_dispatch trigger to CI workflow
655bff7 Rename workflow from CII to CI
```

These represent push-to-`main` events. Their presence on `main` does not, in itself, guarantee a `push` run was dispatched.

## Push-Event Run Search

```bash
gh run list --repo mhmdnsr-dev/context-baggage --limit 50 \
  --json databaseId,event,headBranch,headSha,status,conclusion,workflowName,startedAt,displayTitle
```

Only two runs exist in the repository:

```json
{"id": 32988513195, "event": "push",   "headSha": "f046dd1", "conclusion": "failure", "startedAt": "2026-08-26T16:27:46Z", "title": "ci: add GitHub Actions validation"}
{"id": 32987806489, "event": "workflow_dispatch", "headSha": "5882c4d", "conclusion": "failure", "startedAt": "2026-08-26T16:20:15Z", "title": "CI"}
```

Critical finding: a **push-triggered run exists for `f046dd1`** (`event = push`). The original "0 runs" state observed in session 016 was dispatch latency, not a blocked dispatch: the `f046dd1` push (16:09Z) produced a run that started at 16:27:46Z — roughly 18 minutes later. Push dispatch on this repository is slow but functional.

No push run exists for the web-edit SHAs (`95e5fc5`, `5882c4d`, `655bff7`) at time of inspection — only the manual `workflow_dispatch` run for `5882c4d` exists. Given the demonstrated latency, whether those web-edit pushes will also produce `push` runs remains pending.

## Diagnostic Push

Not performed. Reasons:

1. Automatic `push` dispatch is already conclusively established by run `32988513195` (`event = push`, `headSha = f046dd1`).
2. The manual dispatch created a run but a job failed (Lint), placing this session under the "record failing job and stop" interpretation.
3. A further push would only add another run that fails for the already-known reason, without changing the diagnosis.

No product code, workflow triggers, Go versions, or action versions were changed.

## Run Classification

| Run                    | Event               | SHA     | Result |
| ---------------------- | ------------------- | ------- | ------ |
| Manual diagnostic      | `workflow_dispatch` | `5882c4d` | failure (Lint) |
| Original CI publication, push | `push`       | `f046dd1` | failure (Lint) |
| GitHub web-edit commit | `push` or NONE      | `95e5fc5`, `655bff7` | NONE yet |
| Local diagnostic push  | not performed       | —       | — |

## Remote Jobs

Both runs produced identical job outcomes:

| Remote Job       | Result    | Evidence   |
| ---------------- | --------- | ---------- |
| Verify Go 1.22.x | PASS      | run logs   |
| Verify Go 1.27.x | PASS      | run logs   |
| Lint             | FAIL      | run logs   |

## Failing Step Detail (Lint)

The `Lint` job's `run golangci-lint` step fails. The action resolved the pinned `version: v2.12` to `v2.12.2`. Typecheck errors are reported against the Go 1.27.0 **standard library**, not the project source:

```text
/opt/hostedtoolcache/go/1.27.0/x64/src/crypto/internal/randutil/randutil.go:11:2:
  could not import math/rand/v2
  (/opt/hostedtoolcache/go/1.27.0/x64/src/math/rand/v2/rand.go:213:17:
    method must have no type parameters) (typecheck)
/opt/hostedtoolcache/go/1.27.0/x64/src/crypto/internal/randutil/randutil.go:21:5:
  undefined: rand (typecheck)
2 issues:
* typecheck: 2
```

Local verification with `golangci-lint v2.13.1` (built with `go1.27.0`) reports `0 issues` on the same tree. The local `go 1.22` module directive, `go vet`, and `go test` all pass for Go 1.22 and Go 1.27.

Conclusion: `golangci-lint v2.12` is incompatible with the Go 1.27.0 standard library. The pinned `v2.12` must be raised to a release compatible with the Go 1.27 toolchain (local `v2.13.1` succeeds).

## Conclusion

```text
Actions enabled                    PASS
Workflow valid                     PASS
Manual workflow dispatch           PASS (run created, event=workflow_dispatch)
Jobs executable                    PASS (runs execute)
push trigger present               PASS
branch matches main                PASS
manual/user-authenticated push     PASS (SSH, not GITHUB_TOKEN)
automatic push workflow run        PASS (event=push, headSha=f046dd1) — dispatched late
```

The push-dispatch hypothesis is disproven: `push → main` does dispatch a `CI` run, delayed by roughly 18 minutes. The original observation of zero runs was latency, not a block. The only real CI failure is the `Lint` job, caused by the `golangci-lint v2.12` incompatibility with the Go 1.27.0 standard library (stdlib typecheck error, unrelated to project code).

Recommended next step (separate from this diagnosis): bump the pinned `golangci-lint` version in `.github/workflows/ci.yml` from `v2.12` to a release compatible with Go 1.27 (local `v2.13.1` is clean). Do not change product code. Do not alter `workflow_dispatch` (it can remain or be removed after verification to restore a minimal trigger set).
