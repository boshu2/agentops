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
5. If `$crank` returns `<promise>PARTIAL</promise>`, treat it as one completed wave: run `$validate` on that wave, pass the immutable verdict to `$learn`, and let the lead orchestrator consume `plan_impact` before deciding whether to invoke `$crank` again.
6. Orchestrate phases directly in the current session; do not hand RPI orchestration to wrapper commands.
7. Record phase receipts in `.agents/rpi/execution-packet.json` and each phase summary so `$discovery`, `$crank`, `$validate`, and `$learn` delegation is auditable from disk.
8. For Nightly, evolve, or auto-prompt goals, inspect the last 14 days of Nightly PRs and scheduled Nightly runs before choosing the implementation slice.
9. Classify recurring evidence as code-driven, runtime-artifact-only, or corpus-state-bound; prefer a code-driven fix unless the user explicitly asked for corpus maintenance.
10. Route `br` unavailability, tag push failures, worktree-disposition friction, and security/eval advisory recurrence as prompt/runtime debt rather than treating them as background noise.
11. Enforce `Validate -> Learn -> orchestrator`. Only the lead orchestrator may invoke `$discovery` to change the remaining plan and then `$premortem` on that exact changed plan.
12. claim, release, and consume semantics exactly
13. claim before work, consume on success, release on failure or interruption

## Guardrails

1. Do not silently loop after a partial wave. Preserve the objective, route the wave through Validate and Learn, and make the next orchestrator decision explicit. Do not count runtime-only artifact flips or corpus-state flywheel movement as successful code improvement without a tracked source change or explicit operator request. Do not invoke Dream/overnight from RPI; use Dream evidence only as input, and keep code-mutating work in the RPI lifecycle.

<!-- END AGENTOPS OPERATOR CONTRACT -->
