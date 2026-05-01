---
id: post-mortem-2026-05-01-soc-o6eb-crank
type: post-mortem
date: 2026-05-01
target: soc-o6eb
verdict: WARN
source: "[[.agents/plans/2026-05-01-open-issue-portfolio-discovery.md]]"
---

# Post-Mortem: soc-o6eb Open Issue Portfolio Crank

**Epic:** `soc-o6eb`
**Branch:** `crank/soc-o6eb`
**Commit reviewed:** `9aa22651`
**Duration:** about 35 minutes for post-mortem review

> RPI streak: unavailable | Sessions: unavailable | Last verdict: WARN

## Council Verdict: WARN

The crank run completed the portfolio routing epic correctly: all five children
are closed, durable artifacts exist, and the branch is pushed. The warning is
not about the routing result. It is about the bd/Dolt closeout substrate:
tracker recovery required importing `bushido` from JSONL, and `bd dolt push`
still skips because no Dolt remote is configured.

| Judge | Verdict | Key Finding |
|-------|---------|-------------|
| Plan-Compliance | WARN | Scope stayed routing-only and all waves closed, but the plan's `.beads/issues.jsonl` assumption does not match the current Dolt-backed linked worktree. |
| Tech-Debt | WARN | Server-mode bd remote semantics remain confusing enough to require live recovery during closeout; this is already tracked by `soc-y8b.8`. |
| Learnings | PASS | The run proved the value of lead-only tracker mutation, force-adding ignored evidence, and closure-integrity grace-window audits. |

## Scope Delivered

Delivered:

- W0 proof stabilization and P0 route preservation.
- W1 graph normalization with duplicate, blocker, orphan, and deferral routing.
- W2 selected next work for `soc-8412`, `soc-b8jo`, and `soc-eh1z`.
- W3 selected `psite-agu` and paused overlapping migration lanes.
- W4 routed cross-repo security, P2 groups, operator gates, and P3/P4
  deferrals.
- Crank wave packet, worker result notes, archived shared notes, and checkpoint.

Not delivered, by design:

- Downstream implementation of the selected epics.
- Mass reparenting of existing implementation beads.
- Closure of tactical downstream work such as `psite-355`, `soc-5tky`, or
  platform-lab security items.

## Closure Integrity

| Check | Result | Details |
|-------|--------|---------|
| Evidence Precedence | PASS | 5 closed children checked: 2 commit-backed, 3 grace-window close-before-commit. |
| Phantom Beads | PASS | No generic or empty child beads detected. |
| Orphaned Children | PASS | `bd children soc-o6eb --json` shows all five children linked to the parent. |
| Multi-Wave Regression | PASS | The final crank commit adds only evidence artifacts; no later wave removed earlier work. |
| Stretch Goals | PASS | No stretch children were bulk-closed. |

Evidence modes:

- `commit`: `soc-o6eb.1`, `soc-o6eb.2`
- `grace-window`: `soc-o6eb.3`, `soc-o6eb.4`, `soc-o6eb.5`

Command:

```bash
bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto soc-o6eb
```

## Metadata Verification

Metadata verification returned WARN with seven findings. Six are expected
scope-model mismatches rather than closure failures:

- Three learning paths from the original applied-findings context are absent in
  this checkout.
- Two W2 upstream plan paths are absent; W2 recorded that discovery and routed
  from live bd plus committed scripts/docs instead.
- `.beads/issues.jsonl` is absent in this linked worktree because bd is using
  the canonical Dolt database.
- One command string was detected as a planned path false positive.

The warning should stay visible because the `.beads/issues.jsonl` mismatch was
not merely cosmetic: bd had to be repaired during crank before tracker writes
were reliable.

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---------|---------|----------|
| Code | PASS/NA | No source-code behavior was changed by W2-W4; this was a routing/documentation crank. |
| Documentation | PASS | W0-W4 durable artifacts are committed under `.agents/decisions/`, `.agents/triage/`, and `.agents/crank/`. |
| Examples | PASS/NA | No CLI help, examples, or user-facing command syntax changed. |
| Proof | WARN | L0/L1 gates passed and Git branch is pushed; bd Dolt push skipped because no Dolt remote is configured. |

## Validation Run

- `bd dep cycles`: PASS
- `bd show soc-o6eb soc-o6eb.3 soc-o6eb.4 soc-o6eb.5 --json`: PASS
- `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto soc-o6eb`: PASS
- `bash /home/boful/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex/crank/scripts/validate-wave-checkpoint.sh .agents/crank/wave-1-checkpoint.json`: PASS
- `npx --yes markdownlint-cli ...`: PASS
- `scripts/pre-push-gate.sh --fast`: PASS
- `bash scripts/check-worktree-disposition.sh`: PASS
- `git push -u origin crank/soc-o6eb`: PASS
- `bd dolt commit -m 'Crank soc-o6eb portfolio waves'`: PASS
- `bd dolt push`: WARN, no Dolt remote configured

