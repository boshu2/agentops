---
name: learn
description: 'Optionally analyze collections of durable verdicts for recurring evidence after the critical path. Triggers: "learn from verdicts", "mine validation history".'
---
# Learn

Learn is an optional, off-path consumer of durable `verdict.v2` collections.
It may summarize recurring evidence and propose a candidate deterministic check
for later human or caller evaluation.

## Prompt

```text
Mine .agents/ao/verdicts/ for recurring patterns across the last 20
verdict.v2 records in agentops-wt/train2-c. I want candidate deterministic
checks for anything that shows up as a repeated NOT_PROVEN or FAIL cause,
with digests cited so I can trace each observation back.
```

## It's working if

Observable in the trace, without reading the prose:

- Every observation binds a `verdict.v2` digest and a finding id from
  `.agents/ao/verdicts/`.
- A `NOT_PROVEN` or `FAIL` verdict pair is harvested before a `PASS`-only
  pattern.
- A citation that no longer resolves under `.agents/ao/verdicts/` is
  pruned rather than paraphrased.
- Output written to `.agents/scratch/learn/` is labeled advisory and
  TTL'd, not a source of record.

## Contract

Learn does not run during RPI, validate a subject, alter a verdict, mutate a
plan, promote a rule, choose continuation, or mint lifecycle artifacts. Missing
Learn output never changes whether a candidate is valid.

When invoked, bind every observation to verdict and finding digests, distinguish
repeated objectives from repeated reviews of one objective, disclose the sample
size, and stop at advisory evidence.

Overweight failures: a `NOT_PROVEN` or `FAIL` verdict carries more teaching
value than a PASS, because it names a rule the loop lacked. Harvest kernels
from failed lanes first — the canonical example is the mutating-check
quarantine in `skills/validate/SKILL.md`, a durable rule minted from a
`NOT_PROVEN`-then-`PASS` verdict pair.

Promote a repeat into a check proposal: a finding `class` seen in two repair
rounds of one traversal, or in two separate runs, leaves the output as a
deterministic check proposal named with that class, the exact sentence or
pattern the check would refuse, and the gate tier it would run in. The
proposal is advisory text for a human or caller to weigh; Learn never edits a
gate, a registry, or a check script itself.

Prune for provenance decay: every cited artifact must still resolve — the
file exists or the verdict digest is present under `.agents/ao/verdicts/`. A
citation that no longer resolves gets pruned rather than paraphrased, and
confidence in a lesson that has not been reproduced since its source decayed
goes down, not sideways.

When the caller asks for a durable artifact, write the observations under
`.agents/scratch/learn/` and return the path; otherwise return them inline.
The write is advisory and TTL'd — it is never a source of record, and its
absence never changes whether a candidate is valid.
