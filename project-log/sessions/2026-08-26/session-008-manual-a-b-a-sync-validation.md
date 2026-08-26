# Session 008: Manual A -> B -> A Sync Validation

Date: 2026-08-26

## Scenario

Re-ran the manual two-machine synchronization sequence that originally exposed the false conflict:

```text
Machine A init -> workspace init --sync -> task start -> checkpoint A -> sync push
Machine B init -> sync pull -> checkpoint B -> sync push
Machine A unchanged -> sync pull
Machine A unchanged -> sync pull again
```

All state lived under isolated `/tmp/ctx-bag-*` directories. The user's real Context Baggage home and real Claude/Codex configuration were not touched.

## Result

Manual A -> B -> A regression: PASS.

The previously failing unchanged Machine A pull completed successfully:

```text
Sync pull complete
Hash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

## Important Identities And Hashes

- Machine A device: `d_61ea5bccf706d99139230cad484de08b`
- Machine B device: `d_2aa98a3b3d3e76baa2d0f1906f9e059c`
- Workspace ID: `w_a49d7498d9efa7f4`
- First push hash: `55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab`
- Machine B push hash: `0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a`

## Validation Details

- Device IDs stayed different.
- The same Git remote mapped to the same workspace ID across different local paths.
- Machine B's workspace status reported Git root `/tmp/ctx-bag-machine-b`.
- Machine A's final pull succeeded without `CONFLICT DETECTED`.
- Machine A received both checkpoint records.
- Checkpoint A retained Machine A's device ID.
- Checkpoint B retained Machine B's device ID.
- Machine A's `baseHash` updated to Machine B's pushed hash.
- Repeated Machine A pull succeeded and left exactly two checkpoint records.
- The sync folder did not contain `device.yaml` or `sync/state.yaml`.

## Next Validation Step

Intentionally test a real two-sided conflict.
