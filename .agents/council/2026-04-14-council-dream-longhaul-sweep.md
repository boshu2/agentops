# Sweep Manifest: Dream Long-Haul Post-Mortem

Date: 2026-04-14
Scope: epic `na-22xi`, commits `a01235a3..0cfb0c44`

## Files Reviewed

- `cli/cmd/ao/overnight.go`
- `cli/cmd/ao/overnight_longhaul.go`
- `cli/cmd/ao/overnight_council.go`
- `cli/cmd/ao/overnight_packets.go`
- `cli/internal/overnight/longhaul.go`
- `cli/cmd/ao/overnight_test.go`
- `cli/cmd/ao/overnight_packets_test.go`
- `cli/internal/overnight/longhaul_test.go`
- `scripts/check-retrieval-quality-ratchet.sh`
- `tests/scripts/check-retrieval-quality-ratchet.bats`
- `cli/docs/COMMANDS.md`
- `docs/contracts/dream-report.md`
- `docs/contracts/dream-run-contract.md`

## Categories Checked

- resource leaks
- string/path safety
- dead code / drift
- hardcoded values / policy defaults
- edge cases / empty-state handling
- concurrency / sequencing
- error handling / degradation
- command / contract / proof integrity

## Findings

1. No new correctness or security defect was found in the shipped long-haul controller, packet corroboration lane, or retrieval-ratchet fallback patch.
2. Long-haul value still depends on council reliability work tracked separately in `na-jox1`; the implementation correctly keeps that cost bounded and optional rather than silently making it the default path.
3. Closure integrity for the epic is weaker than the code quality: `na-22xi.1` and `na-22xi.2` cite missing seed artifacts under `.agents/brainstorm/` and `.agents/research/`, so replayable evidence for those closures is incomplete. Follow-up is tracked in `na-22xi.4`.
4. Test coverage is good at L0/L1/L2 for the shipped controller path, but the recommended deeper council-reliability proof remains incomplete until `na-jox1` closes.
