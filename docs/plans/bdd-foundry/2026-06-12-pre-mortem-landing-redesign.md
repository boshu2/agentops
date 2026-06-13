# Pre-Mortem — land.sh landing redesign (37 beads)

> Blind 4-judge council, 2026-06-12. No-self-grading invariant honored: the
> orchestrator authored the plan; all four judges were context-isolated agents
> with zero authoring context. Run on Opus 4.8 (Fable access was revoked
> mid-run — the four judges were relaunched on the inherited model).

## Council Verdict: FAIL

| Axis | Judge verdict |
|---|---|
| Execution ordering / bootstrap | **FAIL** (3 blockers) |
| Wave-validity / write-scope collisions | **FAIL** |
| Acceptance-test feasibility | **FAIL** (2 blockers) |
| Missing requirements / failure modes | **WARN** (1 blocker) |

The plan's *engineering* is strong (hermetic harness, 1:1 bead↔test parity,
cycle-free DAG, count-migration genuinely removes the hand-step, cross-host gap
named-not-hidden). It fails on **bootstrap and ordering**, and the deepest
blocker reduces to the one action this whole effort has been gated on: the
coordinated landing of `chore/ag-seams-wave013`.

## BLOCKERs (deduped across judges) + exact fix

| # | Finding | Fix | Status |
|---|---|---|---|
| B-1 | Acceptance harness absent from main — all 37 beads' `bats …/acceptance-tests` run over a non-existent path → 37 false-reds, audit/sweep classifiers crash | New **bootstrap-root bead** `ag-bootstrap-acceptance-tree`: land both test trees + `helpers*.bash` + `run-acceptance.sh` to main as one arc; wire all 3 current roots (`ag-run2-coverage-manifest-qc41n`, `ag-d3-fixture-guard-yk7rq`, `ag-m1-m2-skeleton-1thaa`) to depend on it. **THIS IS THE COORDINATED LANDING OF `chore/ag-seams-wave013`.** | OPEN — gated on the landing |
| B-2 | `run-acceptance.sh` + `11-meta-suite.bats` bare-grep `bats:focus` matches the meta-suite's own legit references → FATAL exit 2 before any test; B73 (`ag-meta-suite-closure`) can never go green, all of run-1 converges on it | Anchor focus detection to the real directive `^\s*#\s*bats:focus` in both files | **DONE + VERIFIED** (suite now reaches test execution, 0 FATAL-focus) |
| B-3 | `helpers2.bash` resolves `REAL_REPO_ROOT` to the **worktree** (path-walk) while spec/br/rollout mean the **main checkout**; B90 sweep can't find `_beads` (absent in worktree) | `REAL_REPO_ROOT`/`BR_BEADS_DIR` default to `/Users/bo/dev/agentops` (or require env injection); re-verify B90/B78/B79/B81/B86 read the intended tree | OPEN — helpers fix |
| B-4 | No break-glass: doctrine flip (B86) makes land.sh mandatory + pre-push guard (B17/B62/B63) rejects manual push → first land.sh bug halts ALL landing with no sanctioned bypass | New bead `ag-land-break-glass` (B95): audited `LAND_GUARD_BYPASS=<reason>` / `--emergency-push`, documented in `--help` + CLAUDE.md Land section; dep on guard-install | OPEN |
| B-5 | `doctrine-flip` (B86) depends only on `install-chain`, not on rollout-verify → flips "use land.sh" while it's exit-97 and uninstalled on both clones | `br dep add ag-run2-doctrine-flip-xijbk ag-run2-rollout-evidence-uez2d` | OPEN — 1 edge |
| B-6 | 8-bead closeout fan-out is a phantom wave — all co-write the single `land.sh` monolith + shared `.bats`; run-1↔run-2 share `helpers.bash`/`08-crash-recovery.bats` with no edge | Linearize closeout fan-out (sequential dep chain); add cross-run edges (`fixture-gatekeep`,`land-bin-seam` → `d3-fixture-guard`; `b57-repair` → `recovery-abort`); mark epic `mode:sequential` — do NOT crank parallel | OPEN — edges + mode |

## MAJORs

- **Chicken-and-egg (ordering):** cutover beads (count-markers/B80, gate-parity/B81, doctrine-flip/B86) push to hot main directly, editing the exact regen surfaces land.sh exists to protect, before land.sh works. Gate: `br dep add ag-run2-install-chain-ghu7y ag-meta-suite-closure-eg2yn` (no cutover until engine green).
- **B80 non-atomic (ordering):** converts 11 count-docs in one bead; partial death → mixed state, `regen-all --check` red-mains main for every lane, no per-bead rollback. **Split** into `convert` (atomic, all-11) + `wire` (`--check` enforcement only after 100% convert).
- **validate.yml collision (wave):** prose `am`-reservation is invisible to the crank scheduler; +12 live `aug/*` lanes own the file. Sequence `gate-parity` dead-last (`→ ag-meta-suite-closure`) or carve the gate-families key into a separate workflow include so B81 never edits the contended body.
- **Throughput ceiling (completeness):** the multi-minute gate runs INSIDE the lock → 10-lane swarm serializes to a 30–50 min tail (the original bottleneck re-expressed), unmeasured. Add a bounded-hold-time bead, or move the gate OUTSIDE the lock (gate on resolved base, only rebase+push held).
- **`_beads` ledger 2nd race class (completeness):** land.sh ignores the ledger (separate repo, `git -C _beads push`); concurrent ledger pushes race unserialized. Name it as an accepted residual (the cross-host gap set the precedent) or file the `br sync` rebase-retry mitigation.
- **B94 forgeable (acceptance):** cross-host rollout evidence is a checked-in JSON + blob-equality check — a Mac agent can hand-write a `{"host":"bushido"}` record without touching bushido. Add a liveness proof (ssh bushido reads back the installed hook hash) or relabel as operator-attestation, not machine-verified.
- **Timing flake under determinism gate (acceptance):** the suite is timing-saturated AND B73 diffs two runs — any timing flake becomes a hard suite failure even on a correct land.sh. Quarantine timing tests from the determinism diff, or widen tolerances.

## The pattern under the pattern

B-1's fix IS the coordinated landing of `chore/ag-seams-wave013` that has gated
this work since dinner. The pre-mortem independently rediscovered that the plan
to fix landing cannot land until the landing it fixes happens once, by hand, the
old way. Everything else is mechanical (dep edges, 2 new beads, 1 split, 2 file
fixes). The plan is **conditional-PASS, gated on that one human-coordinated
landing** — and faking a green before it is the exact self-grading failure this
whole effort exists to kill.

## Verdict path
FAIL → apply B-2 (done) + B-3/B-5/B-6/B80-split + the 2 new beads (mechanical) →
**WARN, conditional-PASS** → the coordinated landing clears B-1 → re-run the
suite from a fresh main worktree (re-verify parity per MINOR) → PASS.
