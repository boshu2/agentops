# ADR-0006: Re-Steer Policy and Mutation-Safety Contract

- **Status:** Accepted (2026-05-17)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0003](ADR-0003-executable-spec-artifact-durability.md), [ADR-0005](ADR-0005-trace-link-convention.md)
- **Tracking:** epic `soc-58nt` (Executable spec layer), bead `soc-58nt.5.9`

## Context

Epic `soc-58nt` F5 adds auto re-steer: `ao goals measure` accumulates per-directive
`scenario_verdict` outcomes over successive runs, and when the evidence warrants it,
proposes (or, under an explicit opt-in policy, applies) mutations to GOALS.md directives
— re-ordering priority, tightening or loosening `**Setpoint:**`, or (with an additional
opt-in) reversing `**Steer:**` direction.

Before any F5.1–F5.5 implementation starts, this design spike records the definitional
contract that all F5 code implements against. Without a precise shared definition of
"iteration", "failure streak", and "cooldown", each implementation bead would embed
different assumptions, producing contradictory ledger accounting and unpredictable
mutation triggers.

The single most dangerous class of mutation F5 can make is a **Steer-direction flip**:
inverting `increase` ↔ `decrease`. A failing directive does not prove the optimization
direction is wrong — it may prove that the scenario suite, the implementation, or the
measurement window is inadequate. Auto-flipping Steer on evidence as noisy as a few
consecutive failures would systematically corrupt the strategic intent encoded in
GOALS.md. This ADR treats Steer-direction flips as a hard safety invariant that requires
explicit human opt-in even when the general `auto_apply` gate is open.

All GOALS.md mutations produced by F5 route through the non-lossy
`cli/internal/goals/patcher.go` patcher (F1.0 / `soc-58nt.1.0`), never through
`RenderGoalsMD` / `WriteMDGoals`, which would silently drop file sections.

## Definitions

### ITERATION

> **Definition.** An *iteration* is exactly one completed invocation of
> `ao goals measure` (or `ao goals measure --scenarios-only`) that terminates
> without a structural error and writes at least one `scenario_verdict` field to
> the verdict ledger for the directive in question.
>
> An iteration is **not** defined by wall-clock time, calendar period, sprint
> boundary, or RPI phase. It is a discrete ledger event: one call → one record →
> one verdict per directive.

Consequences:

- Two rapid back-to-back `ao goals measure` calls are two iterations, not one.
- A call that exits non-zero due to a structural error (malformed GOALS.md,
  unloadable scenario results) does NOT constitute an iteration; no ledger record
  is written, no streak is advanced.
- A call that exits 0 with `scenario_verdict: "unknown"` (no scenarios linked yet)
  DOES constitute an iteration; it counts as neither pass nor fail and does not
  advance a failure streak.
- The iteration count for a directive is the number of ledger records for that
  directive's stable `d-` ID.

The F5.1 verdict-ledger extension records each iteration as an append-only entry
carrying: `directive_id`, `run_timestamp` (RFC 3339), `scenario_verdict`
(`"pass"` | `"fail"` | `"unknown"`), `scenario_satisfaction` (float 0–1),
`scenario_count` (int), and `evaluated_count` (int).

### FAILURE STREAK

> **Definition.** A *failure streak* for a directive is the length of the longest
> unbroken run of consecutive iterations, ending at the most recent iteration,
> in which the directive's `scenario_verdict` was `"fail"`.
>
> The streak resets to 0 the moment any iteration yields `"pass"` or `"unknown"`.

The streak counter increments only on `"fail"` and resets on any non-`"fail"` verdict
(`"pass"` or `"unknown"`). `"unknown"` (no scenarios linked) is not a failure and
therefore breaks any in-progress streak.

Policy parameter: `failure_streak_length` (integer, default 3). A directive is
eligible for mutation proposal only when its current failure streak meets or exceeds
this value.

### COOLDOWN

> **Definition.** A directive is in *cooldown* if a re-steer mutation (proposed or
> applied) was recorded against it within the last K iterations, where K is the
> policy parameter `cooldown_iterations` (integer, default 5).
>
> "Last K iterations" is measured as the K most recent iteration records for that
> directive in the verdict ledger, not K calendar days or K wall-clock hours.

A directive in cooldown is skipped by the re-steer engine regardless of its current
failure streak. This prevents thrashing: a directive that was just re-steered is
given K iterations to demonstrate whether the mutation had effect before being
eligible again.

A cooldown record is written when:
- A mutation is **proposed** (recommendation-only mode), OR
- A mutation is **applied** (auto-apply mode, requires human confirmation).

The cooldown clock starts from the iteration in which the proposal/application
occurred, not from the current iteration.

## Policy

The re-steer policy is read from a JSON file (path configurable; default
`docs/re-steer-policy.json`) that validates against
`schemas/re-steer-policy.v1.schema.json`. If no policy file is present, the
built-in safe defaults apply (see §Default policy).

### Policy fields

