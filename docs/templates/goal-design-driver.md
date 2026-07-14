---
schema_version: 1
kind: goal-design.driver
id: gd-driver-<slug>
slug: <slug>
created_at: "<RFC3339>"
status: draft
intent_ref:
  path: ".agents/goal-design/<slug>/intent.md"
  sha256: "<sha256 of intent.md>"
  schema_version: 1
loop_routing:
  delivery: "<how this becomes issue, bead, or MR-ready work>"
  rpi: "<candidate beads: one behavior, one proof, one tick>"
  promotion: "<what evidence can steer product decisions>"
  knowledge: "<what gets captured, promoted, or compiled after close>"
candidate_beads:
  - id: B1
    behavior: "<scenario id/name from intent>"
    bounded_context: "<bc-* or repo-local context>"
    first_failing_proof: "<test, gate, or command>"
    write_scope:
      - "<path or glob>"
    close_signal: "<verdict or evidence>"
small_batch_gate:
  one_behavior: true
  one_bounded_context: true
  one_primary_write_scope: true
  one_acceptance_proof: true
  split_required_if:
    - "<split trigger>"
route_back_rules:
  checker_fails: "<where to route>"
  bead_closes_with_new_signal: "<how to choose or change the next candidate>"
  candidate_stale: "<how to recheck or discard>"
  promotion_contradicts_intent: "<how to rescope>"
execution_mode:
  default: single-agent
  escalations:
    ntm_atm: "<attach, steer, durability, or cross-model condition>"
    workflow: "<deterministic structured DAG condition>"
artifact_validation:
  checker_command: "scripts/check-goal-design-packet.sh .agents/goal-design/<slug>"
---
# Goal Design Driver: <slug>

## Source Intent

- Intent artifact: `.agents/goal-design/<slug>/intent.md`
- Intent digest: `<sha256>`

## Loop Routing

| Loop | Driver contract |
| --- | --- |
| Delivery | <how this becomes issue, bead, or MR-ready work> |
| RPI | <candidate beads: one behavior, one proof, one tick> |
| Promotion | <what evidence can steer product decisions> |
| Knowledge | <what gets captured, promoted, or compiled after close> |

## Candidate Beads

| Candidate | Behavior | Bounded context | First failing proof | Write scope | Close signal |
| --- | --- | --- | --- | --- | --- |
| B1 | <scenario name> | <bc> | <test/gate> | <files> | <verdict/evidence> |

## Small-Batch Gate

- One behavior per bead: true
- One bounded context per bead: true
- One primary write scope: true
- One acceptance proof: true
- Split required if: `<split trigger>`

## Route-Back Rules

- If the packet checker fails: `<where to route>`
- If a bead closes but reveals a better next step: `<how to choose or change the next candidate>`
- If a candidate becomes stale: `<how to recheck or discard>`
- If promotion or knowledge contradicts the original intent: `<how to rescope>`

## Execution Mode

- Default: single-agent in-session loop.
- Escalate to NTM/ATM only when attach, steer, durability, or cross-model debate is required.
- Escalate to Workflow only for deterministic structured DAGs.

## Packet Check

- Checker command: `scripts/check-goal-design-packet.sh .agents/goal-design/<slug>`
