---
id: research-2026-05-01-open-issue-portfolio-discovery
type: research
date: 2026-05-01
---

# Research: Open Issue Portfolio Discovery

**Backend:** inline
**Scope:** All open bd issues before the discovery epic was created, plus
existing planning, research, council, product, and repo-execution artifacts.

## Summary

The input backlog snapshot contained 142 open issues: 1 P0, 50 P1, 68 P2, 20
P3, and 3 P4. The issue graph is not one implementation epic. It is a
portfolio containing incident cleanup, active local epics, overlapping
daemon/Dream/Mt. Olympus migration lanes, cross-repo backlog, eval/user-sim
work, and low-priority deferred operations.

This discovery created `soc-o6eb` and five coordinating children after the
snapshot, so the live open count became 148. The new issues are routing work
only; they should not replace existing implementation beads.

## Key Files

| File | Purpose |
|------|---------|
| `.beads/issues.jsonl` | Versioned bd issue graph backing the open backlog. |
| `docs/newcomer-guide.md` | Repo orientation and source-of-truth precedence. |
| `docs/documentation-index.md` | Current documentation catalog and contract map. |
| `PRODUCT.md` | Product-gap tie-breakers for backlog routing. |
| `docs/contracts/repo-execution-profile.md` | Local execution policy, validation, and done criteria. |
| `.agents/plans/2026-05-01-nightly-automation-chain.md` | Existing plan for `soc-b8jo`. |
| `.agents/plans/2026-05-01-pr-validation-toil-reduction.md` | Existing plan for `soc-eh1z`. |
| `.agents/plans/2026-04-29-plan-extract-olympus-into-agentopsd.md` | Prior large migration plan and wave structure. |
| `.agents/planning-rules/f-2026-05-01-001.md` | Dry-run alias mutation prevention rule. |
| `.agents/planning-rules/f-2026-04-25-001.md` | Long-loop closeout and disposition rule. |
| `.agents/learnings/2026-05-01-pending-queue-needs-terminal-state.md` | Queue lifecycle lesson for close-loop pollution. |
| `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md` | Source fix is not operational until installed binary/hook cache is fresh. |
| `.agents/learnings/2026-05-01-worker-context-staleness-regressions.md` | Worker staleness can undo recent fixes. |

## Issue Inventory

Input snapshot before creating `soc-o6eb`:

| Metric | Value |
|--------|------:|
| Open issues | 142 |
| Ready issues | 86 |
| P0 | 1 |
| P1 | 50 |
| P2 | 68 |
| P3 | 20 |
| P4 | 3 |
| Bugs | 9 |
| Chores | 6 |
| Epics | 15 |
| Features | 20 |
| Tasks | 92 |

Post-discovery live graph:

| Metric | Value |
|--------|------:|
| Open issues | 148 |
| Ready issues | 90 |
| New portfolio epic | `soc-o6eb` |
| New portfolio children | `soc-o6eb.1` through `soc-o6eb.5` |

## Cluster Coverage

Every open issue in the input snapshot falls under one of these clusters:

| Cluster | Open non-epic count | Interpretation |
|---------|--------------------:|----------------|
| Unparented | 34 | Mixed P0/P1/P2/P3/P4 work; needs routing before broad execution. |
| `psite-355` | 8 | Morai/OpenClaw argv-size failure and rollout work. |
| `psite-agu` | 12 | Pipeline rearchitecture around agentopsd and headless agents. |
| `psite-mto` | 7 | Mt. Olympus production migration leftovers. |
| `soc-33s` | 1 | Validator pair operating-model dependency. |
| `soc-5of` plus `soc-5of.15` | 5 | Agentops daemon deferred council and GasCity dogfood work. |
| `soc-7ftl` | 5 | Plans projection pilot. |
| `soc-8412` | 6 | Bushido CLI rebuild control plane. |
| `soc-b8jo` | 3 | Local nightly evolution automation. |
| `soc-egzh` | 12 | Showcase Knowledge OS restoration. |
| `soc-eh1z` | 1 | PR validation and closeout toil reduction. |
| `soc-fkt` | 2 | Platform/lab deployment work. |
| `soc-ni8g` | 1 | Mt. Olympus v0.2 production migration parent. |
| `soc-q4c` | 6 | Dream pipeline through OpenClaw execution bus. |
| `soc-v64` | 4 | Cross-repo P0/P1 backlog items. |
| `soc-v64.2` | 6 | Agentopsd design-contract additions. |
| `soc-y8b` | 4 | Dark Factory multi-host operating model. |
| `soc-ygo` | 10 | Older next-work backlog and Morai/T3 items. |

## Findings

### 1. The live P0 should interrupt all portfolio work

