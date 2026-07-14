# Writing the Premortem Output

Write one canonical JSON artifact to
`.agents/council/YYYY-MM-DD-premortem-<topic>.json`.

The artifact conforms to
[`../schemas/plan-verdict.schema.json`](../schemas/plan-verdict.schema.json) and
binds the repository-relative plan path, live SHA-256, distinct author and judge
identities, binary verdict, and complete blocker list.

```json
{
  "schema_version": "premortem-plan-verdict.v1",
  "plan": {
    "path": ".agents/plans/2026-07-14-example.md",
    "sha256": "<64 lowercase hex>"
  },
  "author_id": "planner-context",
  "judge_id": "fresh-judge-context",
  "verdict": "FAIL",
  "blockers_complete": true,
  "blockers": [
    {
      "id": "B1",
      "claim": "The migration inventory omits an active consumer",
      "evidence": ["path/to/manifest.json", "path/to/consumer"]
    }
  ]
}
```

Validate the artifact against both the schema and current plan bytes:

```bash
skills/premortem/scripts/validate-output.sh \
  .agents/council/YYYY-MM-DD-premortem-<topic>.json \
  "$(git rev-parse --show-toplevel)"
```

Optional model metadata is descriptive only. Do not add readiness projections,
attempt history, repair budgets, helper state, implementation state, tracker
state, or delivery authority to this artifact.

Reusable findings may be copied off the critical path only after the
orchestrator accepts them. The immutable plan verdict itself is never rewritten
into a learning registry entry.
