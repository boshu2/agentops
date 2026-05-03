---
id: post-mortem-2026-05-03-baseline-ab-run-id-collision
type: post-mortem
date: 2026-05-03
mode: quick
epic: soc-ocq8
plan_path: .agents/plans/2026-05-03-baseline-ab-run-id-collision.md
pre_mortem: .agents/council/2026-05-03-pre-mortem-baseline-ab-run-id-collision.md
phase_2_summary: .agents/rpi/phase-2-summary-2026-05-03-baseline-ab-run-id-collision.md
vibe_report: .agents/rpi/phase-3-summary-2026-05-03-baseline-ab-run-id-collision.md
---

# Post-Mortem: Baseline A/B Default Run ID Collision

## Council Verdict: PASS

Plan-vs-delivered: clean. The planned implementation scope was exactly three files: `cli/internal/eval/baseline_ab.go`, `cli/internal/eval/baseline_ab_test.go`, and `docs/contracts/eval-baseline-ab.md`. Delivery changed those files, added the RPI phase summaries, and carried forward the cited learning metadata update from `ao metrics cite`.

The implementation preserves explicit `RunID` behavior as the suffix base and fixes only the omitted-`RunID` path by deriving one generated base before assigning `-skill-on` and `-skill-off` per leg.

## Closure Integrity Audit

| Issue | Status | Evidence mode | Evidence | Phantom? | Orphan? |
|---|---|---|---|---|---|
| soc-ocq8 | closed | worktree | focused code/test/doc diff plus passing validation commands before commit | no | no |

No child issues exist for this single-issue RPI packet. No phantom closure: the bead contains a concrete reproduction, scoped files, acceptance criteria, and a substantive close reason.

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---|---|---|
| Code | PASS | `RunBaselineAB` now assigns distinct suffixed leg IDs before calling each leg. |
| Documentation | PASS | `docs/contracts/eval-baseline-ab.md` documents suffixing for supplied and generated run ID bases. |
| Examples | PASS | The public `evals/agentops-core/lid-primitives-demo.json` smoke returns true for distinct suffixed IDs. |
| Proof | PASS | New regression test plus focused eval/CLI tests and `scripts/pre-push-gate.sh --fast` passed. |

## Prediction Accuracy

| Pre-mortem / plan point | Outcome | Score |
|---|---|---|
| Keep fix in `RunBaselineAB`; do not refactor eval engine | Hit | HIT |
| Add default-run-ID regression coverage | Hit | HIT |
| Clarify contract docs | Hit | HIT |
| Preserve `--out`, `--delta-out`, scorecard, and scoring semantics | Hit | HIT |

**Score: 4/4 HITS, 0 MISSES, 0 SURPRISES.**

## Learnings

No new durable learning was promoted. The applicable existing learning, `.agents/learnings/2026-04-29-quick-eval-evidence-kind-baseline-policy.md`, already covered the main reusable rule: keep eval artifact identity semantics explicit instead of fixing only presentation output.

## Follow-Up

No new bd issue recommended. Adjacent broader eval determinism work remains tracked separately as `soc-v7s8` and stayed out of scope.

## Decision

- [x] PASS - bug fix is ready to commit and push.
