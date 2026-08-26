# Session 009 — Manual Real Conflict Validation

Date: 2026-08-26

## Objective

Validate that a real two-sided sync divergence still produces `CONFLICT DETECTED`, with no silent last-write-wins and no loss of either machine's local work.

## Preflight

### Command

```bash
test -x <repo-root>/bin/ctx-bag
```

### Output

```text
<no output>
```

### Command

```bash
<repo-root>/bin/ctx-bag --help
```

### Output

```text
ctx-bag

Commands:
  init
  status
  doctor
  discover
  workspace init [--sync|--no-sync]
  workspace status
  task start <name>
  task status
  task resume <name>
  checkpoint -m <message>
  handoff
  sync init <folder>
  sync status
  sync push
  sync pull
```

### Observation

The built binary exists, is executable, and renders help.

## Starting Conditions

Mode:

```text
REUSED EXISTING CLEAN SHARED BASE
```

The state from `session-008` still existed and matched the completed successful A -> B -> A validation state.

```text
Device A: d_61ea5bccf706d99139230cad484de08b
Device B: d_2aa98a3b3d3e76baa2d0f1906f9e059c
Workspace ID: w_a49d7498d9efa7f4
Shared base hash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
SHARED BASE CHECKPOINT COUNT: 2
```

### Machine A Status Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag status
```

### Output

```text
Context Baggage

Device
  ID: d_61ea5bccf706d99139230cad484de08b
  OS: linux

Workspace
  ctx-bag-machine-a
  w_a49d7498d9efa7f4

Active Task
  test-task

Sync
  configured
  folder: /tmp/ctx-bag-sync
  last push: 2026-08-26T11:14:07Z
```

### Machine A Workspace Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag workspace status
```

### Output

```text
Workspace
Name: ctx-bag-machine-a
ID: w_a49d7498d9efa7f4
Git root: /tmp/ctx-bag-machine-a
Identity: git-remote:example.com/org/example-repo
Sync: true
```

### Machine A Task Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag task status
```

### Output

```text
Tasks
* test-task (active)
```

### Machine A Sync State

```bash
sed -n '1,20p' /tmp/ctx-bag-machine-a-home/sync/state.yaml
```

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:07Z
lastPull: 2026-08-26T11:15:22Z
lastPushHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
lastPullHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Machine B Status Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag status
```

### Output

```text
Context Baggage

Device
  ID: d_2aa98a3b3d3e76baa2d0f1906f9e059c
  OS: linux

Workspace
  ctx-bag-machine-a
  w_a49d7498d9efa7f4

Active Task
  test-task

Sync
  configured
  folder: /tmp/ctx-bag-sync
  last push: 2026-08-26T11:14:55Z
```

### Machine B Workspace Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag workspace status
```

### Output

```text
Workspace
Name: ctx-bag-machine-a
ID: w_a49d7498d9efa7f4
Git root: /tmp/ctx-bag-machine-b
Identity: git-remote:example.com/org/example-repo
Sync: true
```

### Machine B Task Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag task status
```

### Output

```text
Tasks
* test-task (active)
```

### Machine B Sync State

```bash
sed -n '1,20p' /tmp/ctx-bag-machine-b-home/sync/state.yaml
```

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:55Z
lastPull: 2026-08-26T11:14:44Z
lastPushHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
lastPullHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Shared Checkpoints Before Divergence

Machine A:

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
```

Machine B:

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
```

### Starting Assertions

| Assertion | Result | Evidence |
|---|---|---|
| Device A != Device B | PASS | `d_61...e08b` vs `d_2a...059c` |
| Workspace A == Workspace B | PASS | both `w_a49d7498d9efa7f4` |
| BASE A == BASE B | PASS | both `0ec514f1...3ec4a` |
| Portable task state is shared | PASS | both checkpoint files contained the same two records |

## Step 1 — Machine A Local Divergence

### Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag checkpoint \
  -m "Independent change created on simulated machine A"
```

### Output

```text
Checkpoint recorded
```

### State After Step

Machine A checkpoints:

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:30:59Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Independent change created on simulated machine A"}
```

Machine A sync state:

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:07Z
lastPull: 2026-08-26T11:15:22Z
lastPushHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
lastPullHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Observation

Machine A local portable state changed. `baseHash` did not change, which is correct because no sync operation succeeded.

## Step 2 — Machine B Local Divergence

### Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag checkpoint \
  -m "Independent change created on simulated machine B"
```

### Output

```text
Checkpoint recorded
```

### State After Step

Machine B checkpoints:

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:31:11Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Independent change created on simulated machine B"}
```

Machine B sync state:

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:55Z
lastPull: 2026-08-26T11:14:44Z
lastPushHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
lastPullHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Observation

Machine B local portable state changed independently from Machine A. Machine B still has the original shared `baseHash`.

## Step 3 — Machine A Push

### Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag sync push &&
sed -n '1,20p' /tmp/ctx-bag-machine-a-home/sync/state.yaml
```

### Output

```text
Sync push complete
Hash: 26beb41badd6f19ca0966139af4b8992924e3c9b6b5844c5678f3034f4ee1ac8
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:31:26Z
lastPull: 2026-08-26T11:15:22Z
lastPushHash: 26beb41badd6f19ca0966139af4b8992924e3c9b6b5844c5678f3034f4ee1ac8
lastPullHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
baseHash: 26beb41badd6f19ca0966139af4b8992924e3c9b6b5844c5678f3034f4ee1ac8
```

### Observation

Machine A push succeeded as expected because `LOCAL A != BASE` and `REMOTE == BASE`.

## Step 4 — Remote Snapshot Before Conflict

### Command

```bash
find /tmp/ctx-bag-sync -type f | sort
```

### Output

```text
/tmp/ctx-bag-sync/context-baggage-state/config.yaml
/tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/active-task
/tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
/tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/task.yaml
/tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/workspace.yaml
```

### Command

```bash
sed -n '1,30p' /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Output

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:30:59Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Independent change created on simulated machine A"}
```

### Command

```bash
sha256sum /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Output

```text
27ffbe47966f73d1811f887f7a21da39bd353e1b822026c417fe06c100654caf  /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Observation

Remote checkpoint count is 3. Remote contains the shared original checkpoints plus Machine A's independent checkpoint. It does not contain Machine B's independent checkpoint.

## Step 5 — Machine B Conflicting Push

### Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag sync push
echo EXIT:$?
```

### Output

```text
EXIT:1

CONFLICT DETECTED
resource: sync folder
local hash: a24dcfde04e3b28648c5799e36a03214f65858a84a26ce17606d9be30cce2c52
incoming hash: 26beb41badd6f19ca0966139af4b8992924e3c9b6b5844c5678f3034f4ee1ac8
safe next action: inspect /tmp/ctx-bag-sync/context-baggage-state before pushing
```

### Observation

Machine B conflicting push was rejected. This is the expected result for `BASE=0ec514f1...3ec4a`, `LOCAL=B2=a24dcfde...e2c52`, `REMOTE=A2=26beb41b...e1ac8`.

## Step 6 — Remote Preservation Check

### Command

```bash
sed -n '1,30p' /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Output

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:30:59Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Independent change created on simulated machine A"}
```

### Command

```bash
sha256sum /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Output

```text
27ffbe47966f73d1811f887f7a21da39bd353e1b822026c417fe06c100654caf  /tmp/ctx-bag-sync/context-baggage-state/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Observation

Rejected conflicting push did not mutate remote portable state. The remote checkpoint file hash stayed `27ffbe47...54caf`; it still contains Machine A's independent checkpoint and does not contain Machine B's independent checkpoint.

## Step 7 — Machine B Local Preservation Check

### Command

```bash
sed -n '1,30p' /tmp/ctx-bag-machine-b-home/workspaces/w_a49d7498d9efa7f4/tasks/test-task/checkpoints.jsonl
```

### Output

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:31:11Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Independent change created on simulated machine B"}
```

### Observation

Conflict detection preserved Machine B's local divergent work.

## Step 8 — Sync Bookkeeping Check

### Command

```bash
sed -n '1,20p' /tmp/ctx-bag-machine-b-home/sync/state.yaml
```

### Output

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:55Z
lastPull: 2026-08-26T11:14:44Z
lastPushHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
lastPullHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Observation

Machine B's `baseHash` was not falsely advanced to Machine A's A2 hash after the rejected push. `baseHash` still represents the last state Machine B successfully synchronized.

## Step 9 — Conflicting Pull Check

### Command

```bash
cd /tmp/ctx-bag-machine-b &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-b-home \
<repo-root>/bin/ctx-bag sync pull
echo EXIT:$?
```

### Output

```text
EXIT:1

