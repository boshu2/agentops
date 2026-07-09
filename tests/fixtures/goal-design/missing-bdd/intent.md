---
schema_version: 1
kind: goal-design.intent
id: gd-intent-missing-bdd
slug: missing-bdd
created_at: "2026-07-09T09:00:00-04:00"
status: draft
objective: "Create schema-backed goal-design artifacts for AgentOps."
why_it_matters: "The loop needs first-class intent and driver artifacts, not prompt conventions."
domain_terms:
  - term: goal-design
    definition: "The front-door skill shape for converting human intent into loop-ready artifacts."
    source: "docs/contracts/goal-design-artifacts.md"
boundaries:
  bounded_context: bc-loop
  in_scope:
    - "Schema-backed intent and driver markdown artifacts"
  non_goals:
    - "Do not add a CLI in this slice"
  rollback_or_containment: "Revert the schema, template, contract, checker, and fixture additions."
evidence_for_done:
  first_failing_proof: "tests/scripts/check-goal-design-packet.bats"
  validation_command: "scripts/check-goal-design-packet.sh tests/fixtures/goal-design/missing-bdd"
  evidence_path: "tests/fixtures/goal-design/"
  independent_gate: validate
inputs_to_recheck:
  repo_paths:
    - "AGENTS.md"
  prior_artifacts:
    - "issue-123/.agents/rpi/2026-07-09-agentops-prompt-packet-spike.md"
  live_surfaces:
    - "git status --short"
  stale_assumptions:
    - "The .agents write-surface rules may move."
hard_rules:
  - "Require independent validation before use."
---
# Goal Design Intent: missing-bdd
