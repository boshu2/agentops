# Swarm Evidence Contract

**Schema:** [`schemas/swarm-evidence.schema.json`](../../schemas/swarm-evidence.schema.json)
**Validator:** [`scripts/validate-swarm-evidence.sh`](../../scripts/validate-swarm-evidence.sh)

Swarm workers spawned via `/swarm` or `/crank` write one JSON result file per task to:

```
.agents/swarm/results/<task>.json
```

The orchestrator (`/crank`, the swarm skill, post-mortem) consumes these files to determine task outcomes, harvest evidence, and decide whether to advance an epic. This contract specifies the minimum well-formed shape every result file must satisfy.

## Required Fields

| Field | Type | Notes |
|-------|------|-------|
| `task` *(or `task_id`)* | string | Task identifier — issue ID, epic-task slug, or short slug. At least one form must be present. |
| `status` | string enum | One of: `done`, `pass`, `partial`, `failed`, `fail`, `blocked`, `not-applicable`, `already-implemented`, `research-only`, `skipped`. |

## Optional Fields

| Field | Type | Notes |
|-------|------|-------|
| `type` | string | `completion`, `blocked`, `research`, `preflight`. Used by `/crank` for routing. |
| `files_changed` | string[] | Repo-relative paths created or modified. |
| `files_created` | string[] | Subset of `files_changed` that are newly created. |
| `tests` | string \| object | Either `pass`/`fail`/`skipped`/`n/a`, or a structured object. |
| `gate` | string | Pre-push gate outcome: `pass`/`fail`/`skipped`/`n/a`. |
| `before_failures` | number | Failure count baseline (for refactors). |
| `after_failures` | number | Failure count after change. |
| `evidence` | string \| object | Freeform string OR structured object with `required_checks` (array) and `checks` (object map of `check_name` -> `{verdict: PASS\|FAIL\|SKIP, ...}`). Required when `type=completion`. |
| `artifacts` | string[] | Paths to plans, reports, or other artifacts. |
| `notes` | string | Freeform narrative. |
| `summary` | string | One-line outcome summary. |

The schema sets `additionalProperties: true` because workers regularly add task-specific extension fields (e.g. `helpers_extracted`, `before_complexity`). Don't fight that — historical results are evidence.

## Example: minimal

```json
{
  "task": "fix-broken-link",
  "status": "done",
  "files_changed": ["docs/INDEX.md"],
  "notes": "Replaced dead link to old contracts page."
}
```

## Example: completion with structured evidence

```json
{
  "task": "swarm-evidence-schema",
  "type": "completion",
  "status": "done",
  "files_changed": [
    "schemas/swarm-evidence.schema.json",
    "scripts/validate-swarm-evidence.sh",
    "docs/contracts/swarm-evidence.md"
  ],
  "evidence": {
    "required_checks": ["jq-shape", "ajv-optional"],
    "checks": {
      "jq-shape": {"verdict": "PASS", "details": "All required fields present"},
      "ajv-optional": {"verdict": "PASS", "details": "ajv not installed; warning only"}
    }
  },
  "notes": "Schema enforced via jq fallback; ajv optional."
}
```

## Validation

```bash
# Single file
bash scripts/validate-swarm-evidence.sh .agents/swarm/results/task-foo.json

# Whole directory
bash scripts/validate-swarm-evidence.sh .agents/swarm/results/

# Default (scans .agents/swarm/results/ if present)
bash scripts/validate-swarm-evidence.sh
```

The validator:

1. Confirms each file is valid JSON.
2. Enforces `task`-or-`task_id` and the `status` enum.
3. Optionally invokes `ajv` for full schema validation when available (warning, not error, on missing tool — keeps the gate runnable on stock workstations).
4. For files with `type=completion`, additionally requires an `evidence` block with `required_checks` and per-check `verdict` fields, with `PASS` verdicts for all required checks.

Exit codes: `0` = pass, `1` = at least one file failed validation. The validator is wired into the pre-push gate so violations block pushes.

## Backwards compatibility

Schema field names accept both new and legacy conventions:

- `task` (current) and `task_id` (legacy `1.json`-style results)
- `status` accepts `done` (current) and `pass` (legacy)
- `tests` and `gate` accept both `skipped` and `skip`

Existing historical evidence files MUST continue to validate — the schema was tuned against every file in `.agents/swarm/results/` to preserve their value as historical evidence. If a future result file format introduces a new convention, extend the schema, not delete the old files.
