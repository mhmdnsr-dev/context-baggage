# Sync Contract

The central `v0.1` sync model is:

```text
Context Baggage local state
          │
       push/pull
          │
          ▼
Configured filesystem folder
          │
          ▼
External sync mechanism
```

External mechanisms can include Syncthing, Dropbox folders, OneDrive folders, Nextcloud folders, NAS mounts, USB drives, network shares, and other tools. Context Baggage does not integrate with those providers in `v0.1`.

## `sync init`

`ctx-bag sync init <folder>` records a filesystem folder as the explicit push/pull target. The folder must be available locally.

## `sync status`

`ctx-bag sync status` reports whether sync is configured, whether the folder is reachable, and last known push/pull metadata.

## `sync push`

`ctx-bag sync push` exports eligible workspace state into the configured filesystem folder. It must respect workspace sync policy and avoid partial writes where practical.

Device identity and local sync configuration remain machine-local.

## `sync pull`

`ctx-bag sync pull` imports state from the configured filesystem folder. It must stop safely when ambiguous conflicts are detected.

Pull imports exported workspace state without replacing the receiving machine's device identity.

## Workspace Exclusion

Workspace metadata supports:

```yaml
sync: false
```

Workspaces with `sync: false` must not be exported during `sync push`.

## Conflict Safety

Context Baggage must not silently use last-write-wins. When local and incoming state changed incompatibly, output:

```text
CONFLICT DETECTED
```

The command should include the resource, local metadata, incoming metadata, and a safe next action.

Conflict detection compares:

```text
BASE
    last portable state known to be shared

LOCAL
    current local portable state eligible for sync

REMOTE
    current portable state in the configured sync folder
```

No conflict exists when only local changed, only remote changed, or local and remote are already equal. A conflict exists when both local and remote changed from `BASE` and differ from each other.

## Atomicity

Use temporary files and atomic replacement where practical. Avoid leaving partially written canonical files after interruption or errors.

## Security Boundary

No automatic Internet communication occurs through Context Baggage. Any cloud or network transport is provided by the external filesystem synchronization mechanism chosen by the user.
