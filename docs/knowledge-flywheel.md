# Optional Learning Loop

> **Status: design, not implementation.** The schemas and the contract on this
> page exist on disk. The reducer they describe does not. Read this page as a
> declared shape for a loop that may be built, never as a description of
> something running.

AgentOps core ends when Validate returns a fresh judgment and RPI reports it.
Learning and durable verdict storage are deliberately off the critical path.

## What is implemented

The [Memory skill](https://github.com/boshu2/agentops/blob/main/skills/memory/SKILL.md)
provides optional recall, mining, and curation over caller-selected evidence.
Its mine/learn operation absorbs the former `learn` entry point. Neither
ordinary implementation nor final validation requires a learning step.

A mining request names the sources, question, destination, and bounds. It may
return supported observations or no change. Updating a topic page requires
provenance, factual support, and destination-disclosure review. Drafts stay in
caller-selected protected external storage; existing unique evidence is
preserved, with no automatic expiry or deletion.

Memory does not change a completed verdict, select follow-up work, install a
rule, or operate a tracker or delivery transition. A proposal for a reusable
skill or check becomes a separate authorized implementation task.

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
contract pages, and dated audit records. No `SKILL.md` — `memory` included —
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
