---
title: ".agents/ workspace hygiene"
description: "Retention conventions for the .agents/ runtime workspace and how ao doctor keeps it clean."
permalink: /agents-dir-hygiene
last_reviewed: 2026-07-18
---

# `.agents/` workspace hygiene

`.agents/` is the per-repository runtime workspace: gitignored local state
written by the `ao` CLI and by skills while they work. It is not committed
(except a small pinned config under `.agents/ao/`), it is not a deliverable,
and everything in it is safe to regenerate. Two shapes of content live there:

- **Knowledge-shaped directories** — postmortems, pre-mortem checks, handoffs,
  retros, proofs: human-readable records a later session may re-read.
- **Machine-artifact directories** — queue dirs, run scratch, staged outputs:
  consumed once by tooling and then debris.

Left alone, the second class accumulates. The `workspace` subsystem of
`ao doctor` exists to detect and garbage-collect that debris safely.

## The ephemeral-dir contract

Any writer that mints a queue- or run-shaped directory under `.agents/` MUST
follow this contract:

- **Mark abandonment by renaming, never by deleting.** A directory that is
  done or abandoned is renamed to `<name>.stale-<timestamp>` with a UTC
  timestamp in `YYYYMMDDTHHMMSSZ` form, e.g.
  `land-queue-age-h433.19.stale-20260711T221032Z`.
- **Retry attempts get `-retry<N>` suffixes**, e.g.
  `land-queue-age-h433.22-native-retry2`. A retry directory may itself later
  be staled; both suffixes can appear on one name.
- **GC is `ao doctor`'s job.** Writers never delete workspace directories —
  they only rename. Doctor collects stale and retry-chain directories whose
  newest content is older than the TTL.

The GC TTL defaults to **14 days**, measured against the newest regular-file
modification time inside the directory. Override it with
`AO_DOCTOR_WS_TTL_DAYS` (a positive integer day count; invalid values fall
back to the default).

## Canonical directory names

Skills and sessions have historically spelled the same top-level directories
several ways. Doctor normalizes toward one canonical name per concept:

| Canonical | Known drift aliases |
|---|---|
| `postmortem` | `post-mortem`, `post-mortems` |
| `pre-mortem-checks` | `pre-mortem`, `pre-mortems` |
| `handoff` | `handoffs`, `mto-handoff` |
| `retro` | `retros` |
| `proofs` | `proof` |
| `tests` | `test` |

This table is a projection of `workspaceCanonicalAliases` in
`cli/internal/doctor/fix_workspace.go` — the Go table is the source of truth.
If the two disagree, the Go table wins and this page is stale.

## How to clean up

```bash
ao doctor                       # detect-only: report findings, change nothing
ao doctor --fix                 # apply fixers (routes through mutate())
ao doctor --only fm-ws-empty-dirs   # scope to one detector (repeatable)
ao doctor --only workspace      # or scope to the whole subsystem
```

The workspace detectors are:

| Detector ID | Finds | Fix behavior |
|---|---|---|
| `fm-ws-stale-queue-dirs` | `.stale-*` / `-retry<N>` directories past TTL | quarantine (rename) |
| `fm-ws-empty-dirs` | empty top-level dirs (abandoned scaffolding) | quarantine (rename) |
| `fm-ws-naming-drift` | drift-alias directory names (table above) | merge/rename to canonical |
| `fm-ws-dual-store` | learnings split across `.agents/learnings` and `.agents/ao/learnings` | report-only; defers to `ao doctor --fix --only fm-knowledge-orphaned-flywheel-learnings` (moves top-level `*.md`/`*.jsonl` only) |
| `fm-ws-nested-tree` | a `.agents` runtime tree nested under a repo subdirectory | report-only; manual review (may be an intentional nested project) |
| `fm-ws-oversize` | unexpectedly large directories | report-only |

Every mutating run writes receipts under
`.doctor/runs/<timestamp>__<runid>/`: a `report.json` describing findings and
applied fixes, and an `undo.sh` that reverses every mutation the run made.
`.doctor/latest` is a symlink to the most recent run.

## Safety envelope

- **Doctor has no delete operation by design.** What reads as "deletion" is an
  atomic rename into the run's `quarantine/` directory
  (`.doctor/runs/<run>/quarantine/workspace/<dirname>`). Final disposal of
  quarantined content is always the human's call — for old runs,
  `ao doctor gc` requires explicit `--yes` and `--before <date>`.
- **Ambiguous merges are refused, not guessed.** When a drift-alias merge (or
  the knowledge subsystem's learnings consolidation) would collide with
  existing content and the right merge is not mechanically certain, the
  finding lands in a Skipped list with the reason, and nothing moves.
- Detectors are pure reads; every disk write flows through the audited
  mutation path and is backed up before it happens, so `undo.sh` can restore
  the prior state.

## What doctor will NOT decide

Disposal of large archives and eval payloads is a judgment call, not a
mechanical one. `fm-ws-oversize` reports them — path, size, file count — and
stops there. Doctor never quarantines, moves, or deletes content solely for
being big; deciding what a large artifact is worth is the operator's job.
