# Team Coordination

## Wave Execution via Swarm

### Beads Mode

1. **Get ready issues from current wave**
2. **Create TaskList tasks from beads issues:**

For each ready beads issue, create a corresponding TaskList task:
```
TaskCreate(
  subject="<issue-id>: <issue-title>",
  description="Implement beads issue <issue-id>.

Details from beads:
<paste issue details from bd show>

Execute using /implement <issue-id>. Mark complete when done.",
  activeForm="Implementing <issue-id>"
)
```

3. **Add dependencies if issues have beads blockedBy:**
```
TaskUpdate(taskId="2", addBlockedBy=["1"])
```

4. **Invoke swarm to execute the wave:**
```
Tool: Skill
Parameters:
  skill: "agentops:swarm"
```

5. **After swarm completes, verify and close beads with evidence:**
```bash
# For each completed TaskList task, close the beads issue with evidence
# Workers should already have closed via /implement Step 7; this is a safety net
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
bd close <issue-id> --reason "crank-sync wave:${wave} commit:${COMMIT_SHA}" 2>/dev/null
```

### TaskList Mode

Tasks already exist in TaskList (created in Step 1 from plan file/description, or pre-existing). Just invoke swarm directly:

```
Tool: Skill
Parameters:
  skill: "agentops:swarm"
```

Swarm finds unblocked TaskList tasks and executes them.

### Both Modes — Swarm Will:

- Find all unblocked TaskList tasks
- Select runtime backend for the wave (runtime-native first: Claude sessions -> `TeamCreate`, Codex sessions -> `spawn_agent`, fallback tasks only if needed)
- Spawn workers with fresh context (Ralph pattern)
- Workers execute in parallel and report via backend channel (`wait`/`SendMessage`/`TaskOutput`)
- Team lead validates, then cleans up backend resources (`close_agent`/`TeamDelete`/none)

## Verify and Sync to Beads (MANDATORY)

> Swarm executes per-task validation (see `skills/shared/validation-contract.md`). Crank trusts swarm validation and focuses on beads sync.

**For each issue reported complete by swarm:**

1. **Verify swarm task completed:**
   ```
   TaskList() → check task status == "completed"
   ```
   If task is still pending/blocked, swarm validation failed — add to retry queue.

2. **Sync to beads with evidence:**
   ```bash
   COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
   CHANGED_FILES=$(git diff --name-only HEAD~1 2>/dev/null | head -10 | tr '\n' ' ' | sed 's/ $//')
   bd close <issue-id> --reason "commit:${COMMIT_SHA} files:[${CHANGED_FILES}]" 2>/dev/null
   ```

3. **On sync failure** (bd unavailable or error):
   - Log warning but do NOT block the wave
   - Track for manual sync after epic completes

4. **Record ratchet progress (ao integration):**
   ```bash
   if command -v ao &>/dev/null; then
       ao ratchet record implement 2>/dev/null
   fi
   ```

**Note:** Per-issue review is handled by swarm validation. Wave-level semantic review happens in the Wave Acceptance Check.

## Report Remaining Work

After completing a wave:

### Beads Mode
1. Clear completed tasks from TaskList.
2. Inspect `bd ready` only to determine whether work remains.
3. If work remains, emit `PARTIAL` with the wave evidence and remaining-work summary to Validate.
4. If no work remains, emit `DONE` with final wave evidence to Validate.

### TaskList Mode
1. Inspect `TaskList()` only to determine whether pending work remains.
2. If work remains, emit `PARTIAL` with the wave evidence and remaining-work summary to Validate.
3. If all work is complete, emit `DONE` with final wave evidence to Validate.

### Both Modes
- Crank never creates the next wave, invokes another swarm, retries blocked work, or changes the plan from this step.
- Blocked remaining work produces `BLOCKED` evidence for the orchestrator; it does not trigger an inline retry or escalation policy.
- The mandatory handoff is `Validate -> Learn -> orchestrator`. Only the orchestrator decides whether to continue, retry, stop, escalate, or route a changed plan through Discovery and Premortem.
