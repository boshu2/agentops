---
id: post-mortem-2026-05-01-soc-irg1
type: post-mortem
date: 2026-05-01
mode: quick
epic: soc-irg1
plan_path: .agents/plans/2026-05-01-gstack-absorption-tier1.md
pre_mortem: .agents/council/2026-05-01-pre-mortem-gstack-absorption-tier1.md
phase_2_summary: .agents/rpi/phase-2-summary-2026-05-01-crank-soc-irg1.md
vibe_report: .agents/council/2026-05-01-vibe-soc-irg1.md
---

# Post-Mortem: soc-irg1 (gstack absorption Tier 1)

## Council Verdict: PASS

Plan-vs-delivered: clean. All 5 planned issues closed with commit evidence. 2 follow-up issues (`soc-irg1.6` browse build, `soc-re0w` I5 long-tail) filed during execution per documented planning judgment calls — not scope creep, intentional decomposition.

Tech-debt: minimal. 3 functions in `cli/internal/skillshealth/audit.go` at complexity 16-17 (above warn, below fail). Acceptable for parser/comparator code; not refactor candidates.

Learnings: 4 reusable extracts (3 learnings + 1 pattern) — see Phase 2 below.

## Closure Integrity Audit

| Issue | Status | Evidence | Phantom? | Orphan? |
|---|---|---|---|---|
| soc-irg1.1 | closed | commit `63f19ba0` | no | no |
| soc-irg1.2 | closed | commit `9e1a259f` | no | no |
| soc-irg1.3 | closed | commit `890bdf0f` (+ `4591da66` post-wave sync) | no | no |
| soc-irg1.4 | closed | commit `4cb8b8a5` (decision doc only) | no | no |
| soc-irg1.5 | closed | commits `34125b94..2bbeb2dd` (4 commits, per pre-mortem F4) | no | no |

