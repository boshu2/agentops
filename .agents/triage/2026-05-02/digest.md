# Triage Digest — 2026-05-02

**Branch:** triage/2026-05-02
**Elapsed:** ~8m of 30m limit
**bd availability:** unavailable in this environment; no blocking occurred.

---

## Items Rewritten

**Count: 29** (30 attempted, 1 skipped due to validator regression — see note below)

Queue-flood warning was triggered: hit the 30-rewrite cap. 3 additional stale orphan items from line 74 (items 21, 22, 23) were not attempted due to cap. 1 skipped after cap due to validator regression.

### First 5 rewritten items + probe evidence

1. **[line 64 item 9] Rescue orphan: 2026-04-22-bug-hook-prevention-ratchet-toolchain.md**
   - Probe: `test -f .agents/research/2026-04-22-bug-hook-prevention-ratchet-toolchain.md` → ABSENT
   - Evidence: file absent from .agents/research/ — orphan moot (no file to rescue)

2. **[line 64 item 10] Rescue orphan: 2026-04-23-evolve-phased-lifecycle.md**
   - Probe: `test -f .agents/research/2026-04-23-evolve-phased-lifecycle.md` → ABSENT
   - Evidence: file absent from .agents/research/ — orphan moot (no file to rescue)

3. **[line 68 item 2] Cleanup duplicate pend-* artifacts in agentops .agents/learnings and .agents/patterns**
   - Probe: `ls .agents/learnings/ | grep '^pend-' | wc -l` → 0
   - Evidence: ls .agents/learnings/ | grep '^pend-' | wc -l = 0 — cleanup already complete

4. **[line 68 item 3] Tag a release that ships the close-loop lifecycle fix**
   - Probe: `git tag | grep v2.39` → v2.39.0
   - Evidence: git tag shows v2.39.0 — release already tagged and shipped

5. **[line 72 item 0] Merge and release archive-aware promoted-body dedupe fix**
   - Probe: `git log origin/main | grep 64fc3b7d` → "64fc3b7d Merge branch 'fix/close-loop-promoted-dedupe-src'"
   - Evidence: git log shows 64fc3b7d 'Merge fix/close-loop-promoted-dedupe-src' on main; v2.39.0 tagged
   - Note: parent batch (line 72) also marked consumed — it had only 1 item.

### All 29 rewritten items

| Line | Item | Title (truncated) | Probe Evidence |
|------|------|-------------------|----------------|
| 64 | 9 | Rescue orphan: 2026-04-22-bug-hook-prevention-ratchet-toolchain.md | file absent from .agents/research/ |
| 64 | 10 | Rescue orphan: 2026-04-23-evolve-phased-lifecycle.md | file absent from .agents/research/ |
| 64 | 11 | Rescue orphan: 2026-04-23-goals-product-runtime-gap-plan.md | file absent from .agents/research/ |
| 64 | 12 | Rescue orphan: 2026-04-23-pattern-to-skill-pipeline-detection.md | file absent from .agents/research/ |
| 64 | 14 | Rescue orphan: 2026-04-24-bug-beads-dolt-crash.md | file absent from .agents/research/ |
| 68 | 2 | Cleanup duplicate pend-* artifacts | pend-* count = 0 |
| 68 | 3 | Tag a release (close-loop lifecycle fix) | v2.39.0 exists |
| 69 | 2 | Tag release that ships close-loop fix | v2.39.0 exists |
| 69 | 6 | Dedupe two same-day close-loop opt-in learnings | both learning files absent from .agents/learnings/ |
| 70 | 0 | Tag release shipping close-loop fix + council items | v2.39.0 exists |
| 72 | 0 | Merge and release archive-aware promoted-body dedupe fix | merged 64fc3b7d + v2.39.0 released |
| 74 | 0 | Rescue orphan: 2026-04-26-branch-consolidation-audit.md | file absent from .agents/research/ |
| 74 | 1 | Rescue orphan: 2026-04-26-bug-codex-hooks-config.md | file absent |
| 74 | 2 | Rescue orphan: 2026-04-26-docs-index-case-collision.md | file absent |
| 74 | 3 | Rescue orphan: 2026-04-26-proposal-2-external-watchlist-implementation.md | file absent |
| 74 | 4 | Rescue orphan: 2026-04-26-state-since-v2.38.0.md | file absent |
| 74 | 5 | Rescue orphan: 2026-04-26-wire-install-bd-into-bootstrap.md | file absent |
| 74 | 6 | Rescue orphan: 2026-04-27-eval-branch-commit-and-conflict-analysis.md | file absent |
| 74 | 7 | Rescue orphan: 2026-04-27-eval-closure-integrity-and-foundation-isolation.md | file absent |
| 74 | 8 | Rescue orphan: 2026-04-27-eval-suite-domain-security.md | file absent |
| 74 | 9 | Rescue orphan: 2026-04-28-agentops-daemon-atomic-slice-fit.md | file absent |
| 74 | 10 | Rescue orphan: 2026-04-28-agentops-daemon-gascity-vertical-slices.md | file absent |
| 74 | 11 | Rescue orphan: 2026-04-28-pr-issue-ci-status.md | file absent |
| 74 | 12 | Rescue orphan: 2026-04-29-bushido-overnight-soak-introspection.md | file absent |
| 74 | 15 | Rescue orphan: 2026-04-29-remote-compute-agent-session-control-plane.md | file absent |
| 74 | 17 | Rescue orphan: 2026-04-30-brief-render-disposition.md | file absent |
| 74 | 18 | Rescue orphan: 2026-04-30-daemon-absorption-architecture.md | file absent |
| 74 | 19 | Rescue orphan: 2026-04-30-daemon-absorption-map.md | file absent |
| 74 | 20 | Rescue orphan: 2026-04-30-daemon-existing-architecture.md | file absent |

