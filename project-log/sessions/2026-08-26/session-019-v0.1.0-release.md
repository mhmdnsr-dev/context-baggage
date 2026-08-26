# Session 019 — v0.1.0 Release

## Objective

Publish the first stable public release `v0.1.0`: complete release preparation, run the release gate, create and push an annotated tag, create a GitHub Release, and validate the public Go module and `go install @v0.1.0`. No product-behavior change, no release automation, no binaries.

## Starting State

### HEAD

```text
7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

### origin/main

```text
7cfd14b1866ac7725712c0a5ca01a85883c02a86
```

`HEAD == origin/main`.

### Existing Tags

```text
local tags:   (none)
remote tags:  (none)
```

`v0.1.0` did not already exist locally or remotely. No overwrite required.

## Previous CI Gate

Run `32990928587` (`event = push`, `headSha = 7cfd14b`) is the successful run after the golangci-lint fix. Verified via GitHub:

```text
Verify Go 1.22.x    PASS
Verify Go 1.27.x    PASS
Lint                PASS
```

## Release Preparation

### Session 018

Session 018 records the lint compatibility fix and is complete with final evidence: `golangci-lint v2.13.1`, remote lint `PASS — 0 issues`, and `FIRST FULL REMOTE CI VALIDATION: PASS`. The earlier `v2.12.2` stdlib failure remains recorded as the reason for the fix.

### Project Status

`PROJECT_STATUS.md` updated to the current phase (`v0.1.0 RELEASE PREPARATION`) with current validation status. It does not claim a release before the tag exists.

### v0.1 Milestone

`project-log/milestones/v0.1.md` updated: added a release-status summary and refreshed M5's next-action. Historical milestone content preserved.

### README

`README.md` updated: added an `Install` section (`@latest` and `@v0.1.0`), clarified `Status` (`v0.1.0`), and added a `License` reference. No marketing filler and no overstated features.

## Privacy Review

```text
personal absolute paths      NONE introduced (no /home/nasr, /media/nasr, /Users, Windows)
private project names        NONE
credentials / secrets        NONE
runtime Context Baggage state NONE
generated binaries           NONE (bin/ ignored)
```

## Module Review

`go.mod`:

```go
module github.com/mhmdnsr-dev/context-baggage

go 1.22
```

`go mod tidy` produced no changes to `go.mod`/`go.sum`.

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

Pending — recorded at staging.

## Release Commit

### Message

Pending — recorded at commit.

### SHA

Pending — recorded at commit.

## Push

Pending — recorded at push.

## Release-Commit Remote CI

### Run ID

Pending — recorded after CI on the release commit.

### Verify Go 1.22.x

### Verify Go 1.27.x

### Lint

## Tag

### Tag Type

### Tag SHA

### Target Commit

## Tag Push

## GitHub Release

### Title

### URL

### Notes

## Public Go Module Verification

### `go list`

### Public Proxy

## `go install @v0.1.0`

## Installed Binary

## Assertions

Pending final publication evidence.

## Findings

Pending final publication evidence.

## Final State

Pending final publication evidence.

## Conclusion

Pending final publication evidence.
