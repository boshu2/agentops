# Swarm Execution Steps (Detailed)

This reference contains the detailed procedural steps for swarm execution (Steps 0 through 6). The SKILL.md billboard points here when you need concrete spawn/wait/message/cleanup procedures, file-manifest enforcement, conflict checks, and multi-wave base-SHA refresh logic.

## Step 0: Detect Multi-Agent Capabilities (MANDATORY)

Use runtime capability detection, not hardcoded tool names. Swarm requires:
- **Spawn parallel subagents** — create workers that run concurrently
- **Agent messaging** (optional) — for coordination and retry

See `skills/shared/SKILL.md` for the capability contract.

**After detecting your backend, read the matching reference for concrete spawn/wait/message/cleanup examples:**
- Shared Claude feature contract → `skills/shared/references/claude-code-latest-features.md`
- Local mirrored contract for runtime-local reads → `claude-code-latest-features.md`
- Claude Native Teams → `backend-claude-teams.md`
- Codex Sub-Agents / CLI → `backend-codex-subagents.md`
- Background Tasks → `backend-background-tasks.md`
- Inline (no spawn) → `backend-inline.md`

See also `local-mode.md` for swarm-specific execution details (worktrees, validation, git commit policy, wave repeat).

## Step 0.5: Select Spawn Backend (Before Worker Dispatch)

Select the worker-dispatch backend by the ladder **NTM > runtime-native > beads floor** (see `skills/shared/SKILL.md` "Selection policy" for the canonical statement):

```bash
if [[ "${AGENTOPS_ORCHESTRATION:-}" == "off" ]]; then
    SWARM_BACKEND="beads"   # global opt-out — single-agent inline, work tracked via bd
elif command -v ntm &>/dev/null && ntm --robot-capabilities 2>/dev/null | jq -e '.spawn == true' >/dev/null 2>&1; then
    SWARM_BACKEND="ntm"     # top tier — capability-probed
else
    SWARM_BACKEND="native"  # runtime-native: Claude teams / Codex sub-agents; beads floor if neither
fi
```

- `AGENTOPS_ORCHESTRATION=off` is the global opt-out (mirrors `AGENTOPS_HOOKS_DISABLED=1`): no spawn backend, degrade to the beads floor.
- `SWARM_BACKEND="ntm"`: dispatch and monitor workers through NTM's robot API.
- `SWARM_BACKEND="native"`: spawn via Claude Native Teams or Codex sub-agents per the runtime; if neither is available, fall to the beads floor (`--quick`).
- **Output-contract parity holds on every tier:** workers write results to `.agents/swarm/results/`, lead verifies-then-trusts. Workers already use `bd` for issue tracking — no change needed.

> **`gc` is NOT a selectable backend (DEPRECATION).** The Gas City (`gc`) CLI bridge was removed (soc-2rtm0); `runtime=gc` is rejected by the CLI (see `agentops/CLAUDE.md`). The `SWARM_BACKEND="gc"` dispatch block later in this file is retained for historical reference only — it is never selected. The top tier is **NTM**.

## Step 1: Ensure Tasks Exist

Use TaskList to see current tasks. If none, create them:

```
TaskCreate(subject="Implement feature X", description="Full details...",
  metadata={"issue_type": "feature", "files": ["src/feature_x.py", "tests/test_feature_x.py"], "validation": {...}})
TaskUpdate(taskId="2", addBlockedBy=["1"])  # Add dependencies after creation
```

### Task Typing + File Manifest

Every TaskCreate **must** include `metadata.issue_type` plus a `metadata.files` array. `issue_type` drives active constraint applicability and validation policy; `files` enable mechanical conflict detection before spawning a wave.
This is how the prevention ratchet applies shift-left mechanically: active compiled findings use issue type plus changed files to decide whether a task should be blocked, warned, or left alone.

- Use canonical issue types: `feature`, `bug`, `task`, `docs`, `chore`, `ci`.
- Preserve the same `metadata.issue_type` on TaskUpdate / TaskCompleted payloads so task-validation can apply active constraints without guessing.
- Pull file lists from the plan, issue description, or codebase exploration during planning.
- If you cannot enumerate files yet, add a planning step to identify them before spawning workers. An empty or missing manifest signals the need for more planning, not unconstrained workers.
- Workers receive the manifest in their prompt and are instructed to stay within it (see `local-mode.md` worker prompt template).
- The worker prompt MUST include the `metadata.files` array as the FILE MANIFEST section. Workers grep for existing function signatures before writing new code to avoid duplication.
- Per-worker model/tool/prompt isolation specs: see [`worker-specs.md`](worker-specs.md) and [`schemas/worker-spec.v1.schema.json`](../../../schemas/worker-spec.v1.schema.json). When a wave's tasks declare a `metadata.worker_spec` reference, the spawned worker honors the named spec's model/tool/prompt allowlist instead of inheriting the lead agent's surface.

