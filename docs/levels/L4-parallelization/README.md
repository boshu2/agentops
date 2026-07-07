# L4 — Parallelization

Execute independent tasks in parallel with wave-based execution using the swarm pattern.

## What You'll Learn

- Identifying independent (unblocked) work via TaskList
- Using `/swarm` for parallel multi-agent execution
- Wave-based dependency resolution
- The Ralph Wiggum pattern for fresh context

## Prerequisites

- Completed L3-state-management
- Understanding of task dependencies (blockedBy)
- Comfortable with TaskCreate/TaskUpdate/TaskList

## Available Commands

| Command | Purpose |
|---------|---------|
| `/swarm` | Execute unblocked tasks in parallel via background agents |
| `/plan <goal>` | Same as L3 |
| `/research <topic>` | Same as L2 |
| `/implement [id]` | Execute single task |
| `/post-mortem [topic]` | Same as L2 |

## Key Concepts

- **Wave**: Set of independent tasks executed together
- **Native teams**: Each wave creates a team (`TeamCreate`), workers join as teammates, communicate via `SendMessage`
- **Fresh context**: Each team = clean slate (Ralph Wiggum pattern). New team per wave.
- **Lead-only commit**: Workers write files, lead validates + commits. Hooks block workers from `git commit`.
- **Dependency resolution**: Only unblocked tasks run in each wave

## The Ralph Wiggum Pattern

The swarm follows Ralph Wiggum's core insight: fresh context per iteration.

```
Ralph's loop:               Swarm equivalent:
while :; do                 Mayor identifies ready tasks
  cat PROMPT.md | claude    TeamCreate → spawn workers as teammates
done                        Workers complete, report via SendMessage
                            Lead validates + commits
                            TeamDelete → new team for next wave
```

Why this matters:
- **Internal loops accumulate context** → degrades over iterations
- **Fresh spawns stay effective** → each agent is a clean slate
- **Team-per-wave** → new team = new context, no bleed-through

## Swarm vs Crank vs Ratchet

These are easy to mix up:

| You Want | Use | Notes |
|----------|-----|------|
| Fresh context per iteration (“Ralph Wiggum Pattern”) | `/swarm` | Mayor owns the loop; each background agent is one atomic unit of work |
| “Do all issues until the epic is done” | `/crank` | Epic execution loop (usually beads-driven), not the Ralph pattern primitive |
| RPI checkpoints (Research→Plan→Implement→Validate) | `/flywheel` (retired — `ao flywheel status`) | Gate/record progress; pair with `/crank` or `/swarm` for execution |

## Wave Workflow

```
1. TaskList → identifies unblocked tasks
2. /swarm → TeamCreate + spawn workers as teammates
3. Workers complete → send completion via SendMessage
4. Lead reconciliation:
   a. Verify work (check files, run tests)
   b. Commit all changes (lead-only)
   c. shutdown_request workers → TeamDelete
   d. TaskList to find newly unblocked tasks
5. New team for next wave (fresh context)
```

## Lead Reconciliation Step

After workers report completion via `SendMessage`, the lead must verify before committing:

```
# For each completed worker:
1. Check the files created/modified
2. Run tests (npm test, pytest, etc.)
3. Run lint (npm run lint, etc.)
4. If valid: commit changes (lead-only — workers cannot commit)
5. If invalid: SendMessage retry instructions to idle worker
   (worker wakes with full context, no re-spawn needed)

# After all verified:
shutdown_request each worker → TeamDelete()
TaskList() → shows newly unblocked tasks → new team for next wave
```

This prevents marking broken work as complete. The `git-worker-guard` hook enforces lead-only commits.

## Agent Prompts (Atomic)

Each spawned agent gets a simple, single-task prompt:

```
# Good (atomic):
"Create users endpoint in src/routes/users.ts. Include GET /users,
POST /users, GET /users/:id routes. Follow existing patterns."

# Bad (complex loop):
"Create users endpoint, then test it, then if tests fail fix them,
then validate, then update status, then check for more work..."
```

Agents do ONE thing. Mayor handles orchestration.

## Example Session

```
1. /plan "Build auth system"
   → Creates tasks with dependencies:
   #1 [pending] Create User model
   #2 [pending] Add password hashing (blockedBy: #1)
   #3 [pending] Create login endpoint (blockedBy: #1)
   #4 [pending] Write tests (blockedBy: #2, #3)

2. /swarm
   → Wave 1: Spawns agent for #1 (only unblocked)
   → Agent completes, Mayor marks #1 completed

3. /swarm
   → Wave 2: Spawns agents for #2 and #3 in parallel
   → Both complete

4. /swarm
   → Wave 3: Spawns agent for #4
   → All done

5. /vibe → Validate everything
```

## What's NOT at This Level

- No `/crank` (full autonomous execution without human wave triggers)
- Human triggers each wave

## Next Level

Once comfortable with waves, progress to [L5-orchestration](../L5-orchestration/) for full autonomy with `/crank`.
