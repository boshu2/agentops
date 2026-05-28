# ADR-0003: Executable-Spec Artifact Durability

- **Status:** Accepted (2026-05-17)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0002](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md)
- **Tracking:** epic `soc-58nt` (Executable spec layer), bead `soc-58nt.6`

## Context

Epic `soc-58nt` makes `GOALS.md` directives the executable BDD specification:
each directive links to behavioral scenarios, and the fitness gate measures
scenario satisfaction instead of code metrics. This only works if the
acceptance criteria are durable.

Today they are not:

- `GOALS.md` is git-tracked. The directive→scenario *link* survives.
- `ao scenario add` writes scenario JSON to `.agents/holdout/`.
- Repo-root `.agents/` is **never git-tracked** — `.gitignore` excludes
  `/.agents/` explicitly, and `AGENTS.md` declares it local/private runtime
  state. It was wiped by routine cleanup on 2026-05-07 (Directive 11 /
  `soc-rv5p`).

A directive that points at a scenario file living only in gitignored
`.agents/holdout/` has acceptance criteria that vanish on `git clean`. A spec
whose acceptance criteria can disappear is not a spec. F1/F3/F4/F5 of this
epic all assume both ends of the directive→scenario link are durable.

The holdout directory also serves a real, separate purpose: holdout scenarios
are deliberately **isolated from implementing agents** (the
`holdout-isolation-gate` hook blocks reads without
`AGENTOPS_HOLDOUT_EVALUATOR=1`). That isolation property must not be lost when
solving durability.

## Decision

Distinguish two scenario lifecycle states and give each a home.

### 1. `local_holdout` scenarios — stay in `.agents/holdout/`

`ao scenario add` continues to write to `.agents/holdout/`. These are ad hoc,
agent-isolated holdout scenarios used for evaluation. They are intentionally
untracked and intentionally unreadable by implementing agents. Nothing about
the holdout workflow changes.

### 2. `promoted_spec` scenarios — live in tracked `spec/scenarios/`

A scenario that is **linked to a `GOALS.md` directive** is a *promoted spec
scenario*. It is part of the executable specification and MUST be durable.

- **Tracked path:** `spec/scenarios/<scenario-id>.json`
  (`spec/` is not gitignored; these files are committed).
- Promotion **copies** the scenario JSON from `.agents/holdout/` (or creates
  it directly) into `spec/scenarios/`, and stamps the scenario with the stable
  `directive_id` of the directive it satisfies (the `directive_id` field is
  added to the scenario schema by F1.1 / `soc-58nt.1.1`).
- The `GOALS.md` directive's `Scenarios:` attribute line references the
  scenario by ID. Resolution searches `spec/scenarios/` first.
- A promoted spec scenario is **not** subject to holdout isolation — it is part
  of the published spec and readable by any agent. Promotion is the explicit
  act of moving a scenario from "evaluation holdout" to "published acceptance
  criterion."

`ao goals scenarios --create` (F1.3 / `soc-58nt.1.3`) writes directly to
`spec/scenarios/` — a scenario created already linked to a directive is born
promoted.

### 3. Domain manifests — tracked under `docs/domains/`

Domain-slice manifests (F3 / `soc-58nt.3.x`) are durable project artifacts, not
runtime state. They live at `docs/domains/<name>/manifest.yaml`, git-tracked.
Optional runtime mirrors may sit under `.agents/domains/`, but the source of
truth is tracked.

### 4. Schemas and fixtures — always tracked

All schemas (`schemas/**`, `docs/contracts/*.schema.json`) and all test
fixtures (`tests/**`) are git-tracked. No schema or fixture for the
executable-spec layer may live under untracked `.agents/`.

### Summary table

| Artifact | Path | Tracked | Agent-isolated |
|---|---|---|---|
| Ad hoc holdout scenario | `.agents/holdout/<id>.json` | no | yes |
| Promoted spec scenario | `spec/scenarios/<id>.json` | **yes** | no |
| Domain-slice manifest | `docs/domains/<name>/manifest.yaml` | **yes** | no |
| Scenario-result run artifact | `.agents/rpi/scenario-results.json` | no (ephemeral run output) | n/a |
| Schemas | `schemas/**`, `docs/contracts/*.schema.json` | **yes** | n/a |
| Fixtures | `tests/**` | **yes** | n/a |

Rule: the directive→scenario→domain link endpoints are all tracked. Only
ephemeral *run output* (e.g. `.agents/rpi/scenario-results.json`) stays under
untracked `.agents/`.

## Backfill / migration

Existing `.agents/holdout/` scenarios:

- Scenarios with **no** `directive_id` and not referenced by any `GOALS.md`
  `Scenarios:` line remain `local_holdout` — untouched.
- Scenarios that **are** referenced by a directive (once F1 lands the
  `Scenarios:` attribute) are promoted: copied to `spec/scenarios/<id>.json`,
  stamped with the directive's stable `directive_id`, and committed. The
  holdout copy may be left in place or removed by the operator; the tracked
  copy is authoritative.
- F1.4 link-lint (`soc-58nt.1.4`) treats a directive that references a scenario
  resolvable only under `.agents/holdout/` (not yet promoted) as a **warning**
  ("scenario not promoted to `spec/scenarios/`"), not an error, until backfill
  completes. After backfill the lint may be ratcheted to error.

No automated bulk migration ships with this ADR — promotion happens
incrementally as directives gain `Scenarios:` lines through F1.

## Consequences

### Positive

- The executable spec is durable: `git clean` / `.agents/` cleanup cannot
  destroy a directive's acceptance criteria.
- Holdout isolation is preserved for genuine evaluation scenarios.
- Promotion is an explicit, reviewable git event — a spec scenario entering the
  tracked tree shows up in PR diffs.
- F1/F3/F4/F5 can assume both link endpoints resolve to tracked files.

### Negative

- Two homes for scenario JSON means resolution must search both
  (`spec/scenarios/` then `.agents/holdout/`). F1's scenario reader owns this.
- Promotion is a copy, so a scenario can briefly exist in both places;
  `spec/scenarios/` is authoritative and lint flags drift.
- Operators must remember that linking a scenario to a directive implies
  promotion (and loss of holdout isolation for that scenario). Tooling makes
  this explicit; docs must state it.

## Acceptance

This ADR is accepted when:

- The tracked spec-scenario path (`spec/scenarios/`) and domain-manifest path
  (`docs/domains/`) are named and recorded here. ✓
- The `local_holdout` vs `promoted_spec` distinction is defined. ✓
- Backfill semantics for pre-existing `.agents/holdout/` scenarios are
  defined. ✓
- F1/F3/F4/F5 beads reference this ADR for artifact locations.

## References

- [ADR-0002](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md)
- `schemas/scenario.v1.schema.json` — scenario schema (gains `directive_id` in F1.1)
- Epic `soc-58nt` DESIGN field — full executable-spec revision plan
