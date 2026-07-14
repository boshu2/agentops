---
schema_version: 1
kind: goal-design.intent
id: gd-intent-mismatched-slug
slug: mismatched-slug
created_at: "2026-07-09T10:00:00-04:00"
status: draft
objective: "Prove the checker rejects driver and intent slug drift."
why_it_matters: "A valid digest is not enough if the driver names a different packet identity."
domain_terms:
  - term: goal-design
    definition: "The packet contract that turns human intent into loop-ready work."
    source: "docs/contracts/goal-design-artifacts.md"
bdd:
  feature: "Goal-design identity checks"
  scenarios:
    - id: S1
      name: "Reject slug drift"
      given:
        - "A packet has intent and driver artifacts"
      when:
        - "The checker validates cross-file identity"
      then:
        - "The driver slug must match the intent slug"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Cross-file semantic identity"
  non_goals:
    - "Do not add CLI behavior"
  rollback_or_containment: "Remove this negative fixture and checker assertion."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/mismatched-slug"
  evidence_path: "tests/fixtures/goal-design/mismatched-slug"
inputs_to_recheck:
  repo_paths:
    - "scripts/check-goal-design-packet.sh"
  prior_artifacts:
    - "tests/fixtures/goal-design/valid"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "Slug identity might move from checker logic into schema later."
hard_rules:
  - "Keep this fixture digest-current so it isolates slug drift."
---
# Goal Design Intent: mismatched-slug
