package app

const docWorkspace = `
Commands:
  workspace init
  workspace status
  workspace available
  workspace attach

Typical cross-machine continuation:
  workspace available
  workspace attach <workspace-id>
  sync pull
`

const docWorkspaceInit = `
Behavior:
  Initializes the current directory as a workspace and records its local path.
  By default a new workspace is not exported. Use --sync to opt the workspace
  into portable shared state, or --no-sync to keep it local-only.
`

const docWorkspaceStatus = `
Behavior:
  Shows the current workspace name, canonical workspace ID, local path, identity
  type, and sync participation.
`

const docWorkspaceAvailable = `
Behavior:
  Lists portable workspaces that can be explicitly attached, read from the
  authoritative v2 shared state.

Important:
  Reads only. Attachable workspaces are non-Git and Git-without-remote kinds;
  workspaces with a usable Git remote already have deterministic identity.
`

const docWorkspaceAttach = `
Behavior:
  Attaches the current local directory to an existing portable workspace by its
  exact Workspace ID.

Important:
  Attachment is explicit; folder basename or path never infers cross-machine
  identity. Attach does not pull automatically — run sync pull afterward. A
  workspace that already contains unsafe local context is protected. Git
  repositories with a usable remote normally do not need attachment.

Example:
  ctx-bag workspace attach w_abc123
  ctx-bag sync pull

See also:
  workspace available
  sync pull
`
