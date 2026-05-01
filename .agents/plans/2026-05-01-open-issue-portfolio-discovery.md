---
id: plan-2026-05-01-open-issue-portfolio-discovery
type: plan
date: 2026-05-01
source: "[[.agents/research/2026-05-01-open-issue-portfolio-discovery]]"
epic: soc-o6eb
---

# Plan: Open Issue Portfolio Discovery

## Context

This plan addresses the open bd issue portfolio as a routing and sequencing
problem. The input snapshot had 142 open issues before this discovery created
`soc-o6eb` and five routing children. The live graph now has 148 open issues,
with `soc-o6eb` serving as a coordinating epic rather than a replacement for
existing implementation epics.

Applied findings:

- `f-2026-05-01-001` - Dry-run paths can mutate tracked runtime aliases; all
  dry-run validation in this portfolio must prove tracked-file stability.
- `f-2026-04-25-001` - Long autonomous loops can pass product gates while
  failing disposition or closure replay; portfolio waves must close with
  worktree and tracker checks.
- `.agents/learnings/2026-05-01-pending-queue-needs-terminal-state.md` -
  close-loop work must treat pending knowledge as a queue with terminal state.
- `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md` - source fixes
  are not operational fixes until deployed binaries and hook caches are fresh.
- `.agents/learnings/2026-05-01-worker-context-staleness-regressions.md` -
  workers must start from fresh branches after recent related fixes.

## Files to Modify

| File | Change |
|------|--------|
| `.beads/issues.jsonl` | New `soc-o6eb` routing epic and five children; later waves update statuses, links, and deferrals. |
| `.agents/brainstorm/2026-05-01-open-issue-portfolio-discovery.md` | Brainstorm artifact for approach selection. |
| `.agents/research/2026-05-01-open-issue-portfolio-discovery.md` | Backlog analysis and issue-cluster inventory. |
| `.agents/plans/2026-05-01-open-issue-portfolio-discovery.md` | This plan. |
| `.agents/council/2026-05-01-pre-mortem-open-issue-portfolio-discovery.md` | Quick pre-mortem report. |
| `.agents/rpi/ranked-packet-2026-05-01-open-issue-portfolio-discovery.json` | Ranked prior knowledge and active risks packet. |
| `.agents/rpi/execution-packet.json` | Latest discovery execution packet alias. |
| `.agents/rpi/runs/20260501T131505-0400/execution-packet.json` | Archived execution packet for this run. |
| `.agents/rpi/phase-1-summary-2026-05-01-open-issue-portfolio-discovery.md` | Discovery phase summary. |

## Boundaries

**Always:** Treat the input count as the 142 open issues captured before this
plan created the routing epic. Preserve existing implementation epics. Use bd
for tracking. Prefer closing, relating, deferring, or selecting existing beads
over filing duplicate implementation work.

**Ask First:** Reparenting large existing epics under `soc-o6eb`, deleting
archives, changing cross-repo ownership, or picking a long-running
daemon/Dream/Mt. Olympus migration lane for more than one day of execution.

**Never:** Run all migration epics concurrently, close stale issues without
evidence, treat a source fix as deployed proof, or let dry-run commands dirty
tracked runtime aliases without a follow-up bug.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| Input open issues | `bd list --status open --limit 0 --json \| jq 'length'` before creating `soc-o6eb` | 142 |
| Input priority mix | `bd list --status open --limit 0 --json \| jq -r 'group_by(.priority)[] \| "priority \\(.[0].priority): \\(length)"'` | P0 1, P1 50, P2 68, P3 20, P4 3 |
| Input type mix | `bd list --status open --limit 0 --json \| jq -r 'group_by(.issue_type)[] \| "type \\(.[0].issue_type): \\(length)"'` | bug 9, chore 6, epic 15, feature 20, task 92 |
| Input ready count | `bd ready --limit 0 --json \| jq 'length'` before creating `soc-o6eb` | 86 |
| Live open issues after routing epic | `bd list --status open --limit 0 --json \| jq 'length'` | 148 |
| Live ready issues after routing epic | `bd ready --limit 0 --json \| jq 'length'` | 90 |
| New routing graph | `bd list --status open --limit 0 --json \| jq -r 'map(select(.id\|startswith("soc-o6eb")))[] \| .id'` | `soc-o6eb`, `soc-o6eb.1` through `soc-o6eb.5` |

## Implementation

### 1. Wave 0: Stabilize Incident and Tracker Hygiene

Owner issue: `soc-o6eb.1`.

Work:

- Verify `soc-2ctn` against the deployed `ao` binary, installed hook cache, and
  archived pending replay.
- Decide whether `soc-xn5s` remains a separate close-loop dedup bug or is
  linked into `soc-2ctn`.
- Collapse duplicate RPI dry-run alias bugs `soc-7wwp` and `soc-qvpb` into one
  canonical issue.
- Record `agentops-ikm` worktree disposition so closeout is not blocked by an
  old validator worktree.

Acceptance:

