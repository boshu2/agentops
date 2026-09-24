---
date: 2026-04-29
package: cli/internal/types/quest
status: active (atomic-write utilities only)
---

# cli/internal/types/quest/

## Ownership

Tiny utility package: atomic file-write helpers used wherever cli/internal/* needs durable file writes.

## Read for the change

For write semantics or permissions, read [atomic.go](atomic.go), then the
canonical implementation in [storage/atomicfile.go](../../storage/atomicfile.go).
These wrappers delegate the temp-file, write, fsync, and atomic-rename algorithm
to storage; keep a single implementation. `AtomicWriteYAML` marshals before
delegating. `AtomicWriteFile` uses `0o600`; callers needing another mode use
`AtomicWriteFileWithPerm`.
Preserve atomic replacement: concurrent readers must see complete old or new
bytes, never a partial write.

## Non-obvious

- The package name "quest" is historical; Olympus domain types were removed
  on 2026-04-29 (bead `agentops-3ga.2`). Keep the surviving utility API compatible.
- Do not put domain types here. This is utility-only.

## Consumers

Search imports of `internal/types/quest` before changing a signature.
[OpenClaw snapshots](../../openclaw/snapshot.go) use the permission-aware wrapper
for both versioned and latest snapshots; preserve their `0o600` permissions.

## Completion

Run `go test ./internal/types/quest ./internal/storage` from `cli/` and the
root contract's required checks. Inspect [atomic_fault_test.go](atomic_fault_test.go)
for the exact evidence: simulated faults and concurrent-reader checks do not
establish observed recovery from a real SIGKILL or power loss.
