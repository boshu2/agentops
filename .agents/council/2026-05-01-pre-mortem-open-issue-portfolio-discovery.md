---
id: pre-mortem-2026-05-01-open-issue-portfolio-discovery
type: pre-mortem
date: 2026-05-01
source: "[[.agents/plans/2026-05-01-open-issue-portfolio-discovery]]"
scope_mode: hold
prediction_ids:
  - pm-20260501-001
  - pm-20260501-002
  - pm-20260501-003
  - pm-20260501-004
  - pm-20260501-005
  - pm-20260501-006
  - pm-20260501-007
  - pm-20260501-008
---

# Pre-Mortem: Open Issue Portfolio Discovery

## Council Verdict: WARN

Quick inline review. Scope mode: **HOLD SCOPE**. The plan's scope is accepted:
do not expand this into a mega-implementation epic, and do not reduce it below
the P0-first portfolio routing gate.

| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-20260501-001 | Missing-Requirements | The coordinating epic can become duplicate tracking unless it stays routing-only. | moderate | Workers may implement `soc-o6eb.*` instead of the existing implementation beads. |
| pm-20260501-002 | Feasibility | The P0 close-loop incident may appear fixed in source while deployed `ao` and cached hooks still reproduce it. | significant | Portfolio execution resumes while `pend-*` growth continues in the live repo. |
| pm-20260501-003 | Scope | Daemon, Dream, OpenClaw, and Mt. Olympus lanes overlap and can create plan oscillation. | significant | Multiple migration epics modify adjacent runtime assumptions at once. |
| pm-20260501-004 | Verification | Dry-run and routing checks can dirty tracked aliases or mutate the bead graph without proof. | moderate | A "read-only" triage wave creates unrelated `.agents/rpi/execution-packet.json` or `.beads` churn. |
| pm-20260501-005 | Spec-Completeness | Input snapshot count and live issue count now differ, and future readers may confuse them. | moderate | Someone treats 148 as the analyzed baseline and misses the six routing issues added by discovery. |
| pm-20260501-006 | Closure-Integrity | Future routing notes under ignored `.agents/decisions/` can be cited by closed beads without being committed. | significant | Closure replay fails because evidence paths cited by issue notes are absent on a fresh checkout. |
| pm-20260501-007 | Concurrency | Wave 2, Wave 3, and Wave 4 all write `.beads/issues.jsonl`; "serialize" is stated but not mechanically enforced. | significant | Parallel workers create conflicting bd updates or reverse dependency direction again. |
| pm-20260501-008 | Product | The plan uses PRODUCT.md as a tie-breaker, but no command checks that W1/W2 actually preserve product-gap priority. | moderate | Low-leverage backlog grooming crowds out flywheel, validation, and worker-context gaps. |

## Known Context

- Latest plan selected automatically: `.agents/plans/2026-05-01-open-issue-portfolio-discovery.md`.
- PRODUCT.md exists and was included inline. Product alignment favors work that improves durable learning, validation gates, Dream autonomy, multi-runtime proof, and worker context propagation.
- `ao lookup` returned relevant context on agentopsd contracts, stale subagent claims, ephemeral discovery paths, and multi-wave ownership.
- Compiled checks matched dry-run alias mutation, long-loop closeout disposition, subagent claim verification, closure-integrity replay, and file ownership.

## Known Risks Applied

- `f-2026-05-01-001` - Dry-run paths can mutate tracked runtime aliases; the plan adds tracked-file stability checks, but execution must treat this as a hard gate.
- `f-2026-04-25-001` - Long autonomous loops can pass product gates while failing disposition; Wave closeout must run repository disposition checks.
- `f-2026-04-27-003` - Specific subagent/git-history claims can be fabricated; W1 must verify cited paths and refs before closing or deferring beads.
- `f-2026-04-14-002` - Closed beads can cite ephemeral discovery paths; W1/W4 must only cite durable committed artifacts or explicit archive paths.
- `f-2026-03-09-003` - Multi-wave plans need file ownership for tests, docs, generated artifacts, and parity files; this applies to routed implementation epics before execution.
- `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md` - Source fixes need deployed binary proof.
- `.agents/learnings/2026-05-01-worker-context-staleness-regressions.md` - Workers need fresh branch context after close-loop fixes.

## Timeline Risks

| Phase | Blocker | Silent Failure | What Compounds |
|-------|---------|----------------|----------------|
| Hour 1: Setup | Canonical root is already on `evolve/prep-2026-04-30`, and worktree disposition gate fails. | Workers assume closeout is clean because git push succeeded. | More routing work lands on a branch/root posture that already violates repo policy. |
| Hour 2: Wave 0 | Installed `ao` or plugin hook cache is stale. | Replay passes against source binary but live hooks keep firing old close-loop behavior. | More `pend-*` files accumulate while the issue is marked stable. |
| Hour 4: Wave 1 | Duplicate/stale issue decisions rely on title similarity instead of issue bodies and evidence paths. | Real work is closed as duplicate, or duplicate work remains ready. | `bd ready` stays noisy and future crank sessions pick the wrong lane. |
| Hour 6+: Waves 2-4 | Multiple workers update `.beads/issues.jsonl` in parallel. | Dependencies are reversed or deferral notes race with closure notes. | The graph looks clean in one session but becomes wrong after sync/readback. |

## Error & Rescue Map

