---
name: swarm
description: 'Dispatch parallel agents. Triggers: "swarm", "dispatch parallel agents.", "swarm skill".'
practices:
- microservices
- team-topologies
- mythical-man-month
hexagonal_role: supporting
consumes:
- implement
- validate
produces:
- .agents/swarm/results/*.json
context_rel:
- kind: customer-of
  with: crank
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  tier: orchestration
  dependencies:
  - implement
  - validate
output_contract: .agents/swarm/results/*.json
---
# Swarm Skill

Spawn isolated agents to execute tasks in parallel. Fresh context per agent (Ralph Wiggum pattern).

## Before you swarm — pick the lightest path that fits

> **Confirm a swarm is even warranted.** A swarm pays off only with **≥2
> genuinely independent units of working-tree work** that benefit from
> isolation. If it's **one deliverable**, **pure analysis/investigation**, or
> **no working-tree edits**, do NOT swarm — use the lighter path below. Reaching
> for the machinery on a small task costs more than the task (real failure
> 2026-06-15: an ATM-codex swarm pointed at a ~9-idea content task wedged on
> codex boot; in-session Agent fan-out did it in one pass).

Three paths, lightest first — reach for the lightest that fits:

| Path | What it is | Use when |
|---|---|---|
| **In-session Agent/Task fan-out** (lightest) | Spawn 2–3 `Agent` subagents in *this* session. No persistence, no worktrees, no attach, dies with the session. Read-only-friendly. | One-shot parallel work: independent drafts, fan-out analysis, fresh-eyes review. **Default for anything small.** See [`automation-shape-routing`](../automation-shape-routing/SKILL.md) shape 0. |
| **`/swarm`** (middle) | Wave-gated working-tree execution with disjoint file ownership + conflict checks (this skill). | ≥2 independent units that **edit the working tree** and need isolation + wave-validity gating. |
| **ATM** ([`/using-atm`](../using-atm/SKILL.md), heaviest) | Persistent tmux panes + human attach/steer + multi-vendor, running whole `/rpi`/`/evolve` loops. | Long-lived epics needing persistence and live steering — **not** one-shot tasks. Boot cost (esp. codex) alone can exceed doing it inline. |

## Loop position

Move **5 (wave execution)** of the [operating loop](../../docs/architecture/operating-loop.md), specifically the parallel-fork primitive `/crank` invokes. Refuses to spawn parallel agents on a wave that has not cleared the wave-validity check in the [slice validation plan](../../docs/templates/slice-validation.md): write scopes must be disjoint, no shared migration/contract/CLI surface, integration order declared when it matters, one owner per slice, discard path per slice. Parallelism is explicit ownership, not swarm chaos. Default to sequential when the wave-validity rows are not all green.

**Coupled-chain rule (DERIVED-surface collision).** Two slices that both regenerate a shared *derived* surface — `cli-command-surface`, `registry.json`, `context-map`, or the codex manifest — **collide even if their source files are disjoint**. Such a coupled chain (`schema→write-model→export→gate→tests`) MUST run sequential: each link branches off the freshly-**MERGED** prior link's SHA, never a pre-fan snapshot. Reserve parallel waves for provably-disjoint surfaces, cap **4–6**.

**Integration modes:**
- **Direct** - Create TaskList tasks, invoke `/swarm`
- **Via Crank** - `/crank` creates tasks from beads, invokes `/swarm` for each wave

> **Requires multi-agent runtime.** Swarm needs a runtime that can spawn parallel subagents. If unavailable, work must be done sequentially in the current session.

## Architecture (Mayor-First)

```
Mayor (this session)
    |
    +-> Plan: TaskCreate with dependencies
    |
    +-> Identify wave: tasks with no blockers
    |
    +-> Select spawn backend (gc if available; runtime-native: Claude teams in Claude runtime, Codex sub-agents in Codex runtime; fallback tasks if unavailable)
    |
    +-> Assign: TaskUpdate(taskId, owner="worker-<id>", status="in_progress")
    |
    +-> Spawn workers via selected backend
    |       Workers receive pre-assigned task, execute atomically
    |
    +-> Wait for completion (wait() | SendMessage | TaskOutput)
    |
    +-> Validate: Review changes when complete
    |
    +-> Cleanup backend resources (close_agent | TeamDelete | none)
    |
    +-> Repeat: New team + new plan if more work needed
```

## Execution

Read [references/execution-steps.md](references/execution-steps.md) when you need the full procedural detail (Steps 0–6): backend detection, gc dispatch, task typing + file manifests, context briefing, manifest auto-population, advisory bead clustering, wave identification, pre-spawn conflict check, test-file naming validation, multi-wave base-SHA refresh, and worker dispatch.

Every TaskCreate **must** include `metadata.issue_type` plus a `metadata.files` array.
Do not spawn workers with overlapping file manifests into the same shared-worktree wave.

## Example Flow

```
Mayor: "Let's build a user auth system"

1. /plan -> Creates tasks:
   #1 [pending] Create User model
   #2 [pending] Add password hashing (blockedBy: #1)
   #3 [pending] Create login endpoint (blockedBy: #1)
   #4 [pending] Add JWT tokens (blockedBy: #3)
   #5 [pending] Write tests (blockedBy: #2, #3, #4)

2. /swarm -> Spawns agent for #1 (only unblocked task)

3. Agent #1 completes -> #1 now completed
   -> #2 and #3 become unblocked

4. /swarm -> Spawns agents for #2 and #3 in parallel

5. Continue until #5 completes

6. /validate -> Validate everything
```

### Scope-Escape Protocol

When a worker discovers work outside their assigned scope, they MUST NOT modify files outside their file manifest. Instead, append to `.agents/swarm/scope-escapes.jsonl`:

```json
{"worker": "<worker-id>", "finding": "<description>", "suggested_files": ["path/to/file"], "timestamp": "<ISO8601>"}
```

For richer scope-escape narration (status classification, concrete next step, evidence), see [references/scope-escape-template.md](references/scope-escape-template.md). Use the template when a single-line JSONL entry is insufficient for the operator to act on.

The lead reviews scope escapes after each wave and creates follow-up tasks as needed.

## Key Points

- **Runtime-native local mode** - Auto-selects the native backend for the current runtime (gc pool, Claude teams, or Codex sub-agents)
- **Universal orchestration contract** - Same swarm behavior across Claude and Codex sessions
- **Pre-assigned tasks** - Mayor assigns tasks before spawning; workers never race-claim
- **Fresh worker contexts** - New sub-agents/teammates per wave preserve Ralph isolation
- **Wave execution** - Only unblocked tasks spawn
- **Mayor orchestrates** - You control the flow, workers write results to disk
- **Thin results** - Workers write `.agents/swarm/results/<id>.json`, orchestrator reads files (NOT Task returns or SendMessage content)
- **Retry via message/input** - Use `send_input` (Codex) or `SendMessage` (Claude) for coordination only
- **Atomic execution** - Each worker works until task done
- **Graceful degradation** - If multi-agent unavailable, work executes sequentially in current session

## Worker report contract + lane authority (cp-hhd7, cards 4 + 16)

### Worker FINAL REPORT — what the orchestrator requires

Workers write raw evidence; the orchestrator does not trust prose summaries.
Each `.agents/swarm/results/<id>.json` or the worker's final message SHOULD include:

```
files_changed: [list of exact paths]
commit_sha: <git rev-parse HEAD>   # or "no commit" + reason
test_tail: <verbatim last N lines of test output>
conflicts_surfaced: [list, or "none"]
```

A result missing `commit_sha` or `test_tail` is treated as **unverified** until the
orchestrator independently confirms persistence and test passage. The audit cost of
trusting summaries exceeds the cost of requiring the fields.

### Lane authority (POLICY, card 4 — applies when running a two-lane swarm)

In a multi-lane deployment (this is a **contextual policy**, not a universal swarm
rule), a decision inside one lane's scope is decided by that lane with the other lane's
view as input — not a vote, not an escalation. Escalate to the human operator only for:
- genuine out-of-both-lanes decision forks
- gate violations
- a loop that cannot self-heal

Escalating an in-lane sequencing call makes the operator a bottleneck. The lane
authority rule is enforced by convention in the control-plane context; the mechanism
(swarm) is general.

## Workflow Integration

This ties into the full workflow:

```
/research -> Understand the problem
/plan -> Decompose into beads issues
/crank -> Autonomous epic loop
    +-- /swarm -> Execute each wave in parallel
/validate -> Validate results
/post-mortem -> Extract learnings
```

**Direct use (no beads):**
```
TaskCreate -> Define tasks
/swarm -> Execute in parallel
```

The knowledge flywheel captures learnings from each agent.

## Task Management Commands

```
# List all tasks
TaskList()

# Mark task complete after notification
TaskUpdate(taskId="1", status="completed")

# Add dependency between tasks
TaskUpdate(taskId="2", addBlockedBy=["1"])
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--max-workers=N` | Max concurrent workers | 5 |
| `--from-wave <json-file>` | Load wave from OL hero hunt output (see OL Wave Integration) | - |
| `--per-task-commits` | Commit per task instead of per wave (for attribution/audit) | Off (per-wave) |

## When to Use Swarm

| Scenario | Use |
|----------|-----|
| **Single one-shot deliverable / no working-tree edits / read-only investigation** | **do NOT swarm** → do it inline, or fan out 2–3 in-session **Agent** subagents (see "Before you swarm" above + [`automation-shape-routing`](../automation-shape-routing/SKILL.md) shape 0) |
| Multiple independent tasks | `/swarm` (parallel) |
| Sequential dependencies | `/swarm` with blockedBy |
| Mix of both | `/swarm` spawns waves, each wave parallel |

## Why This Works: Ralph Wiggum Pattern

Follows the [Ralph Wiggum Pattern](https://ghuntley.com/ralph/): **fresh context per execution unit**.

- **Wave-scoped worker set** = spawn workers -> execute -> cleanup -> repeat (fresh context each wave)
- **Mayor IS the loop** - Orchestration layer, manages state across waves
- **Workers are atomic** - One task, one spawn, one result
- **TaskList as memory** - State persists in task status, not agent context
- **Filesystem for EVERYTHING** - Code artifacts AND result status written to disk, not passed through context
- **Backend messaging for signals only** - Short coordination signals (under 100 tokens), never work details

Ralph alignment source: `../shared/references/ralph-loop-contract.md`.

## Integration with Crank

When `/crank` invokes `/swarm`: Crank bridges beads to TaskList, swarm executes with fresh-context agents, crank syncs results back.

| You Want | Use | Why |
|----------|-----|-----|
| Fresh-context parallel execution | `/swarm` | Each spawned agent is a clean slate |
| Autonomous epic loop | `/crank` | Loops waves via swarm until epic closes |
| Just swarm, no beads | `/swarm` directly | TaskList only, skip beads |
| RPI progress gates | `/flywheel` | Tracks progress; does not execute work |

---

## Codex-Specific Coordination (folded from codex-team)

When the selected backend is **Codex** (Codex CLI on PATH or Codex sub-agents), swarm follows the same wave model, but Codex agents are **headless** — they cannot negotiate locks or wait turns. **The orchestrator IS the lock manager:** all conflict prevention happens before spawning, via file-target analysis.

**Backend selection (Codex path), in order:**
1. `spawn_agent` available → Codex experimental sub-agents (preferred)
2. Codex CLI available → Codex CLI via Bash (`codex exec ...`)
3. `skill` tool read-only (OpenCode) → OpenCode subagents (`task(subagent_type="general", ...)`)
4. None → fall back to the runtime-native swarm backend / sequential

**Decision-time warning (codex boot-wedge).** A Codex-backed swarm adds a **boot-failure surface** — codex can fail to boot at all (the actual 2026-06-15 failure: a codex swarm wedged before doing any work). For a small task the boot cost alone can exceed doing it inline, so first confirm the swarm is warranted (see "Before you swarm" at the top), then run the pre-flight below *before committing* to a Codex swarm.

**Pre-flight (CLI backend only):** verify `which codex`, then test the configured default model with `codex exec --full-auto -C "$(pwd)" "echo ok"`. If either fails, fall back to another backend.

**Canonical command + flag order:**
```bash
codex exec --full-auto -C "$(pwd)" -o <output-file> "<prompt>"
```
Flag order: `--full-auto` → `-C` → `-o` → prompt (insert `-m "<model>"` before `-C` only when intentionally pinning a model; otherwise the user's default is used). **Valid flags:** `--full-auto`, `-m`, `-C`, `-o`, `--json`, `--output-schema`, `--add-dir`, `-s`. **Do NOT use** `-q` / `--quiet` (don't exist).

- **Cross-project tasks:** `--add-dir /path/to/other/repo` (repeatable) grants access beyond `-C`.
- **Progress monitoring:** add `--json` to stream JSONL events (`turn.started` / `turn.completed` with token `usage`). No events for 60s → agent likely stuck.
- **Sandbox levels (`-s`):** `read-only` for judges/reviewers, `workspace-write` (default with `--full-auto`), `danger-full-access` only in externally-sandboxed environments. Prefer `-s read-only` for review/analysis tasks.

**File-target strategy (apply before spawning Codex agents):**

| File Overlap | Strategy | Action |
|-------------|----------|--------|
| All tasks touch same file | **Merge** | Combine into 1 agent with all fixes |
| Some tasks share files | **Multi-wave** | Shared-file tasks go sequential across waves |
| No overlap | **Parallel** | Spawn all agents at once |

For multi-wave Codex runs, the lead reads prior-wave results, then injects a *summary* of what changed (not raw diffs) into the next wave's prompts. **Limits:** ≤6 agents/wave, 120s default timeout, ≤3 waves (reconsider decomposition beyond that). Output dir: `.agents/codex-team/` (or the standard `.agents/swarm/results/`). Concrete tool calls: [references/backend-codex-subagents.md](references/backend-codex-subagents.md).

**Codex troubleshooting:** `codex` not found → `npm i -g @openai/codex` or fall back; default model unavailable → verify the `echo ok` pre-flight or pin with `-m`; empty/missing output → ensure the `-o` directory exists and is writable.

---

## OL Wave Integration

Read [references/ol-wave-integration.md](references/ol-wave-integration.md) when you invoke `/swarm --from-wave <json-file>` — covers pre-flight `ol` CLI check, input JSON format, task creation from wave entries, completion backflow via `ol hero ratchet`, and example flow.

---

## References

- **Executable acceptance:** [references/swarm.feature](references/swarm.feature) — wave-validity gate, fresh-context workers, conflict-free ownership, results+cleanup (soc-qk4b)
- **Local Mode Details:** `skills/swarm/references/local-mode.md`
- **Validation Contract:** `skills/swarm/references/validation-contract.md`

---

## Examples

### Building a User Auth System

**User says:** `/swarm`

**What happens:**
1. Agent identifies unblocked tasks from TaskList (e.g., "Create User model")
2. Agent selects spawn backend using runtime-native priority (Claude session -> Claude teams; Codex session -> Codex sub-agents)
3. Agent spawns worker for task #1, assigns ownership via TaskUpdate
4. Worker completes, team lead validates changes
5. Agent identifies next wave (tasks #2 and #3 now unblocked)
6. Agent spawns two workers in parallel for Wave 2

**Result:** Multi-wave execution with fresh-context workers per wave, zero race conditions.

### Direct Swarm Without Beads

**User says:** Create three tasks for API refactor, then `/swarm`

**What happens:**
1. User creates TaskList tasks with TaskCreate
2. Agent calls `/swarm` without beads integration
3. Agent identifies parallel tasks (no dependencies)
4. Agent spawns all three workers simultaneously
5. Workers execute atomically, report to team lead via SendMessage or task completion
6. Team lead validates all changes, commits once per wave

**Result:** Parallel execution of independent tasks using TaskList only.

---

## Worktree Isolation (Multi-Epic Dispatch)

Read [references/shared-checkout-discipline.md](references/shared-checkout-discipline.md) **first** when the target checkout (`~/dev/<repo>`) is shared with peer agents — it documents when worktrees are mandatory (vs. optional) and the three failure modes (branch-deletion data loss, swarm attribution confounded, destructive-recovery temptation) that motivate the discipline.

Read [references/worktree-isolation.md](references/worktree-isolation.md) when you need to dispatch workers across multiple epics or run waves with overlapping files — covers isolation semantics per backend, effort levels, post-spawn verification, manual worktree creation/routing/merge-back, the Merge Arbiter Protocol, cleanup, and the `--worktrees` / `--no-worktrees` parameters.

**Worktree reaping (teardown).** After a worker's PR is confirmed **MERGED** (`gh pr view --json state` = `MERGED`), reap its tree: `git worktree remove <path> --force` then `git worktree prune`. **Leave unmerged-PR worktrees intact.** Target zero orphaned worktrees — bound the live count to in-flight PRs. (The committed disposition gate scans the tracked file set, not live on-disk worktrees, so this teardown is the operational backstop.)

---

## Troubleshooting

Read [references/troubleshooting.md](references/troubleshooting.md) for full diagnostics.

| Problem | See |
|---------|-----|
| Worktree isolation did not engage | [references/troubleshooting.md](references/troubleshooting.md) |
| Workers produce file conflicts | [references/troubleshooting.md](references/troubleshooting.md) |
| Team creation fails | [references/troubleshooting.md](references/troubleshooting.md) |
| Codex agents unavailable | [references/troubleshooting.md](references/troubleshooting.md) |
| Workers timeout or hang | [references/troubleshooting.md](references/troubleshooting.md) |
| gc backend detected but workers unresponsive | [references/troubleshooting.md](references/troubleshooting.md) |
| Tasks assigned but workers never spawn | [references/troubleshooting.md](references/troubleshooting.md) |

## Related skills

- [`/using-atm`](../using-atm/SKILL.md) — out-of-session ATM substrate when a swarm needs persistent panes and human attach/steer.

## Reference Documents

- [references/shared-checkout-discipline.md](references/shared-checkout-discipline.md)
- [references/agent-genie-coordination-contract.md](references/agent-genie-coordination-contract.md) — Eight-field contract each parallel stream declares before claiming work- [references/conflict-recovery.md](references/conflict-recovery.md)
- [references/cold-start-contexts.md](references/cold-start-contexts.md)
- [references/backend-background-tasks.md](references/backend-background-tasks.md)
- [references/backend-claude-teams.md](references/backend-claude-teams.md)
- [references/backend-codex-subagents.md](references/backend-codex-subagents.md)
- [references/backend-inline.md](references/backend-inline.md)
- [references/claude-code-latest-features.md](references/claude-code-latest-features.md)
- [references/execution-steps.md](references/execution-steps.md)
- [references/local-mode.md](references/local-mode.md)
- [references/ol-wave-integration.md](references/ol-wave-integration.md)
- [references/ralph-loop-contract.md](references/ralph-loop-contract.md)
- [references/troubleshooting.md](references/troubleshooting.md)
- [references/validation-contract.md](references/validation-contract.md)
- [references/worker-pitfalls.md](references/worker-pitfalls.md)
- [references/worker-specs.md](references/worker-specs.md)
- [references/worktree-isolation.md](references/worktree-isolation.md)
- [../shared/references/backend-background-tasks.md](../shared/references/backend-background-tasks.md)
- [../shared/references/backend-claude-teams.md](../shared/references/backend-claude-teams.md)
- [../shared/references/backend-codex-subagents.md](../shared/references/backend-codex-subagents.md)
- [../shared/references/backend-inline.md](../shared/references/backend-inline.md)
- [../shared/references/claude-code-latest-features.md](../shared/references/claude-code-latest-features.md)
- [references/pre-spawn-friction-gates.md](references/pre-spawn-friction-gates.md)
- [references/scope-escape-template.md](references/scope-escape-template.md)
- [references/worker-pre-task-checks.md](references/worker-pre-task-checks.md)
- [../shared/references/ralph-loop-contract.md](../shared/references/ralph-loop-contract.md)
