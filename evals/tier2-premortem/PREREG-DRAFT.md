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

## Pilot protocol (before lock)

1. Author 6–10 tasks across the four flaw classes.
2. **Headroom screen per task:** n=3 control-only runs; a task where control
   false_pass < 1/3 is DEAD for measurement (gap too easy — the t01 smoke's
   single control run already cleared the hidden check via defensive
   whitespace habits; t01's hidden acceptance likely needs a harder gap, e.g.
   locale decimal commas or mixed-number forms).
3. Estimate per-task variance from the screen → `ao eval suite n-required` →
   propose trials/task and MDE to Bo → freeze this document (drop -DRAFT).

## To be locked at freeze

MDE (proposed 0.25 on false_pass at this config), task list + digests, seed
list, trials per task per arm, publication rule verbatim (including
honest-null), budget cap in runs.