```json
{
  "issue_type": "feature",
  "files": ["cli/cmd/ao/goals.go", "cli/cmd/ao/goals_test.go"],
  "validation": {
    "tests": "go test ./cli/cmd/ao/...",
    "files_exist": ["cli/cmd/ao/goals.go"]
  }
}
```

## Step 1a: Build Context Briefing (Before Worker Dispatch)

```bash
if command -v ao &>/dev/null; then
    ao context assemble --task='<swarm objective or wave description>'
fi
```

This produces a 5-section briefing (GOALS, HISTORY, INTEL, TASK, PROTOCOL) at `.agents/rpi/briefing-current.md` with secrets redacted. Include the briefing path in each worker's TaskCreate description so workers start with full project context.

**Output schema size guard:** When 5+ workers in a wave share the same output schema (e.g., `verdict.json`), cache it to `.agents/council/output-schema.json` and reference by path instead of inlining ~500 tokens per worker. For ≤4 workers, inline is fine. See council skill's caching guidance reference for details.

Worker prompt signpost:
- Claude workers should include: `Knowledge artifacts are in .agents/. See .agents/AGENTS.md for navigation. Use \`ao lookup --query "topic"\` for learnings.`
- Codex workers cannot rely on `.agents/` file access in sandbox. The lead should search `.agents/learnings/` for relevant material and inline the top 3 results directly in the worker prompt body.

## Step 1.5: Auto-Populate File Manifests

**Skip this step if all tasks already have populated `metadata.files` arrays.**

If any task is missing its file manifest, auto-generate it before Step 2:

1. **Spawn haiku Explore agents** (one per task missing manifests) to identify files:
   ```
   Agent(subagent_type="Explore", model="haiku",
     prompt="Given this task: '<task subject + description>', identify all files
     that will need to be created or modified. Return a JSON array of file paths.")
   ```

2. **Inject manifests** back into tasks:
   ```
   TaskUpdate(taskId=task.id, metadata={"files": [explored_files]})
   ```

Once all tasks have manifests, proceed to Step 2 where the Pre-Spawn Conflict Check enforces file ownership.

## Step 1.6: Advisory Bead Clustering

When tasks come from bd and `scripts/bd-cluster.sh` exists, run `scripts/bd-cluster.sh --json 2>/dev/null || true` before Step 2. Summarize any clusters as consolidation hints only; never run `--apply` here, and keep Step 2's file-manifest and dependency gates authoritative.

## Step 2: Identify Wave

**Pre-Spawn Friction Gates:** Before spawning workers, execute all 6 friction gates (base sync, file manifest, dependency graph, misalignment breaker, wave cap, base-SHA ancestry). See `pre-spawn-friction-gates.md`.

Find tasks that are:
- Status: `pending`
- No blockedBy (or all blockers completed)

These can run in parallel.

### Pre-Spawn Conflict Check

Before spawning a wave, scan all worker file manifests for overlapping files:

```
wave_tasks = [tasks with status=pending and no blockers]
all_files = {}
for task in wave_tasks:
    for f in task.metadata.files:
        if f in all_files:
            CONFLICT: f is claimed by both all_files[f] and task.id
        all_files[f] = task.id
```

**On conflict detection:**
- **Serialize** the conflicting workers into separate sub-waves (preferred -- simplest fix), OR
- **Isolate** them with worktree isolation (`--worktrees`) so each operates on a separate branch.

Do not spawn workers with overlapping file manifests into the same shared-worktree wave. This is the primary cause of build breaks and merge conflicts in parallel execution.

**Display ownership table** before spawning:
```
File Ownership Map (Wave N):
┌─────────────────────────────┬──────────┬──────────┐
│ File                        │ Owner    │ Conflict │
├─────────────────────────────┼──────────┼──────────┤
│ src/auth/middleware.go      │ task-1   │          │
│ src/auth/middleware_test.go │ task-1   │          │
│ src/api/routes.go           │ task-2   │          │
│ src/config/settings.go      │ task-1,3 │ YES      │
└─────────────────────────────┴──────────┴──────────┘
Conflicts: 1 (resolved: serialized task-3 into sub-wave 2)
```

### Test File Naming Validation

When workers create new test files, validate naming against loaded standards:

