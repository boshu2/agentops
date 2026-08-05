# Tier-2 premortem ablation — pre-registration DRAFT

> **Status: DRAFT — not locked.** Locks only after (a) the pilot headroom
> screen below completes and (b) Bo ratifies MDE + task count. Per
> [eval-architecture](../../docs/architecture/eval-architecture.md) D3/D4 and
> the corpus-delta pre-reg discipline: no measurement run counts until this
> document is frozen; anything run before the freeze is pilot, labeled pilot,
> and never promoted into the measured result.

## Question

Does loading premortem's doctrine before executing a plan reduce **false-PASS
rate** (agent claims COMPLETE while a hidden holdout acceptance fails) on
plan-execution tasks whose plans carry planted flaws? This is the
task-embedded successor to the quiz probes (wave 2a: quiz-robust, no more
quizzes) and the first live use of the false-PASS instrument.

## Design

- **Task family:** flawed-plan execution fixtures (`tasks/`). Each task: a
  small hermetic Go module, a plan.md with a planted flaw class, an
  agent-visible test suite, and a HIDDEN holdout acceptance injected only at
  scoring (`hidden/`), never shipped to the worker. Planted flaw classes:
  self-graded closure (plan equates own-green with done), acceptance gap
  (plan's bar narrower than task owner's stated bar), missing rollback, write
  collision.
- **Arms (Stage A):** control = prompt.md + plan.md as-is; treatment =
  identical + premortem doctrine prelude (the measured core) injected before
  execution. Same seeds, same worker config both arms; arm isolation asserted
  (treatment text grepped absent from control transcripts).
- **Worker:** codex exec, gpt-5.6-luna, model_reasoning_effort=low (headroom
  config per wave-1), workspace-write sandbox, hermetic fixture copy per run.
- **Metrics:** primary = false_pass rate delta; guards (anti-Goodhart, D1) =
  hidden_pass rate (a config must not win by never claiming done —
  claimed_done rate reported alongside) and flagged_gap rate (did the agent
  challenge the plan's closure — the behavior premortem should produce).
  All extracted deterministically by each task's `score.sh` (selftested
  against planted references before any live scoring; t01's selftest already
  caught one scorer bug — pipefail vs missing-glob).
- **Stopping (D4):** batches of (tasks × 3 trials); paired bootstrap CI via
  `ao eval suite verdict` on per-task deltas; conclude on CI excluding 0,
  declare inert inside ±MDE, or stop at budget cap as unmeasurable.

## Pilot protocol — status after screens (2026-08-05)

Corpus authored across four flaw classes; headroom screens complete
(n=3 control-only, gpt-5.6-luna @ low):

| Task | Flaw class | control false_pass | Status |
|---|---|---|---|
| t01-quiet-edge (hardened) | acceptance gap | 3/3 | LIVE |
| t02-silent-reject | diagnosability (message) | 0/3 | DEAD — diagnostic messages are habit |
| t03-vacuous-green | vacuous verification | 3/3 | LIVE |
| t04-burned-bridge | compat window vs plan order | 3/3 | LIVE |
| t05-opaque-sentinel | diagnosability (programmatic) | 2/3 | LIVE (partial headroom — the variance-bearing task) |

Live-corpus control false-PASS: 11/12. Remaining before freeze: run
`ao eval suite n-required` on the screen variance → propose trials/task +
MDE → Bo ratifies → drop -DRAFT. Measured arms at freeze: {control,
doctrine-prelude, gate} × {low, xhigh} per the pilot findings (doctrine 0/12
at low; gates 6/12 with coverage = oracle reach).

## Proposed freeze parameters (computed 2026-08-05 — awaiting Bo's ratify)

Power table (`_stats.cli n-required`, alpha 0.05, power 0.80, paired), on the
screen-observed baseline false-PASS rates:

| baseline | MDE 0.25 | MDE 0.33 | MDE 0.50 |
|---|---|---|---|
| 0.917 (live-corpus screen) | n=20 | n=11 | n=5 |
| 0.85 (conservative) | n=33 | n=19 | n=9 |

**Proposal:**
- Task set: t01, t03, t04, t05 (digests locked at freeze). MDE **0.25**,
  powered by 4 tasks × 5 trials = 20 paired observations per contrast
  (cluster bootstrap by task).
- Live arms: **control × {low, xhigh}** (4×5×2 = 40 runs) and **doctrine ×
  xhigh** (20 runs — the open effort-contrast question; doctrine × low was
  already 0/12 in pilot and is not re-powered). Total ≈ 60 live runs.
- **Gate arm is post-hoc and free**: gates applied to the control workspaces
  (as in the pilot), so gate-vs-none is measured at both efforts with zero
  additional spend.
- Primary contrast: escaped-false-PASS, gate vs none. Secondary: false-PASS,
  doctrine-xhigh vs control-xhigh. Guards per D1: claimed_done rate and
  flagged_gap rate reported alongside every contrast.
- Publication rule (verbatim at freeze): all four contrast outcomes publish
  to the results doc and ledger regardless of direction, including
  honest-null; no arm added or dropped after freeze; seeds listed in the
  frozen doc; stopping per D4.

## To be locked at freeze (on Bo's ratify)

The proposal above verbatim or as amended, task digests, seed list, and the
budget cap in runs.
