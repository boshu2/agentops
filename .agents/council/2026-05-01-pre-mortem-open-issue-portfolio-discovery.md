---
id: pre-mortem-2026-05-01-open-issue-portfolio-discovery
type: pre-mortem
date: 2026-05-01
source: "[[.agents/plans/2026-05-01-open-issue-portfolio-discovery]]"
prediction_ids:
  - pm-20260501-001
  - pm-20260501-002
  - pm-20260501-003
  - pm-20260501-004
---

# Pre-Mortem: Open Issue Portfolio Discovery

## Council Verdict: WARN

| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-20260501-001 | Missing-Requirements | The coordinating epic can become duplicate tracking unless it stays routing-only. | moderate | Workers may implement `soc-o6eb.*` instead of the existing implementation beads. |
| pm-20260501-002 | Feasibility | The P0 close-loop incident may appear fixed in source while deployed `ao` and cached hooks still reproduce it. | significant | Portfolio execution resumes while `pend-*` growth continues in the live repo. |
| pm-20260501-003 | Scope | Daemon, Dream, OpenClaw, and Mt. Olympus lanes overlap and can create plan oscillation. | significant | Multiple migration epics modify adjacent runtime assumptions at once. |
| pm-20260501-004 | Verification | Dry-run and routing checks can dirty tracked aliases or mutate the bead graph without proof. | moderate | A "read-only" triage wave creates unrelated `.agents/rpi/execution-packet.json` or `.beads` churn. |

## Pseudocode Fixes

No code pseudocode is required for this discovery plan. The findings are
process and tracker-shape fixes that have been copied into the plan and the
`soc-o6eb.*` issue descriptions.

## Shared Findings

- Keep `soc-o6eb` routing-only. Existing epics remain the implementation
  surface.
- P0 verification must check source, installed binary, and hook cache.
- Select one migration lane before touching daemon/Dream/Mt. Olympus code.
- Any dry-run proof must end with `git status --short`.

## Known Risks Applied

- `f-2026-05-01-001` - Dry-run paths can mutate tracked runtime aliases; the
  plan adds tracked-file stability checks.
- `f-2026-04-25-001` - Long autonomous loops can pass product gates while
  failing disposition; the plan adds closeout and worktree-disposition checks.
- `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md` - Source fixes
  need deployed binary proof.
- `.agents/learnings/2026-05-01-worker-context-staleness-regressions.md` -
  Workers need fresh branch context after close-loop fixes.

## Concerns Raised

- The input issue count was 142, but the discovery added six routing issues.
  Reports must distinguish input snapshot from live graph.
- Cross-repo tasks cannot be implemented safely from this worktree without
  checking owning repos.
- The plan does not individually validate all 142 issue bodies. It classifies
  all issues by graph cluster and priority, then requires focused validation
  before implementation.

## Recommendation

Proceed with WARN. Run `soc-o6eb.1` first, then `soc-o6eb.2`. Do not start
parallel implementation across active epics until the duplicate/stale graph is
normalized. Treat `soc-o6eb` as a routing control point, not as a replacement
parent for all open issues.

## Decision Gate

[x] PROCEED - Council passed, ready to implement
[ ] ADDRESS - Fix concerns before implementing
[ ] RETHINK - Fundamental issues, needs redesign
