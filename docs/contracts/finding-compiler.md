# Finding Compiler Contract

This contract defines the current promotion ladder that turns normalized findings into preventive outputs. The design goal is simple: discover a reusable failure once, normalize it once, then consume it earlier on the next cycle.

## Canonical Model

The prevention ladder has four layers:

1. `.agents/findings/registry.jsonl`
   The canonical intake ledger governed by [finding-registry.md](finding-registry.md).
2. `.agents/findings/<id>.md`
   The promoted finding artifact governed by [finding-artifact.schema.json](finding-artifact.schema.json).
3. `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md`
   Advisory outputs consumed by `/plan`, `/pre-mortem`, and related judgment/runtime flows.
4. `.agents/constraints/index.json` plus `.agents/constraints/<id>.sh`
   Mechanical outputs for active declarative prevention during validation.

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
| `pre-mortem` | `.agents/pre-mortem-checks/<id>.md` | Surface prior failure modes during plan/spec validation |
| `constraint` | `.agents/constraints/index.json` plus `.agents/constraints/<id>.sh` | Enforce mechanically detectable rules during task validation |

Advisory findings may compile to `plan` and `pre-mortem`. Mechanical findings may compile to `constraint` only when detector metadata is present and valid.

## Constraint Index Contract

.agents/constraints/index.json is the canonical executable surface.

The runtime hook reads only the index. It does not source or execute `.agents/constraints/<id>.sh` directly.

Each index entry must retain:

- `id`
- `finding_id`
- `title`
- `status` (`draft`, `active`, `retired`)
- `source_artifact`
- `review_file`
- `compiled_at`
- `applies_to`
- `detector`

Illustrative shape:

```json
{
  "id": "f-2026-03-09-001",
  "finding_id": "f-2026-03-09-001",
  "title": "Preserve issue type in TaskCreate metadata",
  "status": "draft",
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
    "kind": "content_pattern",
    "mode": "must_contain",
    "pattern": "issue_type"
  }
}
```

## Applicability Inputs

Active constraint applicability is resolved from concrete runtime inputs, not abstract scope tags.

The runtime executor is `hooks/task-validation-gate.sh`. It reads active entries from `.agents/constraints/index.json`, then filters them with these inputs:

- `metadata.issue_type` from the task payload, matched against `applies_to.issue_types`.
- normalized target files from `metadata.files`.
- normalized validation files from `metadata.validation.files_exist`.
- normalized validation files from `metadata.validation.content_check[].file`.
- normalized changed files from staged, unstaged, and untracked git state.
- `applies_to.path_globs` and `applies_to.languages`, applied to the normalized target-file set.

If any active constraint declares `applies_to.issue_types`, task validation requires an issue type in the payload before evaluating the constraint set. This prevents issue-scoped prevention rules from silently guessing applicability.

## Supported Detector Kinds

The v2 compiler contract only recognizes detector kinds that fit the existing hook safety model:

- `content_pattern`
  Literal must-have or must-not-have pattern checks over normalized target files.
- `paired_files`
  Companion-file requirements derived from normalized changed files.
- `restricted_command`
  Allowlisted bare-name commands that remain subject to the hook sandbox.

Broader detector kinds are out of scope until a later contract revision expands the safety model explicitly.

## Companion `.sh` Files

`.agents/constraints/<id>.sh` is a human-reviewable companion artifact.

Its purposes are:

- explain the intent of the rule in shell-adjacent form
- preserve compatibility for older review tooling
- provide a migration stub for humans

It is not a runtime contract. Hooks must never execute it directly.

## Lifecycle

Finding artifact lifecycle:

- `draft`
- `active`
- `retired`
- `superseded`

Constraint index lifecycle:

- `draft`
- `active`
- `retired`

Rules:

- `retired` or `superseded` findings must not leave active downstream outputs behind.
- `superseded` findings should point to their replacement via `superseded_by`.
- `ao constraint activate` and `ao constraint retire` remain the lifecycle surface for constraint index entries.

## Atomicity and Locking

Compiler writes to `.agents/constraints/` must use temp-file-plus-rename semantics.

If a lock is used, the canonical lock path is:

- `.agents/constraints/compile.lock`

Any CLI or hook path that mutates `.agents/constraints/index.json` must follow the same lock and atomic-write contract so compiler and lifecycle operations do not race each other.

## Backward Compatibility

- The registry JSONL line shape remains canonical `version: 1`.
- Older readers may continue to consume `registry.jsonl` directly.
- New promoted artifacts and compiled outputs are additive layers.
- Narrative docs must not claim active enforcement unless the runtime hook actually reads active entries from `.agents/constraints/index.json`.
