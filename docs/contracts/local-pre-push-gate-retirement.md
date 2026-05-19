# Local Pre-Push Gate Retirement — CI Is Sole Source-of-Truth

`scripts/pre-push-gate.sh` is the local mirror of `.github/workflows/validate.yml`.
The mirror has drifted three times in observable ways during the 2026-05-19
session alone, each costing a self-correction PR. This document is the
architecture decision to retire the local mirror and treat CI as the sole
source-of-truth for push-gate enforcement.

## Decision

The local pre-push gate is retired. CI (the GitHub Actions workflows under
`.github/workflows/`) is the sole authoritative gate for whether a change is
fit to land on `main`.

Authoritative gate:

- `.github/workflows/validate.yml`
- `.github/workflows/release.yml`
- the `claude-code-review` required check

Non-authoritative (retiring):

- `scripts/pre-push-gate.sh`
- the ~12 helper scripts under `scripts/check-*.sh` invoked only by the gate
- the ~40 bats files under `tests/scripts/` that test gate helpers in
  isolation (the bats that test the gate itself stay until the gate is
  deleted)

`.githooks/pre-push` keeps its `bd hooks run pre-push` invocation; that piece
is `bd` issue-tracker plumbing, not a gate.

## Why this rather than the alternatives

The bead (`soc-g2r9`) enumerated three options. They were considered together:

| Option | Effort | Drift | Verdict |
|---|---|---|---|
| `accept-drift` | 30 min (1 ADR) | unchanged | rejected — legitimizes the cost we filed the bead to remove |
| `drop-local-gate` (this decision) | ~2-3h across PRs | impossible (no local) | **chosen** |
| `mirror-CI-via-act` | 1-2 days, +30s/push, +2GB Docker | impossible (Docker GH-Actions runtime) | rejected — heaviest, and the 30-90s CI feedback loop is acceptable |

`drop-local-gate` is the option that removes the *category* of drift instead
of paying ongoing alignment cost. The cost we accept is a 30-90s CI feedback
loop per push instead of a 10-20s local one. The session of 2026-05-19 spent
several multi-minute self-correction PR cycles per local-gate-was-wrong event,
so the per-push cost is dominated by the per-incident cost we just removed.

## What replaces local enforcement

Nothing immediate. The retirement is the point. Specific past concerns and
how they map onto the new shape:

- **Speed of feedback.** CI takes 30-90s on the fast path. Operators who
  want a local sanity check can run `cd cli && make test` or `bats
  tests/scripts/<changed>.bats` directly. The omnibus 38-check gate is the
  thing being retired, not the per-tool feedback loops.
- **Pre-push hygiene.** The bd hooks invocation in `.githooks/pre-push` is
  preserved (issue tracker plumbing, not gate enforcement).
- **AP#7 mechanical enforcement (soc-o5kq, PR #356).** Today's mechanical
  check that verifies `Evidence:` PR-body claims against the gate log moves
  to CI as a new validate.yml job. The standalone `scripts/verify-gate-claim.sh`
  tool stays; only its pre-push wiring goes. Tracked as a follow-up bead so
  AP#7 protection has at most a ~1-day gap.
- **Coverage ratchets and similar local-only signals.** Currently surfaced
  via `pre-push-gate.sh`; will migrate to CI jobs that emit the same
  signals on PRs. The signal stays; the surface changes.

## What this PR does

This PR locks in the decision without the deletion churn:

- Adds this ADR.
- Edits `.githooks/pre-push` to skip the `scripts/pre-push-gate.sh`
  invocation. The pre-push hook still runs `bd hooks run pre-push` so issue
  tracking continues to work.
- Files four follow-up beads for the actual removal waves.

After this PR lands, `git push` will no longer run the local gate. CI is the
sole gate. The orphaned files stay on disk until the follow-up waves delete
them, which keeps each follow-up PR a single coherent arc with a single
rollback semantic.

## Follow-up work

Tracked as discovered-from soc-g2r9:

1. **Wave 1 (delete pre-push-gate.sh + helpers).** Delete `scripts/pre-push-gate.sh`,
   `scripts/check-pre-push-gate-wired.sh`, and the ~11 gate-only helpers
   under `scripts/check-*.sh`. Audit callers first; any helper with non-gate
   callers stays.
2. **Wave 2 (retire gate-only bats).** Delete `tests/scripts/pre-push-gate*.bats`
   and gate-helper bats. Keep `tests/scripts/verify-gate-claim.bats` (the
   standalone tool stays).
3. **Wave 3 (migrate AP#7 to CI).** Add a `pr-evidence-claim-verify` job to
   `.github/workflows/validate.yml` that calls `scripts/verify-gate-claim.sh`
   against the PR body's `Evidence:` line(s). Closes the ~1-day mechanical
   AP#7 gap this PR opens.
4. **Wave 4 (doctrine sweep).** Update `skills/ship-loop/SKILL.md`, the Codex
   twin, and `skills/ship-loop/references/anti-patterns.md` to reflect the
   new "CI is sole truth" stance and remove pre-push-gate references.

## Reversibility

The decision is fully reversible by reverting this PR. The orphaned files
(left in place by design) keep working; flipping `.githooks/pre-push` back
to invoking `scripts/pre-push-gate.sh` restores the previous behavior with
zero data migration. The deletion waves are individually reversible too,
but each progressively raises the revert cost — that's by design and matches
the operator's "decide before you delete" preference.

## Acceptance

- ADR exists at `docs/contracts/local-pre-push-gate-retirement.md` (this file).
- `.githooks/pre-push` no longer invokes `scripts/pre-push-gate.sh`.
- Four follow-up beads filed under `discovered-from: soc-g2r9`.
- Linked from `docs/documentation-index.md`.
