---
package: cli/internal/goals
status: active
owner: agentopsd
contract_source: GOALS.md (operator-authored), GOALS.yaml (legacy), this package's GoalFile struct
---

# cli/internal/goals

GOALS.yaml / GOALS.md fitness specification subsystem: load, validate, measure, snapshot, and detect drift on the strategic goal layer.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Operator-facing artifact:** `GOALS.md` at repo root (with `GOALS.yaml` as the legacy format). Either is valid input; `goals.go` parses both into a unified `GoalFile`.
- **Skill surface:** consumed by `skills/goals/SKILL.md` and `/evolve` goal-fitness gates.

## Interfaces

- **Core types:** `Goal`, `GoalFile`, `Directive`, `ContinuousMetric`, `GoalType` (in `goals.go`). `GoalType` is one of `health`, `architecture`, `quality`, `meta`.
- **Top-level ops:**
  - `goals.go` — load + validate.
  - `measure.go` — fitness measurement (per-platform: `measure_unix.go`, `measure_windows.go`).
  - `markdown.go` — render/parse GOALS.md.
  - `template.go` — bootstrap a new GOALS file.
  - `commands.go` — CLI command wiring (subcommand handlers).
  - `drift.go` — detect when measured fitness drifts from the spec.
  - `history.go` — append/query the historical snapshot store.
  - `snapshot.go` — persist a measurement snapshot.
- **Subcommands the CLI exposes through this package:** `ao goals init`, `ao goals measure`, `ao goals measure --directives`, `ao goals steer add/remove/prioritize`, `ao goals migrate --to-md`.

## Non-obvious rules

- **Two file formats, one struct.** `GoalFile` round-trips both `GOALS.yaml` (YAML, legacy) and `GOALS.md` (markdown with structured sections, current). The `Format` field records which one was loaded; preserve it on save unless explicitly migrating.
- **Directives are GOALS.md-only.** `Directive` (numbered strategic intent) does not exist in YAML; it's a markdown-format-only feature. Don't add a YAML serialization without an explicit migration plan.
- **Continuous metrics need a threshold.** `ContinuousMetric` requires both `metric` and `threshold` — drift detection compares against the threshold, not against an absolute baseline.
- **Platform-gated measurement.** `measure_unix.go` and `measure_windows.go` are build-tagged. Adding a new measurement signal requires both implementations or a clean fallback.
- **Snapshots are append-only.** `history.go` writes to `.agents/goals/history/` (or equivalent); never edit historical snapshots, only append.
- **`measure --json` is part of the public CLI contract.** All `--json` flags must produce valid JSON (CI's `json-flag-consistency` job enforces this).

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Skill: `skills/goals/SKILL.md`.
- Operator docs: `GOALS.md` at repo root.
- Pattern source: olympus per-folder `AGENTS.md` ownership convention.
- Sibling packages: `cli/internal/overnight` (Dream consumes goal fitness), `cli/internal/quality` (metrics health overlap).
