# Discovery Output Templates

## Execution Packet

Write the current packet to:

- `.agents/rpi/execution-packet.json` as the latest alias
- `.agents/rpi/runs/<run-id>/execution-packet.json` as the per-run archive when `run_id` exists

```json
{
  "schema_version": 1,
  "run_id": "<run-id or omitted>",
  "objective": "<goal>",
  "epic_id": "<epic-id or omitted>",
  "plan_path": ".agents/plans/<plan-file>.md",
  "contract_surfaces": ["docs/contracts/repo-execution-profile.md"],
  "validation_commands": ["<from repo profile or defaults>"],
  "validation_lanes": [
    {
      "name": "<stable lane id>",
      "command": "<validation command>",
      "read_only": true,
      "writes_artifacts": false,
      "isolated_agents_home": true,
      "release_only": false,
      "mutation_escape_hatch": null,
      "cost_class": "standard",
      "auto_select": "default",
      "timeout_seconds": 180
    }
  ],
  "tracker_mode": "<beads|tasklist>",
  "tracker_health": {
    "healthy": true,
    "mode": "<beads|tasklist>",
    "reason": "<probe summary>"
  },
  "done_criteria": ["<from repo profile or defaults>"],
  "complexity": "<fast|standard|full>",
  "pre_mortem_verdict": "<PASS|WARN>",
  "test_levels": {
    "required": ["L0", "L1"],
    "recommended": ["L2"],
    "rationale": "<why these levels apply>"
  },
  "ranked_packet_path": ".agents/rpi/ranked-packet.json",
  "discovery_timestamp": "<ISO-8601>"
}
```

## acceptance_criteria — per-epic + per-bead

The packet carries criteria at two slots — `epic_criteria` (array, one entry per epic-level acceptance statement) and `bead_criteria` (object keyed by bead ID, value is an array per bead). Both slots are typed by `#/$defs/Criterion` in [`schemas/execution-packet.schema.json`](../../../schemas/execution-packet.schema.json) — that schema is the canonical machine-readable form. Discovery STEP 6 lifts the YAML fences from the plan and serializes them into the packet; do not redefine the shape here.

Source YAML (lifted verbatim from epic + bead bodies emitted by `/plan`):

```yaml
acceptance_criteria:
  - id: ac-<scope>.<n>
    description: "<one-line measurable statement>"
    check_type: test_pass | command_exit_zero | file_exists | grep_match | manual | council_judge | custom_rubric
    check_command: "<shell command or script path>"
    evidence_path: "<glob>"
    evidence_required: true | false
    weight: 0.0–1.0
    optional: true | false
    agent_judge: "<council:name>"  # REQUIRED only when check_type == custom_rubric
```

Packet-side JSON shape (excerpt):

```json
{
  "epic_criteria": [
    { "id": "ac-e1.1", "description": "...", "check_type": "file_exists", "evidence_required": true, "weight": 1.0, "optional": false }
  ],
  "bead_criteria": {
    "soc-bcrn.1.2": [
      { "id": "ac-bcrn.1.2.1", "description": "...", "check_type": "grep_match", "evidence_required": true, "weight": 1.0, "optional": false }
    ]
  }
}
```

`/crank` and `/validation` read these slots; v1 packets without them fall back to the legacy `done_criteria` array.

## Phase Summary

Write to `.agents/rpi/phase-1-summary-YYYY-MM-DD-<goal-slug>.md`:

```markdown
# Phase 1 Summary: Discovery

- **Goal:** <goal>
- **Epic:** <epic-id>
- **Issues:** <count>
- **Complexity:** <fast|standard|full>
- **Pre-mortem:** <PASS|WARN> (attempt <N>/3)
- **Brainstorm:** <used|skipped>
- **History search:** <findings count or skipped>
- **Status:** DONE
- **Timestamp:** <ISO-8601>
```

## Ratchet and Telemetry

```bash
ao ratchet record discovery 2>/dev/null || true
bash scripts/checkpoint-commit.sh rpi "phase-1" "discovery complete" 2>/dev/null || true
bash scripts/log-telemetry.sh rpi phase-complete phase=1 phase_name=discovery 2>/dev/null || true
```
