# Triage Digest — 2026-04-28

## Items Rewritten (1)

| # | Title | Probe Evidence |
|---|-------|---------------|
| 1 | Add security to eval-suite.v1.schema.json domain enum | `.$defs.domain.enum[8] == "security"` confirmed via Python JSON parse of `schemas/eval-suite.v1.schema.json`. Already present at enum index 8. |

**Batch:** `ag-3lx` (line 58), `items[0]` marked `consumed=true`, `claim_status=consumed`, `consumed_by=stale-audit-2026-04-28`. Parent batch left `consumed=false` (items[1] and items[2] still open).

## Cross-Repo Items Skipped (10)

| Count | Target Repo | Item Titles |
|-------|-------------|------------|
| 2 | nami | Resolve command output contract drift for shared output flags; Collapse duplicate skill artifact trees into single-source generation |
| 1 | nami | Encode UAT scenarios as integration test fixtures |
| 3 | nami | Add cmd/ao to coverage baseline; Expand audit-assertion-density.sh to cover all test files; Add fuzz seed correctness assertions |
| 1 | nami | Fix coverage-ratchet.sh cmd/ao event format handling |
| 1 | nami | Add language filter to vibe complexity analysis |
| 2 | 20260419T062730Z-iter-1 | Production command refactors can miss the paired test diff expected by the command/test-pairing gate; Closed beads can cite ephemeral discovery seed paths that are absent from the repo |

**Distinct target repos seen:** `nami`, `20260419T062730Z-iter-1`

## Packets Rewritten

No overnight dir — `.agents/overnight/latest/morning-packets/` does not exist. Section 3 skipped entirely.

## Items Probed but Left (28 items)

Probes run and results ambiguous or work not yet done — left untouched:

| Batch | Title | Probe Result |
|-------|-------|-------------|
| 0.3 | Replace os.Chdir with path-based helpers | 433 `os.Chdir` sites still present in `cli/` — NOT done |
| 1.0 | Pre-seed agent prompts with known framework footguns | No tractable probe target; process improvement |
| 1.3 | Refactor production code to accept projectDir parameter | 127 `os.Getwd()` sites in production Go — NOT done |
| 1.4 | Add scope-escape reporting template for agents | No tractable probe target; process improvement |
| 4.2 | Resolve swarm-remediation-fix unresolved findings (8 items) | Swarm-remediation-fix batch (line 2) has 3 items all consumed, but item references "8 items" — count discrepancy is ambiguous; LEAVE |
| 6.0 | Make metadata verification a blocking CI gate | No fast CI gate probe; ambiguous |
| 6.1 | Enforce strict pre-mortem lock + epic-scoped gating | No fast probe |
| 6.2 | Add contract-atomic namespace refactor gate | No fast probe |
| 6.3 | Re-baseline roadmap closure for delivered vs deferred scope | No fast probe |
| 7.5 | Align TaskCreate command examples with validation allowlist | `taskcreate-examples.md` exists but alignment unverifiable in 5s |
| 9.5 | Add pwd assertions at test section boundaries | Process improvement, no probe target |
| 10.4 | Add grep-for-existing-functions to worker task metadata | Process improvement, no probe target |
| 13.5 | Define consumer contract before wiring scaffolding | Process improvement |
| 14.4 | Add codex mirror delta check to pre-commit sweep | `hooks/codex-parity-warn.sh` exists (warn only); item requests a BLOCK diff — ambiguous |
| 14.5 | Add sweep-findings triage gate (BLOCK vs WARN) | Process improvement |
| 15.4 | Enforce single-epic scope per crank session | Process improvement |
| 16.2 | Add cross-cutting integration wave to UAT template | No fast probe |
| 22.1 | Add count-verification conformance checks for doc plans | No fast probe |
| 31.1 | Collect BF6-BF9 evidence from first real-project application | No fast probe; requires running actual project |
| 36.1 | Audit context injection latency | hooks exist but full audit not verifiable in 5s |
| 56.1 | Decompose skills/crank/SKILL.md to under 248-line limit | 660 lines — NOT done (limit 248) |
| 56.2 | Rename --fast-path to --quick-gates across /rpi | `--fast-path` still present in cli/, skills/, docs/ — NOT done |
| 56.3 | Plan template: default-include skills-codex mirrors | No `skills-codex` mention in `skills/plan/SKILL.md` — NOT done |
| 56.4 | Runtime hook for RPI phase enforcement | `hooks/rpi-phase-enforcement.sh` does NOT exist; but item says "Measure first" — conditional; ambiguous |
| 57.0 | Centralize or fixture-lock agents write-surface scanner parity | Shell script has 0 `filepath.Join`, Go test has 12 — drift still present; NOT done |
| 58.1 | Verify security-toolchain-governance fixture on fresh macOS clone | Requires macOS hardware; unverifiable |
| 58.2 | Rename warn-only CI checks with explicit suffix | Jobs still named `agentops-eval-advisory`, `security-toolchain-gate` without `-warn` suffix — NOT done |
| 59.0 | Investigate GitHub-only agentops-eval-advisory failures | CI environment investigation; too complex for 5s probe |

## Schema Violations Open

All three validators run; one pre-existing failure (unchanged from baseline):

| Validator | Baseline | Post-Triage | Classification |
|-----------|---------|-------------|---------------|
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

Not triggered (1 rewrite, cap is 30).

## Commit SHA

See below (pending commit).

## Baseline Snapshot

- Unconsumed batches at start: 21
- Unconsumed items at start: 39
- Morning packets: 0 (no overnight dir)
- Pre-existing failures: `test-runtime-cursor-smoke.sh`
