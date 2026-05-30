# Harvest Next Work — Implementation Details

## Schema Validation

The canonical machine-checkable contract is the committed JSON Schema pair
[../../../schemas/next-work-batch.v1.schema.json](../../../schemas/next-work-batch.v1.schema.json)
(one JSONL line = one batch entry) and
[../../../schemas/next-work-item.v1.schema.json](../../../schemas/next-work-item.v1.schema.json)
(each `items[]` element), both derived from
[../../../docs/contracts/next-work.schema.md](../../../docs/contracts/next-work.schema.md).
The reusable validator is
[../../../scripts/validate-next-work.sh](../../../scripts/validate-next-work.sh):

```bash
# Advisory (logs violations, exits 0); add --strict to reject and exit non-zero
# naming the offending file:line + field. --json emits a machine-readable verdict.
bash scripts/validate-next-work.sh --strict .agents/rpi/next-work.jsonl
```

The validator reads its enums and required-field lists directly from the schema
files, so the schema is the single source of truth. The inline bash below is a
dependency-free fallback (jq-only) that mirrors the same checks for environments
where running the script is inconvenient; prefer the script when available.

Before writing, validate each harvested item against the tracked schema
contract in [../../../docs/contracts/next-work.schema.md](../../../docs/contracts/next-work.schema.md):

```bash
validate_next_work_item() {
  local item="$1"
  local title=$(echo "$item" | jq -r '.title // empty')
  local type=$(echo "$item" | jq -r '.type // empty')
  local severity=$(echo "$item" | jq -r '.severity // empty')
  local source=$(echo "$item" | jq -r '.source // empty')
  local description=$(echo "$item" | jq -r '.description // empty')
  local proof_kind=$(echo "$item" | jq -r '.proof_ref.kind // empty')
  local proof_target_id=$(echo "$item" | jq -r '.proof_ref.target_id // empty')
  local proof_run_id=$(echo "$item" | jq -r '.proof_ref.run_id // empty')
  local proof_path=$(echo "$item" | jq -r '.proof_ref.path // empty')

  # Required fields
  if [ -z "$title" ] || [ -z "$description" ]; then
    echo "SCHEMA VALIDATION FAILED: missing title or description for item"
    return 1
  fi

  # Type enum validation
  case "$type" in
    tech-debt|improvement|pattern-fix|process-improvement|feature|bug|task|docs|chore) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid type '$type' for item '$title'"; return 1 ;;
  esac

  # Severity enum validation
  case "$severity" in
    high|medium|low) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid severity '$severity' for item '$title'"; return 1 ;;
  esac

  # Source enum validation
  case "$source" in
    council-finding|retro-learning|retro-pattern|evolve-generator|feature-suggestion|backlog-processing|post-mortem-finding|manifest-classification|dream-degraded) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid source '$source' for item '$title'"; return 1 ;;
  esac

  # Optional proof reference validation
  if [ -n "$proof_kind" ]; then
    case "$proof_kind" in
      completed_run)
        [ -n "$proof_run_id" ] || {
          echo "SCHEMA VALIDATION FAILED: completed_run proof_ref requires run_id for item '$title'"
          return 1
        }
        ;;
      evidence_only_closure)
        [ -n "$proof_target_id" ] || {
          echo "SCHEMA VALIDATION FAILED: evidence_only_closure proof_ref requires target_id for item '$title'"
          return 1
        }
        ;;
      execution_packet)
        [ -n "$proof_path" ] || {
          echo "SCHEMA VALIDATION FAILED: execution_packet proof_ref requires path for item '$title'"
          return 1
        }
        ;;
      *)
        echo "SCHEMA VALIDATION FAILED: invalid proof_ref.kind '$proof_kind' for item '$title'"
        return 1
        ;;
    esac
  fi

  return 0
}

# Validate each item; drop invalid items (do NOT block the entire harvest)
VALID_ITEMS=()
INVALID_COUNT=0
for item in "${HARVESTED_ITEMS[@]}"; do
  if validate_next_work_item "$item"; then
    VALID_ITEMS+=("$item")
  else
    INVALID_COUNT=$((INVALID_COUNT + 1))
  fi
done
echo "Schema validation: ${#VALID_ITEMS[@]}/$((${#VALID_ITEMS[@]} + INVALID_COUNT)) items passed"
```

## Write to next-work.jsonl

Canonical path: `.agents/rpi/next-work.jsonl`

```bash
mkdir -p .agents/rpi

# Resolve current repo name for target_repo default
CURRENT_REPO=$(bd config --get prefix 2>/dev/null \
  || basename "$(git remote get-url origin 2>/dev/null)" .git 2>/dev/null \
  || basename "$(pwd)")

# Normalize each validated item before writing:
#   process-improvement → "*" (applies across all repos)
#   all other types     → CURRENT_REPO (scoped to this repo)
for i in "${!VALID_ITEMS[@]}"; do
  item="${VALID_ITEMS[$i]}"
  item_type=$(echo "$item" | jq -r '.type')
  if [ "$item_type" = "process-improvement" ]; then
    VALID_ITEMS[$i]=$(echo "$item" | jq -c '.target_repo = "*"')
  else
    VALID_ITEMS[$i]=$(echo "$item" | jq -c --arg repo "$CURRENT_REPO" '.target_repo = $repo')
  fi
done

# Append one entry per epic (schema v1.4: docs/contracts/next-work.schema.md)
# Only include VALID_ITEMS that passed schema validation
# Each item: {title, type, severity, source, description, evidence, target_repo, proof_ref?}
# Entry aggregate fields: source_epic, timestamp, items[], consumed: false,
#   claim_status: "available", claimed_by: null, claimed_at: null,
#   consumed_by: null, consumed_at: null
# Item lifecycle fields are optional on write and are populated by consumers:
#   claim_status, claimed_by, claimed_at, consumed, consumed_by, consumed_at, failed_at
# Optional proof_ref shape:
#   {kind, target_id?, run_id?, path?}
#     completed_run         => run_id required
#     evidence_only_closure => target_id required
#     execution_packet      => path required
# Consumers may rewrite existing lines to claim, release, fail, or consume
# existing items. The queue is not append-only after initial write.
```

