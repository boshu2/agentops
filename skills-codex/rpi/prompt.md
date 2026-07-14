# rpi

Run Discovery, Crank, Validate, and Learn as four ordered, independently receipted umbrellas. Triggers: "run rpi", "research-plan-implement one turn", "drive a turn through the operating loop".

## Instructions

Load and follow the skill instructions from the sibling `SKILL.md` file for this skill.
Then read local files in `references/` and `scripts/` when needed.


<!-- BEGIN AGENTOPS OPERATOR CONTRACT -->
<!-- Generated from skills-codex-overrides/catalog.json for rpi. -->

## Codex Execution Profile

1. In Codex hookless mode, run `ao codex ensure-start` before phase orchestration; the CLI records startup once per thread and skips duplicates automatically.
2. When beads are present, resolve bead IDs before routing; when beads are absent, preserve the current goal or execution-packet objective across phases.
3. Keep a single lifecycle objective spine across discovery, crank, validation, and learning. Never replace it with a child issue ID or one ready slice from `br ready`, `br show`, or `.agents/rpi/next-work.jsonl`.
4. If discovery does not yield an epic id, invoke `$crank .agents/rpi/execution-packet.json` and standalone `$validate` instead of inventing one.
5. If `$crank` returns `<promise>PARTIAL</promise>`, compare the remaining work with the bound plan. Admit another unchanged wave below the three-wave and 90-minute boundary; at an incomplete soft boundary persist resume evidence and stop without `$validate`, `$learn`, or delivery.
6. Orchestrate phases directly in the current session; do not hand RPI orchestration to wrapper commands.
7. Keep one ordered receipt index in `.agents/rpi/execution-packet.json` that points to canonical Discovery, Crank, Validate, and Learn artifacts; legacy phase summaries are optional link-only projections.
8. Enforce `Validate -> Learn -> orchestrator`. Only the lead orchestrator may invoke `$discovery` to change the remaining plan and then `$premortem` on that exact changed plan.
9. claim, release, and consume semantics exactly
10. claim before work, consume on success, release on failure or interruption

## Guardrails

1. Do not silently loop after a partial wave. Preserve the objective and make the remaining-plan decision explicit; per-wave Validate, Learn, delivery, and duplicate summaries are forbidden.

<!-- END AGENTOPS OPERATOR CONTRACT -->
