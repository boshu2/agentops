# Discovery Output Templates

## Execution packet

Write the latest packet to `.agents/rpi/execution-packet.json` and, when a run
ID exists, archive the same bytes under `.agents/rpi/runs/<run-id>/`.

```json
{
  "schema_version": 3,
  "run_id": "<run-id>",
  "packet_state": "prospective",
  "objective": "<goal>",
  "skills_loaded": [
    {"name": "rpi", "reason": "run orchestrator"},
    {"name": "discovery", "reason": "intent and plan shaping"}
  ],
  "phase_receipts": [
    {"phase": "discovery", "skill": "discovery", "status": "DONE", "artifact": ".agents/rpi/discovery-receipt.json"},
    {"phase": "crank", "skill": "crank", "status": "pending"},
    {"phase": "validate", "skill": "validate", "status": "not_checked"},
    {"phase": "learn", "skill": "learn", "status": "not_checked"}
  ],
  "density": {
    "intent": "<observable behavior>",
    "boundary": {
      "bounded_context": "<context>",
      "non_goals": ["<non-goal>"],
      "write_scope": ["<path>"]
    },
    "evidence": ["<acceptance or deterministic receipt>"],
    "decision": "<why this plan shape was chosen>",
    "constraint": ["<hard constraint>"],
    "next_action": "<exact Crank command or block reason>"
  },
  "artifacts": {
    "research_path": ".agents/research/<topic>.md",
    "plan_path": ".agents/plans/<plan>.md",
    "premortem_path": ".agents/council/<premortem>.json"
  },
  "premortem_verdict": "PASS",
  "discovery_artifacts": [
    ".agents/goal-design/<slug>",
    ".agents/research/<topic>.md",
    ".agents/ideas/<run-id>/idea-challenge.json",
    ".agents/plans/<plan>.md",
    ".agents/council/<premortem>.json"
  ],
  "epic_id": "<epic-id>",
  "tracker_mode": "<beads|tasklist>",
  "test_levels": {
    "required": ["L0", "L1"],
    "recommended": ["L2"],
    "rationale": "<why>"
  },
  "issues": [
    {"id": "<leaf-id>", "title": "<one behavior>", "wave": 1, "blocked_by": []}
  ]
}
```

Omit absent optional artifacts instead of inventing placeholders. Raw research,
plan prose, idea deliberation, and judge deliberation stay in their linked
artifacts.

## Acceptance criteria

The packet carries `epic_criteria` and `bead_criteria` using the canonical
`schemas/execution-packet.schema.json#/$defs/Criterion` shape. Lift the fenced
YAML from Plan without redefining it. Crank and Validate consume these slots.

## Compatibility summary

Only when an existing consumer requires it, write a link-only phase summary:

```markdown
# Phase 1 Summary: Discovery

- **Canonical packet:** <path and digest>
- **Plan:** <path and digest>
- **Premortem:** <JSON verdict path and digest>
- **Status:** DONE
```

Telemetry is optional and never blocks the handoff or becomes a second source
of truth.
