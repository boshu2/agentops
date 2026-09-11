# Routing probes — P(skill loaded | applicable task)

The routing question is whether a natural task makes the appropriate skill
available and selected. This is separate from whether loading the skill helps:
behavioral probes in `evals/skill-probes/` condition on skill content already
being supplied.

## Method (v1 — in-session subagent batch)

A routing scenario is a realistic task prompt in which exactly one (or a small
set of) catalog skill(s) is applicable — the prompt NEVER names the skill or
quotes its trigger phrases verbatim. Dispatch each scenario to a fresh
context using the caller-authorized runtime, model, effort, and bounds. Record
the catalog actually supplied and any inherited context. For the historical
Claude adapter, two observable signals were used:

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
- A subagent or provided-catalog prompt is not an isolated fresh top-level
  installation. Record the actual dispatch configuration and label conclusions
  by that surface; prompt wording alone does not prove runtime isolation.
- Scenario prompts must avoid trigger-phrase leakage: if the prompt quotes the
  skill's own trigger strings, the probe measures string matching, not routing.

## Fixture-isolation rule (learned batch 1, 2026-08-05)

Committed scenario files MUST NOT share distinctive surface strings (feature
names, fake filenames, counts) with the prompts as dispatched: two of three
batch-1 agents found `scenarios.json` on disk mid-investigation and one
disclosed it influenced routing. Scenarios are therefore TEMPLATES; the
runner instantiates placeholder strings freshly per run.

## Scenarios

`templates.json` contains task prompts, applicable owner IDs, and placeholders.
`instantiate.py` supplies fresh scenario values for a selected run. Historical
captures retain the skill names and catalog they actually observed.

## Method (v2 — offline deterministic goldens)

The v1 method needs an authorized live-model run. Its recorded batches have
context and contamination limits; they are not an inventory-wide routing score.
`goldens/` supplies the offline check: hand-authored fixtures in
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

### Historical ranker finding (2026-08-26) — closed on that subject

`rq-04-independent-verdict` was red at authoring: six natural ways to ask for
an independent verdict on finished work failed to surface `validate` in the
top 3; only a phrasing containing the literal word "acceptance" did. Closed by
the pointer-wording-first repair the roadmap prescribes — `validate`'s
description gained the caller's own words (finished, proven, verdict, merge)
and now ranks 1 at 0.333 for the golden's query. The golden pins the repair:
a description regression reopens it, and the advisory nightly job plus the
bats honest-pin go red with it.
