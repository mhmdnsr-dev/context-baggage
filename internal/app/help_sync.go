package app

const docSync = `
Commands:
  sync init
  sync status
  sync push
  sync pull
  sync recover
  sync upgrade

Behavior:
  Portable state is shared through a filesystem folder or a dedicated,
  demonstrably non-public GitHub.com repository.
`

const docSyncInit = `
Behavior:
  Configures a filesystem folder or a managed GitHub.com repository.

Usage forms:
  ctx-bag sync init <folder> [--replace]
  ctx-bag sync init github <repository-url> [--replace]

  A single literal "github" remains a filesystem folder for compatibility.
  The managed form requires an SSH or HTTPS GitHub repository locator.

Managed requirements:
  The repository must already exist, be dedicated to Context Baggage, and be
  demonstrably non-public. Normal Git SSH/HTTPS authentication is used;
  Context Baggage does not read or store tokens.

Example:
  ctx-bag sync init <shared-folder>
`

const docSyncStatus = `
Behavior:
  Shows local configuration, BASE, pending recovery, and last-observed REMOTE
  knowledge. It is offline: "last observed" and "last refreshed" never imply a
  live remote check. Filesystem format status is also shown.
`

const docSyncPush = `
Behavior:
  Writes eligible portable state to the active filesystem or managed GitHub
  destination. Workspaces not opted into sync are excluded.

Important:
  No automatic reconciliation or merge is performed.
`

const docSyncPull = `
Behavior:
  Imports authoritative portable state from the active destination and
  preserves machine-local path metadata. Conflict safety refuses an unsafe
  overwrite. Managed Pull records durable recovery metadata before mutation.

Example:
  ctx-bag workspace attach <workspace-id>
  ctx-bag sync pull

See also:
  workspace attach
`

const docSyncRecover = `
Behavior:
  Conservatively completes an interrupted managed Pull. With LOCAL equal to the
  recorded pre-state it retries the exact recorded target. With LOCAL equal to
  the target it finalizes BASE. Any other LOCAL state is refused.

Important:
  Recovery never follows current remote HEAD and has no force, discard, merge,
  or automatic-retry option. With no pending record it is a successful no-op.
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
