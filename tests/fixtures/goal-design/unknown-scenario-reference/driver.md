---
schema_version: 1
kind: goal-design.driver
id: gd-driver-unknown-scenario-reference
slug: unknown-scenario-reference
created_at: "2026-07-09T10:25:00-04:00"
status: draft
intent_ref:
  path: ".agents/goal-design/unknown-scenario-reference/intent.md"
  sha256: "fdd3b26bec6eaff166b9dff59bc71e7d1f05846342a7899f5c1693c911d4c2e5"
  schema_version: 1
loop_routing:
  delivery: "File one bead only after the packet validates."
  rpi: "Run one inner tick over one behavior and one first failing proof."
  promotion: "Promote the contract only after checker and validator pass."
  knowledge: "Capture checker failures as future guardrails."
candidate_beads:
  - id: B1
    behavior: "S9: reject unknown scenario references"
    bounded_context: bc-loop
    first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
    write_scope:
      - "scripts/check-goal-design-packet.sh"
    close_signal: "Checker rejects a candidate that cites S9."
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
    - "The change starts adding CLI behavior."
route_back_rules:
  checker_fails: "Patch the packet contract before filing work."
  bead_closes_with_new_signal: "Use the close verdict to revise the next candidate."
  candidate_stale: "Re-read the contract and regenerate the driver digest."
  promotion_contradicts_intent: "Revise intent.md and revalidate."
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: "Only for later attach, steer, durability, or cross-model debate needs."
    workflow: "Only for later deterministic structured DAGs."
artifact_validation:
  checker_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/unknown-scenario-reference"
---
# Goal Design Driver: unknown-scenario-reference
