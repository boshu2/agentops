# Decision — making AgentOps evals agent-runnable + agent-monitorable

> **HISTORICAL:** Captures a 2026-06-16 decision about CLI surfaces later removed.

> **Date:** 2026-06-16 · **Method:** mixed (cross-family) council — Claude (Opus 4.8)
> + Codex (gpt-5.3-codex, `codex exec`, read-only) judging the same brief
> independently. **Brief:** `/tmp/eval-orchestration-decision-brief.md`. **Codex
> capture:** `.agents/council/ntm-captures/codex-eval-orchestration-decision.txt`.
> **Bead:** `age-d16-self-hosting-route-nkr.2` (RULER). **Audit:**
> `docs/evals/agentops-effectiveness-evidence.md`.

## Question
Bo: "Evals should be runnable AND monitorable by AI agents — feels like a GREAT use
case for ATM to manage." Decide what to actually do.

## Verdict: **D — evidence-first. Do NOT build orchestration/monitoring before one real PILOT datapoint.** (Both families agree, independently.)

Building `ao eval live` (A), a Workflow (C), or ATM management (B) before a single
live PILOT datapoint optimizes the *management of an unproven measurement* — textbook
cathedral. The bottleneck is **signal, not orchestration**: the live PILOT has never
run, C is unmeasured, and the only measured real deltas are **0** (skill/hook,
ceiling-bound) and **−0.37** (corpus gold). You cannot manage a run you have never
successfully run once.

**Cross-family agreement:** the Codex judge reached D independently, read the code,
and surfaced the key mechanical gap below.

## Key finding (from the Codex judge, verified)
`scripts/eval-agent-harness.sh:115` swallows codex timeout/failure into the score
(`… || true`). A **floored/timed-out** run is therefore indistinguishable from a
**ran-but-failed-the-grader** run. This is why the n=1 smoke floored *silently*
(both arms 0/1) — and it is the real "agent-monitorable" gap: you can't monitor what
you can't distinguish.

## Sequence (locked)
1. **Cheapest signal (now):** run ONE `cd-*` task live at a raised timeout (300s) and
   *observe whether codex produces the deliverable at all* — timeout vs.
   explained-instead-of-wrote vs. wrong-script. If a live agent can't pass a held
   task even at 300s, the **substrate** (not the orchestrator) is the problem — stop
   and fix that.
2. **Minimal bookkeeping (only if step 1 needs it):** add the smallest per-unit status
   JSONL to the existing harness + make timeout/degraded *distinguishable* from
   grader-fail + self-describing `evidence_kind` (folds bug `age-jxq`). ~tens of
   lines, not a new engine. THIS is the cheap "agent-monitorable" unlock.
3. **Run the prereg PILOT once** (first 5 sorted `cd-*` × K=3 = 60 codex calls) via a
   throwaway loop at the fixed timeout. Classify only `positive-signal` /
   `needs-more` / `degraded`. Label PILOT. **No moat claim.**
4. **Build A (`ao eval live`) only if** the PILOT shows life OR unattended runs
   degrade repeatably — and spec it from what actually broke. Deterministic control
   flow → `ao` subcommand, not a Claude-only Workflow (bo-mac prefers codex).
5. **ATM only after A exists** as a pollable substrate: ATM *tends* runs (retry
   degraded shards, rate-limit rotation via `caam`, report status) — it must **not**
   be the eval engine. Aligns with Bo's 2026-06-16 direction: ATM = opt-in
   escalation, not forced substrate.

## On Bo's ATM instinct
Right about the eventual **shape** (a long, metered, rate-limit-prone PROOF campaign
genuinely wants a tend-layer), wrong about the **timing**. ATM earns its place when
unattended-run failure / shard retry / multi-run scheduling becomes the bottleneck —
*after* the first real PILOT, not before.

## Next action
Run step 1 (in flight): bare codex on `cd-door9-1` at `--timeout 300` in a persistent
workspace; inspect the produced artifact. Then decide step 2 vs. straight-to-step-3.

## ROOT CAUSE found by step 1 (2026-06-16) — the live harness has NEVER run codex
The evidence-first probe paid for itself immediately. Running `codex exec` directly on
the task workspace fails instantly:

```
Not inside a trusted directory and --skip-git-repo-check was not specified.
EXIT=1
```

`eval-agent-harness.sh` runs each task in a fresh `mktemp` dir (NOT a git repo). Modern
`codex exec` refuses to run in an untrusted (non-git) directory unless
`--skip-git-repo-check` is passed. The harness does **not** pass it AND swallows the
error (`codex exec … >/dev/null 2>&1 || true`, line 115). Net effect: **codex never
executes, writes nothing, and every `cd-*` task scores 0 in BOTH arms.** The n=1
"floor" (delta=0, both 0/1) was never timeout or capability — it was codex failing to
launch. **No live eval in this repo has ever actually invoked the agent.**

This is the strongest possible vindication of **D**: building `ao eval live`, a
Workflow, or ATM management on top of a runner that has never once launched the agent
would have been pure cathedral — orchestrating zeros.

**Fix (one flag + stop swallowing):** add `--skip-git-repo-check` to the codex
invocation (or `git init` each workspace), and surface a `degraded`/launch-failure
status instead of `|| true` masking it (the same observability gap the Codex judge
flagged, and bug `age-jxq`). This becomes the FIRST eval bead — the substrate must
launch the agent before any PILOT is meaningful.
