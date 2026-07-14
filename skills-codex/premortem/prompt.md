# premortem

Use when: an exact plan needs a verdict. Stress-tests readiness before work with one fresh, independent judge.

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` file for this skill.
Then read local files in `references/` and `scripts/` when needed.


<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->
<!-- Generated from skills-codex-overrides/catalog.json for premortem. -->

## Codex Execution Profile

1. Lead with the verdict, then the smallest set of blocking findings that would change implementation behavior.
2. Between waves, accept only a changed plan from an explicit orchestrator request.

## Guardrails

1. Do not accept direct control transfer from `$validate` or `$learn`.

<!-- END AGENTOPS OPERATOR CONTRACT -->
