---
schema_version: 1
kind: goal-design.intent
id: gd-intent-unknown-scenario-reference
slug: unknown-scenario-reference
created_at: "2026-07-09T10:20:00-04:00"
status: draft
objective: "Prove the checker rejects candidate beads that cite unknown scenario IDs."
why_it_matters: "Candidate work must trace to declared BDD behavior, not invented scenario IDs."
domain_terms:
  - term: scenario reference
    definition: "A candidate bead behavior token matching an intent BDD scenario id."
    source: "docs/contracts/goal-design-artifacts.md"
bdd:
  feature: "Goal-design scenario mapping"
  scenarios:
    - id: S1
      name: "Reject unknown scenario references"
      given:
        - "A packet has intent and driver artifacts"
      when:
        - "The checker validates candidate bead behavior"
      then:
        - "Every cited scenario id must exist in the intent scenarios"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Candidate-to-scenario mapping"
  non_goals:
    - "Do not add CLI behavior"
  rollback_or_containment: "Remove this negative fixture and checker assertion."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/unknown-scenario-reference"
  evidence_path: "tests/fixtures/goal-design/unknown-scenario-reference"
  independent_gate: validate
inputs_to_recheck:
  repo_paths:
    - "scripts/check-goal-design-packet.sh"
  prior_artifacts:
    - "tests/fixtures/goal-design/valid"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "Scenario id syntax might broaden beyond S<number> later."
hard_rules:
  - "Keep this fixture digest-current so it isolates unknown scenario references."
---
# Goal Design Intent: unknown-scenario-reference
