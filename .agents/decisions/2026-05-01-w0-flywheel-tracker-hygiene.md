---
id: decision-2026-05-01-w0-flywheel-tracker-hygiene
type: decision
date: 2026-05-01
issue: soc-o6eb.1
status: accepted
---

# W0 Flywheel and Tracker Hygiene

## Decision

Close W0 as a routing/proof stabilization pass, not as the P0 implementation
itself.

`soc-2ctn` remains the single active P0 because replay archives are absent in
this checkout, so the stronger "replay proof" path cannot be completed here.
The portfolio can still advance to W1 because W0 established a canonical route,
linked adjacent issues, refreshed the installed Codex hook cache, and recorded
the remaining dry-run hazard as its own bug.

## Execution Frame

- **Assumptions:** live `bd` reads are authoritative; `.agents/archive/` replay
  sources are optional proof inputs and must not be invented when absent.
- **Smallest change:** update tracker relationships, close only the obvious
  duplicate, and write one durable proof artifact.
- **Blast radius:** live bd state in the canonical Dolt database and this
  decision artifact. No source or hook code changes.
- **Verification:** bd graph readback, bd storage status, dry-run mutation
  check, installed runtime checks, and final worktree-disposition closeout.

## Evidence

| Surface | Evidence | Result |
|---------|----------|--------|
| P0 route | `bd list --status open --limit 0 --json` filtered to P0 | Only `soc-2ctn` is open P0 |
| Replay archives | `find .agents/archive -maxdepth 3 -type f` | 0 files; replay proof unavailable |
| Pending growth | `find .agents/learnings -name 'pend-*'`, `find .agents/knowledge/pending -type f` | 0 tracked `pend-*`; 0 pending inputs |
| Installed `ao` | `/home/boful/.local/bin/ao`, `ao version` | `ao version dev`; mtime `2026-05-01 12:24:43 -0400` |
| Close-loop fix freshness | `git show -s d73fc3bf 697e78fd 6a6ac370` | Installed binary mtime is after these close-loop fixes |
| Hook source/embedded parity | `sha256sum hooks/*.sh cli/embedded/hooks/*.sh` | Matching hashes for hook source and embedded copies |
| Codex hook cache | `bash scripts/refresh-codex-local.sh`; `ao doctor --json` | Codex Sync PASS: installed native plugin matches `5c6dacb3` |
| bd storage | `bd vc status` | Canonical database: `/home/boful/dev/personal/agentops/.beads/dolt`; no `.beads/issues.jsonl` path exists in this linked checkout |
| Dry-run stability | `ao flywheel close-loop --dry-run --json` followed by `git status --short` | Found mutation of `.agents/findings/f-2026-04-14-002.md`; restored and filed `soc-73tk` |
| Validator worktree | `find /home/boful/dev -maxdepth 4 -type d -name agentops-validator` | No validator worktree path found |

## Tracker Changes

- `soc-2ctn`: remains open P0 and owns terminal-state/replay stability.
- `soc-xn5s`: kept open as separate P2 mixed-case dedup edge-case; related to
  `soc-2ctn`.
- `soc-w7s2`: kept open as binary/path freshness follow-up; related to
  `soc-2ctn`.
- `soc-7wwp`: selected as canonical RPI dry-run execution-packet alias bug.
- `soc-qvpb`: closed as duplicate of `soc-7wwp`.
- `soc-73tk`: created for flywheel close-loop dry-run mutating citation
  metadata.
- `agentops-ikm`: updated with validator-worktree disposition readback.

## Residual Risk

W1 must continue using one writer for `.beads/issues.jsonl`. `soc-2ctn` is
still the active incident until replay proof can be run with the archived
pending source, or until it is explicitly resolved by a later implementation
with durable evidence.