CONFLICT DETECTED
resource: local store
local hash: a24dcfde04e3b28648c5799e36a03214f65858a84a26ce17606d9be30cce2c52
incoming hash: 26beb41badd6f19ca0966139af4b8992924e3c9b6b5844c5678f3034f4ee1ac8
safe next action: inspect /tmp/ctx-bag-sync/context-baggage-state before pulling
```

### Machine B Checkpoints After Rejected Pull

```text
{"type":"checkpoint","timestamp":"2026-08-26T11:14:07Z","deviceId":"d_61ea5bccf706d99139230cad484de08b","message":"Checkpoint created on simulated machine A"}
{"type":"checkpoint","timestamp":"2026-08-26T11:14:55Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Checkpoint created on simulated machine B"}
{"type":"checkpoint","timestamp":"2026-08-26T11:31:11Z","deviceId":"d_2aa98a3b3d3e76baa2d0f1906f9e059c","message":"Independent change created on simulated machine B"}
```

### Machine B Sync State After Rejected Pull

```text
folder: /tmp/ctx-bag-sync
lastPush: 2026-08-26T11:14:55Z
lastPull: 2026-08-26T11:14:44Z
lastPushHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
lastPullHash: 55f00a87d8902d91f5593ca163b5fafe56a752bfccfd9575d2fde3c56cf878ab
baseHash: 0ec514f1b8b44c7b41b7c7a416ff367c471a525a3467373473580bc2ce43ec4a
```

### Observation

Pull conflict protection applies symmetrically. The rejected pull did not erase Machine B's local independent checkpoint and did not advance Machine B's baseline.

## Machine A Health Check

### Status Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag status
```

### Output

```text
Context Baggage

Device
  ID: d_61ea5bccf706d99139230cad484de08b
  OS: linux

Workspace
  ctx-bag-machine-a
  w_a49d7498d9efa7f4

Active Task
  test-task

Sync
  configured
  folder: /tmp/ctx-bag-sync
  last push: 2026-08-26T11:31:26Z
```

### Task Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag task status
```

### Output

```text
Tasks
* test-task (active)
```

### Sync Command

```bash
cd /tmp/ctx-bag-machine-a &&
CONTEXT_BAGGAGE_HOME=/tmp/ctx-bag-machine-a-home \
<repo-root>/bin/ctx-bag sync status
```

### Output

```text
Sync
Folder: /tmp/ctx-bag-sync
Last push: 2026-08-26T11:31:26Z
Last pull: 2026-08-26T11:15:22Z
```

### Observation

Machine A remains healthy after its successful push and Machine B's rejected conflicting operations.

## Assertions

| Assertion | Result | Evidence |
|---|---|---|
| Both machines started from same base | PASS | both `baseHash=0ec514f1...3ec4a` |
| A local change diverged from base | PASS | A checkpoint file gained `Independent change created on simulated machine A`; A `baseHash` stayed `0ec514f1...3ec4a` |
| B local change diverged independently | PASS | B checkpoint file gained `Independent change created on simulated machine B`; B `baseHash` stayed `0ec514f1...3ec4a` |
| A first push succeeded | PASS | `Sync push complete`, hash `26beb41b...e1ac8` |
| B conflicting push was rejected | PASS | exact output contained `CONFLICT DETECTED`, exit `1` |
| Remote A state survived rejected B push | PASS | remote checkpoints still contain A independent change and not B independent change |
| B local work survived rejected push | PASS | B local checkpoints still contain B independent change |
| B baseline was not falsely advanced | PASS | B `baseHash` remained `0ec514f1...3ec4a` |
| No silent last-write-wins occurred | PASS | remote stayed A2 while B local stayed B2 |
| Conflicting pull is protected if applicable | PASS | B pull returned `CONFLICT DETECTED`, exit `1`, and B local checkpoint survived |

## Final State Diagram

```text
                 Original BASE
                 0ec514f1...3ec4a
                      |
              +-------+-------+
              |               |
              v               v
        Machine A          Machine B
        LOCAL=A2           LOCAL=B2
        BASE=A2            BASE=0ec514f1...3ec4a
        26beb41b...e1ac8   a24dcfde...e2c52
              |
              v
        REMOTE=A2
        26beb41b...e1ac8

B push -> CONFLICT DETECTED
B pull -> CONFLICT DETECTED
```

## Conclusion

```text
REAL CONFLICT VALIDATION: PASS
```