- One canonical P0/close-loop route remains.
- `git status --short` is checked after dry-run/replay proof.
- Deployed binary and hook cache freshness are explicitly recorded.

### 2. Wave 1: Normalize the Issue Graph

Owner issue: `soc-o6eb.2`. Depends on Wave 0.

Work:

- Create a triage note for duplicate, stale, orphaned, and blocked work.
- Audit the 34 unparented non-epic input issues and route each to execute,
  relate, defer, or close.
- Check blockers that reference older migration epics or external repos.
- Do not mass-reparent existing implementation work.

Acceptance:

- Duplicate/merge candidates are named.
- Stale blockers are named with proposed action.
- Unparented P1/P2 work has a route.
- P3/P4 items have defer dates, retirement rules, or explicit next actions.

### 3. Wave 2: Drain Active Local Execution Epics

Owner issue: `soc-o6eb.3`. Depends on Wave 1.

Scope:

- `soc-8412`: Bushido CLI rebuild control plane.
- `soc-b8jo`: local nightly evolution automation.
- `soc-eh1z`: PR validation and closeout toil reduction.

Work:

- Select one next executable child per epic.
- Verify the referenced plan/research artifact still matches HEAD.
- Run each lane to a stop condition before starting another local epic.

Acceptance:

- Each active local epic has a selected next bead, validation command, and stop
  rule.
- Completed children are closed or stale scope is updated before the next lane.

### 4. Wave 3: Select One Daemon/Dream/Migration Lane

Owner issue: `soc-o6eb.4`. Depends on Wave 1.

Candidate lanes:

- `soc-q4c`: Dream pipeline through OpenClaw execution bus.
- `soc-ni8g` and `psite-mto.*`: Mt. Olympus v0.2 production migration.
- `psite-agu`: pipeline rearchitecture with agentopsd/headless agents.
- `soc-y8b`: Dark Factory multi-host operating model.
- `soc-v64.2`: agentopsd design-contract additions.

Work:

- Write a decision record naming the one active lane.
- Pause other migration lanes with explicit dependency or defer notes.
- Require activation smoke for any infrastructure lane.

Acceptance:

- One lane is active.
- Paused lanes remain open with rationale.
- Activation smoke and rollback/rescue are named before stateful work starts.

### 5. Wave 4: Route Cross-Repo, Security, and Deferrals

Owner issue: `soc-o6eb.5`. Depends on Wave 1.

Scope:

- `soc-v64`: platform-lab, compound-engineering-plugin, and learning-legacy
  cross-repo items.
- `soc-egzh`: showcase doctrine restoration.
- `soc-tq42`: dolt remote provisioning.
- Standalone P2/P3/P4 issues.

Work:

- Assign owning repo/worktree for each cross-repo issue.
- Keep P1 security and recovery work visible.
- Defer P3/P4 work with dates or retirement criteria.

Acceptance:

- Cross-repo issues have owner repo and validation command.
- P3/P4 issues no longer appear as ambiguous ready work.

## File Dependency Matrix

| Task | File | Access | Notes |
|------|------|--------|-------|
| `soc-o6eb.1` | `.beads/issues.jsonl` | write | Close/link duplicate tracker items after evidence. |
| `soc-o6eb.1` | `.agents/archive/pend-pollution-2026-04-30/` | read | Replay proof source for `soc-2ctn`. |
| `soc-o6eb.1` | `.agents/archive/pending-source-2026-04-30/` | read | Pending replay source for `soc-2ctn`. |
| `soc-o6eb.1` | `.agents/learnings/` | read | Confirm no new `pend-*` growth. |
| `soc-o6eb.2` | `.beads/issues.jsonl` | write | Relate, close, defer, or annotate duplicate/stale work. |
| `soc-o6eb.2` | `.agents/decisions/` | write | Optional triage decision record. |
| `soc-o6eb.3` | `.beads/issues.jsonl` | write | Update existing local epic children only. |
| `soc-o6eb.3` | `.agents/plans/2026-05-01-nightly-automation-chain.md` | read | Existing plan for `soc-b8jo`. |
| `soc-o6eb.3` | `.agents/plans/2026-05-01-pr-validation-toil-reduction.md` | read | Existing plan for `soc-eh1z`. |
| `soc-o6eb.4` | `.beads/issues.jsonl` | write | Pause or select migration lanes. |
| `soc-o6eb.4` | `.agents/decisions/` | write | Required lane-selection decision. |
| `soc-o6eb.5` | `.beads/issues.jsonl` | write | Route cross-repo and deferral items. |
| `soc-o6eb.5` | `.agents/decisions/` | write | Optional cross-repo ownership decision. |

Same-wave write conflicts: none. Wave 2, Wave 3, and Wave 4 all write
`.beads/issues.jsonl`, so serialize those updates or run them in separate
sessions after Wave 1.

## Tests

Required for the discovery/routing layer:

- **L0:** JSON and markdown artifact sanity.
- **L1:** bd graph checks and cycle detection.

Required when implementing routed code issues:

