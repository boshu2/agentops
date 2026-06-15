# W1c corpus-delta — PRE-REGISTRATION (decision rule + scorecard)

> **Pre-registered BEFORE compute (ag-4y7vc, 2026-06-15).** This fixes the
> verdict categories, the statistic, and the publication rule in advance so the
> result cannot be reframed after the fact. The 2026-06-14 durable plan flagged
> that without a pre-registered threshold the run is a vanity metric; this is the
> threshold. Locked: do not edit the decision rule after seeing results — only
> append a dated erratum.

## What the run measures
For each held task (`evals/workbench/tasks/cd-*`), run K≥3 seeds in two arms:
- **context_off** — isolated empty corpus (sandbox HOME, always-loaded roots stripped).
- **context_on** — organic `.agents` corpus present AND retrieved+injected (per
  ag-98g1a once landed; until then the on-arm retrieval path is incomplete and a
  run is PILOT-only, see "Run tiers").
Arm score = pass-rate over seeds (deterministic graders). Per-task delta =
`mean(on) − mean(off)`. Aggregate delta = mean over included tasks.

## Verdict categories (assigned PER TASK first, then aggregate)
| Category | Rule |
|---|---|
| **positive** | per-task delta CI lower bound > 0 (corpus helped, beyond noise) |
| **null** | delta CI straddles 0 with off-arm having real headroom (off < 1.0) — honest "corpus didn't help here" |
| **inconclusive** | CI too wide to call at K (underpowered) — needs more seeds |
| **ceiling-excluded** | off == 1.0 across all seeds → no headroom; task EXCLUDED from aggregate (cold-arm ceiling, ag-2i5jg) — reported, not counted |
| **retrieval-failed** | on-arm's attribution log does NOT contain any of the task's FROZEN known-relevant decision id(s) (empty log, OR none-of-the-frozen-ids retrieved) → a null here is a RETRIEVAL artifact, NOT a moat refutation; EXCLUDED from the moat verdict. Set ONLY by exact mechanical id-comparison — never a human "looks mismatched" call. |
| **degraded-run** | runner error / sandbox-leak sentinel tripped / non-codex agent → run INVALID, re-run |

## The statistic (LOCKED — fixed before compute, no post-hoc choice)
- Seeds are **common-random-number paired across arms**: the same fixed seed list
  runs both context_off and context_on for a task, so per-task delta is paired.
- Per-task pass-rate over the K fixed seeds; per-task delta = paired `on − off`.
- CI method is **fixed: bootstrap over tasks, 10,000 resamples, 95% two-sided CI**
  on the aggregate delta. (Not "bootstrap OR binomial" — bootstrap-over-tasks,
  period.) Per-task CI for category assignment = exact (Wilson) on K seeds.
- **MDE is fixed HERE, not at write-up: MDE = 0.15 absolute aggregate pass-rate
  delta.** Rationale: at K=3 the per-task pass-rate grid is {0, .33, .67, 1.0};
  0.15 aggregate is the smallest effect this design can resolve above seed noise
  without claiming false precision. If observed aggregate delta < 0.15 AND its CI
  straddles 0 → `inconclusive` (need more seeds), NOT `null`. This number is locked;
  changing it post-hoc is a pre-registration violation (append-erratum only).

## Publication decision rule (LOCKED)
- **Claim "moat positive"** ONLY IF: aggregate delta CI lower bound > 0 AND
  ceiling-excluded tasks do NOT dominate (≤ N/3 of the frozen set) AND retrieval-failed
  tasks are excluded AND ≥1 task is independently `positive`.
- **Claim "honest null"** (reshapes the thesis — a VALID, publishable result) ONLY
  IF: off-arm had real headroom on the included tasks (not ceiling'd) AND on-arm
  retrieval was verified present (not `retrieval-failed`) AND delta CI straddles 0.
  A null with ceiling/retrieval contamination is NOT a null — it's `degraded`.
- **Otherwise** → `inconclusive`: report honestly, expand N or K, do not claim
  either direction. **No claim if the aggregate's lower CI ≤ 0 OR ceiling+retrieval
  exclusions exceed N/3 of ALL frozen tasks** (denominator = the frozen set N below,
  not the post-exclusion subset; no tilde — strictly `> N/3`).

## Frozen experiment definition (LOCKED before compute)
- **Task set:** the `evals/workbench/tasks/cd-*` ids present at the run's recorded
  commit AFTER ag-2i5jg's cold-arm pre-screen lands. The scorecard records that
  commit + the exact id list (N). No task is added or dropped after the run begins
  except by the **mechanical** `off==1.0` ceiling rule (ag-2i5jg) — never by hand.
- **Seeds:** fixed list `[1, 2, 3]` (K=3) for PILOT/baseline; PROOF extends to a
  fixed `[1..K]`. The seed list is recorded in the scorecard; no re-rolling a
  task whose result is disliked.
- **Frozen known-relevant decision ids (the retrieval ground truth):** each `cd-*`
  task fixture MUST record, at the recorded commit, the id(s) of the `.agents`
  decision(s) the on-arm is expected to retrieve (the task's "known-relevant
  decision"). Producing these frozen labels is part of **ag-98g1a's acceptance**
  (the retrieval+attribution wiring). No label = the task cannot contribute a
  `retrieval-failed` exclusion (it falls through to null/positive on its score).
- **Category assigner (author ≠ assigner):** `ceiling-excluded`, `retrieval-failed`,
  and `degraded` are assigned **purely mechanically** — `off-arm pass-rate==1.0`;
  the attribution log contains NONE of the task's frozen known-relevant ids (empty
  or no-frozen-id-present — exact id comparison, NO human "mismatch" judgment);
  runner-error/leak sentinel. The residual `positive`/`null`/`inconclusive` call is
  rendered by a **fresh non-author reviewer** against this locked rule, not by the
  run's author. The retrieval-failed exclusion (the moat-saving one) is fully
  mechanical by design — it cannot be hand-adjudicated.

## Run tiers (tier is COMPUTED from observable preconditions, never chosen post-hoc)
- **PROOF** iff, at the run's HEAD commit, the acceptance tests of BOTH ag-98g1a
  (retrieval+attribution wired) AND ag-2i5jg (cold-arm pre-screen) are green —
  recorded as a boolean in the scorecard. A PROOF run is the ONLY tier that may
  assign the LOCKED publication verdict above. Disliking a PROOF result does not
  let you relabel it PILOT — the tier is a fact of the commit, not a choice.
- **PILOT** otherwise: a fixed frozen 5-task subset (the first 5 cd-* ids by sort
  order at the recorded commit) × K=3 = 30 on + 30 off ≤ 60 invocations, codex,
  wall+timeout+cost ceiling, abort-on-error. PILOT de-risks the harness and may emit
  ONLY `positive-signal` / `needs-more` / `degraded` — never a moat verdict, never C.

## Output
A `ContextDeltaScorecard` JSON per the harness + this writeup, recording per-task
category, the aggregate, the CI, the excluded tasks (with reason), the retrieval
attribution per on-arm task, the seeds + commit. Feed the result to the yield
gauge's **C** (or record "C within noise → C≈0 with these exclusions") — never
fabricate C; a degraded/inconclusive run leaves C `pending`.
