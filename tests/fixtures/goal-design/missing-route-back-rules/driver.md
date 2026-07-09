---
schema_version: 1
kind: goal-design.driver
id: gd-driver-valid-packet
slug: valid-packet
created_at: "2026-07-09T09:05:00-04:00"
status: draft
intent_ref:
  path: ".agents/goal-design/valid-packet/intent.md"
  sha256: "35963f3584cd932028b533d1d529cc88a19faf6c556029d71ba46e6458f363dc"
  schema_version: 1
loop_routing:
  delivery: "File one bead only after the packet validates."
  rpi: "Run one inner tick over one behavior and one first failing proof."
  promotion: "Promote the contract only after the checker and independent verdict pass."
  knowledge: "Turn checker failures and closed-bead verdicts into future guidance."
candidate_beads:
  - id: B1
    behavior: "S1: schema-backed artifacts validate"
    bounded_context: bc-loop
    first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
    write_scope:
      - "schemas/goal-design-intent.v1.schema.json"
    close_signal: "Checker tests pass and validate returns PASS."
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
    - "CLI scaffolding is mixed into artifact validation."
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: "Only for later attach, steer, durability, or cross-model debate needs."
    workflow: "Only for later deterministic structured DAGs."
artifact_validation:
  checker_command: "scripts/check-goal-design-packet.sh .agents/goal-design/valid-packet"
  independent_validator: validate
  required_verdict: PASS
---
# Goal Design Driver: missing-route-back-rules
