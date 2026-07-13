# evolve

Run autonomous improvement loops. Triggers: "evolve", "improve everything", "autonomous improvement".

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` file for this skill.
Then read local files in `references/` and `scripts/` when needed.


<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->
<!-- Generated from skills-codex-overrides/catalog.json for evolve. -->

## Codex Execution Profile

1. Drive the lead cycle in-session through the skills; do not shell out to a CLI loop wrapper.
2. Persist loop state under `.agents/evolve/` and recover from disk instead of relying on live context.

## Guardrails

1. Enforce `Validate -> Learn -> orchestrator`; only an orchestrator-owned changed plan enters `$premortem`.

<!-- END AGENTOPS OPERATOR CONTRACT -->
