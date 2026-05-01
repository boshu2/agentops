# Triage Digest — 2026-05-01

Run started: 2026-05-01T00:00:00Z (epoch 1777612244)  
Branch: `triage/2026-05-01` (no collision)  
bd: unavailable (not installed in this environment; no blocking on it)

---

## Items Rewritten (1 item)

| # | Title | Probe Evidence |
|---|-------|---------------|
| 1 | **Rename warn-only CI checks with explicit suffix** (L59[2]) | `grep validate.yml: name: agentops-eval-advisory (warn-only)` and `name: security-toolchain-gate (warn-only)` — both `(warn-only)` suffixes already present in `.github/workflows/validate.yml` |

---

## Cross-Repo Items Skipped (10 items)

| Target Repo | Count | Titles |
|------------|-------|--------|
| `nami` | 8 | Resolve command output contract drift; Collapse duplicate skill artifact trees; Encode UAT scenarios as integration test fixtures; Add cmd/ao to coverage baseline; Expand audit-assertion-density.sh; Add fuzz seed correctness assertions; Fix coverage-ratchet.sh cmd/ao event format; Add language filter to vibe complexity analysis |
| `20260419T062730Z-iter-1` | 2 | Production command refactors miss paired test diff; Closed beads cite ephemeral discovery seed paths |

---

## Packets Rewritten

**No overnight dir** — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

---

## Validator-Blocked Stale Items (5 items — could not rewrite)

These items have `status=completed` or `status=wont_fix` in their item bodies but are in line 62 whose **parent batch already carries `consumed=true` / `claim_status="consumed"`** (set by `rpi-auto-2026-04-30-1648`). Adding `consumed=true` to any individual item triggers the contract-parity validator with:

> FAIL: next-work.jsonl has aggregate/item lifecycle drift: line 62 source_epic=post-mortem-v2.39.0..HEAD-2026-04-30 aggregate=consumed items=available

Per protocol, each rewrite was restored. These items are substantively done but cannot be marked without a targeted repair of the entire line 62 batch (all 10 items must reach terminal state simultaneously, which would require marking in_progress/deferred items — not this triage's domain).

| Item | Status in Item Body | Evidence |
|------|---------------------|---------|
| Wire brief_render into overnight packets or delete dead code (L62[1]) | `completed` | `git log`: commit `2414e98c` `refactor(context): delete unwired brief_render extraction` |
| Fix supervisor silent context-cancellation in daemon (L62[2]) | `wont_fix` | False positive: `TestSupervisor_RunLoopCancelDoesNotFailRunningJob` asserts silent return is correct |
| Make Tier 3 worktree cleanup gate halt waves on stragglers (L62[3]) | `completed` | Commit `2079ff78`; `parallel-wave-isolation.md` now reads "blocking gate: exits non-zero on any unexpected worktree" |
| Wire JobSpec v0 into CLI submission or delete dead routes (L62[7]) | `wont_fix` | False positive: `submitRPIPhasedDaemon` in `rpi_phased_daemon.go:31` does POST `/v1/jobs` |
| Add measurement-command audit pre-push gate for numeric doc claims (L62[9]) | `completed` | Commit `5842445c` |

**Recommendation for /evolve:** The entire line 62 batch needs a single coordinated rewrite that resolves the remaining active items (L62[0] in_progress, L62[4/5/6/8] deferred) before the parent-level drift can be repaired.

---

## Schema Violations Open

### 1. `check-next-work-schema-rows.sh` — EXIT 1 (PRE-EXISTING)

```
FAIL: line 62 item 4 (Add eval determinism rerun harness and gate baseline-audit i): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: line 62 item 9 (Add measurement-command audit pre-push gate for numeric doc ): type=test not in {tech-debt improvement pattern-fix process-improvement feature bug task docs chore}
FAIL: 2 schema violation(s) in /home/user/agentops/.agents/rpi/next-work.jsonl
```

**Classification: PRE-EXISTING** — identical to `baseline.json` `schema_rows_output`. No regression introduced by this triage run.

### 2. `validate-next-work-contract-parity.sh` — PASSED

Matches baseline. No regression.

### 3. `tests/smoke-test.sh` (next-work grep) — FAILED (PRE-EXISTING)

```
[TEST] Testing flywheel loop (next-work round-trip)...
  ✓ next-work.schema.md exists
  ✓ next-work contract parity validator passed
  ✓ next-work.jsonl: all 62 entries have valid schema
FAILED - 1 errors, 0 warnings
```

**Classification: PRE-EXISTING** — failure is `test-runtime-cursor-smoke.sh`, unrelated to next-work. Matches baseline `smoke_grep_output`.

---

## Queue-Flood Warning

Not triggered (1 rewrite << 30 cap).

---

## Commit SHA

See git log on `triage/2026-05-01` branch.
