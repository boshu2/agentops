# Status Dashboard Contract

This reference owns the stable rendering and JSON schema for `$status`. The
skill kernel owns live collection, source precedence, and next-action selection.

## Human layout

Render one screen with these blocks in order:

```text
Current Work
  Epic: <id + title | none | unavailable>
  In progress: <exact ids | none | unavailable>
  Ready: <top exact ids | none | unavailable>
  Ratchet: <phase | idle | unavailable>
  Git: <branch> @ <commit>; <clean | N changes>

Latest Gates
  Reconciliation: <PASS | WARN | FAIL | UNAVAILABLE>
  Pawl: <verdict + artifact + commit | none | unavailable>
  Validation: <verdict + artifact | none | unavailable>

Next Action
  P<priority>: <one executable action>
  Because: <the exact fact that selected this priority>

Coverage: <available>/<attempted>; <unavailable or malformed sources>
```

Never replace unavailable with `none`: `none` means the source was available
and returned an empty set.

## JSON schema

`--json` emits one object:

```json
{
  "schema_version": 1,
  "generated_at": "2026-07-12T13:00:00Z",
  "current_work": {
    "epic": {"id": "age-example", "title": "Example"},
    "in_progress": ["age-example.1"],
    "ready": ["age-example.2"],
    "ratchet_phase": "implement",
    "git": {"branch": "main", "commit": "abc1234", "uncommitted_count": 0}
  },
  "latest_gates": {
    "reconciliation": {"status": "PASS", "high_findings": []},
    "verdicts": [{"kind": "pawl", "verdict": "CONFIRMED", "artifact": ".agents/pawl-verdicts/age-example.1.json", "commit": "abc1234"}]
  },
  "next_action": {
    "priority": 2,
    "message": "Resume age-example.1 in its claimed worktree",
    "because": "age-example.1 is in_progress"
  },
  "coverage": [
    {"source": "ao reconcile --json", "status": "available", "detail": "exit 0"},
    {"source": "inbox", "status": "unavailable", "detail": "optional CLI missing"}
  ]
}
```

Required top-level fields are `schema_version`, `generated_at`, `current_work`,
`latest_gates`, `next_action`, and `coverage`. Coverage status is exactly one of
`available`, `unavailable`, or `malformed`.

## Validation

With `OUT` pointing to captured JSON:

```bash
jq -e '.schema_version==1 and (.generated_at|type)=="string" and (.current_work|type)=="object" and (.latest_gates|type)=="object" and (.next_action|type)=="object" and (.next_action.priority|type)=="number" and (.next_action.message|type)=="string" and (.coverage|type)=="array" and all(.coverage[]; (.source|type)=="string" and (.status=="available" or .status=="unavailable" or .status=="malformed"))' "$OUT"
```

An empty object, missing coverage, unknown coverage status, nonnumeric priority,
or explanatory prose around the JSON is invalid.
