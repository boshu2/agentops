# Finding Compiler Contract

This contract defines the current promotion ladder that turns normalized findings into preventive outputs. The design goal is simple: discover a reusable failure once, normalize it once, then consume it earlier on the next cycle.

## Canonical Model

The prevention ladder has four layers:

1. `.agents/findings/registry.jsonl`
   The canonical intake ledger governed by [finding-registry.md](finding-registry.md).
2. `.agents/findings/<id>.md`
   The promoted finding artifact governed by [finding-artifact.schema.json](finding-artifact.schema.json).
3. `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md`
   Advisory outputs consumed by `/plan`, `/premortem`, and related judgment flows.
4. `.agents/constraints/index.json`
   Mechanical detectors that begin as warn-only shadows and become blocking only
   after precision-backed activation.

The registry is still the canonical intake ledger. Promotion and compilation are additive v2 layers, not a replacement data store.

## Canonical Inputs

- Registry entries are normalized JSONL records in `.agents/findings/registry.jsonl`.
- Promoted finding artifacts live under `.agents/findings/` as Markdown files with YAML frontmatter matching [finding-artifact.schema.json](finding-artifact.schema.json).
- Cross-repo reuse stays file-native. Search, seed, or pull may copy or materialize files, but the contract does not assume a service.

## Promotion Rules

1. Read the registry ledger.
2. Merge or select the canonical entry by `dedup_key`.
3. Promote that entry into `.agents/findings/<id>.md` when it is reusable enough to survive beyond the session-local JSONL row.
4. Compile the promoted artifact into advisory outputs first, then mechanical outputs when detector metadata permits, according to `compiler_targets`.

Promotion must preserve the reusable prevention content:

- `pattern`
- `detection_question`
- `checklist_item`
- applicability hints (`applicable_when`, `applicable_languages`)
- lifecycle state (`status`, `superseded_by`, `ttl_days`, `confidence`)

## Compiler Targets

| Target | Output path | Purpose |
|--------|-------------|---------|
| `plan` | `.agents/planning-rules/<id>.md` | Prevent known-bad decomposition or sequencing during planning |
| `premortem` | `.agents/pre-mortem-checks/<id>.md` | Surface prior failure modes during plan/spec validation |
| `constraint` | `.agents/constraints/index.json` | Observe mechanically detectable rules in shadow, then enforce only after measured activation |

`premortem` and `premortem` remain accepted input aliases during migration,
but writers emit `premortem`. Advisory findings may compile to `plan` and
`premortem`. A mechanical finding compiles to `constraint` only when its regex
matches every stored positive and passes every explicit negative control.

## Constraint Index Contract

.agents/constraints/index.json is the canonical executable surface.

The Go gate reads only the index. It never sources or executes per-constraint
scripts.

Each index entry must retain:

- `id`
- `finding_id`
- `title`
- `status` (`shadow`, `active`, `retired`)
- `enforcement_mode` (`warn` for shadow, `block` for active)
- `source_artifact`
- `review_file`
- `compiled_at`
- `applies_to`
- `detector`
- `evidence` (positive references, negative-control references, and optional
  shadow precision measurement)

Illustrative shape:

```json
{
  "id": "f-2026-03-09-001",
  "finding_id": "f-2026-03-09-001",
  "title": "Preserve issue type in TaskCreate metadata",
  "status": "shadow",
  "enforcement_mode": "warn",
  "source_artifact": ".agents/findings/f-2026-03-09-001.md",
  "review_file": ".agents/constraints/f-2026-03-09-001.sh",
  "compiled_at": "2026-03-09T20:15:00Z",
  "applies_to": {
    "scope": "files",
    "issue_types": ["feature", "bug", "task"],
    "path_globs": ["skills/*.md", "hooks/*.sh"],
    "languages": ["markdown", "shell"]
  },
  "detector": {
    "kind": "regex",
    "mode": "match",
    "pattern": "issue_type"
  },
  "evidence": {
    "positive_refs": ["tests/fixtures/issue-type-positive.md"],
    "negative_control_refs": ["tests/fixtures/issue-type-negative.md"]
  }
}
```

