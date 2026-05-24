# Plan Mutation Audit Trail

> Crank logs every plan mutation to `.agents/rpi/plan-mutations.jsonl` so post-mortem can assess plan drift.

## JSONL Format

Each line is a self-contained JSON object:

```jsonl
{"timestamp":"2026-03-21T10:15:00Z","wave":3,"task_id":"ag-123","mutation_type":"task_added","before":null,"after":{"subject":"Add rate limiting","reason":"Security review gap"}}
{"timestamp":"2026-03-21T10:20:00Z","wave":3,"task_id":"ag-124","mutation_type":"task_removed","before":{"subject":"Migrate legacy tokens","status":"pending"},"after":null}
{"timestamp":"2026-03-21T11:00:00Z","wave":4,"task_id":"ag-125","mutation_type":"task_reordered","before":{"wave":5},"after":{"wave":3,"reason":"Frontend needs docs earlier"}}
{"timestamp":"2026-03-21T11:05:00Z","wave":4,"task_id":"ag-123","mutation_type":"scope_changed","before":{"files":["auth.go"]},"after":{"files":["auth.go","auth_test.go"],"reason":"Tests required per review"}}
{"timestamp":"2026-03-21T11:10:00Z","wave":4,"task_id":"ag-126","mutation_type":"dependency_changed","before":{"blocked_by":["ag-120"]},"after":{"blocked_by":["ag-120","ag-121"],"reason":"Config dependency discovered"}}
```

## Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `timestamp` | ISO 8601 | yes | When the mutation occurred |
| `wave` | integer | yes | Current wave number when mutation was logged |
| `task_id` | string | yes | Issue/task ID affected |
| `mutation_type` | enum | yes | One of the five mutation types below |
| `before` | object/null | yes | State before mutation (null for additions) |
| `after` | object/null | yes | State after mutation (null for removals) |

## Mutation Types

| Type | Trigger | Before | After |
|------|---------|--------|-------|
| `task_added` | New task inserted mid-epic (insert/split) | `null` | Task subject, reason, origin task if split |
| `task_removed` | Task skipped or pruned | Task subject, status | `null` |
| `task_reordered` | Wave assignment changed | Original wave | New wave, reason |
| `scope_changed` | File manifest or acceptance criteria changed | Original files/criteria | Updated files/criteria, reason |
| `dependency_changed` | Blocked-by list modified | Original dependencies | Updated dependencies, reason |

## Mutation Budget

Crank enforces budgets to prevent runaway plan drift:

| Type | Limit | Rationale |
|------|-------|-----------|
| `task_added` | 5 per epic | Prevents scope creep |
| `task_removed` | unlimited | Pruning is healthy |
| `task_reordered` | 3 per epic | Excessive reordering = bad initial plan |
| `scope_changed` | unlimited | Refinement is expected |
| `dependency_changed` | unlimited | Discovery is expected |

When `task_added` budget is exceeded, log a warning and suggest re-running `/plan`.

## Integration Points

### Crank (writer)

Crank appends to the JSONL file at these points in the execution loop:

1. **Epic start (Step 1a.2):** Initialize the file with a header comment (empty file).
2. **Worker failure classified as DECOMPOSE:** Log `task_added` for each new sub-task.
3. **Task skipped or pruned:** Log `task_removed`.
4. **Cross-wave dependency discovered:** Log `task_reordered` and/or `dependency_changed`.
5. **Task validation reveals new requirement:** Log `task_added` for the new task.
6. **File manifest updated after exploration:** Log `scope_changed`.
7. **Wave checkpoint (Step 5.7):** Include `mutations_this_wave` count in checkpoint JSON.

### Post-mortem (reader)

Post-mortem reads the JSONL file to assess plan quality:

- **High mutation count** indicates the plan was underspecified
- **Many task_added** indicates requirements were unclear
- **Many task_reordered** indicates poor dependency analysis
- **Mutations clustered in early waves** indicates insufficient research phase
- **scope_changed on most tasks** indicates the plan lacked file-level specificity

Post-mortem includes a mutation summary table in its report.

### Wave Checkpoint Integration

Each wave checkpoint (Step 5.7) includes mutation counts:

```json
{
  "wave": 3,
  "mutations_this_wave": 2,
  "total_mutations": 5,
  "mutation_budget": {
    "task_added": {"used": 2, "limit": 5},
    "task_reordered": {"used": 1, "limit": 3}
  }
}
```
