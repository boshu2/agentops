---
name: council
description: 'Collect independent perspectives for an explicitly high-stakes or contested judgment. Triggers: "council", "multi-judge review", "independent perspectives".'
practices: [llm-eval-harness, design-by-contract]
hexagonal_role: domain
consumes: [explicit-question, evidence]
produces: [council-report.v1]
context_rel: []
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: judgment
  dependencies: []
  capabilities: [collect_independent_judgments, synthesize_disagreement]
  effects: [write_advisory_council_report]
  canonical_status: canonical
  disposition: keep_strategy
output_contract: council-report.v1
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