## Detector Precision (authoring a sound regex gate)

A `regex` detector is matched against **whole-file text** — it cannot tell code
from a comment, string, or docstring. Compilation requires at least one stored
positive and one explicit negative control; every positive must match and every
negative must pass. The resulting constraint is `shadow` + `warn`, never blocking.
Promotion to `active` + `block` (via the retired `constraint activate` verb)
additionally required cited shadow measurements with at least 95% precision;
since the Cathedral Cut, accepted rules are encoded directly in
repository-owned checks instead.

- **Anchor over substring.** A bare substring has an unbounded false-positive tail. Line-anchor with
  `(?m)^...` and use `[ \t]` (space/tab) rather than `[[:space:]]` (which includes `\n` and can match
  across lines onto a valid separate-line form). Match a path/identifier *segment*, not a fragment that
  also appears mid-word (e.g. `gate` matches `aggregate`).
- **Some anti-patterns are NOT regex-gatable.** A pattern that *commonly* appears in strings/comments
  (e.g. Python bare `except:` inside a docstring code example) cannot be distinguished from real code
  without an AST. Do **not** ship it as a blocking regex constraint — it will false-positive. Leave it
  advisory, or implement a language-aware check.
- **Scope to code with a SUPPORTED glob.** Use `**/*.go` / `**/*.sh` / `**/*.py` (the gate accepts
  `base/**`, `**/*.ext`, `*.ext`, exact, and a single-`*` segment — `cli/**/*.go` is REJECTED and
  fails closed). Code-only globs also keep the doc that *documents* the footgun (a `.md`) out of scope.
- **Measure before blocking.** Record samples, true positives, false positives,
  and an evidence reference. A detector without that evidence remains advisory.
- **Verify ZERO existing violations** with the exact regex before activating.
  Probe shadow/WARN, active/FAIL, and clean/PASS on the built binary.

## Applicability Inputs

Constraint applicability is resolved from concrete repository files. The Go
`constraints.enforce` check evaluates changed files in fast mode and enumerates
repository files in full mode, then applies `applies_to.path_globs`. Shadow
detector hits or evaluation errors WARN. Active detector hits or evaluation
errors FAIL closed.

## Supported Detector Kinds

The compiler currently recognizes `regex` only. Broader detector kinds are out
of scope until their positive/negative replay and shadow evaluation semantics are
defined explicitly.

## Review metadata

`review_file` and `file` may retain `.agents/constraints/<id>.sh` as compatibility
metadata for older review tooling. The compiler does not execute that path and
the gate does not depend on it.

## Lifecycle

Finding artifact lifecycle:

- `draft`
- `active`
- `retired`
- `superseded`

Constraint index lifecycle:

- `shadow`
- `active`
- `retired`

Rules:

- `retired` or `superseded` findings must not leave active downstream outputs behind.
- `superseded` findings should point to their replacement via `superseded_by`.
- Promotion accepted only precision-backed warn-only shadows; retirement
  accepted only active entries (the retired `constraint activate` /
  `constraint retire` verbs; policy now lives in repository-owned checks).

## Atomicity and Locking

Compiler writes to `.agents/constraints/` must use temp-file-plus-rename semantics.

If a lock is used, the canonical lock path is:

- `.agents/constraints/compile.lock`

Any CLI path that mutates `.agents/constraints/index.json` must follow the same
lock and atomic-write contract so compiler and lifecycle operations do not race.

## Backward Compatibility

- The registry JSONL line shape remains canonical `version: 1`.
- Older readers may continue to consume `registry.jsonl` directly.
- New promoted artifacts and compiled outputs are additive layers.
- Narrative docs must not claim active enforcement unless the Go gate actually
  reads active entries from `.agents/constraints/index.json`.
