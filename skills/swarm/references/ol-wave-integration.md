# OL Wave Integration

When `/swarm --from-wave <json-file>` is invoked, the swarm reads wave data from an OL hero hunt output file and executes it with completion backflow to OL.

## Pre-flight

```bash
# --from-wave requires ol CLI on PATH
which ol >/dev/null 2>&1 || {
    echo "Error: ol CLI required for --from-wave. Install ol or use swarm without wave integration."
    exit 1
}
```

If `ol` is not on PATH, exit immediately with the error above. Do not fall back to normal swarm mode.

## Input Format

The `--from-wave` JSON file contains `ol hero hunt` output:

```json
{
  "wave": [
    {"id": "ol-527.1", "title": "Add auth middleware", "spec_path": "quests/ol-527/specs/ol-527.1.md", "priority": 1},
    {"id": "ol-527.2", "title": "Fix rate limiting", "spec_path": "quests/ol-527/specs/ol-527.2.md", "priority": 2}
  ],
  "blocked": [
    {"id": "ol-527.3", "title": "Integration tests", "blocked_by": ["ol-527.1", "ol-527.2"]}
  ],
  "completed": [
    {"id": "ol-527.0", "title": "Project setup"}
  ]
}
```

## Execution

1. **Parse the JSON file** and extract the `wave` array.

2. **Create TaskList tasks** from wave entries (one `TaskCreate` per entry):

```
for each entry in wave:
    TaskCreate(
        subject="[{entry.id}] {entry.title}",
        description="OL bead {entry.id}\nSpec: {entry.spec_path}\nPriority: {entry.priority}\n\nRead the spec file at {entry.spec_path} for full requirements.",
        metadata={
            "issue_type": entry.issue_type,
            "ol_bead_id": entry.id,
            "ol_spec_path": entry.spec_path,
            "ol_priority": entry.priority
        }
    )
```

3. **Execute swarm normally** on those tasks (Step 2 onward from main execution flow). Tasks are ordered by priority (lower number = higher priority).

4. **Completion backflow**: After each worker completes a bead task AND passes validation, the team lead runs the OL ratchet command to report completion back to OL:

```bash
# Extract quest ID from bead ID (e.g., ol-527.1 -> ol-527)
QUEST_ID=$(echo "$BEAD_ID" | sed 's/\.[^.]*$//')

ol hero ratchet "$BEAD_ID" --quest "$QUEST_ID"
```

**Ratchet result handling:**

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Bead complete in OL | Mark task completed, log success |
| 1 | Ratchet validation failed | Mark task as failed, log the validation error from stderr |

5. **After all wave tasks complete**, report a summary that includes both swarm results and OL ratchet status for each bead.

## Example

```
/swarm --from-wave /tmp/wave-ol-527.json

# Reads wave JSON -> creates 2 tasks from wave entries
# Spawns workers for ol-527.1 and ol-527.2
# On completion of ol-527.1:
#   ol hero ratchet ol-527.1 --quest ol-527 -> exit 0 -> bead complete
# On completion of ol-527.2:
#   ol hero ratchet ol-527.2 --quest ol-527 -> exit 0 -> bead complete
# Wave done: 2/2 beads ratcheted in OL
```
