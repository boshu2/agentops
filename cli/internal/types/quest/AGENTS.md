---
date: 2026-04-29
package: cli/internal/types/quest
status: active (atomic-write utilities only)
---

# cli/internal/types/quest/

## Ownership

Tiny utility package: atomic file-write helpers used wherever cli/internal/* needs durable file writes.

## Public surface

- `AtomicWriteFile(path string, data []byte) error` — atomic write with default temp-file perms
- `AtomicWriteYAML(path string, v any) error` — wraps AtomicWriteFile after YAML marshal
- `AtomicWriteFileWithPerm(path string, data []byte, perm os.FileMode) error` — perm-aware variant (added 2026-04-29 by bead agentops-3ga.1)

All three: temp file + write + fsync + atomic rename. Survives mid-write SIGKILL.

## Non-obvious

- The package name "quest" is historical. Originally hosted Olympus domain types (Quest, Bead, Learning, etc.); those were deleted on 2026-04-29 (bead agentops-3ga.2) after investigation confirmed zero real consumers. Atomic helpers stay because they're the canonical dedup target for bead agentops-3ga.4.
- Do not put domain types here. This is utility-only.

## Consumers

After the agentops-3ga.4 dedup, the following packages will import these helpers:
- `cli/internal/overnight/` (4 sites: checkpoint, reduce, measure, commit)
- `cli/internal/openclaw/snapshot.go` (1 site, 0o600 security-sensitive)

## See also
- `~/.agents/olympus-history-index.md` for the full extraction history
- `agentops-3ga.4` for the dedup work
