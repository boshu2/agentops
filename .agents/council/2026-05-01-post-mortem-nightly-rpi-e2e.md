---
id: post-mortem-2026-05-01-nightly-rpi-e2e
type: post-mortem
date: 2026-05-01
target: soc-b8jo
commit: 80a21e2e4fcb8e3959074d5de7b05fe602bef536
---

> RPI streak: unavailable | Sessions: unavailable | Last verdict: WARN

# Post-Mortem: Nightly RPI E2E

## Council Verdict: WARN

The implementation is correct enough to keep shipped. The warning is operational,
not a revert signal: `ao rpi phased --dry-run` still mutates the tracked latest
execution-packet alias, and closed child beads do not expose scoped file paths
well enough for the closure audit parser.

## Scope Reviewed

Recent commits:

- `262b91f0` feat(nightly): add local evolution wrapper
- `21dec0ff` test(nightly): add evolution validation scenarios
- `80a21e2e` fix(rpi): isolate phased run context

Epic: `soc-b8jo` - Build local nightly evolution automation chain.

Plan sources:

- `.agents/plans/2026-05-01-cross-vendor-nightly-automation.md`
- `.agents/plans/2026-05-01-nightly-automation-chain.md`

## Checkpoint Preflight

| Check | Status | Detail |
|---|---|---|
| Chain loaded | PASS | `.agents/ao/chain.jsonl` present |
| Prior phases locked | WARN | Recent chain is for prior epics; no `soc-b8jo` chain yet |
| No FAIL verdicts | PASS | No blocking FAIL verdict found in checked reports |
| Artifacts exist | WARN | Several legacy chain entries point to non-verdict summaries |
| Idempotency | PASS | This is the first `soc-b8jo` post-mortem harvest |

## Closure Integrity

`bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto soc-b8jo`
returned 0 FAIL and 2 WARN:

- `soc-9dxe`: parser miss; no scoped files extracted from the closed child text.
- `soc-b8jo.2`: parser miss; no scoped files extracted from the closed child text.

Manual reconciliation: commit evidence exists in the three reviewed commits, but
future closure quality would improve if close reasons or evidence packets carried
the scoped files explicitly.

## Metadata Verification

Mechanical path extraction found false positives in the plan because it treated
validation commands and illustrative output names as file paths:

- `bash -n scripts/nightly-evolution.sh`
- `shellcheck --severity=error scripts/nightly-evolution.sh`
- `digest.json`
- `digest.md`
- `nightly.yml`
- `nightly-rpi-brief.yml`

Manual verdict: no blocking metadata failure. The first-iteration files exist,
and the docs links in changed Markdown files resolved.

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---|---|---|
| Code | PASS | `scripts/nightly-evolution.sh` and RPI phased setup/context changes are scoped to the plan. |
| Documentation | PASS | `docs/runbooks/nightly-evolution.md`, `docs/CI-CD.md`, and contract index updates document the local lane. |
| Examples | PASS | Runbook includes dry-run, Dream-only, Codex evolve, combined run, and systemd template examples. |
| Proof | WARN | BATS, Go tests, fast pre-push, Codex RPI/lifecycle guards passed; dry-run mutation follow-up remains. |

## Validation Evidence

- `bash -n scripts/nightly-evolution.sh`: PASS
- `bats --print-output-on-failure tests/scripts/nightly-evolution.bats`: PASS, 12/12
- `go test ./cmd/ao ./internal/rpi`: PASS during implementation closeout
- targeted RPI tests: PASS
- `scripts/pre-push-gate.sh --fast --scope worktree`: PASS during implementation closeout
- `bash scripts/validate-codex-rpi-contract.sh`: PASS
- `bash scripts/validate-codex-lifecycle-guards.sh`: PASS
- `./cli/bin/ao rpi phased --dry-run ... soc-b8jo`: PASS for routing; starts at Phase 2 and invokes `/crank soc-b8jo --test-first`

## Test Pyramid Assessment

| Issue | Planned | Actual | Gaps | Action |
|---|---|---|---|---|
| `soc-9dxe` | Shell wrapper, runbook, dry-run proof | BATS wrapper coverage plus fast pre-push | Digest/PR automation still later phase | Covered by `soc-lmoq` |
| `soc-b8jo.2` | Public validation scenarios and BATS guardrails | 4 JSON fixtures and 12 BATS cases | None for first slice | Done |
| RPI routing fix | Skill contract behavior and prompt isolation | Go unit tests plus dry-run prompt proof | Dry-run leaves tracked alias dirty | `soc-7wwp` |

## Prediction Accuracy

No `soc-b8jo` pre-mortem report was found, so prediction scoring is skipped.

## Learnings

1. RPI phase handoffs must be scoped by run ID before fallback summaries are
   injected. Legacy summary fallbacks are useful only when there is no current
   run identity.
2. Bead routing cannot assume one prefix. The RPI skill contract says "input is
   a bead"; runtime routing must use an issue-shaped parser and `bd show`, not
   hard-coded `ag-*` checks.
3. Dry-run means no tracked source/runtime mutation. Prompt inspection that
   dirties `.agents/rpi/execution-packet.json` violates the wrapper's own
   no-source-mutation policy.

## Follow-Up Items

- `soc-7wwp`: Keep RPI dry-run from dirtying tracked execution-packet alias.
- `soc-3wh7`: Repair local bd repo fingerprint and hook drift.
- Existing epic work remains open: `soc-6wuw`, `soc-lmoq`, and `soc-b8jo.1`.

## Knowledge Lifecycle

Phase 3 backlog scan:

- Scanned 4 learning files newer than `.agents/ao/last-processed`.
- Archived 1 low-quality ignored auto-extraction fragment.
- Found no duplicate tracked learning to merge.

Phase 4 activation:

- Added one clean tracked learning.
- Promoted one high-value lesson to `MEMORY.md`.
- Added one finding-registry row.
- Appended one next-work batch for `soc-b8jo`.

Phase 5 retirement:

- Archived the ignored noisy auto-extract fragment under `.agents/learnings/archive/`.

## Flywheel: Next Cycle

Highest-priority follow-up:

> Keep RPI dry-run from dirtying tracked execution-packet alias
> Make RPI dry-run preserve no-source-mutation semantics by avoiding writes to
> the tracked latest execution-packet alias.

Ready to run:

```bash
$rpi "Keep RPI dry-run from dirtying tracked execution-packet alias"
```
