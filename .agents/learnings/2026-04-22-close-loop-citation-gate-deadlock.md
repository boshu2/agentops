---
id: learning-2026-04-22-close-loop-citation-gate-deadlock
type: learning
date: 2026-04-22
status: active
maturity: provisional
utility: 0.8
confidence: 0.9
pattern: flywheel-throughput-deadlock
detection_question: "Does an automated pipeline gate admission on a signal that only accumulates after admission?"
applicable_when: "designing auto-promotion, auto-publish, or auto-index paths that also have a manual sibling path"
source:
  session: 2026-04-22 nightly-dream-cycle-triage
  evidence:
    - cli/cmd/ao/flywheel_close_loop.go
    - cli/cmd/ao/batch_promote.go
    - scripts/nightly-dream-cycle.sh
tags: [flywheel, performance, throughput, promotion-gate, chicken-and-egg]
harmful_count: 0
reward_count: 0
helpful_count: 0
---

# Learning: Chicken-and-Egg Gates Silently Destroy Flywheel Performance

## What Happened

The nightly dream cycle reported `close_loop.added=43, auto_promoted=0, indexed=0,
citation_rewards=0` every night. `sigma=0`, `rho=0`, `escape_velocity=false` followed
deterministically. The pipeline was ingesting 43 candidates, scoring them at silver
tier with `gate_required=false`, then throwing all 43 on the floor.

Root cause: `checkPromotionCriteria` demanded `totalCitations >= 2` for every
candidate. The function was shared between the manual `pool batch-promote` path
(where citation signal is legitimately the primary gate) and the automated
`flywheel close-loop` path (where scoring tier is the primary gate). Fresh
candidates from nightly forge had zero citations — they cannot be cited until
they are promoted and indexed, so the gate blocked the only path to satisfying
itself. A perfect deadlock, running nightly, producing no output.

## Why This Is a Performance Bug

The pipeline burned real CPU: harvest scanned the whole corpus, forge converted
30 markdown sources into session artifacts, ingest wrote 43 pool entries to disk,
and scoring computed a full rubric for every one. Throughput was zero because
the final gate was unreachable, so every unit of upstream work was waste.
"Throughput=0" metrics look like a dead system; this was a deadlocked one,
which is worse because the inputs kept accumulating.

A slow pipeline surfaces in latency dashboards. A deadlocked pipeline surfaces
only in *downstream* metrics (`sigma=0`), one level removed from the actual
break. If you have a compounding knowledge system, measure end-to-end throughput
(items-in vs items-promoted) per run, not just stage success.

## Detection

- **Red flag in JSON output:** `added >> 0 && auto_promoted == 0` over multiple
  runs is a stall, not a steady state. Treat equality as "this run produced no
  useful work."
- **Invariant to assert in tests:** an L2 test of the automated path that feeds
  N fresh, scoring-qualified candidates and asserts `promoted > 0` — not just
  "no error."
- **Shared-gate smell:** a single `check*` function called from both a manual
  command and an automated pipeline. The manual path's idea of "signal" is
  almost never the automated path's idea of "signal." Split the gates.

## Corrective Action

1. Split admission gates by path. Manual promotion can legitimately require
   accumulated signal (citations, user approval, elapsed time). Automated
   promotion must rely only on signals available at admission time (scoring
   tier, rubric rejection, dedup, utility threshold).
2. Parameterize shared helpers rather than branching internally on caller
   identity: `checkPromotionCriteria(..., requireCitations bool)` is clearer
   than a hidden "am I being called from batch-promote?" check.
3. Add an end-to-end assertion: the nightly dream cycle should fail loudly if
   `ingested > 0 && promoted == 0` persists across runs, instead of quietly
   posting another zero-throughput status issue.

## Generalizable Pattern

**"Gates that admit nothing make pipelines feel fast and achieve nothing."**
If a gate's satisfaction depends on the output of the gate itself, it is a
deadlock, not a quality bar. The feeling of rigor is not the same as the
presence of throughput. Always ask: can a fresh, valid input ever pass this
gate on its first encounter? If the answer is no, the gate is broken.

## Cross-References

- `cli/cmd/ao/batch_promote.go` — `checkPromotionCriteria` with
  `requireCitations` parameter.
- `cli/cmd/ao/flywheel_close_loop.go` — close-loop caller passes `false`.
- Issue #117 (nightly dream cycle status) — the downstream symptom that
  surfaced this.
