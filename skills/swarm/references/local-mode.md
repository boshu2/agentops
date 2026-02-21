# Swarm Local Mode: Runtime-Aware Detailed Execution

## Context Budget Rule

> **Workers write results to disk. The orchestrator reads only thin status files.**
>
> When N workers finish, their full output (file reads, tool calls, reasoning) must NOT flood back into the orchestrator context. This is the #1 cause of context explosion in multi-wave epics.

**Result protocol:**
1. Workers write `.agents/swarm/results/<task-id>.json` on completion
2. Orchestrator checks for result files (Glob/Read), NOT full Task/SendMessage output
3. SendMessage used only for coordination signals (blocked, need help) — kept under 100 tokens
4. Task tool return values are acknowledged but NOT parsed for work details

```bash
# Orchestrator creates result directory before spawning
mkdir -p .agents/swarm/results
```

## Step 2b: Pre-Spawn Worktree Setup (Multi-Epic Waves)

> **Skip this step** for single-epic waves or when `--no-worktrees` is set.
> **Required** for multi-epic dispatch or when `--worktrees` is set.

Evidence: shared-worktree multi-epic dispatch produced build breaks and algorithm duplication (`.agents/evolve/dispatch-comparison.md`).

### Claude-Native Isolation (preferred when available)

If running in Claude runtime with modern agent definitions, prefer declarative isolation first:

1. Confirm teammate profiles with `claude agents`
2. Use teammate definitions that set `isolation: worktree`
3. For long-running workers, set `background: true`

Only fall back to manual `git worktree` management when declarative isolation is unavailable.

### Detection

```bash
# Multi-epic: check if tasks span more than one epic prefix
# If wave tasks have subjects like "[ol-527] ..." and "[ol-531] ...", use worktrees.
# Single-epic: tasks share one prefix (e.g., all ol-527.*) → shared worktree OK.
```

### Create Worktrees

```bash
# For each epic ID in the wave:
git worktree add /tmp/swarm-<epic-id> -b swarm/<epic-id>
```

Track the mapping:
```
epic_worktrees = {
  "<epic-id>": "/tmp/swarm-<epic-id>",
  ...
}
```

### Inject into Worker Prompts

Each worker prompt must include:
```
WORKING DIRECTORY: /tmp/swarm-<epic-id>

All file reads, writes, and edits MUST use absolute paths rooted at /tmp/swarm-<epic-id>.
Do NOT operate on the main repo directly.
Result file: write to <main-repo>/.agents/swarm/results/<task-id>.json (always main repo path).
```

### Merge-Back After Validation

After each worker's task passes validation:
```bash
# From main repo:
git merge --no-ff swarm/<epic-id> -m "chore: merge swarm/<epic-id>"
git worktree remove /tmp/swarm-<epic-id>
git branch -d swarm/<epic-id>
```

Merge order must respect task blockedBy dependencies.

---

## Step 3: Spawn Workers

Use whatever multi-agent primitives your runtime provides to spawn parallel workers. Each worker receives a pre-assigned task in its prompt.

### Model Selection

Workers should use **sonnet** (not opus) to minimize cost. The orchestrator (lead) stays on opus for coordination and validation.

When spawning via the Task tool, pass `model: "sonnet"`. When spawning via native teams, teammates inherit from the session model unless overridden — set `COUNCIL_CLAUDE_MODEL=sonnet` or use `model: "sonnet"` in the Task call. For longer tasks, prefer teammate profiles with `background: true`.

| Role | Model | Rationale |
|------|-------|-----------|
| Lead/orchestrator | opus (session default) | Coordination, validation, state management |
| Workers | sonnet | Focused single-task execution, 3-5x cheaper |
| Explorers | sonnet | Read-only search tasks |

### Spawn Protocol

For each ready task:

1. **Pre-assign** — Mark the task owned by `worker-<task-id>` before spawning (prevents race conditions)
2. **Spawn** — Create a parallel subagent with the worker prompt (see below)
3. **Track** — Map `worker-<task-id>` to agent handle for waits/retries/cleanup

All workers in a wave spawn in parallel. New team/agent-group per wave = fresh context (Ralph Wiggum preserved).

### Worker Prompt Template

