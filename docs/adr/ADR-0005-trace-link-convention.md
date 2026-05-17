# ADR-0005: Trace Link Convention

- **Status:** Accepted (2026-05-17)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0003](ADR-0003-executable-spec-artifact-durability.md)
- **Tracking:** epic `soc-58nt` (Executable spec layer), bead `soc-58nt.4.8`

## Context

Epic `soc-58nt` builds a full directive→scenario→bead→artifact→learning trace chain
(`ao goals trace`, F4). Before the F4.1 walker can be implemented, the graph-edge
convention must be specified: what constitutes a valid link, how each edge is
discovered, what confidence level is assigned, and which defects are errors vs
warnings. This ADR is that specification.

The trace graph has six node types:

- **Directive** — a GOALS.md directive block with a stable `d-` ID.
- **Scenario** — a scenario JSON file (promoted spec at `spec/scenarios/<id>.json`
  or ad hoc holdout at `.agents/holdout/<id>.json`).
- **Bead** — a `bd` issue tracked in the Dolt remote.
- **Artifact** — an RPI run artifact produced by agent work (plan doc, verdict
  file, execution packet, etc.).
- **Learning** — a `docs/learnings/<date>-<slug>.md` file with YAML frontmatter.

ADR-0003 defines where each artifact lives; this ADR defines how they link to one
another and how the walker discovers and validates those links.

## Decision

### 1. Edge type catalogue

The F4.1 walker emits exactly six edge types. Each edge has a `confidence` field.

| Edge type | From | To | Discovery method | Confidence rule |
|---|---|---|---|---|
| `directive_has_scenario` | Directive | Scenario | `Scenarios:` attribute in GOALS.md, cross-checked against `directive_id` in scenario JSON | `high` when both sides match; `low` when only one side is present |
| `scenario_result` | Scenario | Artifact | Scenario ID token in artifact file path or frontmatter `scenario_id` field | `high` when exact ID match; `low` when free-text heuristic match |
| `scenario_claimed_by_bead` | Scenario | Bead | Explicit `Scenarios:` line in bead description or notes | `high` when exact scenario ID match; `low` when free-text heuristic |
| `bead_produced_artifact` | Bead | Artifact | Bead ID token in artifact file path or artifact frontmatter `bead_id` field | `high` when exact ID match; `low` when free-text heuristic |
| `artifact_cited_by_learning` | Artifact | Learning | Exact artifact file path in learning body or frontmatter `source` field | `high` when exact path match; `low` when free-text heuristic |
| `directive_has_learning` | Directive | Learning | `directive_id` token in learning frontmatter `directive_id` field or body | `high` when exact ID match; `low` when free-text heuristic |

### 2. Edge specifications

#### 2.1 `directive_has_scenario`

**Forward link (GOALS.md → scenario):** A directive declares linked scenarios via
the `Scenarios:` attribute line (parsed by the F1.0 patcher,
`cli/internal/goals/patcher.go`). The attribute value is a comma/semicolon-separated
list of scenario IDs matching pattern `^s-\d{4}-\d{2}-\d{2}-\d{3}$` or
`^auto-.+$`.

**Reverse link (scenario → directive):** A scenario JSON file may carry a
`directive_id` field (`^d-[a-z0-9][a-z0-9-]*$`) pointing back to the directive.
This field is defined in `schemas/scenario.v1.schema.json`.

**Confidence rules:**

- `confidence: high` — The directive's `Scenarios:` line lists scenario ID `S`,
  AND `spec/scenarios/S.json` (or `.agents/holdout/S.json`) exists with
  `directive_id` matching the directive's stable ID. Both endpoints are present
  and consistent.
- `confidence: low` — Only one side of the bidrectional link is present:
  - The directive's `Scenarios:` line lists `S` but the scenario file has no
    `directive_id`, or `directive_id` points to a different directive.
  - OR the scenario file has `directive_id = D` but no directive in GOALS.md
    declares `D` in its `Scenarios:` line.

**Resolution order:** `spec/scenarios/<id>.json` is checked first (promoted spec).
If absent, `.agents/holdout/<id>.json` is checked (ad hoc holdout). A scenario
resolvable only in `.agents/holdout/` triggers an additional lint warning
("scenario not promoted to `spec/scenarios/`") per ADR-0003 backfill semantics.

#### 2.2 `scenario_result`

A scenario result is an RPI run artifact (e.g. a verdict file, scenario-result
JSON, or RPI phase doc) that records the outcome of running a scenario.

