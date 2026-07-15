---
name: implement
description: 'Execute one bounded RED to GREEN to refactor experiment from a PlanPacket and return a CandidatePacket. Triggers: "implement", "build this plan", "run the experiment".'
practices:
- tdd
- refactoring
- small-batch-flow
hexagonal_role: driving-adapter
consumes:
- plan-packet.v1
produces:
- candidate-packet.v1
context_rel:
- kind: customer-of
  with: plan
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: execution
  dependencies: []
  capabilities: [execute_one_experiment, collect_factual_evidence]
  effects: [modify_declared_subject, write_candidate_packet]
  canonical_status: canonical
  disposition: keep
output_contract: schemas/candidate-packet.v1.schema.json
---

# Implement

Execute exactly one bounded experiment described by a `PlanPacket`. Implement
owns subject edits and factual evidence. It does not own work selection,
tracking, Git, retries, semantic validation, repair, closure, or delivery.

## Workflow

1. Verify the PlanPacket digest and freeze its acceptance and write scope.
2. Run the declared first acceptance check before changing behavior. For a
   behavior change, preserve evidence that it fails for the expected missing
   behavior. For docs-only or pure refactor work, record an honest green
   pre-change baseline instead.
3. Make the smallest in-scope change that satisfies the active behavior.
4. Run the targeted acceptance checks and capture factual results.
5. Refactor only while those checks stay green. Refactoring does not change the
   acceptance test.
6. Enumerate every actual changed path using a caller- or runtime-provided
   comparison. If complete coverage cannot be established, set
   `changed_path_coverage_complete` to false; do not guess.
7. Compute `subject-manifest.v1` with the pure Validate helper and write a
   `candidate-packet.v1` containing author context ID, subject locator, actual
   changed paths, evidence, and results.
8. Return the CandidatePacket and stop.

Specialists such as standards, domain, test, refactor, and security may provide
advice. They are never hard dependencies and cannot add lifecycle authority.

## Boundary

- Do not commit, push, claim, close, release, land, reserve, retry, or invoke a
  semantic validator.
- Do not silently expand acceptance. A different acceptance contract is a new
  intent for a caller to start separately.
- A failed check is evidence in the CandidatePacket, not permission to loop.
