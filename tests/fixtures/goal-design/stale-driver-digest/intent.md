---
schema_version: 1
kind: goal-design.intent
id: gd-intent-valid-packet
slug: valid-packet
created_at: "2026-07-09T09:00:00-04:00"
status: draft
objective: "Create schema-backed goal-design artifacts for AgentOps."
why_it_matters: "The loop needs first-class intent and driver artifacts, not prompt conventions."
domain_terms:
  - term: goal-design
    definition: "The front-door skill shape for converting human intent into loop-ready artifacts."
    source: "docs/contracts/goal-design-artifacts.md"
bdd:
  feature: "Goal-design artifacts"
  scenarios:
    - id: S1
      name: "Validate a complete packet"
      given:
        - "An operator has a goal that should leave their head"
      when:
        - "The goal-design checker validates the packet"
      then:
        - "Both markdown artifacts validate against versioned schemas"
        - "The driver digest matches the current intent artifact"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Schema-backed intent and driver markdown artifacts"
  non_goals:
    - "Do not add a CLI in this slice"
  rollback_or_containment: "Revert the schema, template, contract, checker, and fixture additions."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/valid"
  evidence_path: "tests/fixtures/goal-design/"
inputs_to_recheck:
  repo_paths:
    - "AGENTS.md"
    - "docs/architecture/operating-loop.md"
  prior_artifacts:
    - "issue-123/.agents/rpi/2026-07-09-agentops-prompt-packet-spike.md"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "The .agents write-surface rules may move."
hard_rules:
  - "Require the deterministic packet checker before use."
  - "Keep candidate beads small."
---
# Goal Design Intent: valid-packet