- **L2:** integration tests for CLI, shell, daemon, Dream, or cross-repo
  workflows touched by the selected issue.
- **L3:** only for stateful/infrastructure migration lanes or live
  cross-host activation.

## Conformance Checks

| Issue | Check Type | Check |
|-------|------------|-------|
| `soc-o6eb.1` | command | `bd show soc-2ctn --json` |
| `soc-o6eb.1` | command | `git status --short` after replay/dry-run proof |
| `soc-o6eb.2` | command | `bd dep cycles` |
| `soc-o6eb.2` | command | `bd list --status open --limit 0 --json` |
| `soc-o6eb.3` | command | `bd ready --limit 0 --json` filtered to `soc-8412`, `soc-b8jo`, `soc-eh1z` |
| `soc-o6eb.4` | content_check | decision record names active and paused migration lanes |
| `soc-o6eb.5` | content_check | cross-repo issues name owning repo/worktree and validation command |

## Verification

1. `python3 -m json.tool .agents/rpi/ranked-packet-2026-05-01-open-issue-portfolio-discovery.json >/dev/null`
2. `python3 -m json.tool .agents/rpi/execution-packet.json >/dev/null`
3. `python3 -m json.tool .agents/rpi/runs/20260501T131505-0400/execution-packet.json >/dev/null`
4. `bd dep cycles`
5. `bd show soc-o6eb --json`
6. `git status --short`

## Issues

### Issue `soc-o6eb`: Open issue portfolio discovery and routing

**Dependencies:** None
**Acceptance:** Discovery artifacts exist, all open issues are classified into
routing lanes, and P0/P1 strategy is explicit.
**Description:** Coordinating epic only. Do not use it to replace existing
implementation epics.

### Issue `soc-o6eb.1`: Portfolio W0: Stabilize P0 flywheel and tracker hygiene

**Dependencies:** None
**Acceptance:** P0 and duplicate dry-run/flywheel hygiene items have a canonical
route and deployed proof.
**Description:** Interrupt all other routing work until the P0 and known
tracker/worktree hazards are stable.

### Issue `soc-o6eb.2`: Portfolio W1: Normalize stale duplicates and blocked graph

**Dependencies:** `soc-o6eb.1`
**Acceptance:** Duplicate, stale, orphaned, and deferred work is recorded with
next action.
**Description:** Clean the issue graph before broad execution.

### Issue `soc-o6eb.3`: Portfolio W2: Drain active local execution epics

**Dependencies:** `soc-o6eb.2`
**Acceptance:** `soc-8412`, `soc-b8jo`, and `soc-eh1z` each have selected next
issue, validation command, and stop rule.
**Description:** Finish already-discovered local epics before opening new work.

### Issue `soc-o6eb.4`: Portfolio W3: Select one daemon Dream migration lane

**Dependencies:** `soc-o6eb.2`
**Acceptance:** One migration lane is selected and paused lanes are explicitly
documented.
**Description:** Prevent simultaneous execution across overlapping daemon,
Dream, OpenClaw, and Mt. Olympus migration tracks.

### Issue `soc-o6eb.5`: Portfolio W4: Route cross-repo security and low-priority deferrals

**Dependencies:** `soc-o6eb.2`
**Acceptance:** Cross-repo issues have owners and validation commands; P3/P4
items have defer dates or retirement rules.
**Description:** Keep the long tail visible without letting it dominate the
ready queue.

## Execution Order

**Wave 0:** `soc-o6eb.1`

**Wave 1:** `soc-o6eb.2`

**Wave 2:** `soc-o6eb.3`, `soc-o6eb.4`, `soc-o6eb.5` can proceed after Wave 1,
but all `.beads/issues.jsonl` writes should be serialized.

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical Enforcement | PASS | Every routing issue has bd, JSON, cycle, status, or content checks. |
| PR-002: External Validation | PASS | bd graph checks and file parser checks are independent of the planner. |
| PR-003: Feedback Loops | PASS | Wave 0 explicitly verifies close-loop terminal-state and deployed freshness. |
| PR-004: Separation Over Layering | PASS | Existing implementation epics remain separate; `soc-o6eb` coordinates only. |
| PR-005: Process Gates First | PASS | P0 and graph normalization gate later execution lanes. |
| PR-006: Cross-Layer Consistency | PASS | Product gaps, bd graph, `.agents` plans, and repo execution profile are linked. |
| PR-007: Phased Rollout | PASS | Incident, graph, local epics, migration choice, and deferrals are separated. |

Unchecked rules: 0

## Post-Merge Cleanup

No code is changed by this discovery. After executing routing waves:

- Run `bd dep cycles`.
- Confirm duplicate closures are linked in issue notes.
- Confirm deferred issues have explicit dates or retirement rules.
- Run `bash scripts/check-worktree-disposition.sh` before ending the session.

## Next Steps

- Run `soc-o6eb.1` first.
- Then run `soc-o6eb.2` before broad `/crank`.
- Use PRODUCT.md gaps as tie-breakers after the P0 and graph normalization are
  handled.
