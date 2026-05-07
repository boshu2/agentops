# Per-Criterion Rubric

> Companion reference for `skills/validation/SKILL.md`. Schema source of truth: [`schemas/execution-packet.schema.json#/$defs/Criterion`](../../../schemas/execution-packet.schema.json).

## Per-criterion verdict report shape

Validation emits one row per criterion in the phase-3 summary. The table replaces the vibe-only verdict whenever the execution packet ships `acceptance_criteria` (epic and/or bead level).

| Criterion | Status | Evidence | Notes |
|---|---|---|---|
| ac-foo.1 | PASS | path/to/evidence | one-liner |
| ac-foo.2 | FAIL | (missing) | command exited 1 |

Columns:

- **Criterion** — the `id` (`^ac-<scope>\.<n>$`).
- **Status** — `PASS`, `FAIL`, `WARN`, or `SKIPPED` (the latter for `optional: true` criteria the operator chose not to run).
- **Evidence** — value of `evidence_path`, or `(missing)` when `evidence_required: true` and no artifact matched.
- **Notes** — one-liner with exit code, judge verdict, or operator note.

A row is FAIL when `evidence_required: true` and the `evidence_path` glob matches no artifact, regardless of `check_command` exit. Aggregate verdict uses a GOALS-style weighted average over `weight`; criteria with `optional: true` are non-blocking. Aggregate FAIL → phase-3 FAIL.

## `check_type` runner contract

Closed enum (seven values). Each maps to one operational shape:

- `test_pass` — run `check_command`; PASS on exit 0.
- `command_exit_zero` — run `check_command`; PASS on exit 0.
- `file_exists` — `test -f <evidence_path>` (if `evidence_path` is set) or extract the path from `check_command`; PASS if the file exists.
- `grep_match` — run the grep in `check_command`; PASS on exit 0.
- `manual` — requires operator review; `check_command` is informational only and does not gate the row.
- `council_judge` — invoke the named council/judge skill (e.g., `Skill(skill="council", args="...")`); PASS if the returned verdict is PASS.
- `custom_rubric` — requires the `agent_judge` field; invokes that named judge; PASS if the returned verdict is PASS.

`test_pass` and `command_exit_zero` share semantics; the split exists so plans can label intent (regression test vs. arbitrary command) for downstream tooling.

## Examples

### `command_exit_zero`

```yaml
- id: ac-bcrn.1.4
  description: "Validation SKILL.md documents per-criterion verdict and back-compat fallback"
  check_type: command_exit_zero
  check_command: "grep -F 'per-criterion' skills/validation/SKILL.md && grep -F 'back-compat' skills/validation/SKILL.md"
  evidence_path: "skills/validation/SKILL.md"
  evidence_required: true
  weight: 1.0
  optional: false
```

Runner: shell out `check_command`. Exit 0 → PASS. The evidence file must exist on disk because `evidence_required: true`.

### `custom_rubric`

```yaml
- id: ac-e4.3
  description: "Each principle and anti-pattern has a source citation (file:section)"
  check_type: custom_rubric
  check_command: "manual review during /vibe — no Wikipedia-style unsourced claims allowed"
  evidence_path: "skills/rpi/references/best-practices.md"
  evidence_required: false
  weight: 0.5
  optional: true
  agent_judge: "vibe"
```

Runner: invoke the judge named in `agent_judge` (here, `/vibe`) with the criterion description and evidence path. PASS if the judge returns PASS. The `agent_judge` field is required when `check_type == "custom_rubric"` (enforced by the schema's conditional `required`).

## Back-compat note

When the execution packet has no `acceptance_criteria` (or it's empty), validation falls back to vibe-only verdict and emits a `[deprecated]` WARN. The deprecation horizon is **2026-06-30**; after that, missing criteria become FAIL. See the "Back-compat fallback" section in `skills/validation/SKILL.md`.
