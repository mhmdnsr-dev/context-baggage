# Session 001: Problem and Product Direction

Date: 2026-08-24

## Problems Identified

The developer repeatedly experiences:

- separate configuration for every coding agent;
- inability to remember which agent has which settings;
- differences across operating systems;
- temporary memory/context that should move to another device;
- persistent context for large features/tasks;
- work-related configuration/context that cannot be committed into project Git repositories;
- difficulty carrying agent context between devices.

The broader developer community experiences similar agent-memory synchronization and configuration fragmentation problems.

## Product Direction Discovered

The solution should not simply be:

```text
another AI memory database
```

Instead it should become:

```text
a portable control/continuity layer for coding-agent context
```

It should eventually cover:

```text
agent configuration
workspace context
task state
handoffs
cross-machine continuity
```

Advanced capabilities are intentionally deferred.

## Key Insight

Separate:

```text
Agent configuration
OS/device configuration
Workspace-private context
Task/feature memory
Temporary session state
```

rather than treating all of them as one generic "memory".
