# Plan Mutation Evidence

Crank preserves plan-change evidence in `.agents/rpi/plan-mutations.jsonl` so
the orchestrator can re-plan and Learn or Postmortem can inspect drift. This is
an audit trail, not permission to change the plan or a controller around how
often evidence may change it.

## Request, decide, record

1. When wave evidence implies that a task, dependency, order, scope, or
   acceptance must change, Crank returns a mutation request with the proposed
   before/after state, reason, and evidence references.
2. The orchestrator classifies the evidence. A material change is `REPLAN` and
   returns through Discovery and Premortem; an unchanged accepted plan continues
   without another plan review.
3. Only after the orchestrator changes the plan does Crank append the resulting
   factual event to the JSONL audit trail.

Crank never blocks, approves, or escalates a mutation because of prior event
counts. Counts in a checkpoint describe what happened; they do not confer
authority over the next move.

## JSONL format

Each line is one self-contained recorded event:

```jsonl
{"timestamp":"2026-03-21T10:15:00Z","wave":3,"task_id":"ag-123","mutation_type":"task_added","before":null,"after":{"subject":"Add rate limiting"},"reason":"security review gap","evidence_refs":[".agents/evidence/security.json"]}
{"timestamp":"2026-03-21T10:20:00Z","wave":3,"task_id":"ag-124","mutation_type":"task_removed","before":{"subject":"Migrate legacy tokens","status":"pending"},"after":null,"reason":"acceptance no longer requires migration","evidence_refs":[".agents/evidence/acceptance.json"]}
{"timestamp":"2026-03-21T11:00:00Z","wave":4,"task_id":"ag-125","mutation_type":"task_reordered","before":{"wave":5},"after":{"wave":3},"reason":"documentation is now a dependency","evidence_refs":[".agents/evidence/dependencies.json"]}
```

## Field reference

| Field | Type | Required | Description |
|---|---|---|---|
| `timestamp` | ISO 8601 | yes | When the orchestrator-applied mutation was recorded |
| `wave` | integer | yes | Wave that produced the evidence |
| `task_id` | string | yes | Issue or task affected |
| `mutation_type` | enum | yes | One of the five event types below |
| `before` | object/null | yes | State before the change |
| `after` | object/null | yes | State after the change |
| `reason` | string | yes | Evidence-based reason for the change |
| `evidence_refs` | array | yes | Paths to the facts that justified the decision |

## Event types

| Type | Trigger | Before | After |
|---|---|---|---|
| `task_added` | Accepted plan gains a task | `null` | Task subject and origin task when split |
| `task_removed` | Accepted plan prunes a task | Task subject and status | `null` |
| `task_reordered` | Wave assignment changes | Original wave | New wave |
| `scope_changed` | Files or acceptance change | Original files or criteria | Updated files or criteria |
| `dependency_changed` | Dependency graph changes | Original dependencies | Updated dependencies |

## Integration points

Crank returns a mutation request when worker failure suggests decomposition,
validation reveals a new requirement, exploration changes the file manifest,
or a cross-wave dependency appears. The request stays in wave evidence until
the orchestrator decides it.

After an accepted plan change, Crank appends the recorded event. The wave
checkpoint includes only the factual event counts:

```json
{
  "wave": 3,
  "mutations_this_wave": 2,
  "total_mutations": 5
}
```

Postmortem may summarize high counts, frequent additions, reordering, or early
clusters as evidence that planning was weak. That interpretation remains
advisory and cannot block a wave or select the next disposition.