When a harvested item already maps to a known proof surface, preserve that as
`proof_ref` instead of burying identifiers in `description` or `evidence`. For
example:

```json
{
  "title": "Verify the next-work parity gate after the repair lands",
  "type": "task",
  "severity": "medium",
  "source": "council-finding",
  "description": "Re-run the targeted contract validator after proof propagation changes land.",
  "target_repo": "agentops",
  "proof_ref": {
    "kind": "execution_packet",
    "run_id": "6f36a5640805",
    "path": ".agents/rpi/runs/6f36a5640805/execution-packet.json"
  }
}
```

Use the Write tool to append a single JSON line to `.agents/rpi/next-work.jsonl` with:
- `source_epic`: the epic ID being post-mortemed
- `timestamp`: current ISO-8601
- `items`: array of harvested items (min 0 — if nothing found, write entry with empty items array)
- `consumed`: false, `claim_status`: "available", `claimed_by`: null, `claimed_at`: null, `consumed_by`: null, `consumed_at`: null

## Materialize into durable beads (lessons → beads)

The queue line is a carry-forward signal, not the durable record. Writing it is
only half the flywheel — harvested follow-ups must also become tracked beads so
the lesson compounds into the backlog instead of living only in a queue file.

After appending the queue entry, materialize the fresh items into durable beads:

```bash
# Idempotent: creates one bead per unmaterialized item, stamps bead_id back onto
# the queue item, and carries source_epic + proof_ref on the bead's native
# metadata. Re-runs skip already-materialized / consumed / held-for-review items.
ao next-work materialize
# Preview without creating: ao next-work materialize --dry-run
# Scope to this run's epic:  ao next-work materialize --source-epic <epic-id>
```

The CLI owns the deterministic core (the `bd create` + back-reference); this
skill just invokes it. Held-for-review items (`requires` set or
`status != ready`) are intentionally left in the queue for explicit promotion.

## Queue Lifecycle

Writers always append entries in **available** state. Consumers use a claim/finalize lifecycle. In batched entries, lifecycle is tracked per item; the entry-level fields are aggregate summaries only:

1. **available**: item has `consumed=false`, `claim_status="available"` (or omitted status, which consumers treat as available)
2. **in_progress**: consumer sets item `claim_status="in_progress"`, plus `claimed_by` and `claimed_at`
3. **consumed**: after a successful `/rpi` cycle and regression gate, consumer sets item `consumed=true`, `claim_status="consumed"`, `consumed_by`, and `consumed_at`
4. **release on failure**: failed or regressed cycles clear item `claimed_by` / `claimed_at`, reset `claim_status="available"`, keep `consumed=false`, and may record `failed_at`

Selection rules:
- skip items that are `consumed=true`
- skip items currently claimed with `claim_status="in_progress"`
- keep failed items retryable; `failed_at` is audit/retry-order metadata, not dormancy
- only mark the entry aggregate `consumed=true` once every child item is consumed

Never mark an item consumed at pick-time.

## Prior-Findings Resolution Tracking

Compute resolution tracking from `.agents/rpi/next-work.jsonl`:

```bash
NEXT_WORK=".agents/rpi/next-work.jsonl"

if [ -f "$NEXT_WORK" ]; then
  totals=$(jq -Rs '
    split("\n")
    | map(select(length>0) | fromjson)
    | reduce .[] as $e (
        {entries:0,total:0,resolved:0};
        .entries += 1
        | .total += ($e.items | length)
        | .resolved += ([($e.items // [])[] | select((.consumed // false) == true)] | length)
      )
    | .unresolved = (.total - .resolved)
    | .rate = (if .total > 0 then ((.resolved * 10000 / .total) | round / 100) else 0 end)
  ' "$NEXT_WORK")

  per_source=$(jq -Rs '
    split("\n")
    | map(select(length>0) | fromjson)
    | map({
        source_epic,
        total: (.items | length),
        resolved: ([((.items // [])[]) | select((.consumed // false) == true)] | length)
      })
    | group_by(.source_epic)
    | map({
        source_epic: .[0].source_epic,
        total: (map(.total) | add),
        resolved: (map(.resolved) | add),
        unresolved: ((map(.total) | add) - (map(.resolved) | add)),
        rate: (if (map(.total) | add) > 0
          then (((map(.resolved) | add) * 10000 / (map(.total) | add)) | round / 100)
          else 0 end)
      })
  ' "$NEXT_WORK")

  echo "Prior findings totals: $totals"
  echo "Prior findings by source epic: $per_source"
else
  echo "No next-work.jsonl found; resolution tracking unavailable."
fi
```

Write the totals and per-source rows into `## Prior Findings Resolution Tracking` in the post-mortem report.
