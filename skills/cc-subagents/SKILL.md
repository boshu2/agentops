---
name: cc-subagents
user-invocable: false
description: |-
  Use when dispatching scoped Claude Code subagents with worktrees, roles, tools, memory, and evidence gates.
  Triggers:
practices:
- team-topologies
- mythical-man-month
hexagonal_role: supporting
consumes: []
produces:
- subagent-dispatch-plan
context_rel:
- kind: supplier-to
  with: cc-loop-driver
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: none
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: orchestration
  dependencies: [ntm, cc-hooks]
  external_dependencies: [agent-fungibility-philosophy]
  stability: stable
output_contract: "Spawn plan (table of role → model/tools/effort/isolation/background) + per-worker Task invocations + SubagentStop gate; no client-facing artifacts."
---

# cc-subagents

Operator-side skill for driving **fungible** Claude Code workers — the Task tool plus reusable `.claude/agents/*.md` definitions — with correct background, isolation, model/tools/memory, and effort scoping, plus a clean SubagentStop gate. This drives the harness; it is never client-facing.

## Overview / When to Use

A subagent is a Task-tool worker that runs in its **own context window** with its own tool set. It returns one final report to the orchestrator; the orchestrator never sees its intermediate transcript. That isolation is the whole value — and the whole hazard.

Two ways to spawn:
1. **Inline ad-hoc** — call the Task tool with a `subagent_type` of `general-purpose` (or a defined agent) and a self-contained prompt. Fast, no file needed.
2. **Defined teammate** — author `.claude/agents/<role>.md` once, then spawn it by name. Reusable, version-controlled, scoped. Confirm it loads with `claude agents` before a multi-agent run.

Use this skill to decide **whether** to spawn, **which** role profile to use, and **how** to gate the return. It composes with `ntm` (when you outgrow Task and need persistent tmux panes), `cc-hooks` (for the SubagentStop gate), and `agent-fungibility-philosophy` (the why: interchangeable workers over bespoke ones).

## ⚠️ Critical Constraints

- **Rule 1: Never use `claude -p` / `--print` to spawn a worker.** **Why:** `-p` bills the API per token, not the Max subscription — it silently burns money. Use the Task tool (in-session, on the sub) or NTM panes / `codex exec` for out-of-session workers. (Overnight factory runs have burned API this exact way.)
- **Rule 2: Define non-overlapping file ownership before spawning parallel workers.** **Why:** subagents run blind to each other — two workers editing the same file race and clobber. If two tasks touch one file, combine them into one worker. Use `isolation: worktree` when overlap is unavoidable.
- **Rule 3: A subagent gets a fresh context — it carries NO conversation state.** **Why:** the worker only knows what you put in its prompt + its CLAUDE.md/memory. Pass every fact it needs (paths, the spec, the acceptance check). "As we discussed" means nothing to it.
- **Rule 4: Read-only / least-tools by default.** **Why:** a fungible worker should hold only the tools its role needs. An explorer gets Read/Grep/Glob, not Edit/Bash. Scope `tools:` in the agent file; an over-privileged worker is a blast-radius bug.
- **Rule 5: Independently verify a worker's "done."** **Why:** the orchestrator can't see the worker's reasoning, only its claim. Re-run the test or re-read the artifact (a SubagentStop hook or a separate validator) — agents are biased toward "looks good."

## Workflow / Methodology

### Phase 1: Decide — spawn or inline?

Spawn a subagent **only** when at least one is true:
- The work is **independent** and parallelizable (N tasks, no shared write target).
- It is **long-running / exploratory** and would pollute or blow the orchestrator's context (large codebase sweep, research).
- It needs **write isolation** (a worktree) so parallel edits can't collide.

Inline it when the task is sequential, small, or tightly coupled to current context — spawning costs a full context load and a round-trip.

**Checkpoint:** State the parallelism and file-ownership map out loud before spawning. If two workers would touch one file, collapse to one worker.

### Phase 2: Pick the role profile

Map the role to frontmatter (see `references/agent-profiles.md` for full examples):

| Role | model | tools | effort | background | isolation |
|------|-------|-------|--------|------------|-----------|
| Explorer / research | fast (haiku/sonnet) | Read, Grep, Glob | low | true | none |
| Judge / validator | sonnet | Read, Grep, Bash(test) | low–medium | true | none |
| Implementor | opus/sonnet | Read, Edit, Write, Bash | high | per-task | worktree |
| Long background teammate | sonnet | role-scoped | medium | true | worktree |

`effort` right-sizes reasoning: `low` for explorers/judges, `high` for implementors/hard debugging (set via /effort or the agent profile). Use `memory:` to scope what durable context the role loads.

