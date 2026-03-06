# Harvest Next Work — Implementation Details

## Schema Validation

Before writing, validate each harvested item against the schema contract (`.agents/rpi/next-work.schema.md`):

```bash
validate_next_work_item() {
  local item="$1"
  local title=$(echo "$item" | jq -r '.title // empty')
  local type=$(echo "$item" | jq -r '.type // empty')
  local severity=$(echo "$item" | jq -r '.severity // empty')
  local source=$(echo "$item" | jq -r '.source // empty')
  local description=$(echo "$item" | jq -r '.description // empty')
  local target_repo=$(echo "$item" | jq -r '.target_repo // empty')

  # Required fields
  if [ -z "$title" ] || [ -z "$description" ]; then
    echo "SCHEMA VALIDATION FAILED: missing title or description for item"
    return 1
  fi

  # target_repo required (v1.2)
  if [ -z "$target_repo" ]; then
    echo "SCHEMA VALIDATION FAILED: missing target_repo for item '$title'"
    return 1
  fi

  # Type enum validation
  case "$type" in
    tech-debt|improvement|pattern-fix|process-improvement) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid type '$type' for item '$title'"; return 1 ;;
  esac

  # Severity enum validation
  case "$severity" in
    high|medium|low) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid severity '$severity' for item '$title'"; return 1 ;;
  esac

  # Source enum validation
  case "$source" in
    council-finding|retro-learning|retro-pattern) ;;
    *) echo "SCHEMA VALIDATION FAILED: invalid source '$source' for item '$title'"; return 1 ;;
  esac

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

# Assign target_repo to each validated item (v1.2):
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

# Append one entry per epic (schema v1.2: .agents/rpi/next-work.schema.md)
# Only include VALID_ITEMS that passed schema validation
# Each item: {title, type, severity, source, description, evidence, target_repo}
# Entry fields: source_epic, timestamp, items[], consumed: false
```

Use the Write tool to append a single JSON line to `.agents/rpi/next-work.jsonl` with:
- `source_epic`: the epic ID being post-mortemed
- `timestamp`: current ISO-8601
- `items`: array of harvested items (min 0 — if nothing found, write entry with empty items array)
- `consumed`: false, `consumed_by`: null, `consumed_at`: null

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
        | .resolved += (if ($e.consumed // false) then ($e.items | length) else 0 end)
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
        resolved: (if (.consumed // false) then (.items | length) else 0 end)
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
