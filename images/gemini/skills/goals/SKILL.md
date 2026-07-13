---
name: goals
description: 'Maintain AgentOps goals. Triggers: "goals", "maintain agentops goals.", "goals skill".'
practices:
- dora-metrics
- lean-startup
- agile-manifesto
hexagonal_role: domain
consumes: []
produces:
- result.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: product
  dependencies: []
output_contract: GOALS.md
---
# /goals — Fitness Goal Maintenance

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
- **Why: preserve lineage.** Treat scenario, directive, bead, verdict, and
  learning links as evidence-bearing graph edges; broken references are errors.
- **Why: prevent false completion.** Report failures and partial mutations
  exactly; verify the command exit and resulting file before claiming success.

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

Use [operations.md](references/operations.md) for full flags, examples,
troubleshooting, and mode-specific procedures after routing.

## Core Workflow

1. Identify the active goals file and requested mode. If the request is
   ambiguous, default to measurement.
2. Run the read-only observation for that mode before any mutation. For steer,
   init enrichment, add, prune, migrate, or apply, show the relevant current
   state first.
3. Execute the exact `ao goals` command. Capture its exit status and structured
   output where available.
4. For mutations, inspect the resulting `GOALS.md` diff and run
   `ao goals validate --json`.
5. Re-measure or run the mode-specific proof so the result is grounded in the
   post-change state.
6. Emit the output specification below, including failures and next action.

## Measure Mode

Run:

```bash
ao goals measure --json
```

Extract each gate's status, weight, failure evidence, and overall fitness. For
`GOALS.md`, also assess directives:

```bash
ao goals measure --directives
```

Correlate directives with recent commits and the repository's own tracker.
Classify each as `addressed`, `partially-addressed`, or `gap`; do not infer
progress from titles alone.

Scenario satisfaction is part of fitness. A directive below its configured
ratio is RED. For a fast executable-spec check:

```bash
ao goals measure --scenarios-only -o json
```

The aggregation and exit-code contract is in
[executable-spec-chain.md](references/executable-spec-chain.md).

## Mutation Rules

### Initialize

Run `ao goals init` (or `--non-interactive` when explicitly requested), then
enrich only from repository evidence:

- add at least one outcome-oriented north star;
- derive anti-stars from recurring verified failure modes when evidence exists;
- add product directives with a direction and measurable target;
- suggest product gates only for live infrastructure.

Generation heuristics and examples live in
[generation-heuristics.md](references/generation-heuristics.md).

## Steer Mode

Measure first. Recommend removing completed directives, repairing chronic
failure, and covering measurable product gaps. Use non-lossy commands:

```bash
ao goals steer add "Title" --description="..." --steer=increase
ao goals steer remove 3
ao goals steer prioritize 2 1
ao goals steer recommend
```

Apply a recommendation only with the consent constraints above. Re-run measure
and validate after mutation.

## Add and Migrate Modes

- `add`: supply a stable ID, executable check, weight, description, and type.
- `migrate`: preserve the original as a backup and validate the converted file.

## Prune Mode

Run `ao goals prune --dry-run` first. Remove only gates whose referenced paths
are actually stale, then validate and re-measure.

Schema details for formats, weights, snapshots, and meta-goals are in
[goals-schema.md](references/goals-schema.md).

## Executable-Spec Operations

- `ao goals scenarios` lists and manages directive/scenario links; `--lint`
  checks the graph.
- `ao goals trace --from <id>` renders lineage from a stable directive,
  scenario, or bead ID.
- `ao goals trace --orphans --strict` fails on warnings as well as broken
  references.
- `ao goals render --out spec.feature` exports linked scenarios as Gherkin.

Use stable directive IDs (`d-...`) as anchors, not display numbers. The compact
behavioral contract is [goals.feature](references/goals.feature).

## Output Specification

Return a concise report with these fields:

- **Path:** structured command output goes to `stdout`; repository mutations
  land in the active `GOALS.md`, and generated artifacts use the requested path.
- **Filename:** the filename convention is `GOALS.md` for the canonical spec;
  exports and renders use the explicit `--out` filename supplied by the user.
- **Format:** the serialization/schema format is command JSON for structured
  results, Markdown v4 for goals, and Gherkin for rendered scenarios.
- **Validation command:** validate mutations with
  `ao goals validate --json`, plus the relevant measure, trace, or render proof.
- **Downstream handoff:** `GOALS.md` is consumed by the operating loop and
  `/evolve`; exported JSON or Gherkin is handed to the requested CI/BDD consumer.

```text
Mode: <measure|init|steer|add|drift|history|export|meta|validate|prune|migrate|scenarios|trace|render>
Source: <GOALS.md|GOALS.yaml|none>
Command: <exact command executed>
Result: <PASS|WARN|FAIL> — <exit/effect summary>
Fitness: <passing>/<total> (<percent>) or n/a
Directives: <addressed/partial/gap counts> or n/a
Evidence: <specific output, diff, snapshot, or artifact paths>
Next action: <single concrete action or none>
```

For measurement, list failed gates and RED directives with their direct
evidence. For mutation, list the exact changed goal/directive IDs and the
post-change validation result. JSON/export/render modes may return the artifact
plus this envelope; do not replace structured output with an unsupported prose
claim.

## Quality Rubric

A complete result satisfies all of the following:

- **Correct routing:** the command matches the user's intent and active format.
- **Truthful fitness:** every score and status comes from current command output.
- **Safe mutation:** current state was observed, the diff is narrow, and consent
  requirements were honored.
- **Executable goals:** gates have runnable checks; directives have measurable
  outcomes and healthy scenario links where applicable.
- **Verified result:** mutations pass `ao goals validate --json` and a relevant
  post-change measurement or graph check.
- **Actionable report:** failures name direct evidence and one concrete next
  action without hiding partial success.

If any required item is missing, report `WARN` or `FAIL`; do not label the work
complete.

## References

- [operations.md](references/operations.md) — detailed modes, examples, and troubleshooting
- [executable-spec-chain.md](references/executable-spec-chain.md) — scenario satisfaction, lineage, and re-steer policy
- [generation-heuristics.md](references/generation-heuristics.md) — goal and directive design patterns
- [goals-schema.md](references/goals-schema.md) — v1-v4 schemas and snapshot contract
- [goals.feature](references/goals.feature) — executable behavior examples
