---
name: operationalize
description: 'Distill repeated, evidence-backed expertise into a proposed skill, check, reference, or workflow artifact. Triggers: "operationalize this", "turn this expertise into a reusable capability".'
practices: [continuous-learning, design-by-contract]
hexagonal_role: supporting
consumes: [evidence-backed-expertise]
produces: [operationalization-proposal.v1]
context_rel:
- kind: supplier-to
  with: skill-builder
- kind: supplier-to
  with: workflow-builder
skill_api_version: 1
user-invocable: true
metadata:
  tier: meta
  dependencies: []
  capabilities: [distill_expertise, propose_artifact_shape]
  effects: [write_advisory_proposal]
  canonical_status: canonical
  disposition: keep_specialist
output_contract: operationalization-proposal.v1
---

# Operationalize

Turn repeated, cited expertise into a proposal for a reusable artifact.

1. Require at least two distinct examples or one explicit authoritative source.
2. State the triggering situation, desired behavior, inputs, outputs, negative
   examples, and evidence.
3. Choose the smallest fitting shape: reference, skill, deterministic check, or
   caller-owned workflow.
4. Search existing capabilities and prefer extension over duplication.
5. Provide an activation example, holdout/negative example, owner, and rollback
   or deletion condition.
6. Return the proposal to the caller or an authoring specialist.

Operationalize does not create tracker work, promote policy, start a factory,
validate its own output, or control another invocation.
