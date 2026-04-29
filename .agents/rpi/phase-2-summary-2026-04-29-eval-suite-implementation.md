# Phase 2 Summary: Crank Implementation

- **Epic:** ag-v29
- **Status:** PARTIAL
- **Timestamp:** 2026-04-29T20:52:00Z
- **Completed issues:** ag-v29.1, ag-v29.2
- **Remaining issues:** ag-v29.3, ag-v29.4, ag-v29.5, ag-v29.6, ag-cxh
- **Branch:** codex/eval-suite-implementation

## Delivered

- Added first-class eval `evidence_kind` schema fields, Go constants, validation, coverage aggregation, JSON output, and `--require-evidence-kind`.
- Documented what each evidence kind proves and does not prove in the eval environment contract.
- Added `ao eval baseline-audit` to compare suite baseline policy with promoted baseline files.
- Aligned 54 public canary suites with promoted baselines using `baseline_policy.mode=compare`; kept the two suites with no baseline as deliberate `none`.
- Made `scripts/eval-agentops.sh --fast` policy-aware so missing-baseline warnings only fire when suite policy expects a baseline.
- Updated brittle canaries affected by the new command and generated CLI docs.

## Proof

- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval coverage --root ../evals/agentops-core --json | jq -e '.evidence_kinds'`
- `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval baseline-audit --root ../evals/agentops-core --baseline-dir ../.agents/evals/baselines --json | jq -e '.policy_mismatch_count == 0'`
- `scripts/eval-agentops.sh --fast`
- `scripts/pre-push-gate.sh --fast --scope worktree`

## Follow-Up

- ag-cxh records the 54 stale baseline suite hashes surfaced by the new audit. Policy is aligned, but baseline hashes were not refreshed in this implementation pass.