| Method/Codepath | What Can Go Wrong | Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-------|----------|---------------|-----------|
| `bd list --status open --limit 0 --json` | Tracker unavailable or stale after local sync. | non-zero exit or malformed JSON | Y | Stop routing and run `bd status`/`bd vc status`; do not close or defer issues. | "Tracker unhealthy; routing blocked." |
| `bd dep add/remove` | Dependency direction is inverted. | command succeeds but graph semantics wrong | Y | Read back `bd list`/`bd show`, then run `bd dep cycles`; fix before commit. | Corrected graph or explicit blocker. |
| `bd update` / `bd close` | Evidence path is ignored or absent. | closure replay later fails | Partial | Before closure, verify cited paths exist and are committed or archived. | Closure is blocked until evidence is durable. |
| `ao lookup` / `ao metrics cite` | `ao` unavailable or stale. | non-zero exit | Y | Treat lookup as advisory; never block pre-mortem on citation metrics. | Review proceeds with warning if needed. |
| `git status --short` | Tracked runtime alias or ignored artifact is dirty. | dirty output | Y | Stop wave closeout; either commit intended artifact or file follow-up. | "Dirty tracked state blocks closure." |
| Replay of `.agents/archive/pending-source-2026-04-30/` | Archive missing or incomplete. | path missing or replay inconclusive | Y | Leave `soc-2ctn` open and record missing proof. | P0 remains active. |
| Cross-repo routing | Owning repo/worktree is unavailable. | missing path or dirty foreign worktree | Y | Defer or create owning-repo issue; do not edit from this root. | Cross-repo item remains routed, not implemented. |

## FAIL Pattern Risks

| Pattern | Status | Finding |
|---------|--------|---------|
| Missing mechanical verification | WARN | The plan has commands, but W1's triage note needs a durable format and readback command before closing issues. |
| Self-assessment | WARN | The same worker could route and close duplicates. Require bd readback plus evidence path checks before closure. |
| Context rot | WARN | The plan classifies all 142 input issues but does not read every issue body. W1 must validate each closure/defer action against current issue text. |
| Propagation blindness | WARN | Cross-repo issues need owning repo/worktree validation before implementation. |
| Plan oscillation | WARN | W3 exists specifically to prevent simultaneous migration lanes; enforce it before daemon/Dream code changes. |
| Dead infrastructure activation | WARN | W3 must require activation smoke for any selected infrastructure lane. |
| Missing rollback/rescue map | WARN | Bulk bd graph edits need pre-change export or readback checkpoints so they can be undone. |
| Four-surface closure | PASS | This discovery has no product feature surface, but routed implementation epics must keep Code/Docs/Examples/Proof in their own plans. |

## Test Coverage Gaps

No source-code files are modified by this plan, so L0/L1 discovery checks are
sufficient for the routing artifact itself:

- JSON parsing for execution/ranked packets.
- `bd dep cycles`.
- `bd show soc-o6eb --json`.
- `git diff --check` / `git status --short`.

Routed implementation issues still need their own test pyramid checks. Any
selected CLI, daemon, shell, Dream, or cross-repo implementation issue should
carry L1 behavior tests plus L2 integration tests; stateful or cross-host
migration lanes need L3 smoke/activation proof.

## Input Validation Check

No new enum-like config fields are introduced. Existing bounded values are bd
statuses, priorities, dependency types, and labels; validation is delegated to
bd. The plan should not add free-form replacement statuses during W1.

## Pseudocode Fixes

No code pseudocode is required. Findings are process and tracker-shape issues.
The actionable fixes are:

- Keep `soc-o6eb` routing-only.
- Run `soc-o6eb.1` before any broad execution.
- Require W1 closure/defer decisions to cite durable evidence paths.
- Serialize all `.beads/issues.jsonl` mutation waves or use one worker.
- Treat product gaps as a required tie-breaker after W0/W1, not optional prose.

## Shared Findings

- Keep existing implementation epics as the execution surface.
- P0 verification must check source, installed binary, hook cache, and replay.
- Select one daemon/Dream/Mt. Olympus migration lane before touching code.
- Any dry-run proof must end with `git status --short`.
- Any closure citing `.agents/` artifacts must ensure those artifacts are committed or intentionally archived.

## Concerns Raised

- The plan's snapshot/live count distinction is easy to lose in downstream summaries.
- Cross-repo tasks cannot be implemented safely from this worktree without checking owning repos.
- The plan does not individually validate all 142 issue bodies; it requires W1 to do that before closing or deferring specific issues.
- `.beads/issues.jsonl` mutation is the highest practical concurrency risk.

## Recommendation

Proceed with WARN, but only into `soc-o6eb.1`. Do not run broad `$crank` across
the portfolio yet.

Required before W2/W3/W4:

1. Finish W0 and prove P0 close-loop stability against deployed runtime.
2. Run W1 with a single writer for bd graph mutations.
3. For every closure/defer decision, cite a durable committed artifact or leave the issue open.
4. Re-run this pre-mortem or a focused W1 review if W1 discovers more than five stale/duplicate issues that change the execution order.

## Reusable Findings

No new reusable finding was persisted. The actionable risks are already covered
by active compiled checks `f-2026-05-01-001`, `f-2026-04-25-001`,
`f-2026-04-27-003`, `f-2026-04-14-002`, and `f-2026-03-09-003`.

## Decision Gate

[x] PROCEED - Council passed, ready to implement W0 only
[ ] ADDRESS - Fix concerns before implementing
[ ] RETHINK - Fundamental issues, needs redesign
