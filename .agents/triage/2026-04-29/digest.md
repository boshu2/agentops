# Triage Digest — 2026-04-29

## Items Rewritten (3)

| # | Batch | Title | Probe Evidence |
|---|-------|-------|---------------|
| 1 | 1.3 | Refactor production code to accept projectDir parameter | `grep -r 'os\.Getwd\(\)' cli/ --include='*.go' \| grep -v '_test.go'` → 0 results; production os.Getwd() fully eliminated by 2026-04-28 nightly commits |
| 2 | 10.4 | Add grep-for-existing-functions to worker task metadata | `grep 'grep_check' skills/crank/SKILL.md` → line 355: `metadata.grep_check` mandate present; `skills/swarm/SKILL.md:115` also mandates workers grep before writing new code |
| 3 | 59.0 | Investigate GitHub-only agentops-eval-advisory failures | PR #172 (`67a783ac`) merged to main with `fix(ci): install eval advisory dependencies`; `validate.yml` confirmed has dependency install step for jq/ripgrep/bats/gocyclo/bd |

All three items marked `consumed=true`, `claim_status=consumed`, `consumed_by=stale-audit-2026-04-29`, `consumed_at=2026-04-29T00:00:00Z`.

Parent batches: batch_59 (line 59) had only 1 item — parent marked `consumed=true` as well. Batches 1 and 10 have other open items; parent left `consumed=false`.

## Cross-Repo Items Skipped (10)

Same set as 2026-04-28 triage (no new cross-repo items added overnight):

| Count | Target Repo | Item Titles (sample) |
|-------|-------------|---------------------|
| 2 | `nami` | Resolve command output contract drift; Collapse duplicate skill artifact trees |
| 1 | `nami` | Encode UAT scenarios as integration test fixtures |
| 3 | `nami` | Add cmd/ao to coverage baseline; Expand audit-assertion-density.sh; Add fuzz seed correctness assertions |
| 1 | `nami` | Fix coverage-ratchet.sh cmd/ao event format handling |
| 1 | `nami` | Add language filter to vibe complexity analysis |
| 2 | `20260419T062730Z-iter-1` | Production command refactors miss paired test diff; Closed beads cite ephemeral seed paths |

**Distinct target repos seen:** `nami`, `20260419T062730Z-iter-1`
**cross_repo_skipped: 10**

## Packets Rewritten

No overnight dir — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

## Items Probed but Left (25 items)

These were probed (by today's run or 2026-04-28 triage); work not done or ambiguous — untouched:

| Batch | Title | Probe Result |
|-------|-------|-------------|
| 0.3 | Replace os.Chdir with path-based helpers for t.Parallel readiness | 287 `os.Chdir` sites still in `cli/` test files; partial progress, not complete |
| 1.0 | Pre-seed agent prompts with known framework footguns | No tractable probe target (process improvement) |
| 1.4 | Add scope-escape reporting template for agents | Swarm skill has protocol but item scope is `*` (all agents); ambiguous |
| 4.2 | Resolve swarm-remediation-fix unresolved findings (8 items) | Referenced batch has count discrepancy; ambiguous |
| 6.0 | Make metadata verification a blocking CI gate | No metadata-verif gate in `.github/workflows/validate.yml`; NOT done |
| 6.1 | Enforce strict pre-mortem lock + epic-scoped gating | Pre-mortem gate exists but epic-scoped gating unclear; ambiguous |
| 6.2 | Add contract-atomic namespace refactor gate | Not found in hooks/scripts; NOT done |
| 6.3 | Re-baseline roadmap closure for delivered vs deferred scope | No roadmap closure doc in docs/; NOT done |
| 7.5 | Align TaskCreate command examples with validation allowlist | `validation-contract.md` has TaskCreate but allowlist diff unverifiable in 5s |
| 9.5 | Add pwd assertions at test section boundaries | Process improvement; no probe target |
| 13.5 | Define consumer contract before wiring scaffolding | Process improvement |
| 14.4 | Add codex mirror delta check to pre-commit sweep | `hooks/codex-parity-warn.sh` is warn-only; item requests BLOCK — ambiguous |
| 14.5 | Add sweep-findings triage gate (BLOCK vs WARN) | Process improvement |
| 15.4 | Enforce single-epic scope per crank session | Process improvement |
| 16.2 | Add cross-cutting integration wave to UAT template | Process improvement |
| 22.1 | Add count-verification conformance checks for doc plans | No fast probe |
| 31.1 | Collect BF6-BF9 evidence from first real-project application | Requires running actual project; NOT done (N/A still in test-pyramid.md) |
| 36.1 | Audit context injection latency | JIT re-arch shipped (86a822bb) but audit not yet run; ambiguous |
| 56.1 | Decompose skills/crank/SKILL.md to under 248-line limit | 660 lines — NOT done |
| 56.2 | Rename --fast-path to --quick-gates across /rpi | `--fast-path` still present in `cli/cmd/ao/rpi_phased.go` — NOT done |
| 56.3 | Plan template: default-include skills-codex mirrors | No `skills-codex` default mention in plan skill — NOT done |
| 56.4 | Runtime hook for RPI phase enforcement | `hooks/rpi-phase-enforcement.sh` NOT found; item conditional ("Measure first") — ambiguous |
| 57.0 | Centralize or fixture-lock agents write-surface scanner parity | PR #167 fixed a specific instance; centralization not yet done |
| 58.1 | Verify security-toolchain-governance fixture on fresh macOS clone | Requires macOS hardware; unverifiable |
| 58.2 | Rename warn-only CI checks with explicit suffix | Job names `agentops-eval-advisory` / `security-toolchain-gate` unchanged — NOT done |

## Schema Violations Open

All three validators run; one pre-existing failure unchanged from baseline:

| Validator | Baseline | Post-Triage | Classification |
|-----------|----------|-------------|---------------|
| `check-next-work-schema-rows.sh` | PASS | PASS | ✅ No change |
| `validate-next-work-contract-parity.sh` | PASS | PASS | ✅ No change |
| `smoke-test.sh` (next-work grep) | FAILED - 1 errors | FAILED - 1 errors | Pre-existing (`test-runtime-cursor-smoke.sh`); no regression |

Raw smoke output (filtered):
```
[TEST] Testing flywheel loop (next-work round-trip)...
  ✓ next-work.schema.md exists
  ✓ next-work contract parity validator passed
  ✓ next-work.jsonl: all 60 entries have valid schema
FAILED - 1 errors, 0 warnings
```
The 1 error is `test-runtime-cursor-smoke.sh failed` — documented in `baseline.json`, pre-existing on main.

## Queue-Flood Warning

Not triggered (3 rewrites; cap is 30).

## Baseline Snapshot

- Unconsumed items at start: 38 (down from 39 yesterday; 1 consumed by 2026-04-28 triage)
- Morning packets: 0 (no overnight dir)
- Pre-existing failures: `test-runtime-cursor-smoke.sh`

## bd Availability

bd CLI not available in this session; noted — did not block triage.

## Commit SHA

`c077a9d4` — PR #176: https://github.com/boshu2/agentops/pull/176
