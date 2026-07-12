---
name: status
description: "Show AgentOps project status."
---
# $status — Workflow Dashboard

> **Purpose:** Produce a one-screen, evidence-backed answer to: what is active, what passed or failed recently, and what exact action should happen next?

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Critical Constraints

- Read live repository, tracker, gate, and artifact state; never infer progress from conversational memory. **Why:** status is a truth surface, not a narrative summary.
- Distinguish `PASS`, `WARN`, `FAIL`, `UNAVAILABLE`, and `UNKNOWN`; never render a missing source as healthy or empty. **Why:** fail-soft collection must preserve coverage gaps.
- Use exact issue ids, branch/worktree state, commit ids, verdict paths, and timestamps when available. **Why:** the dashboard must remain resumable after compaction.
- Apart from the required idempotent `ao codex ensure-start` lifecycle record, keep dashboard collection read-only. Do not close beads, clean worktrees, delete stale files, start substrates, or mutate product state while reporting it. **Why:** the startup guard orients the thread; observation after it must not change the system being observed.
- Use native Codex plus the local shell; do not start NTM, Agent Mail, managed agents, Gas City, or another runtime unless explicitly requested. **Why:** a dashboard query does not authorize orchestration.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, repair collection/parsing/rendering, and rerun the same source. **Why:** an ordinary status defect is recovery evidence, not a human andon.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold the affected claim and use one bounded local-shell helper to reconcile contradictory live sources. **Why:** one recovery pass can separate stale cache from a genuine split-brain state.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** unresolved authority, contradictory release truth, refusal, or exhausted recovery requires the operator.

## Codex Execution Profile

In Codex hookless mode, run `ao codex ensure-start` before gathering dashboard state; the CLI records startup once per thread and skips duplicates automatically.

Default to a one-screen layout with three blocks in this order: `Current Work`, `Latest Gates`, `Next Action`.

Use exact issue ids, branch/worktree state, and file-backed artifacts instead of conversational summaries.

Keep the output resumable after compaction.

Leave `ao codex ensure-stop` to a closeout skill such as `$validate`,
`$post-mortem`, or `$handoff`.

## Guardrails

- Treat `ao codex ensure-start` as a startup context guard, not as a session or substrate bootstrap.
- The idempotent thread-start record written by `ao codex ensure-start` is the only permitted status-side mutation; all dashboard probes after it are read-only.
- Use Codex-native tools and local shell commands only; no Claude print-mode or cross-runtime fallback.
- Do not turn optional inbox/corpus sources into blockers; record them in coverage.
- Do not claim completion or landing from branch-local state without a verdict artifact and remote-main evidence.

## Quick Start

```bash
$status              # Full dashboard
$status --json       # Machine-readable output
$status --recover    # Post-compaction continuation view
```

`ao beads exec` resolves the private `br` (beads_rust) ledger; do not use the
unrelated bd/Dolt substrate store for AgentOps repository status.

## Recovery Mode

For `--recover` or post-compaction re-orientation:

1. Run the normal gather below.
2. Read the newest `.agents/handoff/*.md` and `.agents/rpi/execution-packet.json` when present.
3. Re-read `AGENTS.md` before resuming a claimed bead.
4. Report the in-flight objective, exact next action, and claimed-but-unfinished beads.

Use [the recovery playbook](references/recovery-playbook.md) only when normal continuation surfaces are insufficient.

## Execution Workflow

### 1. Gather live state

Run independent read-only calls in parallel when useful:

```bash
ao codex ensure-start 2>/dev/null || true
ao reconcile --json 2>/dev/null || echo RECONCILE_UNAVAILABLE
ao beads exec list --type epic --status open --json 2>/dev/null || echo EPIC_UNAVAILABLE
ao beads exec list --status in_progress --json 2>/dev/null || echo IN_PROGRESS_UNAVAILABLE
ao beads exec ready --json 2>/dev/null || echo READY_UNAVAILABLE
ao ratchet status --json 2>/dev/null || echo RATCHET_UNAVAILABLE
ao task-status --json 2>/dev/null || echo TASK_STATUS_UNAVAILABLE
git branch --show-current
git log --oneline -3
git status --short
```

Also inspect present continuation and verdict artifacts under `.agents/ao/`,
`.agents/council/`, `.agents/pawl-verdicts/`, `.agents/signals/`, and
`.agents/handoff/`.

