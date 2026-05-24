# Finding Extraction (Flywheel Closure)

After council consensus, significant findings from WARN/FAIL verdicts are extracted as candidates for the knowledge flywheel. Candidates are **not** auto-promoted to MEMORY.md — they are staged for human review or post-mortem consumption.

## Extraction Criteria

Extract a finding when ALL conditions are met:

1. **Verdict is WARN or FAIL** — PASS verdicts are skipped (nothing to learn from success).
2. **Finding severity >= significant** — minor/style findings are excluded.
3. **Confidence >= MEDIUM** — LOW-confidence findings are too speculative to persist.

## Output

Append one JSON object per line to `.agents/council/extraction-candidates.jsonl`.

### Schema (one line per finding)

```json
{
  "date": "YYYY-MM-DD",
  "council_id": "YYYY-MM-DD-<type>-<target>",
  "finding_description": "What was found (from finding.description)",
  "severity": "critical | significant",
  "source_judge": "judge-1 | judge-security | ...",
  "candidate_type": "learning | finding | rule",
  "dedup_key": "<sha256 hex of finding_description>"
}
```

### Field Details

| Field | Source | Notes |
|-------|--------|-------|
| `date` | Current date | ISO 8601 date |
| `council_id` | Report filename stem | Matches `.agents/council/YYYY-MM-DD-<type>-<target>.md` |
| `finding_description` | `finding.description` from judge output | Verbatim text |
| `severity` | `finding.severity` | Only `critical` or `significant` pass the filter |
| `source_judge` | Judge identifier | The judge that reported this finding |
| `candidate_type` | Inferred by lead | `learning` = process insight, `finding` = code/design issue, `rule` = repeatable constraint |
| `dedup_key` | SHA-256 of `finding_description` | Prevents duplicate entries across councils |

## Deduplication

Before appending, compute `sha256(finding_description)` and check whether that hash already exists in `extraction-candidates.jsonl`. If it does, skip the duplicate.

## Candidate Type Heuristics

| Pattern | Type |
|---------|------|
| Finding about a process failure or workflow gap | `learning` |
| Finding about a specific code defect or design flaw | `finding` |
| Finding that implies a repeatable constraint or invariant | `rule` |

When ambiguous, default to `finding`.

## What This Does NOT Do

- Does **not** write to MEMORY.md or any persistent knowledge store.
- Does **not** auto-create issues or beads.
- Does **not** modify council verdicts or consensus logic.
- Candidates are consumed by `/post-mortem`, manual review, or future flywheel automation.
