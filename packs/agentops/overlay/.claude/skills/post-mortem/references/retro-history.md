# Persistent Retro History

Store post-mortem summaries in `.agents/retro/` for trend analysis across epics.

## Directory Structure

```
.agents/retro/
├── YYYY-MM-DD-<epic-slug>.json    # structured summary per post-mortem
└── index.jsonl                     # append-only index for fast lookups
```

## Summary Schema (`YYYY-MM-DD-<epic-slug>.json`)

```json
{
  "id": "retro-YYYY-MM-DD-<epic-slug>",
  "date": "YYYY-MM-DD",
  "epic_id": "<epic-id or 'recent'>",
  "verdict": "PASS|WARN|FAIL",
  "duration_minutes": 45,
  "cycle_time_trend": "faster|slower|stable",
  "learnings_extracted": 5,
  "learnings_promoted": 2,
  "stale_retired": 1,
  "prediction_accuracy": {
    "hits": 1,
    "misses": 2,
    "surprises": 1,
    "rate": 0.33
  },
  "footguns": ["<short description>"],
  "top_learning": "<single most impactful learning>",
  "improvements_proposed": 3,
  "tags": ["<category tags from learnings>"]
}
```

## Index Schema (`index.jsonl`)

One JSON object per line, append-only:

```json
{"id": "retro-2026-03-12-auth-system", "date": "2026-03-12", "epic_id": "ag-abc", "verdict": "PASS", "duration_minutes": 45, "learnings_extracted": 5}
```

## Write Rules

1. Write the full summary JSON first, then append to `index.jsonl`
2. Use atomic write (temp file + rename) for the summary JSON
3. If `.agents/retro/` does not exist, create it: `mkdir -p .agents/retro`
4. If `index.jsonl` does not exist, create it with the first entry
5. Dedup by `id` — if a retro with the same id exists, overwrite the summary and skip the index append

## Read Rules (for trend analysis)

### Recent History

```bash
# Last 5 retros
tail -5 .agents/retro/index.jsonl | jq -s '.'
```

### Verdict Trend

```bash
# Win/loss streak
tail -10 .agents/retro/index.jsonl | jq -r '.verdict'
```

### Cycle Time Trend

```bash
# Duration trend over last 10 retros
tail -10 .agents/retro/index.jsonl | jq -s '[.[].duration_minutes] | {min: min, max: max, avg: (add/length)}'
```

### Recurring Footguns

```bash
# Footguns that appear in 2+ retros
jq -s '[.[].footguns[]?] | group_by(.) | map(select(length > 1) | {footgun: .[0], count: length})' .agents/retro/*.json
```

## Fail-Open Behavior

- Missing `.agents/retro/` directory → create silently
- Missing `index.jsonl` → create on first write
- Malformed existing JSON → warn once, skip that entry
- Unwritable directory → warn once, continue without persisting history
