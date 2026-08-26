# Session 007: Sync False Conflict Fix

Date: 2026-08-26

## Discovery

Manual `A -> B -> A` sync validation produced a false conflict. Machine A pushed a portable snapshot, made no portable workspace/task changes, then failed to pull Machine B's later checkpoint.

## Root Cause

`sync.Pull` calculated local state with `store.HashDir(s.Home)`. That hashed the whole Context Baggage home, including machine-local bookkeeping such as `sync/state.yaml` and machine identity such as `device.yaml`.

Because `sync push` updates `sync/state.yaml`, Machine A's local hash changed after a successful push even though its portable workspace/task state did not change.

## Fix

- Pull conflict detection now hashes the same eligible portable snapshot that push exports.
- Sync state now stores `baseHash`, the last portable state known to be shared with the sync folder.
- Conflict detection uses explicit `BASE`, `LOCAL`, and `REMOTE` semantics.
- `LOCAL == REMOTE` is always safe.
- Only true divergence remains a conflict.

## Safety Preserved

- No silent last-write-wins.
- Real divergence still reports `CONFLICT DETECTED`.
- Device identity remains local.
- Sync metadata remains local.
- `sync: false` workspaces remain excluded from export.

## Verification

- `go fmt ./...`: passed.
- `go vet ./...`: passed.
- `go test ./...`: passed.
- `golangci-lint config verify`: passed using the local `golangci-lint` binary.
- `golangci-lint run`: passed with 0 issues using the local `golangci-lint` binary.
- `go build -o ./bin/ctx-bag ./cmd/ctx-bag`: passed.
- `./bin/ctx-bag --help`: passed.

## Next Step

Re-run the same guided manual `A -> B -> A` synchronization scenario, then intentionally test a real two-sided conflict.
