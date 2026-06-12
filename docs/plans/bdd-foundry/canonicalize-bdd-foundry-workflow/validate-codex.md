# Codex Validation - Proposed Bead Set

Verdict: **18/23 crank-ready**.

Definition used: a crank-ready bead has acceptance that is both (1) a concrete
invocable command and (2) materially checks the bead's own promised outcome from
`behaviors.md`, not just a downstream smoke condition. All 23 beads have runnable
commands; five are too thin to hand to crank unchanged.

| # | Bead | Scenario | Concrete invocable acceptance? | Crank-ready? | Note |
|---:|---|---|---|---|---|
| 1 | `prestate-worktree` | E4 | Yes: `bats edge.bats --filter '^E4 '` | Yes | Directly checks worktree evidence and captured pre-state. |
| 2 | `candidate-sweep` | S10 | Yes: `bats happy.bats --filter '^S10 '` | Yes | Directly checks sweep artifacts, candidate hashes, winner, and rule. |
| 3 | `source-snapshot` | S9 | Yes: `bats happy.bats --filter '^S9 '` | Yes | Directly checks snapshot file, checksum, and ordering against canonical commit. |
| 4 | `reconciliation-vacuous` | E2 | Yes: `bats edge.bats --filter '^E2 '` | Yes | Conditional/vacuous branch is still mechanically asserted. |
| 5 | `canonical-file` | S2 | Yes: `bats happy.bats --filter '^S2 '` | Yes | Directly checks byte delta against immutable snapshot. |
| 6 | `lineage-highest` | E1 | Yes: `bats edge.bats --filter '^E1 '` | Yes | Concrete highest-lineage check, though the v5/v6/v7 ground-fact drift should be cleaned up. |
| 7 | `precommit-syntax-gate` | X1 | Yes: `bats error.bats --filter '^X1 '` | **No** | Thin: it only proves landed canonical commits parse; it does not exercise a failing candidate or prove the pre-commit gate aborts before commit. |
| 8 | `meta-pure-literal` | S5 | Yes: `bats happy.bats --filter '^S5 '` | Yes | Direct static check of the exported meta block. |
| 9 | `suite-amendments` | S4 | Yes: `bats happy.bats --filter '^S4 '` | **No** | Thin: the bead promises two test-suite amendments, including X3 string-literal handling, but acceptance only reruns S4 and does not inspect the test files or the X3 amendment. |
| 10 | `install-script` | S11 | Yes: `bats happy.bats --filter '^S11 '` | Yes | Direct clean-HOME/idempotency/portability fixture. |
| 11 | `install-backup` | E5 | Yes: `bats edge.bats --filter '^E5 '` | Yes | Direct divergent-installed-file fixture. |
| 12 | `drift-check-script` | E6 | Yes: `bats edge.bats --filter '^E6 '` | Yes | Directly checks scoped blocking/report-only sibling behavior and remediation bead evidence. |
| 13 | `marker-check-script` | X2 | Yes: `bats error.bats --filter '^X2 '` | Yes | Direct mutation fixtures for marker/enforcement loss. |
| 14 | `blocking-parent` | S7 | Yes: `bats happy.bats --filter '^S7 '` | Yes | Directly proves mutated drift fails through both child check and parent. |
| 15 | `gate-registration` | X4 | Yes: `bats error.bats --filter '^X4 ' && (cd cli && go test ./internal/gates/...)` | **No** | Thin: concrete command, but it does not itself require `workflow.install-drift` to be registered unless a separate Go test is added correctly. A missing registration plus missing test can still pass. |
| 16 | `land-main` | S1 | Yes: `bats happy.bats --filter '^S1 '` | Yes | S1 is a concrete landed-state check; gate/citation details are covered by E3/S8/S12 beads. |
| 17 | `gate-surface-clean` | E3 | Yes: `bats edge.bats --filter '^E3 '` | Yes | Direct changed-surface and gate check at `LANDED_SHA`. |
| 18 | `apply-real-home` | S6 | Yes: `bats happy.bats --filter '^S6 '` | Yes | Direct installed-follow check with named command evidence. |
| 19 | `hazard-line-replaced` | S3 | Yes: `bats happy.bats --filter '^S3 '` | Yes | Direct canonical + installed HAZARD retirement check. |
| 20 | `evidence-artifacts` | S12 | Yes: `bats happy.bats --filter '^S12 '` | **No** | Thin: the bead title promises `evidence.env` with all 12 required keys, but S12 only requires `LANDED_SHA`; other keys are only indirectly consumed elsewhere. |
| 21 | `law0-exceptions` | X3 | Yes: `bats error.bats --filter '^X3 '` | **No** | Thin/misaligned: behaviors allow comment or string-literal LAW-0 exceptions listed file:line, but the current X3 test only accepts comment-prefixed hits and does not enumerate allowed recorded commands. |
| 22 | `bead-discipline` | S8 | Yes: `bats happy.bats --filter '^S8 '` | Yes | Direct bead id, close note, commit citation, and private-ledger checks. |
| 23 | `tracker-main-only` | X5 | Yes: `bats error.bats --filter '^X5 '` | Yes | Concrete artifact-based proxy for main-checkout tracker discipline. |

## Thin Beads

- `precommit-syntax-gate`: add a fixture or branch-local mutation that proves the verification command rejects a syntax-broken candidate before it can be committed, not just that final commits parse.
- `suite-amendments`: acceptance should grep/assert the actual `happy.bats` and `error.bats` amendments, including the X3 string-literal exception clause and provenance comments.
- `gate-registration`: acceptance should explicitly grep or unit-test the `workflow.install-drift` registration fields: ID, Fast|Full tiers, blocking=true, no Match globs, and backing script.
- `evidence-artifacts`: acceptance should require all 12 `evidence.env` keys in one place, not rely on incidental later test consumers.
- `law0-exceptions`: acceptance must match frozen X3: static-only recorded commands from the allowed set, no live workflow execution, and comment/string-literal LAW-0 exceptions listed file:line.

## Biggest Systemic Gap

The bead set leans on scenario-level end-state Bats filters as acceptance, but several
beads are actually changing the **test/evidence machinery itself**. Those beads need
self-checking acceptance for the machinery they mutate. Without that, crank can close a
bead because a downstream scenario is green while the promised amendment, gate
registration, or evidence schema was never enforced. The v5/v6/v7 source-version drift
across `behaviors.md`, `acceptance-tests.md`, and the manifest is a symptom of the same
problem: the executable suite is not being validated as a first-class artifact before
being used as the crank gate.
