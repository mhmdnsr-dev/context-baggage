# Session 006: Lint Hardening

Date: 2026-08-26

## Goal

Resolve all findings from the first real `golangci-lint run`.

## Baseline

```text
20 issues
18 errcheck
2 staticcheck
```

The reported files were:

```text
internal/app/app.go
internal/app/app_test.go
internal/store/store.go
internal/sync/sync.go
internal/config/config.go
```

## Important Distinction Learned

```text
Writable Close()
    -> error matters and should normally propagate.

Read-only Close()
    -> cleanup may be best-effort when reading has completed.

Remove/RemoveAll cleanup
    -> may be intentionally best-effort when it cannot affect the operation result.
```

## Work Completed

- Checked and propagated CLI output write failures.
- Checked writable file `Close()` calls before treating writes as successful.
- Made read-only file close cleanup explicitly best-effort.
- Made temporary file/directory cleanup explicitly best-effort where failures do not affect the operation result.
- Replaced paragraph-style Go error strings with composable lower-case errors.
- Fixed test working-directory cleanup.
- Preserved the existing v0.1 behavior and command surface.

## Verification

- `go fmt ./...`: passed.
- `go vet ./...`: passed.
- `go test ./...`: passed.
- `golangci-lint config verify`: passed using the local `golangci-lint` binary.
- `golangci-lint run`: passed with 0 issues using the local `golangci-lint` binary.
- `go build -o ./bin/ctx-bag ./cmd/ctx-bag`: passed.
- `./bin/ctx-bag --help`: passed.

## Final Lint Status

```text
0 issues
```

## Next Step

Resume the guided manual `v0.1` validation.