1. **Detection:** Same language detection as /crank (go.mod → Go, pyproject.toml → Python, etc.)
2. **Validation:** Load the Testing section of the relevant standard. For Go, this means:
   - New test files must match `<source>_test.go` or `<source>_extra_test.go`
   - Reject `cov*_test.go` or arbitrary prefixes
3. **Serial-first for monolith packages:** If multiple workers target the same package AND that package has a shared `testutil_test.go` or `>5` existing test files, force serial execution within that package.

## Step 2.5: Pre-Spawn Base-SHA Refresh (Multi-Wave Only)

When executing wave 2+ (not the first wave), verify workers branch from the latest commit — not a stale SHA from before the prior wave's changes were committed.

```bash
# PSEUDO-CODE
# Capture current HEAD after prior wave's commit
CURRENT_SHA=$(git rev-parse HEAD)

# If using worktrees, verify they're up to date
if [[ -n "$WORKTREE_PATH" ]]; then
    (cd "$WORKTREE_PATH" && git pull --rebase origin "$(git branch --show-current)" 2>/dev/null || true)
fi
```

**Cross-reference prior wave diff against current wave file manifests:**

```bash
# PSEUDO-CODE
# Files changed in prior wave
PRIOR_WAVE_FILES=$(git diff --name-only "${WAVE_START_SHA}..HEAD")

# Check for overlap with current wave manifests
for task in $WAVE_TASKS; do
    TASK_FILES=$(echo "$task" | jq -r '.metadata.files[]')
    OVERLAP=$(comm -12 <(echo "$PRIOR_WAVE_FILES" | sort) <(echo "$TASK_FILES" | sort))
    if [[ -n "$OVERLAP" ]]; then
        echo "WARNING: Task $task touches files modified in prior wave: $OVERLAP"
        echo "Workers MUST read the latest version (post-prior-wave commit)"
    fi
done
```

**Why:** Without base-SHA refresh, wave 2+ workers may read stale file versions from before wave 1 changes were committed. This causes workers to overwrite prior wave edits or implement against outdated code. See crank Step 5.7 (wave checkpoint) for the SHA tracking pattern.

## Steps 3-6: Spawn Workers, Validate, Finalize

**For detailed local mode execution (team creation, worker spawning, race condition prevention, git commit policy, validation contract, cleanup, and repeat logic), read `local-mode.md`.**

> **Platform pitfalls:** Include relevant pitfalls from `worker-pitfalls.md` in worker prompts for the target language/platform. For example, inject the Bash section for shell script tasks, the Go section for Go tasks, etc. This prevents common worker failures from known platform gotchas.

> **Pre-task checks:** Inject the Quick-Reference Inject Block from `worker-pre-task-checks.md` into every worker dispatch prompt — grep-for-existing-impls, file-manifest existence, deletion-adjacent symbol verify. Prevents workers from duplicating existing utilities or operating on stale plan symbols.

### gc Worker Dispatch — DEPRECATED, historical reference only

> **gc tier removed (soc-2rtm0); retained for historical reference only — NOT selected.** The Gas City (`gc`) CLI bridge was severed and `runtime=gc` is rejected by the CLI (see `agentops/CLAUDE.md`). Step 0.5 never sets `SWARM_BACKEND="gc"`. The top tier is **NTM**. The block below documents the old gc dispatch shape for archival purposes only — do not select or invoke it.

When gc was the selected backend (no longer reachable), dispatch and monitoring went through gc sessions instead of Claude teams or Codex sub-agents:

```bash
# Dispatch a task to a gc-managed worker
gc session nudge <worker-alias> "Implement task #<id>: <subject>. Files: <manifest>. Write results to .agents/swarm/results/<id>.json"

# Monitor worker progress
gc session peek <worker-alias> --lines 50

# Check all worker statuses
gc status --json | jq '.sessions[] | {alias, state, last_activity}'
```

**gc dispatch follows the same orchestration contract as native backends:**
- Pre-assigned tasks (mayor assigns before nudge)
- File manifest enforcement (included in nudge prompt)
- Results written to `.agents/swarm/results/<id>.json`
- Lead-only commit policy (workers do not commit)
- Scope-escape protocol (workers append to `.agents/swarm/scope-escapes.jsonl`)

**gc-specific behaviors:**
- Worker lifecycle managed by gc pool auto-scaling — no explicit cleanup needed
- Use `gc session peek` for progress checks instead of `SendMessage` / `send_input`
- If a worker is idle or unresponsive, `gc session nudge` can re-prompt it
- gc sessions persist across waves — the same worker alias can be reused without respawning