**Discovery:** The walker checks artifact file paths and YAML frontmatter for an
exact scenario ID token. Accepted signals, in priority order:

1. Frontmatter field `scenario_id: <id>` — `confidence: high`.
2. Artifact file path contains `/scenario-results/<id>` or `/<id>/` —
   `confidence: high`.
3. Free-text occurrence of the scenario ID pattern in the artifact body —
   `confidence: low`.

#### 2.3 `scenario_claimed_by_bead`

`bd` has no native executable-spec field. A bead claims a scenario via an explicit
`Scenarios: <id>[, <id>...]` line in the bead's description or notes field. The
format mirrors the GOALS.md `Scenarios:` attribute: comma/semicolon-separated
scenario IDs.

**Discovery:** The walker reads bead descriptions and notes via `bd` query output.
An exact regex match against `Scenarios:\s+<scenario-id-pattern>` is
`confidence: high`. Occurrence of a scenario ID token elsewhere in bead text is
`confidence: low`.

**Claim semantics:** A bead claiming a scenario asserts "this bead's work
satisfies or contributes to the acceptance criterion expressed by that scenario."
Multiple beads may claim the same scenario; the walker records all edges. A
scenario with zero claiming beads is not an error (the work may be done outside
`bd`), but is surfaced as a warning under `--strict`.

#### 2.4 `bead_produced_artifact`

An artifact is linked to a bead when the bead that produced it is traceable.

**Discovery signals, in priority order:**

1. Artifact YAML/JSON frontmatter field `bead_id: <bead-id>` — `confidence: high`.
2. Artifact file path contains the bead ID pattern (e.g. `soc-58nt.4.8`) —
   `confidence: high`.
3. Free-text occurrence of a bead ID pattern (`[a-z]+-[a-z0-9]+\.[0-9]+(\.[0-9]+)*`)
   in the artifact body — `confidence: low`.

**Artifact scope:** RPI run artifacts live under `.agents/rpi/runs/<run-dir>/`
per ADR-0003 (ephemeral run output, not git-tracked). The walker scans this
directory tree. Promoted artifacts (plans committed to `docs/`) are also scanned
if present.

#### 2.5 `artifact_cited_by_learning`

A learning cites an artifact when the learning's `source` frontmatter field or
body contains the artifact's path.

**Discovery:**

1. Frontmatter `source:` field contains the artifact's relative path —
   `confidence: high`.
2. Artifact's file path appears verbatim in the learning body —
   `confidence: high`.
3. Artifact's filename (without path) appears in the learning body —
   `confidence: low`.

**Learning location:** Learnings live at `docs/learnings/<date>-<slug>.md` (git-
tracked). The walker scans this directory only.

#### 2.6 `directive_has_learning`

A learning is linked to a directive when the learning explicitly records the
directive ID.

**Discovery:**

1. Frontmatter field `directive_id: <id>` — `confidence: high`.
2. The directive's stable ID pattern (`d-[a-z0-9][a-z0-9-]*`) appears verbatim
   in the learning body — `confidence: high` if the ID is also a known directive
   in GOALS.md; `confidence: low` if the ID is unrecognized.
3. Free-text heuristic match (learning title or tags match directive title words) —
   `confidence: low`.

### 3. Confidence rule (global)

> **Exact ID matches are `confidence: high` and count as closure proof. Heuristic
> free-text matches are `confidence: low` and NEVER count as closure proof.**

"Closure proof" means that the walker treats the linked node as satisfying its
upstream acceptance criterion. An edge with `confidence: low` is surfaced in
walker output (so operators can review it) but never promotes a directive to
"closed" status, never marks a scenario as "satisfied", and never completes a
bead-to-scenario claim.

This rule is strict by design: if the tooling cannot make a deterministic claim,
neither can the trace report.

### 4. Defect classification

The F4.1 walker classifies link defects as **errors** or **warnings**.

#### 4.1 Errors (always surfaced)

A defect is an error when an **explicit link is broken** — one side of a declared
edge references an ID that does not resolve.

| Defect | Edge | Condition |
|---|---|---|
| `broken_scenario_ref` | `directive_has_scenario` | GOALS.md `Scenarios:` line lists scenario ID `S`, but no file at `spec/scenarios/S.json` or `.agents/holdout/S.json` exists |
| `broken_directive_backref` | `directive_has_scenario` | Scenario file has `directive_id: D`, but no directive in GOALS.md declares stable ID `D` |
| `broken_bead_scenario_claim` | `scenario_claimed_by_bead` | Bead notes contain `Scenarios: S` but scenario ID `S` does not resolve |
| `broken_artifact_bead_ref` | `bead_produced_artifact` | Artifact frontmatter declares `bead_id: B` but bead `B` does not exist in `bd` |
| `broken_learning_directive_ref` | `directive_has_learning` | Learning frontmatter declares `directive_id: D` but `D` is not a known directive |

