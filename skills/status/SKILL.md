---
name: status
spine: true
description: 'Show AgentOps work status. Triggers: "status", "show agentops work status.", "status skill".'
practices:
- dora-metrics
- sre
hexagonal_role: driving-adapter
consumes:
- br
produces:
- stdout
context_rel: []
skill_api_version: 1
allowed-tools: Read, Grep, Glob, Bash
model: haiku
context:
  window: inherit
  intent:
    mode: none
  intel_scope: none
metadata:
  graph_root: true
  tier: session
  dependencies: [sbh]
output_contract: 'stdout: dashboard'
---
# /status — Workflow Dashboard

> **Purpose:** Produce a one-screen, evidence-backed answer to: what is active, what passed or failed recently, and what exact action should happen next?

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Critical Constraints

- Read live repository, tracker, gate, and artifact state; never infer progress from conversational memory. **Why:** status is a truth surface, not a narrative summary.
- Distinguish `PASS`, `WARN`, `FAIL`, `UNAVAILABLE`, and `UNKNOWN`; never render a missing source as healthy or empty. **Why:** fail-soft collection must preserve coverage gaps.
- Use exact issue ids, branch/worktree state, commit ids, verdict paths, and timestamps when available. **Why:** the dashboard must remain resumable after compaction.
- Keep collection read-only. Do not close beads, clean worktrees, delete stale files, start substrates, or mutate state while reporting it. **Why:** observation must not change the system being observed.
- Use the current agent and local shell; do not start NTM, Agent Mail, managed agents, Gas City, or another runtime unless explicitly requested. **Why:** a dashboard query does not authorize orchestration.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, repair collection/parsing/rendering, and rerun the same source. **Why:** an ordinary status defect is recovery evidence, not a human andon.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold the affected claim and use one bounded local-shell helper to reconcile contradictory live sources. **Why:** one recovery pass can separate stale cache from a genuine split-brain state.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** unresolved authority, contradictory release truth, refusal, or exhausted recovery requires the operator.

## Quick Start

```bash
/status              # Full dashboard
/status --json       # Machine-readable output
/status --recover    # Post-compaction continuation view
```

`quickstart` routes to the same next-action decision table. CLI dependencies
(`ao`, `br`, and optional inbox tools) are capability-probed; missing tools are
reported in coverage instead of treated as empty state.

## Recovery Mode

For `--recover` or post-compaction re-orientation:

1. Run the normal gather below.
2. Read the newest `.agents/handoff/*.md` and `.agents/rpi/execution-packet.json` when present.
3. Re-read `AGENTS.md` before resuming a claimed bead.
4. Report the in-flight objective, exact next action, and claimed-but-unfinished beads.

Use [the recovery playbook](references/recovery-playbook.md) only when the normal continuation surfaces are insufficient.

## Execution Workflow

### 1. Gather live state

Run independent read-only calls in parallel when the runtime supports it:

```bash
# Reconciliation and gate truth
ao reconcile --json 2>/dev/null || echo RECONCILE_UNAVAILABLE

# AgentOps tracker (ao resolves the private br ledger)
ao beads exec list --type epic --status open --json 2>/dev/null || echo EPIC_UNAVAILABLE
ao beads exec list --status in_progress --json 2>/dev/null || echo IN_PROGRESS_UNAVAILABLE
ao beads exec ready --json 2>/dev/null || echo READY_UNAVAILABLE

# Ratchet and task truth
ao ratchet status --json 2>/dev/null || echo RATCHET_UNAVAILABLE
ao task-status --json 2>/dev/null || echo TASK_STATUS_UNAVAILABLE

# Repository truth
git branch --show-current
git log --oneline -3
git status --short
```

Also inspect, when present:

- `.agents/ao/chain.jsonl` for the last ratchet transition;
- `.agents/council/` and `.agents/pawl-verdicts/` for recent verdict artifacts;
- `.agents/signals/session-quality.jsonl` for the last ten quality signals;
- `.agents/ao/sessions/` for recent session summaries;
- `.agents/learnings/`, `.agents/patterns/`, and `.agents/forge/` for corpus counts.

**Checkpoint:** every displayed value must have a live command or file source; record unavailable and malformed sources in `coverage` before rendering.

### 2. Normalize facts

Normalize into the schema in [dashboard-contract](references/dashboard-contract.md):

- `current_work`: active epic, in-progress/ready ids, ratchet phase, and git state;
- `latest_gates`: reconciliation plus recent independent verdicts;
- `next_action`: first matching priority and one executable action;
- `coverage`: one entry per attempted source with `available|unavailable|malformed`.