No orphans. No phantom beads. No multi-wave regressions detected (Wave 2 didn't remove Wave 1 code; it migrated callers to use new APIs).

## Plan vs Delivered Scope

| Planned | Delivered | Variance |
|---|---|---|
| 5 issues (I1-I5) | 5 issues closed | 0 |
| 2 waves | 2 waves | 0 |
| ~82-file refactor in I5 (per plan) | ~15-file focused exemplar pass + warn-only ratchet + follow-up | DELTA: orchestrator scope-refinement (see learning 2026-05-01-orchestrator-scope-refinement) |
| Browse build (Tier 1 #2) | Decision doc only + follow-up issue soc-irg1.6 | DELTA: per plan recommendation #1 (defer until design doc) |

Both deltas were planned operator decisions, not slip.

## Prediction Accuracy (vs Pre-Mortem)

| Pre-mortem ID | Prediction | Outcome | Score |
|---|---|---|---|
| F1 | I5 surface enumeration extends beyond cli/cmd/ao + hooks (must include cli/embedded + make sync-hooks) | HIT — actual baseline 305 files vs plan's ~82; orchestrator + worker both ran sync-hooks | HIT |
| F2 | I3 hook needs L2 fires test, not just `bash -n` | HIT — `tests/hooks/test-edit-scope-guard-fires.sh` shipped, 7/7 PASS | HIT |
| F3 | I3 hook needs malformed-input fail-open defense | HIT — verbatim block at hooks/edit-scope-guard.sh:18-27, test case 3 verifies | HIT |
| F4 | I5 mass refactor needs ≥4 commits per package family | HIT — exactly 4 commits, each independently green | HIT |

**Score: 4/4 HITS, 0 MISSES, 0 SURPRISES.** Prediction quality: high. The `--quick` pre-mortem proved sufficient for standard-complexity 5-issue epic.

## Test Pyramid Assessment

| Issue | Planned | Actual | Gaps |
|---|---|---|---|
| I1 | L1 + L2 cross-language | L1 (env precedence, 6 cases) + L2 (TestShellGoAgreement, 3 subcases) | none |
| I2 | L1 + L2 integration | L1 (frontmatter parser, parity comparator) + L2 (audit against real repo) | none |
| I3 | L1 + L2 race + L4 hook-fires | L1 (lock RW, IsAllowed) + L2 (TestWrite_AtomicityUnderConcurrency 100 goroutines, scope CLI roundtrip) + L4 (hook-fires 7-case) | none |
| I4 | n/a (decision doc) | n/a | n/a |
| I5 | L0 build, no regressions | full `make test` PASS, hook-fires still PASS, parity still PASS | none |

Weighted score 0.33, just above 0.3 threshold. PASS.

## Findings — Reusable

Persisted to sidecar (`.agents/findings/pending-2026-05-01-gstack-absorption.jsonl` — registry.jsonl is in UU state from prior session). 6 findings already there from research + pre-mortem; no new findings from post-mortem (the 4 learnings below capture the value).

## Phase 2: Learnings Extracted (4)

| File | Topic | Promotion |
|---|---|---|
| `.agents/learnings/2026-05-01-pre-mortem-amendments-held-orchestrator-pre-flight-correct.md` | Pre-mortem amendments held 4/4 via bd-notes propagation | learning (provisional, utility 0.7) |
| `.agents/learnings/2026-05-01-orchestrator-scope-refinement-after-baseline.md` | When pre-mortem flags undersized surface, orchestrator refines scope before worker dispatch | learning (provisional, utility 0.8) |
| `.agents/patterns/2026-05-01-hook-fires-test-pattern.md` | L4 hook-fires test pattern (simulate harness stdin contract) | **pattern** (provisional, utility 0.7) — promoted from learning to pattern because the test shape is directly reusable |
| `.agents/learnings/2026-05-01-new-audits-surface-pre-existing-debt.md` | New audit tools surface pre-existing debt; classify before treating as regression | learning (provisional, utility 0.6) |

The "atomic-write reuse" learning was considered but folded into the existing pattern doc `.agents/patterns/2026-05-01-state-path-resolver.md` (already shipped by I5). The "codex parity discipline held" observation is captured in the vibe report's Specific Concern §3 — promoting it to a standalone learning would be redundant with existing parity gate pattern docs.

## Tech Debt Identified

1. **6 broken references** in `discovery/rpi/validation` SKILLs pointing at missing `references/strict-delegation-contract.md` (surfaced by I2's `ao skills check`). Filed implicitly in vibe report; recommend filing as bd issue (suggested command in vibe report).
2. **`.agents/findings/registry.jsonl` UU state** from prior session blocks promoting the 6 sidecar findings into the canonical registry. Operator action: resolve merge, then `cat sidecar >> registry`.
3. **State-path resolver long tail** (~140 Go files) — tracked at `soc-re0w` (P3). Warn-only ratchet provides metric; flip-to-blocking after baseline data.
4. **`cli/embedded/` re-sync** is a manual step today (`make sync-hooks`). Could be a PostToolUse hook on `hooks/*` or `lib/*` writes. Not in scope for this epic.

## Flywheel: Next Cycle

Highest-priority follow-up from this post-mortem:

> **File bd issue for the 6 broken-ref bugs in discovery/rpi/validation SKILLs** (bug, P3)
> The new `ao skills check --strict` audit catches these on every CI run; without a tracked issue they'll show up in every future post-mortem.

Ready to run:
```bash
bd create --title "Fix 6 broken references/strict-delegation-contract.md links in /discovery, /rpi, /validation SKILLs" \
  --description "ao skills check (introduced by soc-irg1.2) surfaces 6 broken refs. Either create the missing reference file or remove the dead links. Discovered during soc-irg1 post-mortem 2026-05-01." \
  --type bug --priority 3
```

Other harvested items (in `.agents/rpi/next-work.jsonl`):
- Continue state-path resolver migration (already filed as `soc-re0w`)
- Implement browse contract-only skill (already filed as `soc-irg1.6`)
- Resolve `.agents/findings/registry.jsonl` UU state and promote 6 sidecar findings

## Decision

- [x] PASS — epic ready to close (operator decision; 2 follow-ups remain open under epic)
