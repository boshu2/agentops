# crank

Execute the next ready epic wave and return evidence before any between-wave decision. Triggers: "crank an epic", "execute the next wave", "drive the bead wave plan".

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` file for this skill.
Then read local files in `references/` and `scripts/` when needed.


<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->
<!-- Generated from skills-codex-overrides/catalog.json for crank. -->

## Codex Execution Profile

1. In Codex hookless mode, run `ao codex ensure-start` before the first wave; the CLI records startup once per thread and skips duplicates automatically.
2. Use the current leaf owner for one write scope; use Codex subagents only for at least two explicitly admitted disjoint lanes.

## Guardrails

1. End after one wave and return targeted evidence to RPI. RPI may admit another unchanged wave without `$validate` or `$learn`; those run once only after the leaf is complete and frozen.

<!-- END AGENTOPS OPERATOR CONTRACT -->