Never merge contradictory values silently. Prefer executable/generated truth by
repository precedence, show the disagreement, and route a high-severity conflict
to the next action.

### 3. Choose the next action

Evaluate top-to-bottom and stop at the first match:

| Priority | Condition | Next action |
|---:|---|---|
| 0 | Reconciliation has a high finding or sources contradict | Resolve the named reconciliation blocker |
| 1 | Recent WARN/FAIL/REFUTED verdict exists | Repair its named findings and rerun the same gate |
| 2 | Claimed/in-progress bead exists | Resume the exact bead and worktree |
| 3 | Uncommitted changes exist | Validate the current diff |
| 4 | Ready bead exists | Implement the first ready id |
| 5 | Research complete without a plan | Run `/plan` |
| 6 | Plan exists without implementation | Run `/implement <id>` |
| 7 | Pending knowledge items exist | Inspect and promote or discard them deliberately |
| 8 | Clean state | Start `/research` or `/plan` |

**Checkpoint:** the suggestion must cite the fact that selected it and must not
recommend new backlog work while a higher-priority blocker or claimed bead exists.

### 4. Render

Default output uses exactly three blocks in this order: `Current Work`, `Latest
Gates`, `Next Action`, followed by a compact coverage note. `--json` emits only
the schema object. Do not wrap JSON in explanatory prose.

The complete visual and JSON contract lives in
[dashboard-contract](references/dashboard-contract.md); keep this kernel focused
on collection, precedence, and decision behavior.

## Output Specification

**Artifact directory:** stdout by default; recovery reads existing artifacts under `.agents/` but creates none.

**Filename convention:** no file for normal output; when a caller explicitly captures JSON, use `status-<UTC-timestamp>.json`.

**Serialization/schema format:** human output is the three-block dashboard; `--json` is one JSON object with `schema_version`, `generated_at`, `current_work`, `latest_gates`, `next_action`, and `coverage` as defined in [dashboard-contract](references/dashboard-contract.md).

**Validator command:** with `OUT=<captured-status.json>`, run `jq -e '.schema_version==1 and (.generated_at|type)=="string" and (.current_work|type)=="object" and (.latest_gates|type)=="object" and (.next_action|type)=="object" and (.next_action.priority|type)=="number" and (.next_action.message|type)=="string" and (.coverage|type)=="array" and all(.coverage[]; (.source|type)=="string" and (.status=="available" or .status=="unavailable" or .status=="malformed"))' "$OUT"`.

**Downstream handoff:** pass the generated timestamp, exact active ids/worktree/commit, latest gate verdicts and artifact paths, selected priority/fact/action, and all unavailable or contradictory sources to the operator or recovery workflow.

## Quality Checklist

- [ ] Every displayed fact is backed by a current command or file artifact.
- [ ] Missing and malformed sources appear in coverage, never as healthy or empty.
- [ ] Tracker, git, verdict, and reconciliation state use exact ids and timestamps.
- [ ] The next action is the first applicable priority and cites its selecting fact.
- [ ] Human and JSON outputs describe the same normalized state.
- [ ] Collection remained read-only and did not start an orchestration substrate.
- [ ] WARN/FAIL/REFUTED consulted the pawl before any human andon.

## Examples

### Resume an active landing

**User says:** `/status`

**What happens:** live `br`, remote git, reconciliation, and verdict artifacts
show one in-progress bead with a CONFIRMED verdict but no remote-main bind.

**Result:** `Next Action` says to resume that exact bead/worktree and complete
the canonical land door; it does not suggest unrelated ready work.

## Troubleshooting

| Problem | Response |
|---|---|
| `ao` unavailable | Report tracker/reconciliation coverage unavailable; show git and file-backed facts only |
| Tracker and handoff disagree | Prefer live tracker, show the stale handoff timestamp, select reconciliation as next action |
| Malformed JSON | Preserve the source/exit code, mark malformed, and rerun the narrow command |
| Suggested action conflicts with intent | Show the selecting fact and priority; explicit operator intent may replace the suggestion |

## Reference Documents

- [references/dashboard-contract.md](references/dashboard-contract.md) — stable visual layout, JSON schema, and valid/invalid examples
- [references/status.feature](references/status.feature) — executable dashboard behavior
- [references/recovery-playbook.md](references/recovery-playbook.md) — deep recovery when normal continuation artifacts are insufficient
