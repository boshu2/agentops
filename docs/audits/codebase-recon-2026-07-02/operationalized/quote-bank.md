# Quote Bank — evidence anchors for the operationalized kernel

> Every rule in `triangulated-kernel.md` and every operator card in `operator-library.md` cites an anchor here (`[[Qn]]`). Anchors point to executable code or to a verified finding in the 2026-07-02 recon. Provenance is auditable: each carries a file:line or artifact reference. Evidence-first is non-negotiable.

## Design-invariant anchors (from the codebase)

- **Q1** — Exit code IS the verdict. `Execute()` unwraps 9 typed sentinel errors to exit codes. `cli/cmd/ao/root.go:83-167`. 9 `*ExitError` structs (`gate/doctor/pawl/planPawl/governor/beads/corpusScan/wikiHealth/tick`).
- **Q2** — Hash-chained append-only ledger recurs in 5 packages: `provenancegraph` (`edge.go:218` `payload_hash=sha256(canonical minus chain fields)`, `hash=sha256(payload_hash+"\n"+prev_hash)`), `turnstate`, `rpi`, `drwitness`, `drrebuild`.
- **Q3** — Fail-CLOSED on absence is the pervasive rule: missing verdict ledger = empty not pass (`verdictledger/loader.go:22`); malformed constraint index = FAIL (`gates/checks/constraints.go:36`); timed-out review = no verdict never CONFIRM (`docs/contracts/pawls.md:171`); zero-evidence = `unknown` not vacuous pass (`goalsfitness/satisfaction.go:148`).
- **Q4** — The ONE violation of Q3: `gates/scriptrunner.go:44-45` maps a missing/unlaunchable blocking backing script to `GateStatusUnknown`, which `report.go:28-34` `ExitCode()` excludes from `isBlockingFail` → blocking check silently passes. Empirically proven in a scratch repo (22 blocking checks all UNKNOWN, exit-code contribution 0). Latent since `3af00293a` (2026-06-07).
- **Q5** — No-self-grade centralized in one function: `liveness.Disjoint(author, validator)`, consumed by `evidencedturn:351`, `governor`, `aostate`, `ports`, `adapters`. Mirrored in `pawls.md` (`context_id != author`) with family normalization so a rename can't spoof "different model."
- **Q6** — Determinism inversion (model proposes, pure predicate decides): `planpawl/decide.go:125`, `orchestration/shape.go:57`, `gates/checks/constraints.go`, `evidencedturn/evidencedturn.go:88`. Fail-closed default: unrecognized input = the unsafe verdict.
- **Q7** — Drift-proof generated contract by reflecting over the live tree: `capabilities.go:104` and `robot_docs.go:87` walk `rootCmd.Groups()/Commands()`; `make regen-check` is the drift gate over `registry.json`/`COMMANDS.md`.
- **Q8** — Atomic write (tmp+rename) canonical helper already exists: `storage.AtomicWriteFile` (`storage/atomicfile.go:26`); ~13 sites delegate, ~many still private (`search`, `doctor×4`, `wiki`, `feedbackcompiler`, `llm`, `pool.atomicMove` which still omits fsync).
- **Q9** — Capability-probe not `command -v`: `orchestration/select.go:17` ladder NTM→Claude→beads-floor; `ntm_probe.go:67` runs `ntm --robot-capabilities`; every tier emits the same output-contract shape.
- **Q10** — `init()`-based decentralized self-registration; duplicate ID panics at startup: ~92 `rootCmd.AddCommand` sites; ~90 gate checks self-register in `gates/checks/seed.go`.

## Recon-methodology anchors (from how the analysis was run + the diff)

- **Q11** — Verify against git history, not documents: `ao rpi` removed `f61c5f0e7` (2026-06-19) IS an ancestor of the 06-24 base `abc018c42`, yet the 06-24 synthesis still called it "compiled legacy." `git merge-base --is-ancestor`.
- **Q12** — Same-code / different-lens ≠ regression: Q4's fail-open was byte-identical at the 06-24 base; the 06-24 security-grep lens missed it. A finding new to the *record* can be old in the *code*.
- **Q13** — Green tests can be false-green: 06-24 ran the `safety` package tests, saw green, concluded "Security STRONG." Those tests re-implement deleted-hook logic in Go (`simulateRunRestricted`) and assert nothing about shipping enforcement (hooks removed `e431339c4`, 2026-05-24). Cross-ref `.claude/rules/go.md` "fixture fidelity."
- **Q14** — A drift gate with a blind spot hides drift: `check-docs-no-retired-tech.sh:43` regex omits `ao rpi|orchestrate|evolve|flywheel`, so ~60 docs teaching removed commands pass green.
- **Q15** — The prior run named the class my finding instances: 06-24 Pattern P5 = "every guard declares fail-open vs fail-closed; fail-open must never be silent." Q4 is the live uncaught instance.
- **Q16** — Recon self-drift: docs authored by the recon runs themselves drift (`codebase-overview.md:330-332` stale counts; disposition-triage doc off by 3/3/2). A recon must audit its own prior output.
- **Q17** — The recon workflow can silently fail: 06-24 synthesis records a prior 06-24 attempt that "silently produced zero reports" → action "surface RUN FAILED — k/N reports."

## Disputed / unproven (must NOT enter the consensus kernel)

- **Q18** — Flywheel/corpus moat is data-starved and UNPROVEN (ADR-0004, ADR-0011): a competent membrane makes escapes structurally rare (0 escapes / 130 production verdicts), so self-improvement is anti-correlated with membrane quality. The repo refuses to market it. This is DISPUTED, not kernel.