**Checkpoint:** every displayed value must have a live command or file source; record unavailable and malformed sources in `coverage` before rendering.

### 2. Normalize facts

Normalize into [dashboard-contract](references/dashboard-contract.md):

- `current_work`: active epic, in-progress/ready ids, ratchet phase, and git state;
- `latest_gates`: reconciliation plus recent independent verdicts;
- `next_action`: first matching priority and one executable action;
- `coverage`: one entry per attempted source with `available|unavailable|malformed`.

Prefer executable/generated truth by repository precedence. Show contradictions
instead of merging them silently.

### 3. Choose the next action

| Priority | Condition | Next action |
|---:|---|---|
| 0 | Reconciliation has a high finding or sources contradict | Resolve the named reconciliation blocker |
| 1 | Recent WARN/FAIL/REFUTED verdict exists | Repair findings and rerun the same gate |
| 2 | Claimed/in-progress bead exists | Resume the exact bead and worktree |
| 3 | Uncommitted changes exist | Validate the current diff |
| 4 | Ready bead exists | Implement the first ready id |
| 5 | Research complete without a plan | Run `$plan` |
| 6 | Plan exists without implementation | Run `$implement <id>` |
| 7 | Pending knowledge items exist | Inspect and promote or discard them deliberately |
| 8 | Clean state | Start `$research` or `$plan` |

**Checkpoint:** cite the selecting fact and never recommend backlog work while a higher-priority blocker or claimed bead exists.

### 4. Render

Render `Current Work`, `Latest Gates`, and `Next Action` in that order, followed
by a compact coverage note. `--json` emits only the schema object in
[dashboard-contract](references/dashboard-contract.md).

## Output Specification

**Artifact directory:** stdout for dashboard output; `ao codex ensure-start` may update its idempotent thread lifecycle record before collection, while the dashboard itself creates no artifact.

**Filename convention:** no file for normal output; explicit JSON capture uses `status-<UTC-timestamp>.json`.

**Serialization/schema format:** human output is the three-block dashboard; `--json` is one JSON object with `schema_version`, `generated_at`, `current_work`, `latest_gates`, `next_action`, and `coverage`.

**Validator command:** with `OUT=<captured-status.json>`, run `jq -e '.schema_version==1 and (.generated_at|type)=="string" and (.current_work|type)=="object" and (.latest_gates|type)=="object" and (.next_action|type)=="object" and (.next_action.priority|type)=="number" and (.next_action.message|type)=="string" and (.coverage|type)=="array" and all(.coverage[]; (.source|type)=="string" and (.status=="available" or .status=="unavailable" or .status=="malformed"))' "$OUT"`.

**Downstream handoff:** pass the generated timestamp, exact active ids/worktree/commit, latest gate verdicts and artifact paths, selected priority/fact/action, and all unavailable or contradictory sources.

## Quality Checklist

- [ ] Every displayed fact is backed by a current command or file artifact.
- [ ] Missing and malformed sources appear in coverage, never as healthy or empty.
- [ ] Tracker, git, verdict, and reconciliation state use exact ids and timestamps.
- [ ] The next action is the first applicable priority and cites its selecting fact.
- [ ] Human and JSON outputs describe the same normalized state.
- [ ] After the bounded `ensure-start` lifecycle record, collection remained read-only and did not start an orchestration substrate.
- [ ] WARN/FAIL/REFUTED consulted the pawl before any human andon.

## Examples

### Resume an active landing

**User says:** `$status`

**What happens:** live `br`, remote git, reconciliation, and verdict artifacts
show one in-progress bead with a CONFIRMED verdict but no remote-main bind.

**Result:** `Next Action` says to resume that exact bead/worktree and complete
the canonical land door; it does not suggest unrelated ready work.

## Troubleshooting

| Problem | Response |
|---|---|
| `ao` unavailable | Report tracker/reconciliation unavailable; show git and file-backed facts only |
| Tracker and handoff disagree | Prefer live tracker, show the stale timestamp, select reconciliation |
| Malformed JSON | Preserve source/exit code, mark malformed, rerun the narrow command |
| Suggestion conflicts with intent | Show selecting fact; explicit operator intent may replace the suggestion |

## Local Resources

- [dashboard-contract.md](references/dashboard-contract.md) — visual layout and JSON schema
- [status.feature](references/status.feature) — executable dashboard behavior
- [recovery-playbook.md](references/recovery-playbook.md) — deep recovery fallback