### Skipped item (validator regression)

**[line 62 item 1] Wire brief_render into overnight packets or delete dead code**
- Probe: `test -f cli/internal/context/brief_render.go` → ABSENT (dead code was deleted)
- Reason skipped: marking this item consumed revealed a pre-existing parent/item lifecycle drift on line 62. The parent batch at line 62 already had `consumed=true` in the original file, but 9 of 10 items had no explicit lifecycle fields. Adding explicit lifecycle to item 1 triggered the `lifecycle_drift` check in validate-next-work-contract-parity.sh. Reverted per protocol. The pre-existing drift itself is a validator concern for `/evolve` — not repaired here.

### Queue-flood (cap hit)

3 stale orphan items not attempted (line 74 items 21, 22, 23: daemon-prior-corpus-extract.md, daemon-tdd-bdd-spec-pattern.md, quality-warning.md — all ABSENT from .agents/research/).

---

## Cross-repo Items Skipped

**Count: 30**

| target_repo | Count |
|-------------|-------|
| nami | 8 |
| dogfood-2026-05-01-iter-1 | 19 |
| 20260419T062730Z-iter-1 | 2 |
| 20260429T041912Z-iter-1 | 1 |

---

## Dream Packets Rewritten

**No overnight dir** — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

---

## Schema Violations Open

### check-next-work-schema-rows.sh
**Status: PRE-EXISTING (matches baseline)**
```
FAIL: line 62 item 4 (Add eval determinism rerun harness and gate baseline-audit i): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: line 62 item 9 (Add measurement-command audit pre-push gate for numeric doc ): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: 2 schema violation(s) in /home/user/agentops/.agents/rpi/next-work.jsonl
```
Diff against baseline: identical output. No regression introduced.

### validate-next-work-contract-parity.sh
**Status: PASSING** (matches baseline)
```
next-work contract parity validation passed.
```

### tests/smoke-test.sh (next-work/FAILED/PASSED grep)
**Status: PRE-EXISTING FAILURE (matches baseline)**
```
✓ next-work.schema.md exists
✓ next-work contract parity validator passed
✓ next-work.jsonl: all 74 entries have valid schema
FAILED - 1 errors, 0 warnings
```
Root failure: `test-runtime-cursor-smoke.sh failed` — unrelated to next-work, pre-existing.

---

## Queue-Flood Warning

**TRIGGERED.** Hit the 30-rewrite cap. 3 additional stale orphan items (line 74 items 21-23) and 1 item skipped due to validator regression (line 62 item 1). Total stale items identified: ~33.

---

## Commit SHA

579f6696 — PR: https://github.com/boshu2/agentops/pull/207
