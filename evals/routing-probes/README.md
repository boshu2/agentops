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

## Method (v2 — offline deterministic goldens)

The v1 method needs a live model, so it cannot run unattended and has produced
exactly three rows (2026-08-05, one of them contaminated). `goldens/` adds the
half that CAN run offline every night: hand-authored fixtures in
[`schemas/pack-quality-expectations.v1.schema.json`](../../schemas/pack-quality-expectations.v1.schema.json)
shape, graded against `ao skills find` — the repo's own deterministic
token-overlap discovery surface — by
[`scripts/check-routing-probe-goldens.sh`](../../scripts/check-routing-probe-goldens.sh).

```bash
bash scripts/check-routing-probe-goldens.sh          # human table
bash scripts/check-routing-probe-goldens.sh --json   # machine-readable
bash scripts/validate-manifests.sh --repo-root .     # goldens vs. the contract
```

Each golden declares one query, the pack size it is graded at, the ids that
must appear, the ids that must not, regexes that must not hold rank 1, a
provenance-density floor, and a token ceiling for the pack. Findings are named
per fixture: `MISS`, `LEAK`, `MISROUTE`, `PROVENANCE`, `TOKENS`, `SCHEMA`. Zero
goldens is a hard failure in both the grader and `validate-manifests.sh` — a
retrieval eval with an empty denominator reports green forever.

### What v2 does NOT measure

It grades **the catalog's discoverability on a deterministic ranker**, not what
a model loads. A green run says the declared skill still wins for the declared
phrasing; it says nothing about efficacy, and it does not replace the v1 live
batches. Treat the two as separate factors, exactly as the v1 honesty section
does.

The fixture-isolation rule above does **not** bind `goldens/`: the grader is
not an agent and `ao skills find` never reads this directory, so committed
literal queries cannot contaminate it. That protection is why goldens are
graded offline and why a golden query must never be reused as a live-dispatch
prompt.

### Standing finding (2026-08-26) — CLOSED same day

`rq-04-independent-verdict` was red at authoring: six natural ways to ask for
an independent verdict on finished work failed to surface `validate` in the
top 3; only a phrasing containing the literal word "acceptance" did. Closed by
the pointer-wording-first repair the roadmap prescribes — `validate`'s
description gained the caller's own words (finished, proven, verdict, merge)
and now ranks 1 at 0.333 for the golden's query. The golden pins the repair:
a description regression reopens it, and the advisory nightly job plus the
bats honest-pin go red with it.
