package app

const docTask = `
Commands:
  task start
  task status
  task resume

Behavior:
  Tasks give a workspace long-running continuity that survives restarts and
  cross-machine sync. Checkpoints and a Markdown handoff attach to a task.
`

const docTaskStart = `
Behavior:
  Starts a task in the current workspace and makes it the active task.

Example:
  ctx-bag task start implement-parser
`

const docTaskStatus = `
Behavior:
  Lists tasks in the current workspace and marks the active one.
`

const docTaskResume = `
Behavior:
  Makes an existing task the active task so checkpoints and handoff apply to it.

Example:
  ctx-bag task resume implement-parser
`
