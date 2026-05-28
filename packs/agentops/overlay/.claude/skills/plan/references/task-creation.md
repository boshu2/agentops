# Task Creation and Validation Blocks

> Extracted from plan/SKILL.md on 2026-04-11.
> Covers TaskCreate + beads issue creation, validation-block embedding, post-creation verification.

## Step 7: Create Tasks for In-Session Tracking

**Use TaskCreate tool** for each issue:

```
Tool: TaskCreate
Parameters:
  subject: "<issue title>"
  description: |
    <Full description including:>
    - What to do
    - Acceptance criteria
    - Dependencies: [list task IDs that must complete first]
  activeForm: "<-ing verb form of the task>"
```

**After creating all tasks, set up dependencies:**

```
Tool: TaskUpdate
Parameters:
  taskId: "<task-id>"
  addBlockedBy: ["<dependency-task-id>"]
```

## Create Persistent Beads Issues for Ratchet Tracking

If bd CLI available, create beads issues to enable progress tracking across sessions:

```bash
# Create epic first
bd create --title "<goal>" --type epic --label "planned"

# Create child issues (note the IDs returned)
bd create --title "<wave-1-task>" --body "<description>" --parent <epic-id> --label "planned"
# Returns: na-0001

bd create --title "<wave-2-task-depends-on-wave-1>" --body "<description>" --parent <epic-id> --label "planned"
# Returns: na-0002

# Add blocking dependencies to form waves
bd dep add na-0001 na-0002
# Now na-0002 is blocked by na-0001 → Wave 2
```

## Include Conformance Checks in Issue Bodies

When creating beads issues, embed the conformance checks from the plan as a fenced validation block in the issue description. This flows to worker validation metadata via /crank:

````
bd create --title "<task>" --body "Description...

\`\`\`validation
{\"files_exist\": [\"src/auth.go\"], \"content_check\": {\"file\": \"src/auth.go\", \"pattern\": \"func Authenticate\"}}
\`\`\`
" --parent <epic-id>
````

## Include Cross-Cutting Constraints in Epic Description

"Always" boundaries from the plan should be added to the epic's description as a `## Cross-Cutting Constraints` section. /crank reads these from the epic (not per-issue) and injects them into every worker task's validation metadata.

## Waves Are Formed by `blocks` Dependencies

- Issues with NO blockers → Wave 1 (appear in `bd ready` immediately)
- Issues blocked by Wave 1 → Wave 2 (appear when Wave 1 closes)
- Issues blocked by Wave 2 → Wave 3 (appear when Wave 2 closes)

**`bd ready` returns the current wave** — all unblocked issues that can run in parallel.

Beads-backed issues are the preferred path because they give `/crank` richer dependency data and make ratchet progress easier to inspect. When bd is unavailable or degraded, keep the plan file + execution packet path accurate and continue in file-backed mode for `/crank` and `/validation`.

## Step 7b: Verify Validation Blocks (Post-Creation Check)

After creating all beads issues, verify that every issue body contains a fenced validation block. Missing validation blocks break the plan-to-crank pipeline — `/crank` cannot extract conformance checks from issues that lack them.

```bash
if command -v bd &>/dev/null && [[ -n "$EPIC_ID" ]]; then
    MISSING_VALIDATION=()
    for ISSUE_ID in $ALL_CREATED_ISSUES; do
        if ! bd show "$ISSUE_ID" 2>/dev/null | grep -q '```validation'; then
            MISSING_VALIDATION+=("$ISSUE_ID")
        fi
    done
    if [[ ${#MISSING_VALIDATION[@]} -gt 0 ]]; then
        echo "WARNING: ${#MISSING_VALIDATION[@]} issue(s) missing validation blocks: ${MISSING_VALIDATION[*]}"
        echo "  /crank will fall back to default files_exist checks for these issues."
        echo "  Consider adding ```validation``` blocks with conformance checks."
    else
        echo "All ${#ALL_CREATED_ISSUES[@]} issues have validation blocks."
    fi
fi
```

This is a warning gate, not a blocker — plans can proceed without validation blocks, but crank execution will use weaker fallback checks.
