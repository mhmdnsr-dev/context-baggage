package app

const docSync = `
Commands:
  sync init
  sync status
  sync push
  sync pull
  sync upgrade

Behavior:
  Portable state is shared through an explicitly configured filesystem folder.
  Context Baggage does not perform network transport itself; the folder may be
  synchronized by an external tool.
`

const docSyncInit = `
Behavior:
  Configures a local filesystem folder as the sync target for this machine.

Example:
  ctx-bag sync init <shared-folder>
`

const docSyncStatus = `
Behavior:
  Shows the configured sync folder, last push/pull times, and the shared-state
  format. When a legacy and a v2 namespace both exist, v2 is authoritative and
  legacy is ignored.
`

const docSyncPush = `
Behavior:
  Writes eligible portable state to the shared folder. Workspaces that are not
  opted into sync are excluded. Conflict safety can refuse a push when it cannot
  safely determine a direction.

Important:
  No automatic reconciliation or merge is performed.
`

const docSyncPull = `
Behavior:
  Imports authoritative portable state from the shared folder and preserves
  machine-local path metadata. Conflict safety refuses an unsafe overwrite.

Example:
  ctx-bag workspace attach <workspace-id>
  ctx-bag sync pull

See also:
  workspace attach
`

const docSyncUpgrade = `
Behavior:
  Converts a legacy v0.1 shared representation into the sanitized v2 shared
  state. The legacy namespace is preserved and v2 becomes authoritative once it
  exists.

Important:
  Mixed v0.1 and v0.2 devices do not share current portable state; upgrade all
  devices that use the folder to v0.2-compatible behavior.
`