`soc-2ctn` is the only input P0. It covers flywheel close-loop pollution:
pending inputs were repeatedly re-ingested, producing `pend-*` amplification.
The most relevant prior learnings say knowledge promotion must be treated as a
queue with terminal states, and that a source fix is not an operational fix
until the installed `ao` binary and hook cache are fresh.

Applied knowledge:

- `.agents/learnings/2026-05-01-pending-queue-needs-terminal-state.md`
- `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md`
- `.agents/learnings/2026-05-01-worker-context-staleness-regressions.md`

Planning effect: Wave 0 must verify deployed binary freshness, hook cache
freshness, replay stability, and no new `pend-*` growth before starting broad
execution.

### 2. The backlog has too many ready lanes for naive `bd ready`

Before creating the discovery epic, `bd ready --limit 0 --json` returned 86
ready issues: 1 P0, 30 P1, 39 P2, 14 P3, and 2 P4. That is a queue-selection
problem. Executing in raw ready order would interleave Bushido, Dream,
OpenClaw, Mt. Olympus, PR-toil, user-sim, platform-lab, and dotfiles work.

Planning effect: use a portfolio wave gate before running broad `/crank`.

### 3. Several current P1 tracks already have discovery

Existing plans cover the highest-value local tracks:

- `soc-b8jo`: `.agents/plans/2026-05-01-nightly-automation-chain.md`
- `soc-eh1z`: `.agents/plans/2026-05-01-pr-validation-toil-reduction.md`
- `soc-8412`: existing bd children and plan reference in issue bodies

Planning effect: do not create replacement implementation issues. Route these
epics to their next executable child after the P0 and graph-normalization pass.

### 4. Daemon, Dream, OpenClaw, and Mt. Olympus work overlap

The issue graph contains at least five related migration lanes:

- `soc-q4c`: Dream pipeline through OpenClaw execution bus
- `soc-ni8g`: Mt. Olympus v0.2 production migration
- `psite-agu`: pipeline rearchitecture around agentopsd/headless agents
- `soc-y8b`: Dark Factory multi-host rollout
- `soc-v64.2`: agentopsd design-contract additions

Planning effect: select one lane at a time, document paused lanes, and require
activation smoke for infrastructure work.

### 5. Duplicate and stale-scope cleanup is real work

The input snapshot has obvious duplicate or related failures:

- `soc-7wwp` and `soc-qvpb`: same RPI dry-run execution-packet alias bug.
- `soc-xn5s` and `soc-2ctn`: close-loop dedup/terminal-state adjacent.
- `soc-w7s2` and close-loop stale-binary learnings: path freshness adjacent.
- Several deferred P3/P4 items have no immediate execution trigger.

Planning effect: create a graph-normalization wave that merges, relates,
defers, or closes stale work before expanding implementation.

### 6. Product gaps are useful tie-breakers, not the whole order

PRODUCT.md names Dream autonomy, pattern-to-skill promotion polish,
multi-runtime proof, messaging unity, and retrieval/worker knowledge
propagation as known gaps. These should break ties between P1/P2 lanes, but
they should not override active P0 incident cleanup or security work.

Planning effect: after P0, prefer work that improves compounding loops,
validation gates, and worker context propagation.

## Coverage Validation

Explored:

- bd open issue counts, ready counts, priorities, types, parents, dependencies
- high-priority issue titles and parent clusters
- existing recent plans for nightly automation, PR-toil reduction, and
  Olympus/agentopsd migration
- product constraints and repo execution profile
- relevant planning rules, pre-mortem checks, and close-loop learnings

Gaps:

- This did not read every full issue body for all 142 input issues.
- Cross-repo issues under `psite-*`, platform-lab, dotfiles, and showcase need
  their owning repo checked before implementation.
- Existing `soc-8412` plan body is mostly embedded in bd descriptions; a
  follow-up should verify the referenced plan path exists in the owning repo.

## Depth Validation

| Area | Depth | Notes |
|------|------:|-------|
| bd inventory and dependency graph | 3 | Quantified and grouped from live bd JSON. |
| P0 flywheel incident | 3 | Cross-checked against recent learnings and post-mortems. |
| Active local epics | 2 | Read current plans for two; used bd bodies for Bushido. |
| Cross-repo migration lanes | 2 | Enough to route, not enough to implement. |
| Low-priority P3/P4 backlog | 1 | Classified for defer/retire, not individually validated. |

## Assumptions

- "All issues" means all open bd issues, not closed historical records.
- The correct output is discovery and routing, not immediate implementation of
  142 issues.
- `soc-o6eb` should coordinate; it should not become the parent of every
  existing issue unless later evidence says reparenting is valuable.
