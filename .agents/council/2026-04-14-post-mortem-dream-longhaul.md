# Post-Mortem: Dream Long-Haul

Date: 2026-04-14
Scope: epic `na-22xi`, commits `a01235a3..0cfb0c44`
Duration: 8m
Cycle-Time Trend: Faster than the 2026-04-14 evolution-loop post-mortem because this review covered one epic and two landed commits instead of a multi-slice ratchet run.
Verdict: WARN

## Summary

The long-haul Dream slice delivered the intended product shape:

- default Dream remains short unless `--long-haul` is explicitly enabled
- long-haul now records trigger reason, exit reason, probe count, and yield deltas
- a packet-corroboration probe can strengthen the morning handoff before council runs
- retrieval-ratchet validation no longer hard-fails on a missing operator-local manifest when the checked-in fallback exists

Proof collected for this post-mortem:

- `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/overnight -timeout 2m` passed on 2026-04-14
- `bats tests/scripts/check-retrieval-quality-ratchet.bats` passed on 2026-04-14
- `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto na-22xi` found replay gaps in two closed child beads
- metadata verification over `a01235a3^..0cfb0c44` passed for changed-file existence and changed-doc cross-links
- the implementation already produced a real bounded Dream proof artifact at `/tmp/ao-dream-longhaul-vv6G0l/repo/.agents/overnight/validation-dream-longhaul-3/summary.json`, showing `long_haul.active=true`, packet corroboration, and top-packet confidence rising `medium -> high`

## Checkpoint Policy

| Check | Status | Detail |
|---|---|---|
| Chain loaded | SKIP | `.agents/ao/chain.jsonl` is absent in this standalone post-mortem worktree |
| Prior phases locked | SKIP | No chain rows were available to replay in this worktree |
| No FAIL verdicts | SKIP | Chain replay unavailable; relied on direct artifacts instead |
| Artifacts exist | WARN | The execution packet and Dream proof artifacts exist, but the seed artifacts cited by `na-22xi.1` and `na-22xi.2` do not |
| Idempotency | PASS | No existing next-work batch for source epic `na-22xi` was present before this post-mortem |

## Council Verdict

| Judge | Verdict | Key Finding |
|---|---|---|
| Plan-Compliance | WARN | The done criteria were largely met, but the epic is still open for `na-jox1`, and the closed-child proof surfaces were not fully durable |
| Tech-Debt | WARN | Council timeout behavior remains a product-risk follow-up, and closure-evidence hygiene needed a new bug bead (`na-22xi.4`) |
| Learnings | PASS | The strongest reusable pattern from this slice is to spend extra Dream runtime on corroboration before paying for council |

### Implementation Assessment

The shipped code matches the core execution packet objective in
`.agents/rpi/execution-packet.json`: it keeps the default path short, makes
long-haul opt-in, records additive decision telemetry, and spends extra runtime
on a bounded read-only probe that can materially improve morning packets. The
retrieval-ratchet fallback fix is also justified because the long-haul work
introduced a new validation dependency on retrieval proof that should not be
gated on an operator-local untracked manifest.

No new code-level defect emerged from the focused sweep over the changed source,
tests, docs, and script surfaces. The main problems left are operational:
council reliability still lags (`na-jox1`), and two closed child beads cite
missing discovery seed artifacts, which weakens the proof surface even though
the code itself landed correctly.

## Plan Vs Delivered

Planned from `.agents/rpi/execution-packet.json`:

- keep long-haul opt-in and preserve the short default
- add measurable yield telemetry and long-haul decision evidence
- spend time on a bounded read-only probe lane
- bound council cost
- keep docs and contracts in sync

Delivered:

- `cli/cmd/ao/overnight.go`, `cli/cmd/ao/overnight_longhaul.go`, and `cli/internal/overnight/longhaul.go` added the opt-in controller, budget parsing, artifact hydration, probe planning, and early-exit logic
- `cli/cmd/ao/overnight_packets.go` now applies packet corroboration before emitting morning packets
- tests landed in `cli/cmd/ao/overnight_test.go`, `cli/cmd/ao/overnight_packets_test.go`, and `cli/internal/overnight/longhaul_test.go`
- `cli/docs/COMMANDS.md`, `docs/contracts/dream-report.md`, and `docs/contracts/dream-run-contract.md` reflect the shipped controller and telemetry
- `scripts/check-retrieval-quality-ratchet.sh` and `tests/scripts/check-retrieval-quality-ratchet.bats` closed the gate regression caused by relying on a missing local manifest

