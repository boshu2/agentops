---
name: council
description: Collect independent perspectives for an
---
# Council

Council is an optional judgment strategy, not a lifecycle or delivery gate. Use
it when one fresh validator is insufficient for a named irreversible,
high-blast-radius, or genuinely contested decision.

1. Freeze one question, acceptance surface, evidence set, and subject digest.
2. Give each judge an independent context and the same bounded packet.
3. Require each judge to cite evidence, disclose omissions, and return its own
   judgment without seeing other answers first.
4. Synthesize agreement and disagreement without majority laundering. Preserve
   minority evidence and unresolved assumptions.
5. Write `council-report.v1` and return it to the caller.

Council does not write `verdict.v2`, edit the subject, retry work, choose a next
action, or authorize Git, closure, release, or delivery. When Council is used as
a Validate strategy, one accountable fresh validator consumes its report and
Validate remains the sole durable verdict writer.
