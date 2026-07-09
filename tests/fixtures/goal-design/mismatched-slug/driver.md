---
schema_version: 1
kind: goal-design.driver
id: gd-driver-other-slug
slug: other-slug
created_at: "2026-07-09T10:05:00-04:00"
status: draft
intent_ref:
  path: ".agents/goal-design/mismatched-slug/intent.md"
  sha256: "525101add8f953f622ca152a95f5e87b18d58a720bf8e93474d61dd0bd728a0e"
  schema_version: 1
loop_routing:
  delivery: "File one bead only after the packet validates."
  rpi: "Run one inner tick over one behavior and one first failing proof."
  promotion: "Promote the contract only after checker and validator pass."
  knowledge: "Capture checker failures as future guardrails."
candidate_beads:
  - id: B1
    behavior: "S1: reject slug drift"
    bounded_context: bc-loop
    first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
    write_scope:
      - "scripts/check-goal-design-packet.sh"
    close_signal: "Checker rejects mismatched driver and intent slugs."
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
    - "The change starts adding CLI behavior."
route_back_rules:
  validation_fails: "Patch the packet contract before filing work."
  bead_closes_with_new_signal: "Use the close verdict to revise the next candidate."
  candidate_stale: "Re-read the contract and regenerate the driver digest."
  promotion_contradicts_intent: "Revise intent.md and revalidate."
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: "Only for later attach, steer, durability, or cross-model debate needs."
    workflow: "Only for later deterministic structured DAGs."
artifact_validation:
  checker_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/mismatched-slug"
  independent_validator: validate
  required_verdict: PASS
---
# Goal Design Driver: other-slug
