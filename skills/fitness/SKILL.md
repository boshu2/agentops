---
name: fitness
description: 'Measure declared project fitness goals without recommending or applying work. Triggers: "fitness", "check project fitness", "measure goals".'
practices:
- dora-metrics
- lean-startup
hexagonal_role: domain
consumes: []
produces:
- goal-measurement-report
- optional-goal-baseline-snapshot
- optional-rendered-goal-spec
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  intel_scope: topic
metadata:
  capabilities: [fitness]
  effects: [read_goals_source, read_goal_history_and_evidence, optionally_write_goal_snapshot, optionally_write_rendered_spec]
  canonical_status: canonical
  disposition: keep_specialist
  tier: product
  dependencies: []
output_contract: goal measurement report with observed reads/writes plus any fixed-path snapshot or explicitly approved rendered-spec path
---
# Fitness — goal measurement with declared derived writes

Inspect the active goals document and run only the caller-selected measurement,
validation, drift, history, export, or meta-goal command.

Renamed from `goals` (2026-07-29): the semantic skill is `fitness`; the
`ao goals` CLI command family is a separate product surface and keeps its
name. A thin `goals` compatibility alias resolves to this skill.

Measurement stays trustworthy only because it cannot mutate what it measures;
the moment a fitness report edits a goal, the next report measures the editor,
not the project.

Named failure mode — **advice creep**: a measurement report that ends with
"you should…" has silently become work selection.

Anti-pattern: padding the report with recommendations to look helpful.
Corrective: return the numbers, the evidence gaps, and checked/not-checked
scope, and let the caller decide.

## Constraints

- **Why goals remain authoritative.** The goals source is always read-only. Hash it before and after the command;
  any change is explicit failure, never a successful measurement.
- **Why callers need truthful effects.** Declare the selected subcommand's effects before execution. `validate`,
  `history`, `meta`, `scenarios`, and stdout-only `render` write nothing.
  `measure`, `drift`, and `export` may create/replace only their fixed derived
  snapshot below `.agents/ao/goals/baselines/`. `render --out` may write only
  the exact caller-approved derived path.
- A render target that is the goals source, a non-derived project file, a
  symlink, outside the declared target root, or already exists without explicit
  overwrite approval is rejected before writing. Snapshot/render writes use a
  same-directory temporary file, fsync, atomic rename, and post-write digest.
- Return honest effects: goals/history/evidence paths read, gate commands
  observed or executed, stdout emitted, directories/files created or replaced,
  and failed/unchecked writes. A command with no writes reports `writes: []`.

## Quality checks

- The pre/post goals-source digests match for every subcommand and mismatch is
  an explicit failure, never a measurement result.
- Each command's reported `reads`, `writes`, and `stdout` match the paths and
  stream effects observed in a disposable fixture.
- Render/snapshot writes use only their declared target, and forbidden,
  symlinked, or unapproved existing targets retain their original bytes.

## Boundary

- Prefer `GOALS.md` when both Markdown and legacy YAML exist.
- Preserve stable directive and gate identities in the report.
- Every measured gate must name its executable check and observed outcome.
- Do not add, remove, prioritize, recommend, apply, prune, migrate, or otherwise
  mutate goals.
- Do not translate a fitness gap into work selection or a next action.
- No subcommand edits the goals source through its own logic. `measure`,
  `drift`, and `export` persist a best-effort JSON snapshot under the fixed
  derived path `.agents/ao/goals/baselines/`. `render --out <file>` writes a
  Gherkin spec to whatever path the caller names — the CLI does not constrain
  it, so never point `--out` at the goals source or any non-derived file.

## Commands

All eight subcommands read the goals source without mutating it. Their transitive
effects are explicit here and apply identically through the `goals` alias.

| Subcommand | Reads | Writes |
|---|---|---|
| `measure` | goals + declared gate evidence | fixed baseline snapshot |
| `validate` | goals | none |
| `drift` | goals + prior snapshots | fixed baseline snapshot |
| `history` | goals + prior snapshots | none |
| `export` | goals + declared gate evidence | fixed baseline snapshot + stdout export |
| `meta` | goals | none |
| `scenarios` | goals | none |
| `render` | goals | stdout only, or one approved derived file with `--out` |

```bash
ao goals measure --json
ao goals validate --json
ao goals drift
ao goals history
ao goals export
ao goals meta --json
ao goals scenarios
ao goals render          # append --out <file> to write the spec instead of stdout
```

Run the requested command once. Return the command, exit code, goal-level
results, aggregate measurement, missing evidence, checked/not-checked scope,
and the observed `reads`, `writes`, and `stdout` effect lists. Verify the goals
source digest and every written artifact, serialize those facts as
`fitness-command-receipt.v1`, validate it with
`bash skills/fitness/scripts/validate-output.sh <receipt.json>`, then stop.
