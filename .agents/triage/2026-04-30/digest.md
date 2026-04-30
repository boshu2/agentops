# Triage Digest — 2026-04-30

**Branch:** `triage/2026-04-30`  
**Timer start:** 2026-04-30T00:00:00Z  
**Run by:** stale-audit-2026-04-30

---

## Items Rewritten (4)

| # | Line/Item | Title | Probe Evidence |
|---|-----------|-------|---------------|
| 1 | line=1 item=4 | Add scope-escape reporting template for agents | `skills/swarm/SKILL.md` has scope-escape protocol with standard JSON template; workers append to `.agents/swarm/scope-escapes.jsonl` |
| 2 | line=4 item=2 | Resolve swarm-remediation-fix unresolved findings (8 items) | `swarm-remediation-fix` batch (lines[2]) is `consumed=True`; all 3 items consumed; item description of "8 items at 0% resolution" was an incorrect count |
| 3 | line=10 item=4 | Add grep-for-existing-functions to worker task metadata | `skills/crank/SKILL.md`: "Grep-for-existing-functions (REQUIRED for new function issues)"; `skills/swarm/SKILL.md`: "Workers grep for existing function signatures before writing new code" |
| 4 | line=16 item=2 | Add cross-cutting integration wave to UAT template | `skills/crank/references/uat-integration-wave.md` exists with full UAT integration wave template including cross-feature pipeline validation |

**Batch parents also sealed:** lines[4] (evolve-cycle-6-coverage-85pct) and lines[10] (context-orchestration-leverage) — all items consumed after rewrites.

---

## Cross-Repo Items Skipped (10)

`cross_repo_skipped = 10`

| target_repo | Items |
|-------------|-------|
| `nami` | Resolve command output contract drift; Collapse duplicate skill artifact trees; Encode UAT scenarios as integration test fixtures; Add cmd/ao to coverage baseline; Expand audit-assertion-density.sh; Add fuzz seed correctness assertions; Fix coverage-ratchet.sh cmd/ao event format handling; Add language filter to vibe complexity analysis (8 items) |
| `20260419T062730Z-iter-1` | Production command refactors can miss paired test diff; Closed beads can cite ephemeral discovery seed paths (2 items) |

---

## Dream Packets Rewritten: 0

**No overnight dir** — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

---

## Schema Violations Open

### check-next-work-schema-rows.sh
```
PASS: 60 row(s) in next-work.jsonl conform to v1.4 schema enums
```
**Classification:** No violations. Same as baseline.

### validate-next-work-contract-parity.sh
```
next-work contract parity validation passed.
```
**Classification:** No violations. Same as baseline.

### smoke-test.sh (next-work/FAILED/PASSED lines)
```
[TEST] Testing flywheel loop (next-work round-trip)...
  ✓ next-work.schema.md exists
  ✓ next-work contract parity validator passed
  ✓ next-work.jsonl: all 60 entries have valid schema
FAILED - 1 errors, 0 warnings
```
**Classification:** The FAILED line is **pre-existing** (identical to baseline). Failure is `test-runtime-cursor-smoke.sh` — unrelated to next-work. No new violations introduced.

---

## Queue Status

| Metric | Before | After |
|--------|--------|-------|
| Unconsumed items | 38 | 34 |
| Rewrites | — | 4 |
| Cross-repo skipped | — | 10 |
| Batches sealed | — | 2 |

---

## Notes

- `bd` unavailable in this environment; not blocking.
- Queue-flood threshold (30 rewrites) not hit.
- 2026-04-28 triage had left items [3], [4], [13] as "no tractable probe" — this run found tractable probes via direct file/symbol greps.