| Field | Type | Default | Description |
|---|---|---|---|
| `minimum_evidence_count` | integer ≥ 1 | 5 | Minimum number of iterations (ledger records) for a directive before it is eligible for any mutation proposal. Prevents premature decisions on sparse data. |
| `failure_streak_length` | integer ≥ 2 | 3 | Number of consecutive `"fail"` verdicts required before the directive is eligible for mutation proposal. **Must be ≥ 2** — a single fresh failure never triggers an applied mutation. |
| `cooldown_iterations` | integer ≥ 1 | 5 | Number of iterations a directive must "cool down" after a mutation proposal/application before it is eligible again. |
| `allowed_mutation_types` | array of enum | `["priority_bump", "setpoint_tighten", "setpoint_loosen"]` | Which mutation types the policy permits. `steer_flip` is intentionally absent from the default and requires explicit opt-in. |
| `max_priority_bump` | integer ≥ 1 | 3 | Maximum number of positions a directive may be moved up in priority in a single re-steer event. Prevents a single failing directive from monopolizing the top of GOALS.md. |
| `auto_apply` | boolean | `false` | When `false` (default), mutations are recommendations only — printed to stdout, written to a proposal file, but GOALS.md is not modified. When `true`, mutations are applied to GOALS.md via the non-lossy patcher after the operator confirms interactively. |
| `allow_steer_flip` | boolean | `false` | When `false` (default, **hard safety invariant**), the `steer_flip` mutation type is never performed regardless of `allowed_mutation_types`. When `true`, `steer_flip` must also appear in `allowed_mutation_types` for it to take effect. Requires both flags to prevent accidental activation. |

### Mutation types (enum values for `allowed_mutation_types`)

| Value | Effect | GOALS.md attribute modified |
|---|---|---|
| `priority_bump` | Move directive up by at most `max_priority_bump` positions (calls `ao goals steer prioritize` equivalent via patcher) | Display number (reorder) |
| `setpoint_tighten` | Make the `**Setpoint:**` target harder (direction inferred from `**Steer:**`) | `**Setpoint:**` |
| `setpoint_loosen` | Make the `**Setpoint:**` target easier | `**Setpoint:**` |
| `steer_flip` | Invert `**Steer:**` direction (`increase` ↔ `decrease`). **Gated by `allow_steer_flip: true` AND explicit opt-in in `allowed_mutation_types`.** | `**Steer:**` |

## Hard Safety Invariants

The following invariants are checked by the F5.2 policy loader before any mutation
is proposed or applied. Violation causes the re-steer run to abort with a non-zero
exit code and a diagnostic message; GOALS.md is never touched.

### I-1: Single fresh failure never triggers an applied mutation

`failure_streak_length` **must be ≥ 2**. The schema enforces `minimum: 2` on this
field. A policy file with `failure_streak_length: 1` is **schema-invalid** and is
rejected at load time. This invariant cannot be overridden by any flag.

Rationale: a single `"fail"` can result from flaky scenario evaluation, a
transient CI environment issue, or a scenario that has never been run. It is
categorically insufficient evidence to modify a strategic directive.

### I-2: Default behavior is recommendation-only

`auto_apply` defaults to `false`. Unless the operator explicitly sets
`auto_apply: true` in the policy file **and** confirms interactively at the prompt
F5.4 presents, GOALS.md is never modified by the re-steer engine. The prompt is
non-bypassable in interactive mode; in `--non-interactive` mode with `auto_apply:
true`, the confirmation is skipped and a machine-readable log record is written
instead (F5.4 implementation contract).

### I-3: Steer-direction flip requires dual explicit opt-in

Even when `auto_apply: true`, the re-steer engine **never flips** `**Steer:**`
unless BOTH conditions hold:

1. `allow_steer_flip: true` in the policy file, AND
2. `"steer_flip"` is present in `allowed_mutation_types`.

Requiring both fields prevents accidental activation: a copy-paste error that sets
one but not the other leaves the invariant intact. This invariant is checked by the
F5.2 policy loader on every run, not only when a streak threshold is reached.

Rationale: a failing directive proves the implementation is not satisfying its
scenarios. It does NOT prove the optimization direction encoded in `**Steer:**` is
inverted. Steer encodes strategic intent ("we want to increase X") — that intent
may be correct even when current implementation falls short. Auto-inverting it on
noisy iteration data would corrupt the strategic model that GOALS.md represents.

### I-4: Minimum evidence before any mutation

No mutation is proposed or applied for a directive unless its verdict-ledger record
count is ≥ `minimum_evidence_count`. The default (5) means a directive must have
been measured at least 5 times. This prevents mutations on newly-added directives
that have only one or two data points.

### I-5: Cooldown is enforced regardless of streak

A directive in cooldown (per §COOLDOWN definition) is skipped entirely. Its failure
streak may meet the threshold, but the cooldown gate takes precedence. This prevents
a re-steer event from repeating before the previous mutation has had time to
influence the metric.

## Default Policy

The default policy, stored at `docs/re-steer-policy.default.json` and usable as a
starting point by copying to `docs/re-steer-policy.json`, encodes the safe
defaults:

```json
{
  "$schema": "https://agentops.dev/schemas/re-steer-policy.v1.schema.json",
  "minimum_evidence_count": 5,
  "failure_streak_length": 3,
  "cooldown_iterations": 5,
  "allowed_mutation_types": ["priority_bump", "setpoint_tighten", "setpoint_loosen"],
  "max_priority_bump": 3,
  "auto_apply": false,
  "allow_steer_flip": false
}
```

Key properties of the default:
- `auto_apply: false` → no GOALS.md modifications without operator action.
- `allow_steer_flip: false` → Steer direction is immutable by automation.
- `failure_streak_length: 3` → three consecutive failures required (satisfies I-1).
- `minimum_evidence_count: 5` → at least 5 measured iterations before eligibility.
- `steer_flip` absent from `allowed_mutation_types` → even if `allow_steer_flip`
  were set to `true`, no flip would occur without also adding it here.

## Relationship to Existing Surfaces

### F1.0 patcher (`cli/internal/goals/patcher.go`)

All GOALS.md mutations produced by F5 MUST route through `GoalsPatcher.SetAttribute`
and the reorder logic exposed by the patcher. Never call `RenderGoalsMD` /
`WriteMDGoals` (lossy). This is the same invariant established for F1–F4 and is
re-stated here as a load-bearing constraint for F5 implementers.

### F2 `ao goals measure` scenario verdicts

The `scenario_verdict` field in `directiveScenarioReport`
(`cli/cmd/ao/goals_measure_scenarios.go`) is the source of truth for each
iteration's pass/fail outcome. F5.1 reads this field from the measure output (or
from the verdict ledger it writes) to advance streak counters. F5 does NOT
re-evaluate scenarios independently.

### F5.3 `ao goals steer` extension

F5.3 adds a `ao goals steer re-steer` subcommand (or `ao goals re-steer`; exact
shape is an F5.3 decision). It reads the policy file, evaluates eligibility per the
rules above, and either prints recommendations or applies mutations after
confirmation. It is additive to the existing `steer add`, `steer remove`, and
`steer prioritize` subcommands; it does not modify their behavior.

## Consequences

### Positive

- F5.1–F5.5 implementation beads have a precise shared contract: "iteration",
  "failure streak", and "cooldown" are unambiguous.
- The dual opt-in requirement for Steer flips (I-3) makes the most dangerous
  mutation class resistant to both accidental policy misconfiguration and
  copy-paste errors.
- Recommendation-only default (I-2) means operators can deploy F5 immediately and
  observe proposals before opting into automation.
- The schema-enforced `minimum: 2` on `failure_streak_length` (I-1) makes the
  single-fresh-failure safety guarantee machine-checkable, not a convention.

### Negative

- The dual opt-in for Steer flips means legitimate cases where the Steer direction
  really is wrong require two config changes, not one. This is intentional friction.
- The minimum-evidence requirement (I-4, default 5 iterations) means newly-added
  directives are silently skipped for several measure cycles. Operators expecting
  immediate proposals on new directives may be confused; the re-steer output should
  explain why a directive was skipped.
- Cooldown is iteration-count-based, not time-based. In repos where `ao goals
  measure` runs many times per day, a cooldown of 5 iterations is very short. Teams
  with high-frequency measurement pipelines should raise `cooldown_iterations`.

## Acceptance

This ADR is accepted when:

- "Iteration", "failure streak", and "cooldown" are defined and documented. ✓
- The policy JSON has a schema (`schemas/re-steer-policy.v1.schema.json`). ✓
- A default policy instance validates against the schema. ✓
- The default policy disallows Steer-direction flips (`allow_steer_flip: false`,
  `steer_flip` absent from default `allowed_mutation_types`). ✓
- A single fresh failure never triggers an applied mutation (`failure_streak_length`
  schema minimum is 2). ✓
- All five hard safety invariants are stated and their enforcement mechanism is
  named. ✓
- ADR-0003 and ADR-0005 are cross-referenced. ✓

## References

- [ADR-0003](ADR-0003-executable-spec-artifact-durability.md) — artifact durability
  rule; schemas are tracked; the policy file at `docs/re-steer-policy.json` is tracked
- [ADR-0005](ADR-0005-trace-link-convention.md) — trace link convention; verdict
  ledger records are the iteration source of truth for the re-steer engine
- `schemas/re-steer-policy.v1.schema.json` — JSON Schema for the policy file
- `docs/re-steer-policy.default.json` — default policy instance
- `cli/internal/goals/patcher.go` — F1.0 non-lossy patcher; all F5 mutations route
  through it
- `cli/cmd/ao/goals_measure_scenarios.go` — `directiveScenarioReport`; defines
  `scenario_verdict` (the per-iteration pass/fail signal)
- `cli/cmd/ao/goals_steer.go` — existing directive-management surface F5.3 extends
- `cli/cmd/ao/rpi_ledger.go` — existing verdict ledger F5.1 extends
- Epic `soc-58nt`, bead `soc-58nt.5.9` — design spike that produced this ADR
