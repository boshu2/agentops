---
id: post-mortem-2026-05-01-promoted-body-dedupe
type: post-mortem
date: 2026-05-01
target: fix/close-loop-promoted-dedupe-src
commit: 075d3178e70c51733d7c415e42ff58bc59f27ba4
mode: inline structured retrospective
---

> RPI streak: unavailable | Sessions: unavailable | Last verdict: PASS

# Post-Mortem: Promoted Body Dedupe and Runtime Dogfood

## Council Verdict: PASS

The AgentOps substrate defect is fixed on branch `fix/close-loop-promoted-dedupe-src`.
The fix blocks close-loop, startup maintenance, pool promotion, and batch promotion
from recreating known learning/pattern bodies when `promoted-index.jsonl` is
missing or stale and when prior cleanup moved duplicates into archive/defrag
locations. The deployed `ao` on PATH was updated and validated against Mt.
Olympus.

Inline review was used instead of spawning council agents because this Codex
runtime does not expose the same council hook surface and the implementation had
already been verified with focused regression tests plus dogfood validation.

## Scope Reviewed

Commit: `075d3178e70c51733d7c415e42ff58bc59f27ba4`

Changed files:

- `cli/internal/pool/promoted_content.go`
- `cli/internal/pool/pool.go`
- `cli/cmd/ao/pool_reindex.go`
- `cli/cmd/ao/pool_ingest.go`
- `cli/cmd/ao/flywheel_close_loop.go`
- `cli/cmd/ao/batch_promote.go`
- `cli/internal/lifecycle/close_loop.go`
- `cli/cmd/ao/flywheel_promoted_body_dedupe_test.go`
- `cli/internal/pool/promoted_content_test.go`

Scope delta: one necessary addition beyond the first hypothesis. Active artifact
live-scan was not enough; cleanup archives under `.agents/archive/dedup/` and
`.agents/defrag/*/files/.agents/{learnings,patterns}/` also had to become
known-body inputs.

## Checkpoint Preflight

| Check | Status | Detail |
|---|---|---|
| Chain loaded | PASS | `.agents/ao/chain.jsonl` present |
| Prior phases locked | WARN | Chain contains legacy/prior-epic entries, not a dedicated entry for this branch |
| No FAIL verdicts | PASS | No blocking FAIL verdict found in checked chain context |
| Artifacts exist | PASS | Branch commit and changed files exist on `origin/fix/close-loop-promoted-dedupe-src` |
| Idempotency | PASS | This is the first post-mortem harvest for commit `075d3178` |

## Closure Integrity

bd is available, but this repair was driven by the user-supplied RPI incident
packet rather than a single bead. Closure resolves on commit evidence:

- Remote branch exists: `origin/fix/close-loop-promoted-dedupe-src`
- Commit exists remotely: `075d3178e70c51733d7c415e42ff58bc59f27ba4`
- All nine changed paths exist in the branch tip
- Work was pushed after tests and runtime validation

No phantom bead or orphan-child audit applies to this evidence-only RPI closure.

## Metadata Verification

Mechanical path check: all changed files named by the commit exist at branch tip.
No documentation files or CLI command/flag definitions changed, so no CLI
reference regeneration was required. `ao pool reindex --help` was verified
against the installed PATH binary.

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---|---|---|
| Code | PASS | Shared canonical promoted-body extraction and hash loading now cover active artifacts, sidecar index, batch promotion, pool promotion, close-loop ingest, and cleanup archives. |
| Documentation | N/A | Internal substrate behavior changed; no user-facing command, flag, or skill contract changed. |
| Examples | PASS | Runtime `ao pool reindex --help` works from PATH and documents the command examples already present in source. |
| Proof | PASS | Regression tests, full Go tests, runtime binary install, PATH capability check, and Mt. Olympus dogfood sequence passed. |

## Validation Evidence

Regression-first proof:

- Initial regression failed before the fix: close-loop dry-run reported
  `Added=1`, and startup maintenance wrote a duplicate `2026-05-01-pend-*`
  artifact.
- After the fix, targeted close-loop/startup tests passed for active promoted
  bodies, archived promoted bodies, idempotency, and audit skip reasons.

AgentOps validation:

- `go test ./cmd/ao -run 'Test.*(CloseLoop|PoolReindex|CodexStart|Dedup|Promotion)'`: PASS
- `go test ./cmd/ao ./internal/pool`: PASS
- `go test ./...`: PASS
- `go build -o bin/ao ./cmd/ao`: PASS
- `./bin/ao pool reindex --help`: PASS
- Installed runtime: `/home/boful/.local/bin/ao`
- `ao pool reindex --help`: PASS from PATH

Mt. Olympus dogfood validation:

- `bash scripts/dev/olympus-corpus-clean.sh --quiet`: cleaned the known drift
- `ao pool reindex --json`: `new_entries=0`, index already covered active corpus
- `ao flywheel close-loop --dry-run --json`: `ingest.added=0`, `auto_promote.promoted=0`
- `bash scripts/dev/olympus-corpus-clean.sh --check --quiet`: `clean (actionable_groups=0)`
- `ao codex ensure-start --no-maintenance`: no corpus drift
- `ao codex ensure-start`: no corpus drift
- Forced fresh startup maintenance with a new `CODEX_THREAD_ID`: no corpus drift
- Final corpus check: `clean (actionable_groups=0)`

## Substrate vs Product Health

AgentOps substrate health: PASS locally. The installed runtime binary now has
`pool reindex`, uses archive-aware known-body dedupe, and kept close-loop/startup
maintenance from recreating known learning/pattern bodies during dogfood
validation. Remaining risk is release/distribution, not local source behavior.

Mt. Olympus product/runtime health: PASS for this incident. No Mt. Olympus
product/runtime files were changed for the fix. Corpus cleanliness held after
cleanup, reindex, close-loop dry-run, and startup maintenance.

Dogfood proof health: PASS with one caveat. The evidence is trustworthy for this
machine because it used the PATH `ao` binary, not `go run`; it still needs a
normal AgentOps release so other installs receive the same substrate behavior.

## Learnings

1. Cleanup archives are part of the promoted-body truth set. If cleanup moves a
   duplicate into `.agents/archive/dedup/` or `.agents/defrag/*/files/`, future
   lifecycle code must still treat that body as already known.
2. Promotion dedupe must not depend on `promoted-index.jsonl` being present or
   current. The sidecar is an acceleration/cache layer, not the authority.
3. Runtime verification is part of fixing AgentOps substrate bugs. A source-only
   fix is insufficient when users and dogfood repos execute a stale PATH binary.

## Follow-Up Items

- Merge and release the archive-aware promoted-body dedupe branch so normal
  installs inherit the local PATH fix.
- Keep the existing broader release follow-up for close-loop lifecycle fixes
  open; this branch should be included in that release scope.

## Flywheel: Next Cycle

Highest-priority follow-up:

> **Merge and release archive-aware promoted-body dedupe fix** (task, high)
> Ship commit `075d3178` through the normal release path so other AgentOps
> installs cannot recreate archived learning/pattern bodies.

Ready to run:

```bash
$rpi "Merge and release archive-aware promoted-body dedupe fix"
```
