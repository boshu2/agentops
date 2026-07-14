---
schema_version: 1
kind: goal-design.intent
id: gd-intent-misleading-intent-path
slug: misleading-intent-path
created_at: "2026-07-09T10:10:00-04:00"
status: draft
objective: "Prove the checker rejects a driver that points at the wrong packet path."
why_it_matters: "A digest can match local bytes while the rendered driver tells agents to read a different intent."
domain_terms:
  - term: intent_ref.path
    definition: "The canonical packet-relative identity pointer in driver.md."
    source: "docs/contracts/goal-design-artifacts.md"
bdd:
  feature: "Goal-design identity checks"
  scenarios:
    - id: S1
      name: "Reject misleading intent path"
      given:
        - "A packet has intent and driver artifacts"
      when:
        - "The checker validates cross-file identity"
      then:
        - "The driver intent path must name this packet's intent artifact"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Cross-file semantic identity"
  non_goals:
    - "Do not add CLI behavior"
  rollback_or_containment: "Remove this negative fixture and checker assertion."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/misleading-intent-path"
  evidence_path: "tests/fixtures/goal-design/misleading-intent-path"
inputs_to_recheck:
  repo_paths:
    - "scripts/check-goal-design-packet.sh"
  prior_artifacts:
    - "tests/fixtures/goal-design/valid"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "Path identity might move from checker logic into schema later."
hard_rules:
  - "Keep this fixture digest-current so it isolates path drift."
---
# Goal Design Intent: misleading-intent-path
