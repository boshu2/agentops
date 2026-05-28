# Previous-run audit (anchor: origin/nightly/2026-05-05)

Anchor SHA: 46ef488 (Nightly 2026-05-05 final commit)
Anchor baseline: code-driven score n/a (older JSON shape; overall 77.58%, 16/22 passing)
Current baseline: code-driven score 100.0% (19/19 passing, 1 skipped); runtime-artifact 0/2 (expected)

## Anchor failures resolved
- flywheel-compounding w=3 → SKIP (corpus dormant; f-2026-04-30-002 SKIP precondition implemented)
- dream-end-user-coverage w=3 → PASS
- go-complexity-ceiling w=6 → PASS
- flywheel-lifecycle w=6 → PASS

## Anchor failures persisting (expected, runtime-artifact)
- compile-freshness w=4 (tagged runtime-artifact; flips every run, excluded from headline)
- compile-no-oscillation w=4 (same)

## Regressions (was passing, now failing)
None.

## PRs merged since anchor
17 PR-tagged merges (skill-builder/auditor pair, deps bumps via Renovate, parity/spec docs for managed-agents launch, worktree fix, gitignore force-track regression in ce4c015 — fixed this run).

## Notes
- Two-day gap because nightly/2026-05-06 ran on a date where neither baseline-goals.json nor final-goals.json propagated to main (gitignore regression from ce4c015 hid them). PR #235 body claims "94.29 → 100.00" but no anchor JSON is force-tracked in main from that run.
- This run's gitignore re-fix means tomorrow's nightly will have a real anchor.
