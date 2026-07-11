---
name: goals
description: 'Maintain measurable project goals. Triggers: "$goals", "goal status", "maintain project goals".'
---
# $goals — Fitness Goal Maintenance

Maintain `GOALS.md` (canonical v4) as an executable fitness specification.
`GOALS.yaml` is legacy and survives only for migration through
`ao goals migrate`. Execute the selected workflow; do not merely describe it.

## Critical Constraints

- **Why: preserve fields.** Use the `ao goals` command surface; do not
  hand-render a whole goals file when a non-lossy command exists.
- **Why: keep one truth.** When both formats exist, `GOALS.md` wins; never
  silently treat legacy YAML as active.
- **Why: prove effect.** Measure before mutating, and preserve stable directive
  IDs and content unless the selected operation explicitly changes them.
- **Why: protect declared intent.** `recommend` is read-only. `apply` requires
  operator confirmation or explicit `--auto --yes` consent and an allowing policy.
- **Why: avoid false fitness.** Do not invent gates for infrastructure that
  does not exist; every gate needs an executable check and measurable outcome.
- **Why: preserve lineage.** Broken directive/scenario/bead/verdict/learning
  references are errors; do not paper over them in prose.
- **Why: prevent false completion.** Verify command exits and resulting files
  before claiming fitness or a successful mutation.

## Codex Execution Profile

1. Prefer compact status summaries with explicit failing goals, directive gaps, and file-backed updates.
2. Keep goal maintenance output ready to feed `$evolve`, `$status`, and future Codex sessions.
3. Run repository commands with native Codex and the local shell.
4. Inspect the active format and current measurement before proposing a write.

## Guardrails

1. Prefer measurable fitness language over subjective prose.
2. Do not mutate `GOALS.md` for a read-only measure, history, drift, trace, or recommendation request.
3. Do not auto-apply re-steering without explicit consent and an allowing policy.
4. Preserve stable IDs and non-lossy directive blocks.

## Mode Routing

| Intent | Command |
|---|---|
| measure/status (default) | `ao goals measure --json` |
| initialize | `ao goals init` |
| manage directives | `ao goals steer` |
| add a gate | `ao goals add` |
| compare snapshots | `ao goals drift` |
| inspect history | `ao goals history` |
| export snapshot | `ao goals export` |
| run meta-goals | `ao goals meta --json` |
| validate structure | `ao goals validate --json` |
| remove stale gates | `ao goals prune` |
| migrate formats | `ao goals migrate` |
| manage scenario links | `ao goals scenarios` |
| audit lineage | `ao goals trace` |
| export Gherkin | `ao goals render` |

## Core Workflow

1. Identify the active goals file and requested mode; default ambiguous requests
   to measurement.
2. Observe the relevant current state before any mutation.
3. Execute the exact `ao goals` command and capture exit status plus structured
   output where available.
4. For mutations, inspect the `GOALS.md` diff and run
   `ao goals validate --json`.
5. Re-measure or run the relevant graph/render proof against post-change state.
6. Return the output envelope below with failures and one next action.

## Measure Mode

```bash
ao goals measure --json
ao goals measure --directives
```

Extract each gate's status, weight, evidence, and overall fitness. Correlate
directives with recent commits and the repository's own tracker, then classify
each as `addressed`, `partially-addressed`, or `gap`. Do not infer progress from
titles alone.

Scenario satisfaction is part of fitness. A directive below its configured
ratio is RED. For a fast executable-spec check:

```bash
ao goals measure --scenarios-only -o json
```

## Mutation Rules

### Initialize

Run `ao goals init` (or `--non-interactive` only when requested), then enrich
from repository evidence:

- add at least one outcome-oriented north star;
- derive anti-stars from recurring verified failure modes when evidence exists;
- add product directives with direction and measurable targets;
- suggest product gates only for live infrastructure.

Use [generation-heuristics.md](references/generation-heuristics.md) for examples.

## Steer Mode

Measure first. Recommend removing completed directives, repairing chronic
failure, and covering measurable product gaps. Use non-lossy commands:

```bash
ao goals steer add "Title" --description="..." --steer=increase
ao goals steer remove 3
ao goals steer prioritize 2 1
ao goals steer recommend
```

Apply only under the consent constraints above; then validate and re-measure.

## Add and Migrate Modes

- `add`: supply a stable ID, executable check, weight, description, and type.
- `migrate`: preserve the original as a backup and validate the conversion.

## Prune Mode

Run `ao goals prune --dry-run` first. Remove only actually stale gates, then
validate and re-measure.

Schema details are in [goals-schema.md](references/goals-schema.md).

## Executable-Spec Operations

- `ao goals scenarios --lint` checks directive/scenario links.
- `ao goals trace --from <id>` renders lineage from a stable ID.
- `ao goals trace --orphans --strict` fails on warnings and broken references.
- `ao goals render --out spec.feature` exports linked scenarios as Gherkin.

Use stable directive IDs (`d-...`) as anchors, not display numbers.

## Output Specification

- **Path:** structured command output goes to `stdout`; mutations land in the
  active `GOALS.md`, and generated artifacts use the requested path.
- **Filename:** the filename convention is `GOALS.md`; exports and renders use
  the explicit `--out` filename supplied by the user.
- **Format:** the serialization/schema format is command JSON for structured
  results, Markdown v4 for goals, and Gherkin for rendered scenarios.
- **Validation command:** validate mutations with `ao goals validate --json`
  plus the relevant measure, trace, or render proof.
- **Downstream handoff:** `GOALS.md` is consumed by `$evolve` and `$status`;
  exported JSON or Gherkin goes to the requested CI/BDD consumer.

```text
Mode: <selected mode>
Source: <GOALS.md|GOALS.yaml|none>
Command: <exact command executed>
Result: <PASS|WARN|FAIL> — <exit/effect summary>
Fitness: <passing>/<total> (<percent>) or n/a
Directives: <addressed/partial/gap counts> or n/a
Evidence: <specific output, diff, snapshot, or artifact paths>
Next action: <single concrete action or none>
```

For measurement, list failed gates and RED directives with direct evidence. For
mutation, list exact changed IDs and post-change validation.

## Quality Rubric

- **Correct routing:** command matches the user's intent and active format.
- **Truthful fitness:** scores and statuses come from current command output.
- **Safe mutation:** baseline observed, diff narrow, consent honored.
- **Executable goals:** gates run and directives have measurable outcomes.
- **Verified result:** mutations pass validation plus relevant post-change proof.
- **Actionable report:** failures name direct evidence and one next action.

If a required item is missing, report `WARN` or `FAIL`; do not label the work
complete.

## References

- [operations.md](references/operations.md)
- [executable-spec-chain.md](references/executable-spec-chain.md)
- [generation-heuristics.md](references/generation-heuristics.md)
- [goals-schema.md](references/goals-schema.md)
- [goals.feature](references/goals.feature)
- `scripts/validate.sh`
