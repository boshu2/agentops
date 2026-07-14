---
schema_version: 1
kind: goal-design.intent
id: gd-intent-unknown-behavior-label
slug: unknown-behavior-label
created_at: "2026-07-09T10:30:00-04:00"
status: draft
objective: "Prove the checker rejects candidate behavior labels that do not map to a scenario."
why_it_matters: "Candidate work must trace to declared BDD behavior, not free-floating task labels."
domain_terms:
  - term: behavior label
    definition: "A candidate bead behavior phrase that should contain a scenario id or scenario name from intent.md."
    source: "docs/contracts/goal-design-artifacts.md"
bdd:
  feature: "Goal-design scenario mapping"
  scenarios:
    - id: S1
      name: "Reject unknown behavior labels"
      given:
        - "A packet has intent and driver artifacts"
      when:
        - "The checker validates candidate bead behavior"
      then:
        - "Every candidate behavior must map to an intent scenario"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Candidate-to-scenario mapping"
  non_goals:
    - "Do not add CLI behavior"
  rollback_or_containment: "Remove this negative fixture and checker assertion."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/unknown-behavior-label"
  evidence_path: "tests/fixtures/goal-design/unknown-behavior-label"
inputs_to_recheck:
  repo_paths:
    - "scripts/check-goal-design-packet.sh"
  prior_artifacts:
    - "tests/fixtures/goal-design/valid"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "Scenario labels might later become first-class schema fields."
hard_rules:
  - "Keep this fixture digest-current so it isolates unknown behavior labels."
---
# Goal Design Intent: unknown-behavior-label
