# Triage Digest — 2026-04-27

**Branch:** triage/2026-04-27  
**Wall-clock elapsed:** ~12 minutes (within 30-min budget)  
**Rewrites:** 8 items consumed, 4 parent batches promoted to fully-consumed  
**Packets rewritten:** no overnight dir

---

## 1. Items Rewritten (8 total)

| # | Batch/Item | Title | Probe Evidence |
|---|-----------|-------|----------------|
| 1 | batch 0 item 2 | Create pre-flight checklist for parallel worktree sprints | `scripts/preflight-swarm.sh` exists; header says "pre-flight checklist for parallel worktree sprints" |
| 2 | batch 2 item 1 | Produce footgun entries as required post-mortem output | `skills/post-mortem/references/maintenance-phases.md:401` — "**Footgun entries (REQUIRED):**" declared as mandatory post-mortem artifact |
| 3 | batch 8 item 4 | Add empirical threshold calibration pass for research-loop-detector | `hooks/research-loop-detector.sh:15` — "Calibration source: empirical observation across 200+ agentops sessions" |
| 4 | batch 11 item 2 | Add test-fixture impact count to plan baseline audit | `skills/plan/SKILL.md:91` — "test fixture counts are mandatory checks" in baseline audit step |
| 5 | batch 12 item 3 | Require explicit mapping tables in /plan for struct filtering | `skills/plan/references/implementation-detail.md:104-116` — explicit mapping table requirement with rationale |
| 6 | batch 13 item 4 | Add go-build verification for plan code snippets | `skills/plan/references/implementation-detail.md:36` — "Verify all inline snippets compile with `go build ./...` before including them in issue descriptions" |
| 7 | batch 15 item 5 | Document adhoc ID 1-second collision behavior | `cli/cmd/ao/inject_context_paths.go:27-38` — detailed collision behavior documented with crypto/rand fallback notes |
| 8 | batch 56 item 0 | Sweep skills-codex/ DAG bodies for Skill() to $skill notation | `grep -rn "Skill(" skills-codex/ --include="SKILL.md"` returned no actual Skill() invocations |

**Parent batches promoted to fully-consumed:** batch 2, batch 8, batch 11, batch 12

---

## 2. Cross-Repo Items Skipped (10 total)

**cross_repo_skipped: 10**

| Target Repo | Items |
|-------------|-------|
| `nami` (8 items) | Resolve command output contract drift for shared output flags; Collapse duplicate skill artifact trees; Encode UAT scenarios as integration test fixtures; Add cmd/ao to coverage baseline; Expand audit-assertion-density.sh to cover all test files; Add fuzz seed correctness assertions; Fix coverage-ratchet.sh cmd/ao event format handling; Add language filter to vibe complexity analysis |
| `20260419T062730Z-iter-1` (2 items) | Production command refactors can miss the paired test diff; Closed beads can cite ephemeral discovery seed paths |

---

## 3. Morning Packets

**Status: no overnight dir** — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

---

## 4. Schema Violations

All three validators run pre- and post-rewrite. Results:

### check-next-work-schema-rows.sh
```
PASS: 57 row(s) in .agents/rpi/next-work.jsonl conform to v1.4 schema enums
```
**Classification:** PASSING (matches baseline — no regression)

### validate-next-work-contract-parity.sh
```
next-work contract parity validation passed.
```
**Classification:** PASSING (matches baseline — no regression)

### smoke-test.sh (next-work lines)
```
✓ next-work.schema.md exists
✓ next-work contract parity validator passed
✓ next-work.jsonl: all 57 entries have valid schema
FAILED - 1 errors, 0 warnings
```
**Classification:** PRE-EXISTING failure — `test-runtime-cursor-smoke.sh` was failing in baseline.json before any rewrites. Not caused by triage. Not repaired (that's /evolve's domain).

---

## 5. Queue-Flood Warning

Not triggered (8 rewrites well under cap of 30).

---

## 6. Items Probed but Left (ambiguous / not done)

Items probed and left for /evolve to handle (not stale):

- **batch 0 item 3** — Replace os.Chdir with path-based helpers: 3 production `os.Chdir` calls remain (was 20+, work ongoing)
- **batch 1 item 3** — Refactor production code to accept projectDir: 124 `os.Getwd()` calls remain in production
- **batch 4 item 2** — Resolve swarm-remediation-fix unresolved findings: no batch with that ID found; ambiguous
- **batch 56 item 1** — Decompose skills/crank/SKILL.md: 660 lines (limit 248) — not done
- **batch 56 item 2** — Rename --fast-path to --quick-gates: `--fast-path` still in cli/cmd/ao/rpi_phased.go
- **batch 56 item 4** — Runtime hook for RPI phase enforcement: hooks/rpi-phase-enforcement.sh does not exist
- **batch 36 item 1** — Audit context injection latency: audit task, no evidence of completion

---

## 7. Commit SHA

See git log — commit made on branch `triage/2026-04-27`.
