# Failure Recovery

## Validation Failure Handling

**On swarm validation failure:**

1. Preserve the failed issue identifier and evidence in the wave result.
2. Return the proposed adjustment to the orchestrator; do not mutate tracker
   terminal state from Crank.
3. After 3 failures, take one bounded helper pass — hand the blocker, the
   evidence, and what was tried to a fresh context or cross-family model
   (`codex exec`, `/council`); resume on UNSTUCK. Escalate only what survives
   it (never a second pass on the same blocker class):
   ```bash
   bd update <issue-id> --labels BLOCKER 2>/dev/null
   bd comments add <issue-id> "ESCALATED: 3 validation failures. Helper pass: <ESCALATE|skipped>. Human review required." 2>/dev/null
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

## Final Evidence Handoff

When all issues complete, Crank assembles the wave checkpoints and acceptance
roll-up for one final Validate invocation by the caller/orchestrator. Per-wave
checks do not substitute for that independent verdict. Crank itself does not
invoke Validate, Learn, Discovery, or Premortem.

Wave checkpoint verdicts may advise the caller about validation depth, but
never authorize a skip:

```bash
# Check wave checkpoint verdicts — clean waves scale the gate down, never skip it
ALL_PASS=true
for checkpoint in .agents/crank/wave-*-checkpoint.json; do
    verdict=$(jq -r '.acceptance_verdict // "UNKNOWN"' "$checkpoint" 2>/dev/null)
    if [[ "$verdict" != "PASS" ]]; then
        ALL_PASS=false
        break
    fi
done
```

**If all waves passed:** suggest the default one fresh independent Validate
judge. **If any wave had WARN, FAIL, or missing evidence:** include those facts
in the Validate packet; deeper review remains an explicit caller choice.

Changed files remain part of the handoff:

```bash
# Get list of changed files from recent commits
git diff --name-only HEAD~10 2>/dev/null | sort -u
```

The resulting Validate verdict must flow to Learn and then the orchestrator.
No direct retry occurs in this reference.

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

**RETRY:** Return a proposed retry adjustment with the failure evidence. Only
the orchestrator may place it into a later wave or update its tracker record.

**DECOMPOSE:** Return a proposed split with acceptance and write scopes for each
candidate slice. Do not create or close tracker records inside Crank.

**PRUNE:** Take one bounded helper pass, then return only what survives it.
Hand the blocker, the evidence, and what was tried to a fresh context or
cross-family model (`codex exec`, `/council`); on UNSTUCK resume with its next
action; on ESCALATE (or a refusal-lane / explicit-judgment class, which skips
the helper) mark the wave evidence for human judgment. The orchestrator owns any
tracker change.
Never a second helper pass on the same blocker class.

### Step 3: Budget Check

| Action | Cost | Running Total |
|--------|------|--------------|
| RETRY | 1 | +1 |
| DECOMPOSE | 2 | terminal (no further repair) |
| PRUNE | 0 | terminal (escalated) |

Max budget per task: 2. Exhausted budget → auto-PRUNE.

## Escalation

When issues cannot be resolved automatically:
- Take one bounded helper pass per blocker class first (fresh context,
  cross-family model, or `/council` — [pawls.md §Escalation](../../../docs/contracts/pawls.md#escalation-the-circuit-breaker-model));
  refusal-lane / explicit-judgment classes skip it
- Mark what survives with BLOCKER label (beads mode)
- Output `<promise>BLOCKED</promise>` with reason
- List remaining issues for human review
