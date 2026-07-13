# Premortem Prediction Tracking

Track premortem predictions through the lifecycle to measure accuracy.

## Prediction ID Format

```
pm-YYYYMMDD-NNN
```

Example: `pm-20260312-001`, `pm-20260312-002`

Generated during premortem report writing (Step 4). Each finding gets a unique prediction ID.

## Report Frontmatter

Add to premortem report frontmatter:

```yaml
prediction_ids:
  - pm-20260312-001
  - pm-20260312-002
```

## Finding Format with Prediction ID

Each finding in the premortem report includes its prediction ID:

```markdown
| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-20260312-001 | Feasibility | Registry write race | significant | Will cause data loss in parallel waves |
| pm-20260312-002 | Scope | Commit advisor creep | significant | Implementers will add auto-apply |
```

## Downstream Correlation

### In /validate (Step 3.6)

When a premortem report exists for the current epic:
1. Load prediction IDs from the most recent premortem report
2. For each vibe finding, check if it matches a premortem prediction
3. Tag matched findings with the prediction ID: `predicted_by: pm-20260312-001`
4. Tag unmatched findings as: `predicted_by: none` (surprise issue)

### In /postmortem (Phase 2)

Add "Prediction Accuracy" section to the report:

```markdown
## Prediction Accuracy

| Prediction ID | Predicted | Actual | Hit? |
|---------------|-----------|--------|------|
| pm-20260312-001 | Registry write race | No race detected | MISS |
| pm-20260312-002 | Commit advisor creep | Advisor stayed suggestion-only | MISS |
| — | — | Vibe found missing test | SURPRISE |

**Accuracy: 0/2 predictions confirmed (0%). 1 surprise issue.**
```

## Accuracy Scoring

- **HIT**: Premortem prediction matched an actual vibe/implementation finding
- **MISS**: Premortem prediction did not materialize
- **SURPRISE**: Actual issue that no premortem prediction covered

High miss rate is acceptable — premortem is precautionary. High surprise rate suggests premortem perspectives need expansion.
