# Optional Learning Loop

> **Status: design, not implementation.** The schemas and the contract on this
> page exist on disk. The reducer they describe does not. Read this page as a
> declared shape for a loop that may be built, never as a description of
> something running.

AgentOps core ends when Validate returns a fresh judgment and RPI reports it.
Learning and durable verdict storage are deliberately off the critical path.

## What is implemented

One thing: the [`learn`](https://github.com/boshu2/agentops/blob/main/skills/learn/SKILL.md)
skill, an optional, caller-invoked consumer of durable `verdict.v2`
collections. It is a skill — prose an agent follows — with no CLI command and
no scheduled or automatic invocation behind it.

Invoked, `learn` summarizes recurring evidence across a caller-supplied verdict
collection and may propose a candidate deterministic check for later human or
caller evaluation. Its own contract bounds the output hard:

> When the caller asks for a durable artifact, write the observations under
> `.agents/scratch/learn/` and return the path; otherwise return them inline.
> The write is advisory and TTL'd — it is never a source of record, and its
> absence never changes whether a candidate is valid.

`.agents/scratch/` is disposable state by [ADR-0016](adr/ADR-0016-state-tiers.md):
rebuildable or expirable, never authority. So the implemented half of this loop
reads durable evidence and writes only to a tier that nothing is allowed to
trust.

`learn` does not:

- change a completed verdict;
- repair or re-plan work;
- choose a next invocation;
- activate a rule or deterministic check;
- mutate Git, a tracker, or delivery state; or
- block RPI when its own storage or analysis is unavailable.

## What is designed but not built

The reduction from repeated Validate findings to an advisory producer-rule
candidate — group finding observations by defect class, count distinct
objectives, emit one candidate per recurring class. That reduction is specified
in [the producer-defect recurrence contract](contracts/producer-defect-register.md)
and typed by three schemas:

| Schema | Declares | Consumers |
|---|---|---|
| `schemas/finding-observation.v1.schema.json` | one evidence-backed finding observation | none |
| `schemas/producer-rule-candidate.v1.schema.json` | one advisory producer-rule candidate | none |
| `schemas/learning.v1.schema.json` | a learning record | none |

"None" is literal. A search across `cli/` and `scripts/` finds no code that
reads, writes, or validates against any of the three; the only non-schema hits
in the repository are `scripts/insert-schema-practices.py` (which tags every
schema's `practices` field and knows nothing about their content), the two
contract pages, and dated audit records. No `SKILL.md` — `learn` included —
cites them either, so nothing instructs an agent to emit these shapes by hand.

The schemas stay on disk deliberately: they are the declared contract if the
loop is ever built. They are not evidence that it was.

## Why the write half is gone

The compounding claim it was meant to serve was never proven, and the machinery
was cut rather than kept as a green-looking shell:

- [ADR-0004](adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md) — the
  corpus moat is unproven; do not position on it.
- [ADR-0011](adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)
  — escape-corpus compounding is structurally data-starved: a competent
  membrane catches at review, so escapes are rare by construction.
- [ADR-0014](adr/ADR-0014-catch-to-producer-loop-judgment-catches-need-a-producer-route.md)
  — the catch-to-producer route, superseded by the Cathedral Cut. Its commands,
  automatic routing, receipts, and producer-side mutation are not active
  product behavior.
- [ADR-0016](adr/ADR-0016-state-tiers.md) — one authority per claim;
  projections and scratch are never authoritative.

`ao flywheel` and the whole knowledge-compounding command family were removed
with that cut; see [MIGRATION.md](MIGRATION.md) for the per-verb replacements.

## If you want the loop

Promotion from an observed pattern into a skill, test, or repository rule is a
separate caller-authorized change with its own Plan, Implement, and Validate
cycle. That is the currently supported path, and it is a human-driven one. It
preserves the useful compounding idea without making bookkeeping a condition
for finishing ordinary work — and without a metric that reports compounding
nobody measured.