**Checkpoint:** Confirm each parallel worker's `tools:` is the minimum for its job and write-workers carry `isolation: worktree`.

### Phase 3: Spawn

- **Defined teammate:** ensure `.claude/agents/<role>.md` exists, run `claude agents` to confirm it loads, then dispatch by `subagent_type`.
- **Background:** set `background: true` so the orchestrator keeps working while the teammate runs; it re-engages when the worker finishes.
- **Worktree:** `isolation: worktree` gives the worker its own git worktree (optionally `worktree.sparsePaths` to limit checkout in a big monorepo). Fall back to manual `git worktree` only on runtimes without native support.
- Give each worker a **self-contained prompt**: the exact files it owns, the spec, and the acceptance check it must satisfy.

**Checkpoint:** All workers dispatched; orchestrator has the list of expected return artifacts per worker.

### Phase 4: Handle SubagentStop / collect

- Wire a **SubagentStop** hook (via `cc-hooks`) to run on each worker's completion: lint the diff, run the worker's test, or block-and-report if the acceptance check fails. (Related events: `TaskCompleted`, `TeammateIdle` for background teammates.)
- The orchestrator reads each worker's **final report only** — never assume the transcript. Re-verify load-bearing claims independently.
- After **all** workers complete, run the repo's full validation gate once (not a remembered subset), then integrate worktree branches.

**Checkpoint:** Every worker's output verified by evidence (test/read), not self-report, before integration.

## Output Specification

**Format:** markdown (the spawn plan + verdicts) — operator-facing only.
**Filename:** none required; if persisted, `.agents/briefings/cc-subagents-<task>-<date>.md`.
**Structure:**
- A **spawn plan** table: role → model · tools · effort · background · isolation · owned files.
- The per-worker **Task invocations** (subagent_type + self-contained prompt).
- The **SubagentStop gate** definition (hook command or validator) and final integration verdict.

Never emit client-facing artifacts; this skill drives the harness. Apply `jargon-translator` if any byproduct could leak to a client.

## Quality Rubric

- [ ] Every spawn justified by Phase-1 criteria (independent / long-running / needs isolation) — no gratuitous subagents.
- [ ] No worker uses `claude -p` / `--print`.
- [ ] Non-overlapping file ownership declared; write-workers carry `isolation: worktree`.
- [ ] Each worker's `tools:` is the minimum for its role; explorers/judges are read-only.
- [ ] `effort` set per role (low for explore/judge, high for implement).
- [ ] Each worker prompt is self-contained (paths + spec + acceptance check); no reliance on conversation state.
- [ ] Background teammates set `background: true`; `claude agents` confirms defined profiles load.
- [ ] SubagentStop (or a separate validator) verifies each worker by evidence before integration; full repo gate run once after all complete.

## Examples

**Fan-out per-file audit:** 6 source files, one read-only explorer per file (`model: haiku`, `tools: Read, Grep, Glob`, `effort: low`, `background: true`). No isolation (read-only). Collect 6 reports, synthesize.


**Inline instead:** "fix this one typo in README" — do NOT spawn; edit inline. Spawning would cost a full context load for a one-line change.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Workers clobbered each other's edits | Overlapping file ownership, no isolation | Combine into one worker, or add `isolation: worktree` and merge after |
| Worker "did nothing useful" | Prompt assumed conversation state | Make the prompt self-contained: paths, spec, acceptance check |
| API charges spiked overnight | A worker spawned via `claude -p` | Banned — use Task / NTM panes / `codex exec`; add a dcg/guard rule |
| Defined agent not picked up | `.claude/agents/<role>.md` not loaded | Run `claude agents`; check name matches `subagent_type`; verify settings precedence |
| Worker reported done but work is broken | Trusted self-report | Wire SubagentStop to re-run the test / re-read the artifact; verify by evidence |
| Orchestrator blocked while worker runs | Foreground spawn | Set `background: true` so the orchestrator continues and re-engages on completion |

## See Also / References

- `references/agent-profiles.md` — full `.claude/agents/*.md` frontmatter examples per role.
- `claude-code-latest-features.md` (`skills/shared/references/`) — canonical Task/agent/worktree/hook primitives.
- `ntm` skill — when you outgrow Task and need persistent tmux panes (out-of-session workers).
- `cc-hooks` skill — authoring the SubagentStop / TaskCompleted gate.
- `agent-fungibility-philosophy` skill — why interchangeable workers beat bespoke ones.
- Upstream: `https://code.claude.com/docs/en/sub-agents`, `https://code.claude.com/docs/en/hooks`, `https://code.claude.com/docs/en/cli-reference`.

## References
- [agent-profiles](references/agent-profiles.md)
