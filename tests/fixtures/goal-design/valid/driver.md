---
schema_version: 1
kind: goal-design.driver
id: gd-driver-valid-packet
slug: valid-packet
created_at: "2026-07-09T09:05:00-04:00"
status: draft
intent_ref:
  path: ".agents/goal-design/valid-packet/intent.md"
  sha256: "1b1ef3d67fd22ae15ba6401c69c52a66cc7a146231697a4943652e46ad42130d"
  schema_version: 1
loop_routing:
  delivery: "File one bead only after the packet checker passes."
  rpi: "Run one inner tick over one behavior and one first failing proof."
  promotion: "Promote the contract only after the deterministic checker passes."
  knowledge: "Turn repeated checker failures into future guidance."
candidate_beads:
  - id: B1
    behavior: "S1: schema-backed artifacts validate"
    bounded_context: bc-loop
    first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
    write_scope:
      - "schemas/goal-design-intent.v1.schema.json"
      - "schemas/goal-design-driver.v1.schema.json"
      - "scripts/check-goal-design-packet.sh"
    close_signal: "Goal-design packet checker passes."
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
    - "CLI scaffolding is mixed into artifact validation."
route_back_rules:
  checker_fails: "Repair the packet shape, refresh the digest, and rerun the deterministic checker."
  bead_closes_with_new_signal: "Use the close verdict to choose or revise the next candidate."
  candidate_stale: "Re-read the named canonical docs, refresh the driver digest, and rerun the checker."
  promotion_contradicts_intent: "Revise intent.md, update the driver, and return the changed packet to Discovery or Plan."
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: "Only for later attach, steer, durability, or cross-model debate needs."
    workflow: "Only for later deterministic structured DAGs."
artifact_validation:
  checker_command: "scripts/check-goal-design-packet.sh .agents/goal-design/valid-packet"
---
# Goal Design Driver: valid-packet

## Source Intent

- Intent artifact: `.agents/goal-design/valid-packet/intent.md`
- Intent digest: `416bdb45f717067c1a09a846b3f357ca98de64bd0fb3e873a7af4ee3429846f7`
## Packet Boundary

This packet owns deterministic intent conformance only. Premortem judges the
exact final plan after Discovery and Plan have shaped it.