Every worker receives this prompt (adapt to your runtime's spawn mechanism):

```
You are worker-<task-id>.

Your Assignment: Task #<id>: <subject>
<description>

Instructions:
1. Execute your pre-assigned task independently — create/edit files as needed, verify your work
2. Write your result to .agents/swarm/results/<task-id>.json (see format below)
3. Send a SHORT completion signal to the lead (under 100 tokens)
4. If blocked, write blocked result to same path and signal the lead

RESULT FILE FORMAT (MANDATORY — write this BEFORE sending any signal):

On success:
{"type":"completion","issue_id":"<task-id>","status":"done","detail":"<one-line summary max 100 chars>","artifacts":["path/to/file1","path/to/file2"]}

If blocked:
{"type":"blocked","issue_id":"<task-id>","status":"blocked","detail":"<reason max 200 chars>"}

CONTEXT BUDGET RULE:
Your message to the lead must be under 100 tokens.
Do NOT include file contents, diffs, or detailed explanations in messages.
The result JSON file IS your full report. The lead reads the file, not your message.

Rules:
- Work only on YOUR pre-assigned task
- Do NOT claim other tasks
- Do NOT message other workers
- Do NOT run git add, git commit, or git push — the lead commits
```

## Race Condition Prevention

Workers do NOT race-claim tasks from TaskList. The team lead assigns each task
to a specific worker BEFORE spawning. This prevents:
- Two workers claiming the same task
- Workers seeing stale TaskList state
- Non-deterministic assignment order

Workers only transition their assigned task: in_progress -> completed.

## Git Commit Policy

**Workers MUST NOT commit.** The team lead is the sole committer.

| Actor | Git Permissions |
|-------|----------------|
| Team lead (mayor) | git add, commit, push |
| Workers | Read-only git. Write files only. |

**Rationale:**
- Workers share a worktree (per native teams semantics research)
- Concurrent git add/commit from multiple workers corrupts the index
- Lead-only commits ensure atomic, reviewable changesets per wave

**Worker instructions:** Include in every worker prompt:
"Do NOT run git add, git commit, or git push. Write your result to .agents/swarm/results/<task-id>.json, then send a short signal (under 100 tokens) via your runtime channel. The team lead reads result files, not messages."

## Step 4: Wait for Completion

Wait for all workers to signal completion using your runtime's wait mechanism. Workers write result files to disk and send a minimal signal.

**Result data** — always read from disk, never from agent messages:

Check `.agents/swarm/results/<task-id>.json` for each worker. These are ~200 bytes each. Do NOT parse agent messages or return values for work details — they contain the worker's full conversation (5-20K tokens per worker) and will explode the lead's context.

**CRITICAL**: Do NOT mark complete yet — validation required first.

## Step 4a: Validate Before Accepting (MANDATORY)

> **TRUST ISSUE**: Agent completion claims are NOT trusted. Verify then trust.

**The Validation Contract**: Before marking any task complete, Mayor MUST run validation checks. See `skills/shared/validation-contract.md` for full specification.

**Validation flow:**

```
<task-notification> arrives
        |
        v
    RUN VALIDATION
        |
    +---+---+
    |       |
  PASS    FAIL
    |       |
    v       v
 complete  retry/escalate
```

**For each completed task notification:**

1. **Check task metadata for validation requirements:**
   ```
   TaskList() -> find task -> check metadata.validation
   ```

2. **Execute validation checks (in order):**

   | Check Type | Command | Pass Condition |
   |------------|---------|----------------|
   | `files_exist` | `ls -la <paths>` | All files exist |
   | `command` | Run specified command | Exit code 0 |
   | `content_check` | `grep <pattern> <file>` | Pattern found |
   | `tests` | `<test_command>` | Tests pass |
   | `lint` | `<lint_command>` | No errors |

3. **On validation PASS:**
   ```
   TaskUpdate(taskId="<id>", status="completed")
   ```

4. **On validation FAIL:**
   - Increment retry count for task
   - If retries < MAX_RETRIES (default: 3): send a follow-up message to the worker via your runtime's messaging mechanism: "Validation failed: <specific failure>. Fix and retry."
   - If retries >= MAX_RETRIES: mark task as blocked and escalate to user

**Minimal validation (when no metadata):**

If task has no explicit validation requirements, apply default checks:

```bash
# Check that worker wrote the expected files
git status --porcelain  # Should show unstaged changes from worker

# Check for obvious failures in recent output
# (agent should not have ended with errors)
```

**Example task with validation metadata:**

```
TaskCreate(
  subject="Add authentication middleware",
  description="...",
  metadata={
    "validation": {
      "files_exist": ["src/middleware/auth.py", "tests/test_auth.py"],
      "command": "pytest tests/test_auth.py -v",
      "content_check": {"file": "src/middleware/auth.py", "pattern": "def authenticate"}
    }
  }
)
```

## Step 5: Review & Finalize

When workers complete AND pass validation:
1. Check git status for changes (workers wrote files but did not commit)
2. Review diffs
3. Run any additional tests/validation
4. Team lead commits all changes for the wave (sole committer)

## Step 5a: Cleanup

After wave completes:

```bash
# Clean up result files from this wave (prevent stale reads in next wave)
rm -f .agents/swarm/results/*.json
```

Shut down all workers via your runtime's cleanup mechanism. Then clean up the agent group/team.

### Reaper Cleanup Pattern

Cleanup MUST succeed even on partial failures:

1. Request graceful shutdown for each worker
2. Wait up to 30s for acknowledgment
3. If any worker doesn't respond, log warning, proceed anyway
4. Always run cleanup — lingering agents pollute future sessions

### Timeout Configuration

| Timeout | Default | Description |
|---------|---------|-------------|
| Worker timeout | 180s | Max time for worker to complete its task |
| Shutdown grace period | 30s | Time to wait for shutdown acknowledgment |
| Wave timeout | 600s | Max time for entire wave before forced cleanup |

## Step 6: Repeat if Needed

If more tasks remain:
1. Check TaskList for next wave
2. Spawn a NEW wave worker set (new sub-agents or new team) for fresh context
3. Execute the next wave
4. Continue until all done

## Partial Completion

**Worker timeout:** 180s (3 minutes) default per worker.

**Timeout behavior:**
1. Log warning: "Worker {name} timed out on task {id}"
2. Mark task as failed with reason "timeout"
3. Add to retry queue for next wave
4. Continue with remaining workers

**Quorum:** Swarm does not require quorum -- each worker is independent.
Each completed task is accepted individually.