## Prediction Accuracy

| Prediction | Result | Evidence |
|------------|--------|----------|
| `pm-20260501-001` routing epic becomes duplicate tracking | HIT, mitigated | The epic stayed routing-only and downstream implementation beads remained outside `soc-o6eb`. |
| `pm-20260501-002` source fix looks deployed while runtime is stale | HIT, mitigated | W0 checked installed `ao`, hook cache, and left `soc-2ctn` open without replay proof. |
| `pm-20260501-003` migration lanes overlap | HIT, mitigated | W3 selected `psite-agu` and explicitly paused `soc-q4c`, `soc-ni8g`, `soc-y8b`, and `soc-v64.2`. |
| `pm-20260501-004` dry-run mutates tracked state | HIT | W0 found `ao flywheel close-loop --dry-run` mutating finding metadata and filed `soc-73tk`. |
| `pm-20260501-005` 142-vs-live-count confusion | MISS, mitigated | W1 preserved the 142-input snapshot distinction. |
| `pm-20260501-006` ignored evidence paths break replay | HIT, mitigated | W2-W4 `.agents/` artifacts had to be force-added before commit. |
| `pm-20260501-007` parallel bd writes race | HIT, mitigated | Workers avoided bd mutations; lead serialized tracker writes. |
| `pm-20260501-008` product priority not mechanically checked | PARTIAL | W4 routed high-leverage security/operator work, but no product-priority command gate exists. |
| Surprise | HIT | bd/Dolt remote and project-state drift required recovery before the crank could close. |

## Learnings

### What Went Well

- Lead-only bd mutation kept W2-W4 parallel worker output from corrupting the
  tracker graph.
- The closure-integrity audit correctly accepted close-before-commit children
  through the grace-window path after the evidence commit landed.
- The crank checkpoint and archived shared notes made the wave replayable.

### What Was Hard

- The bd recovery path consumed more risk than the portfolio routing itself.
  The local metadata expected `bushido`, the served state exposed stale
  `beads_agentops`, and the eventual Dolt push had no configured remote.
- Metadata verification is still noisy around command snippets and applied
  context paths.

### Do Differently Next Time

- Run a bd context sanity check before claiming a crank wave:
  `bd vc status`, metadata database name, project id, and push-mode semantics.
- Treat absent upstream plan paths as first-class W2 evidence rather than a
  worker footnote.
- Keep force-add of ignored `.agents/` proof artifacts explicit in the wave
  closeout checklist.

## Test Pyramid Assessment

| Issue | Planned | Actual | Gaps | Action |
|-------|---------|--------|------|--------|
| `soc-o6eb.1` | L0/L1 tracker proof | L0/L1 bd readback, runtime freshness, hook cache checks | No replay archive proof | Keep `soc-2ctn` open until replay proof or equivalent closure. |
| `soc-o6eb.2` | L0/L1 graph normalization | L0/L1 duplicate/blocker/orphan scans and `bd dep cycles` | None for routing scope | Downstream implementations need their own tests. |
| `soc-o6eb.3` | L0 routing artifact | L0 artifact existence and targeted `rg` checks | No downstream tests, by design | Implement selected children separately. |
| `soc-o6eb.4` | L0 routing artifact | L0 artifact existence and targeted `rg` checks | No daemon smoke run, by design | Run activation smoke before `psite-agu` implementation. |
| `soc-o6eb.5` | L0 routing artifact | L0 artifact existence and targeted `rg` checks | No cross-repo validation run, by design | Run per-repo validation in owning worktrees. |

## Knowledge Lifecycle

Phase 3 scanned 27 learning files newer than `.agents/ao/last-processed`.
No duplicate merge or stale-retirement mutation was performed in this run.

Phase 4 wrote one new learning, promoted one memory entry, updated the
last-processed marker, and appended one next-work item.

Phase 5 archived no learnings.

## Next Work

Highest-priority harvested follow-up:

> **Repair bd server-mode push and project-state closeout contract**
> Resolve the bd/Dolt remote-contract drift exposed by `soc-o6eb`, including
> the no-remote push skip and stale/mismatched served database state.

Ready to run:

```text
/rpi "Repair bd server-mode push and project-state closeout contract"
```

Existing bd issue: `soc-y8b.8`.
