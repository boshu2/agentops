---
id: sweep-2026-05-01-nightly-rpi-e2e
type: post-mortem-sweep
date: 2026-05-01
target: soc-b8jo
---

# Sweep Manifest: Nightly RPI E2E

Scope reviewed:

- `scripts/nightly-evolution.sh`
- `tests/scripts/nightly-evolution.bats`
- `docs/runbooks/nightly-evolution.md`
- `cli/cmd/ao/rpi_phased_setup.go`
- `cli/cmd/ao/rpi_phased_context.go`
- `cli/cmd/ao/rpi_phased_handoff.go`
- paired RPI tests under `cli/cmd/ao/*_test.go`

## Findings

1. WARN: `ao rpi phased --dry-run` still writes the tracked latest execution packet alias `.agents/rpi/execution-packet.json`. The post-mortem run reset that file before writing this report. Follow-up bead: `soc-7wwp`.
2. WARN: `closure-integrity-audit.sh --scope auto soc-b8jo` returned parser warnings for `soc-9dxe` and `soc-b8jo.2` because the closed bead descriptions do not carry scoped file metadata. Commit evidence exists, but the mechanical closure proof is weaker than it should be.
3. PASS: RPI prompt contamination fix has direct tests for stale legacy summaries and structured handoffs from other run IDs.
4. PASS: Prefix-agnostic bead routing has direct tests for epic, child-to-parent, degraded-tracker, and healthy-tracker miss cases.
5. PASS: Nightly wrapper first-slice behavior has public scenario fixtures plus 12 BATS cases for dry-run, scheduler rendering, AI readiness, kill switch, lock, advisory brief failure, branch suffixing, Dream, and evolve invocation.

## Checklist

| Category | Result | Notes |
|---|---|---|
| Resource leaks | PASS | Wrapper uses lock cleanup trap; RPI tests avoid persistent fake harness files. |
| String safety | PASS | Shell paths are quoted in the wrapper tests exercised. |
| Dead code | PASS | New RPI helpers are covered by targeted tests. |
| Hardcoded values | WARN | Scheduler default is documented; acceptable for first iteration. |
| Edge cases | PASS | Degraded tracker and healthy tracker missing issue are covered. |
| Concurrency | PASS | Wrapper lock and kill switch are covered. |
| Error handling | WARN | Dry-run write side effect remains a follow-up. |
| Security | PASS | No credentials or network mutation added. |
