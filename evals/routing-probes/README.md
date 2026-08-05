# Routing probes — P(skill loaded | applicable task)

> Every efficacy number in `evals/skill-probes/` measures the skill's effect
> GIVEN it was injected. This directory measures the multiplier the benchmarks
> all skip (the ecological-validity critique in
> docs/research/skill-eval-sota-standards-2026-08.md §1.9): with ~150 skills in
> a flat list, does a real session load the right one unprompted?

## Method (v1 — in-session subagent batch)

A routing scenario is a realistic task prompt in which exactly one (or a small
set of) catalog skill(s) is applicable — the prompt NEVER names the skill or
quotes its trigger phrases verbatim. Dispatch each scenario to a fresh
in-session subagent (sonnet-class worker tier; the sanctioned Claude lane —
subagents see the same skill listing real sessions do). Two deterministic
signals per run:

1. the session telemetry log (`.agents/ao/skill-telemetry.jsonl`, written by
   the opt-in PostToolUse hook when wired) gains a row for the expected skill
   during the run window;
2. the subagent's transcript shows the Skill tool invocation.

Outcome per run: `ROUTED` (expected skill invoked) | `MISSED` (task attempted
hand-rolled) | `MISROUTED` (a different, non-applicable skill invoked).
Report the three rates; MISSED is the product problem, MISROUTED is the
context-pollution problem.

## Honesty

- Measures ROUTING, not efficacy — a ROUTED-but-useless skill still counts as
  routed; efficacy lives in skill-probes/.
- Subagent context is not byte-identical to a fresh top-level session (it
  inherits project CLAUDE.md but not user history); treat rates as an upper
  bound on discoverability, label the runner in every scorecard.
- Scenario prompts must avoid trigger-phrase leakage: if the prompt quotes the
  skill's own trigger strings, the probe measures string matching, not routing.

## Fixture-isolation rule (learned batch 1, 2026-08-05)

Committed scenario files MUST NOT share distinctive surface strings (feature
names, fake filenames, counts) with the prompts as dispatched: two of three
batch-1 agents found `scenarios.json` on disk mid-investigation and one
disclosed it influenced routing. Scenarios are therefore TEMPLATES; the
runner instantiates placeholder strings freshly per run.

## Scenarios

`scenarios.json`: id, prompt, applicable (skill slugs), decoys-tempting
(skills a confused router might pick), rationale.