#### 4.2 Warnings (surfaced by default; escalated to errors under `--strict`)

A defect is a warning when an **optional yield is absent** — no explicit link is
broken, but expected traceability is missing.

| Defect | Condition | `--strict` escalation |
|---|---|---|
| `scenario_not_promoted` | Directive's scenario resolves only in `.agents/holdout/`, not `spec/scenarios/` | No (stays warning; backfill is incremental per ADR-0003) |
| `directive_no_scenarios` | Directive has no `Scenarios:` attribute line | Yes — under `--strict` a directive with no scenarios is an error |
| `scenario_no_bead_claim` | Active scenario has no `scenario_claimed_by_bead` edge | Yes |
| `scenario_no_result` | Active scenario has no `scenario_result` edge | Yes |
| `bead_no_artifact` | Bead's claim has no `bead_produced_artifact` edge | No |
| `no_learning_yield` | Directive has no `directive_has_learning` edge and no scenario with an `artifact_cited_by_learning` edge | Yes |
| `low_confidence_only` | All edges for a closure path are `confidence: low` | Yes — under `--strict` a path with only low-confidence edges is treated as absent |

### 5. Walker output contract

The F4.1 walker emits a JSON stream (one object per line) with at minimum these
fields:

```json
{
  "edge_type": "directive_has_scenario",
  "from_id": "d-fitness-gate-bdd",
  "to_id": "s-2026-05-17-001",
  "confidence": "high",
  "defects": []
}
```

A defect entry within `defects` carries `code` (one of the codes from §4) and
`severity` (`error` | `warning`).

A summary record is emitted last:

```json
{
  "summary": true,
  "error_count": 0,
  "warning_count": 2,
  "low_confidence_edges": 1
}
```

The walker exits non-zero if `error_count > 0`, or if `warning_count > 0` and
`--strict` is set.

## Consequences

### Positive

- The full directive→scenario→bead→artifact→learning chain is defined before any
  walker code is written, eliminating ambiguity about what the walker should emit.
- The `confidence: high` / `confidence: low` distinction prevents heuristic noise
  from masquerading as closure proof.
- Error vs warning split allows operators to adopt the trace gate incrementally:
  run default (errors only) first, add `--strict` when traceability is mature.
- All six edge types follow the same schema, so the F4.1 walker can use a uniform
  edge-emit path regardless of which edge it is processing.

### Negative

- Beads must be annotated with `Scenarios: <id>` lines by the agent or operator
  — there is no native `bd` field for this today. This adds a manual discipline
  step that is not tooling-enforced at bead creation time.
- Learning frontmatter gains new optional fields (`directive_id`, `scenario_id`)
  that are not yet in a learning schema. If a learning schema is introduced later,
  these fields must be reconciled.
- The walker must query `bd` for bead content at scan time, adding an external
  process dependency. If `bd` is unavailable, `scenario_claimed_by_bead` and
  `bead_produced_artifact` edges cannot be discovered.

## Acceptance

This ADR is accepted when:

- All six edge types are named, their discovery methods are specified, and their
  confidence rules are stated. ✓
- The `confidence: high` / `confidence: low` closure-proof rule is stated. ✓
- Errors (broken explicit links) vs warnings (missing optional yield) are
  enumerated. ✓
- `--strict` escalation targets are defined. ✓
- The walker output contract (edge JSON + summary record) is specified. ✓
- The F4.1 walker bead can implement directly against this spec without further
  design work.

## References

- [ADR-0003](ADR-0003-executable-spec-artifact-durability.md) — artifact location
  conventions (promoted spec scenarios at `spec/scenarios/`, learnings at
  `docs/learnings/`, run artifacts under `.agents/rpi/runs/`)
- `cli/internal/goals/patcher.go` — F1.0 patcher; defines `AttrScenarios`,
  `splitScenarioList`, `ParsedDirective.Scenarios`, and `directiveIDRe`
- `schemas/scenario.v1.schema.json` — scenario schema; defines `directive_id`
  field and scenario ID pattern
- `cli/cmd/ao/scenario_add.go` — scenario ID format (`s-YYYY-MM-DD-NNN`)
- Epic `soc-58nt` DESIGN field — full executable-spec revision plan
