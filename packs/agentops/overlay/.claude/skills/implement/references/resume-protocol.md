# Resume Protocol

> Extracted from implement SKILL.md Step 0. Handles session continuation and checkpoint detection.

## Check Issue State (Resume Logic)

Before starting implementation, check if resuming:

1. **Check if issue is in_progress:**
```bash
bd show <issue-id> --json 2>/dev/null | jq -r '.status'
```

2. **If status = in_progress AND assigned to you:**
   - Look for checkpoint in issue notes: `bd show <id> --json | jq -r '.notes'`
   - Resume from last checkpoint step
   - Announce: "Resuming issue from Step N"

3. **If status = in_progress AND assigned to another agent:**
   - Report: "Issue claimed by <agent> - use `bd update <id> --assignee self --force` to override"
   - Do NOT proceed without explicit override

4. **Store checkpoints after each major step:**
```bash
bd update <issue-id> --append-notes "CHECKPOINT: Step N completed at $(date -Iseconds)" 2>/dev/null
```
