# Failure Recovery

## Validation Failure Handling

**On swarm validation failure:**

1. Do NOT close the beads issue
2. Add failure context:
   ```bash
   bd comments add <issue-id> "Validation failed: <reason>. Retrying..." 2>/dev/null
   ```
3. Re-add to next wave
4. After 3 failures, escalate:
   ```bash
   bd update <issue-id> --labels BLOCKER 2>/dev/null
   bd comments add <issue-id> "ESCALATED: 3 validation failures. Human review required." 2>/dev/null
   ```

## Wave Limit Enforcement

```bash
# CHECK GLOBAL LIMIT before each wave
if [[ $wave -ge 50 ]]; then
    echo "<promise>BLOCKED</promise>"
    echo "Global wave limit (50) reached. Remaining issues:"
    # Beads mode: bd children <epic-id> --status open
    # TaskList mode: TaskList() → pending tasks
    # STOP - do not continue
fi
```

## Pre-flight Check: Issues Exist

**Verify there are issues to work on:**

**If 0 ready issues found (beads mode) or 0 pending unblocked tasks (TaskList mode):**
```
STOP and return error:
  "No ready issues found for this epic. Either:
   - All issues are blocked (check dependencies)
   - Epic has no child issues (run /plan first)
   - All issues already completed"
```

Also verify: epic has at least 1 child issue total. An epic with 0 children means /plan was not run.

Do NOT proceed with empty issue list - this produces false "epic complete" status.

## Final Batched Validation

When all issues complete, check whether a full /vibe is needed:

```bash
# Check wave checkpoint verdicts — skip final vibe if ALL waves passed clean
ALL_PASS=true
for checkpoint in .agents/crank/wave-*-checkpoint.json; do
    verdict=$(jq -r '.acceptance_verdict // "UNKNOWN"' "$checkpoint" 2>/dev/null)
    if [[ "$verdict" != "PASS" ]]; then
        ALL_PASS=false
        break
    fi
done
```

**If ALL waves passed acceptance check with PASS verdict (no WARNs, no retries):**
Skip the final /vibe — per-wave acceptance checks already validated acceptance criteria. Proceed directly to Step 8 (learnings extraction).

**If ANY wave had WARN, FAIL, or missing verdicts:**
Run ONE comprehensive vibe on recent changes:

```bash
# Get list of changed files from recent commits
git diff --name-only HEAD~10 2>/dev/null | sort -u
```

```
Tool: Skill
Parameters:
  skill: "agentops:vibe"
  args: "recent"
```

**If CRITICAL issues found:**
1. Fix them
2. Re-run vibe on affected files
3. Only proceed to completion when clean

## Node Repair Operator

Structured recovery replaces simple retry logic. When a task fails:

### Step 1: Classify

Read the failure output and classify:

| Signal | Classification |
|--------|---------------|
| "timeout", "connection refused", "EAGAIN", test passed on retry | RETRY |
| Partial completion, >3 files changed, merge conflict mid-task | DECOMPOSE |
| "blocked by", "spec impossible", "missing API", external dep | PRUNE |

### Step 2: Execute Recovery

**RETRY:** Re-add issue to next wave with context:
```bash
bd comments add <issue-id> "RETRY (attempt N/2): <failure reason>. Adjustment: <what to try differently>"
```
Worker gets the adjustment context in its task prompt.

**DECOMPOSE:** Split the issue:
```bash
# Create sub-issues
bd create --title "<original-title> — part A" --body "<scoped description>" --parent <epic-id>
bd create --title "<original-title> — part B" --body "<scoped description>" --parent <epic-id>
# Close original as decomposed
bd update <issue-id> --labels decomposed
bd close <issue-id>
bd comments add <issue-id> "DECOMPOSED into <new-id-a>, <new-id-b>"
```

**PRUNE:** Escalate immediately:
```bash
bd update <issue-id> --labels BLOCKER
bd comments add <issue-id> "PRUNED: <reason>. Human review required."
```

### Step 3: Budget Check

| Action | Cost | Running Total |
|--------|------|--------------|
| RETRY | 1 | +1 |
| DECOMPOSE | 2 | terminal (no further repair) |
| PRUNE | 0 | terminal (escalated) |

Max budget per task: 2. Exhausted budget → auto-PRUNE.

## Escalation

When issues cannot be resolved automatically:
- Mark with BLOCKER label (beads mode)
- Output `<promise>BLOCKED</promise>` with reason
- List remaining issues for human review
