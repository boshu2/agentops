# crank

Execute the next ready epic wave and return evidence before any between-wave decision. Triggers: "crank an epic", "execute the next wave", "drive the bead wave plan".

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` file for this skill.
Then read local files in `references/` and `scripts/` when needed.


<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->
<!-- Generated from skills-codex-overrides/catalog.json for crank. -->

## Codex Execution Profile

1. In Codex hookless mode, run `ao codex ensure-start` before the first wave; the CLI records startup once per thread and skips duplicates automatically.
2. Prefer direct Codex session-agent orchestration for disjoint workers; preserve the source skill's one-wave boundary.

## Guardrails

1. End after one wave. RPI routes evidence through `$validate`, `$learn`, and the orchestrator before another `$crank` invocation.

<!-- END AGENTOPS OPERATOR CONTRACT -->
