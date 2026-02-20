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

5. **After swarm completes, verify beads status:**
```bash
# For each completed TaskList task, close the beads issue
bd update <issue-id> --status closed 2>/dev/null
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

2. **Sync to beads:**
   ```bash
   bd update <issue-id> --status closed 2>/dev/null
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

## Check for More Work

After completing a wave:

### Beads Mode
1. Clear completed tasks from TaskList
2. Check if new beads issues are now unblocked: `bd ready`
3. If yes, return to wave execution (create new TaskList tasks, invoke swarm)
4. If no more issues after 3 retry attempts, proceed to final validation

### TaskList Mode
1. `TaskList()` → any remaining pending tasks with no blockers?
2. If yes, loop back to wave execution
3. If all completed, proceed to final validation

### Both Modes
- **Max retries:** If issues remain blocked after 3 checks, escalate: "Epic blocked - cannot unblock remaining issues"
