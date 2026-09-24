---
package: cli/internal/goals
status: active
owner: agentopsd
contract_source: GOALS.md (operator-authored), GOALS.yaml (legacy), this package's GoalFile struct
---

# cli/internal/goals

GOALS.yaml / GOALS.md fitness specification subsystem: load, validate, measure, snapshot, and detect drift without mutating strategic intent.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Operator-facing artifact:** `GOALS.md` at repo root (with `GOALS.yaml` as the legacy format). Either is valid input; `goals.go` parses both into a unified `GoalFile`.
- **Skill surface:** consumed by `skills/reality-check/SKILL.md` as optional measurement context.

## Read the owner for the change

- **Formats and validation:** [goals.go](goals.go) owns `GoalFile` and accepted
  goal types; [markdown.go](markdown.go) parses Markdown and
  [render.go](render.go) renders it. Read [patcher.go](patcher.go) when inspecting
  directive attributes: despite its name, it is a read-only parsed view.
- **Measurement or drift:** start at [measure.go](measure.go) and
  [drift.go](drift.go); inspect both platform implementations for a new signal.
- **Stored observations:** [snapshot.go](snapshot.go) and [history.go](history.go)
  own snapshots and history.
- **CLI behavior:** [commands.go](commands.go) owns measurement and analysis
  handlers. Use `ao goals --help` for the current command inventory.

## Non-obvious rules

- **Two file formats, one struct.** `GoalFile` reads both `GOALS.yaml` (YAML, legacy) and `GOALS.md` (markdown with structured sections, current). The CLI does not migrate or rewrite either format.
- **Directives are GOALS.md-only.** `Directive` (numbered strategic intent) does not exist in YAML; it's a markdown-format-only feature. Don't add a YAML serialization without an explicit migration plan.
- **Continuous metrics need a threshold.** `ContinuousMetric` requires both `metric` and `threshold` — drift detection compares against the threshold, not against an absolute baseline.
- **Platform-gated measurement.** `measure_unix.go` and `measure_windows.go` are build-tagged. Adding a new measurement signal requires both implementations or a clean fallback.
- **Snapshots are observations.** Measurement may append snapshots, but never rewrites `GOALS.md` or routes subsequent work.
- **`measure --json` is part of the public CLI contract.** All `--json` flags
  must produce valid JSON; [the CLI JSON check](../../../tests/cli/test-json-flag-consistency.sh)
  is the executable contract.

## Completion

For format changes, exercise both Markdown and legacy YAML cases in the
package tests. For measurement changes, cover the affected signal and platform
fallback, and confirm measurement leaves strategic-intent files unchanged.
Run the relevant package tests from `cli/` with `go test ./internal/goals`, then
the checks required by the root contract. Passing measurements establish
observations, not authorization to change intent or route work.
