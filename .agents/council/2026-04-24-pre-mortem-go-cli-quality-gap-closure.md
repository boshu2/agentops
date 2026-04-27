---
id: pre-mortem-2026-04-24-go-cli-quality-gap-closure
type: pre-mortem
date: 2026-04-24
plan: "[[.agents/plans/2026-04-24-go-cli-quality-gap-closure.md]]"
research: "[[.agents/research/2026-04-24-go-cli-quality-gap-analysis.md]]"
epic: soc-n67
verdict: WARN
---

# Pre-Mortem: Go CLI Quality Gap Closure

## Verdict

WARN. The plan is good enough to implement after one contract hardening fix: Issue 2 must emit a machine-readable command-surface sidecar, and later JSON/completion gates must consume that sidecar instead of parsing a human Markdown table.

The fix has been applied to the plan and to beads `soc-n67.2` and `soc-n67.3`.

## Inputs Reviewed

- `.agents/plans/2026-04-24-go-cli-quality-gap-closure.md`
- `.agents/research/2026-04-24-go-cli-quality-gap-analysis.md`
- `.agents/research/2026-04-24-go-cli-local-code-explore.md`
- `.agents/rpi/ranked-packet-2026-04-24-go-cli-quality.md`
- `.agents/pre-mortem-checks/f-2026-04-14-001.md`
- `.agents/pre-mortem-checks/f-2026-04-14-002.md`
- `pre-mortem/references/council-fail-patterns.md`

## Checks

| Check | Result | Notes |
|-------|--------|-------|
| Mechanical enforcement | PASS | Each issue has command/test/file conformance checks. |
| External validation | PASS | Validation uses Go tests, generated-doc gates, shell gates, and CI-aligned scripts. |
| Paired command tests | PASS | Issues touching `cli/cmd/ao` production files require paired tests. |
| Durable handoff paths | PASS | Issues point at durable plan/research paths, not ephemeral context. |
| Dependency shape | PASS | Issue 3 and Issue 4 are blocked by Issue 2; Issue 5 is blocked by Issue 3. |
| Scope control | PASS | The plan pilots thin command migration instead of rewriting the CLI. |
| Data contract clarity | WARN | The original Issue 2 shape created a risk that gates would parse `docs/cli-surface.md`. |

## Findings

### WARN: Command-Surface Data Contract Was Too Human-Oriented

Issue 2 originally required `docs/cli-surface.md`, while Issue 3 and Issue 4 planned to consume the classification. That creates a likely implementation failure: JSON and completion gates could parse a prose Markdown table, making the gate brittle as soon as docs formatting changes.

Required fix:

```text
Issue 2 must produce docs/cli-surface.json as the script/test contract.
Issue 3 and Issue 4 must consume docs/cli-surface.json read-only.
docs/cli-surface.md remains the human rendering.
```

Applied remediation:

- Added `docs/cli-surface.json` to the plan's file list, implementation details, conformance checks, file dependency matrix, and conflict matrix.
- Updated Issue 2 acceptance so the sidecar must exist and be consumable by later gates.
- Updated Issue 3 to consume the sidecar instead of duplicating classification or parsing Markdown.
- Added notes and metadata updates to `soc-n67.2` and `soc-n67.3`.

### PASS: Baseline Failure Is Properly Isolated

The full Go suite failure is concrete and narrowly scoped to `TestGCBridgeVersion_Live`. Issue 1 has the right shape: deterministic parser tests plus live skip/compatibility behavior. This avoids normalizing a panic as an environmental flake.

### PASS: Output-Contract Tightening Is Sequenced Correctly

The plan classifies commands before making JSON consistency stricter. This avoids punishing intentionally stateful or args-required commands before the supported automation surface is explicit.

### PASS: Architecture Work Is Appropriately Bounded

The command-layer work is a two-command pilot (`contradict`, `notebook`) after output gates exist. That follows the repo learning to bridge and standardize existing behavior instead of cloning or rewriting broad systems.

### PASS: Complexity Policy Change Has a Guardrail

Issue 6 explicitly forbids lowering fail thresholds without a separate baseline and migration plan. That prevents a quality pass from turning into unrelated CI churn.

## Decision

Proceed with implementation under epic `soc-n67`.

Recommended first wave:

1. `soc-n67.1` - fix live `gc` bridge failure.
2. `soc-n67.2` - command surface taxonomy plus `docs/cli-surface.json`.
3. `soc-n67.6` - complexity policy and hot-file budget alignment.

Do not start `soc-n67.3` or `soc-n67.4` until `soc-n67.2` has landed and the sidecar contract is available.