Adjusted / deferred scope:

- `na-jox1` remains open, so council is still an optional enhancer instead of a default Dream cost
- post-mortem found a new closure-integrity bug: `na-22xi.4`

## Prediction Accuracy

Skipped with warning: the execution packet records `pre_mortem_verdict: WARN`,
but no matching `*pre-mortem*` report for this epic is present on disk, so
prediction IDs could not be scored.

## Four-Surface Closure

Code: PASS. The long-haul controller, corroboration lane, and gate fallback are
implemented and covered by targeted tests.

Documentation: PASS. CLI reference and Dream contracts were updated alongside
the code.

Examples: PASS. The shipped command/reference surfaces now document the
long-haul flags and generated artifacts instead of leaving operator behavior
implicit.

Proof: WARN. Local validation and a real bounded Dream artifact exist, but two
closed child beads still reference non-durable seed paths, so proof replay is
not fully clean.

## Closure Integrity

| Check | Result | Details |
|---|---|---|
| Evidence Precedence | FAIL | `na-22xi.1` and `na-22xi.2` resolve to `timing_miss` because their cited seed artifacts are not present in commit, staged, worktree, or packet evidence |
| Phantom Beads | PASS | Closed child titles and descriptions are specific |
| Orphaned Children | PASS | The epic child set resolved cleanly through `bd children na-22xi` |
| Multi-Wave Regression | PASS | The retrieval-ratchet fix extends the long-haul landing; it does not remove earlier Dream behavior |
| Stretch Goals | PASS | The unresolved council-timeout work stayed open as `na-jox1` instead of being hand-waved as complete |

Follow-up bug filed from this audit:

- `na-22xi.4` — persist Dream long-haul discovery evidence for closure audits

## Metadata Verification

Mechanical checks:

- 13 changed files in `a01235a3^..0cfb0c44`; all 13 exist on disk
- changed markdown/doc files resolved their changed-link references
- execution packet and shipped Dream report contracts remain present

Metadata warnings:

- the plan and research seed paths referenced in `na-22xi.1` and `na-22xi.2`
  are not present on disk, so those closures cannot use them as durable proof

## Test Pyramid Assessment

| Scope | Planned | Actual | Gaps | Action |
|---|---|---|---|---|
| `na-22xi` long-haul controller | L0/L1/L2 required, L3 recommended | L0/L1: `go test ./cmd/ao ./internal/overnight -timeout 2m`; L2: real bounded Dream artifact + ratchet BATS coverage | recommended L3 council-reliability proof still incomplete | keep council opt-in and resolve `na-jox1` |

## Learnings

### What Went Well

- The controller focused on yield instead of runtime vanity; packet corroboration produced a measurable `medium -> high` confidence gain without needing council.
- The retrieval-ratchet gate fix stayed small and honest: fallback only when no explicit override is set and the checked-in manifest exists.

### What Was Hard

- Closure proof was weaker than the implementation quality because the epic referenced seed artifacts that are no longer on disk.
- The council value story is still bottlenecked by the Claude timeout lane.

### Do Differently Next Time

- Persist or explicitly promote discovery artifacts before closed child beads cite them as proof.
- Treat “council is still optional” as a product contract until the slow lane is actually reliable.

### Patterns to Reuse

- Spend Dream long-haul budget on the cheapest probe that can raise morning-packet confidence before invoking council.

### Anti-Patterns to Avoid

- Using ephemeral `.agents/*` seed paths in closed-bead descriptions as if they were durable proof surfaces.

## Next Work

Highest-value next cycle:

`Investigate Dream Council Claude timeout behavior` (`na-jox1`)

Follow-on queue from this post-mortem:

1. `na-jox1` — investigate and fix the Dream Council Claude timeout behavior so council can justify default participation
2. `na-22xi.4` — persist Dream long-haul discovery evidence for closure audits

Suggested command:

```bash
$rpi na-jox1
```

## Prior Findings Resolution Tracking

Before this post-mortem, `.agents/rpi/next-work.jsonl` contained 53 entries,
204 items total, 98 resolved, and 106 unresolved (`48.04%` resolved). This
post-mortem adds one new `na-22xi` batch with two unresolved items.
