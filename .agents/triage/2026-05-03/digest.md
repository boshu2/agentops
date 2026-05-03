# Triage Digest — 2026-05-03

**Branch:** triage/2026-05-03  
**Timer:** ~22 minutes of 30-minute budget  
**Rewrites:** 8 items marked consumed  
**Cross-repo skipped:** 31 items  
**Packets rewritten:** no overnight dir  

---

## Items Rewritten (8 total)

### 1. Line 5 IDX=2 — Resolve swarm-remediation-fix unresolved findings (8 items)
**Probe:** Python audit of next-work.jsonl showed swarm-remediation-fix batch (line 3) has parent consumed=True and 0/3 unconsumed items.  
**Evidence:** `swarm-remediation-fix batch (line 3) now has 0 unconsumed items; parent consumed=True, claim_status=consumed`  
**Note:** Parent batch (line 5, evolve-cycle-6-coverage-85pct) now ALL consumed; parent updated to consumed=True.

### 2. Line 59 IDX=2 — Rename warn-only CI checks with explicit suffix
**Probe:** `grep -n "continue-on-error" + grep -B2 "continue-on-error" .github/workflows/validate.yml`  
**Evidence:** `validate.yml: agentops-eval-advisory (warn-only), security-toolchain-gate (warn-only), doctor-check (warn-only), check-test-staleness (warn-only): all 4 continue-on-error jobs have (warn-only) suffix`

### 3. Line 61 IDX=1 — Fix installed post-mortem evidence writer path assumptions
**Probe:** `sed -n '1,30p' skills/post-mortem/scripts/write-evidence-only-closure.sh`  
**Evidence:** `write-evidence-only-closure.sh lines 13-24: dynamic WORKSPACE_ROOT detection with 3 candidate paths (_workspace_has_assets probe); no longer hardcodes plugin cache path`

### 4. Line 65 IDX=0 — Fix scripts/generate-cli-reference.sh to walk all Cobra subtrees
**Probe:** `bash scripts/generate-cli-reference.sh` regenerated COMMANDS.md; `grep -c "^### \`ao"` and `grep -c "^#### \`ao"`  
**Evidence:** `COMMANDS.md now has 62 level-3 + 152 level-4 = 214 command entries (was 58); cli/cmd/ao/cobra_conformance_test.go promoted from .agents/staging/ to production`

### 5. Line 68 IDX=2 — Cleanup duplicate pend-* artifacts in agentops .agents/learnings and .agents/patterns
**Probe:** `ls .agents/learnings/ | grep "^pend-" | wc -l` and `ls .agents/patterns/ | grep "^pend-" | wc -l`  
**Evidence:** `0 pend-* files in learnings and patterns (4038 duplicates cleaned)`

### 6. Line 69 IDX=0 — Add binary-deployment gate to /implement skill
**Probe:** `grep -n "binary.deployment" skills/implement/SKILL.md` and `test -f skills/implement/references/binary-deployment-gate.md`  
**Evidence:** `skills/implement/SKILL.md:315 references binary-deployment-gate.md; file EXISTS with full gate spec`

### 7. Line 69 IDX=3 — Add manifest + threshold-pause to ao dedup --merge
**Probe:** `grep -n "DedupManifest\|BuildDedupManifest\|WriteDedupManifest\|CountArchiveCandidates" cli/internal/lifecycle/dedup.go`  
**Evidence:** `DedupManifest struct + BuildDedupManifest + WriteDedupManifest + CountArchiveCandidates all present; MergeDedupGroups calls BuildDedupManifest at lines 213,228 and WriteDedupManifest before file moves`

### 8. Line 83 IDX=3 — Decide whether shared backend-codex-subagents.md should mention --mixed
**Probe:** `grep -n "--mixed" skills/shared/references/backend-codex-subagents.md` and mirrored copies  
**Evidence:** `grep => line 3; same in skills/research/ and skills/council/ mirrors: decision made, all 3 copies mention --mixed`

---

## Items Found Stale But NOT Rewritten

### Line 62 IDX=0 — Verify pend- triple-ID fix and clean up polluted learning files
**Why stale:** `ls .agents/learnings/ | grep "pend-" | wc -l => 0`; `ls .agents/learnings/ | grep "pend-.*pend-" | wc -l => 0`  
**Why not rewritten:** Parent batch (line 62) has `consumed=True` with 8 other unconsumed items. Adding explicit lifecycle fields to any item in this batch triggers `validate-next-work-contract-parity.sh` lifecycle drift detection (aggregate=consumed, items=available). Revert confirmed necessary. Left for /evolve to resolve parent inconsistency.

### Line 62 IDX=7 — Wire JobSpec v0 into CLI submission or delete dead routes
**Why stale:** `grep -rn "v1/jobs" cli/cmd/ao/` → daemon_jobs.go:328 and plans.go:612 both POST to /v1/jobs. Item already has status=wont_fix with detailed rejection reason.  
**Why not rewritten:** Same pre-consumed parent batch issue as IDX=0 above.

---

## Cross-Repo Items Skipped

**Count:** 31  
**Target repos seen:**
- `dogfood-2026-05-01-iter-1`: 19 items (line 73)
- `nami`: 8 items (lines 17, 19, 20, 23)
- `20260419T062730Z-iter-1`: 2 items (line 56)
- `20260429T041912Z-iter-1`: 1 item (line 63)
- `external-mlx-lm`: 1 item (line 85 IDX=0)

---

## Morning Packets

No overnight dir — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped.

---

## Schema Violations (Section 4)

### check-next-work-schema-rows.sh — FAIL (PRE-EXISTING)
```
FAIL: line 62 item 4 (Add eval determinism rerun harness and gate baseline-audit i): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: line 62 item 9 (Add measurement-command audit pre-push gate for numeric doc ): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: 2 schema violation(s) in next-work.jsonl
```
**Classification:** Pre-existing (matches baseline.json schema_rows_output). These two items in the pre-consumed batch (line 62) have type=test which is not in the allowed enum.

### validate-next-work-contract-parity.sh — PASS
No violations. Matches baseline.

### smoke-test.sh — FAILED (PRE-EXISTING)
```
[TEST] Testing flywheel loop (next-work round-trip)...
  ✓ next-work.schema.md exists
  ✓ next-work contract parity validator passed
  ✓ next-work.jsonl: all 88 entries have valid schema
FAILED - 1 errors, 0 warnings
```
**Classification:** Pre-existing (matches baseline.json smoke_grep_output). Failure cascades from schema-rows violation.

---

## Pre-Existing Batch Anomaly (Informational for /evolve)

Line 62 (source_epic likely containing eval/council items) has `consumed=True` on the parent object but 8 unconsumed items with valid types, plus 2 items with invalid type=test. This inconsistency pre-dates this triage run. Resolution options: (a) mark all 8 remaining items consumed after verification, or (b) fix item 4 and 9 type fields and then run a full consume pass.

---

## Commit SHA

`788d6ea` — PR #221: https://github.com/boshu2/agentops/pull/221
