# Triangulated Kernel — AgentOps design invariants + recon method

> Consensus rules only. Each is corroborated by ≥2 independent code sites or by both a code site and a verified finding. Disagreements/unproven claims are quarantined in the DISPUTED section and MUST NOT be treated as law. Sections are marker-bounded for deterministic parsing.

<!-- KERNEL:START -->

## K-A — AgentOps design invariants (the laws the codebase keeps re-discovering)

- **K-A1 — No verdict = not done; absence is fail-closed.** Any path that cannot produce a proof must route to the *unsafe* side (FAIL/HOLD/REDO/unknown), never to pass. Evidence: [[Q3]]. Live violation to fix: [[Q4]].
- **K-A2 — The process exit code is the machine-readable verdict.** Gate-style commands return a typed error carrying an int; shells/CI/agents branch on the number, never on parsed stdout. Evidence: [[Q1]]. Corollary: any published contract (`ao capabilities`) MUST enumerate every code the CLI emits.
- **K-A3 — Tamper-evidence is a hash-chained append-only ledger.** `payload_hash` over canonical JSON minus chain fields; `hash = sha256(payload_hash + prev_hash)`; idempotent, flock-serialized append; committed JSONL wins over any DB projection. Evidence: [[Q2]].
- **K-A4 — Separation of duties: author ≠ validator, enforced in one function.** No self-grade; "different model" is normalized so a rename can't spoof it. One definition, many callers. Evidence: [[Q5]].
- **K-A5 — The model proposes; a pure, fail-closed predicate decides.** Every binding gate is a windshield over observable ground truth, not a chatbot; unrecognized input counts as the unsafe verdict. Evidence: [[Q6]]. "Trust the environment, not the agent."
- **K-A6 — Generated contracts are derived by reflecting over the live artifact, and drift-gated.** Never hand-write what you can derive from the executable's own structure; a `--check` mode is the CI gate. Evidence: [[Q7]].
- **K-A7 — Every persisted artifact is written atomically (tmp+fsync+rename), through the shared helper.** A crash never leaves a half-written ledger. Helper exists (`storage.AtomicWriteFile`); finish adoption + parent-dir fsync. Evidence: [[Q8]].
- **K-A8 — Detect dependencies by capability, and degrade with output-contract parity.** Probe "can it do the job," not presence-on-PATH; every fallback tier emits the same schema so degradation is correctness-preserving. Evidence: [[Q9]].
- **K-A9 — Registries self-register via `init()`; duplicate IDs panic at startup.** No central switch; adding a unit is one file; collisions are build-time programmer errors. Evidence: [[Q10]].

## K-B — Recon methodology (how to run a rigorous codebase audit)

- **K-B1 — Verify against git history, not documents.** Either document (the code's docs or the prior audit) can be wrong. Ground every "changed/resolved/regressed" claim in `git show`/`git log -S`/`git merge-base`. Evidence: [[Q11]].
- **K-B2 — Distinguish regression from lens-difference.** Before calling a finding "new," check whether the code is new. A finding new to the record is often old in the code, surfaced by a rotated lens. Evidence: [[Q12]].
- **K-B3 — Rotate the lens across runs.** A single competent lens (e.g. security-grep) can rate a codebase "STRONG" while a fail-open sits in its release gate. Re-run with a different question (drift/robustness/liveness) to surface what the prior pass waved through. Evidence: [[Q12]], [[Q15]].
- **K-B4 — Green tests are not enforcement proof.** A test that re-implements the logic it "covers" (instead of driving shipping code) is false-green. Trace green to the shipping code path before trusting it. Evidence: [[Q13]], `.claude/rules/go.md` fixture-fidelity.
- **K-B5 — A drift gate is only as good as its match set; audit the gate's own coverage.** A regex/allowlist with a blind spot hides exactly the drift it purports to catch. Evidence: [[Q14]].
- **K-B6 — Audit the recon's own output.** Docs produced by prior recon runs drift; re-verify their counts against live state each run. Evidence: [[Q16]].
- **K-B7 — Fail loud on partial runs.** A multi-agent recon that loses agents must report "k/N landed," never hand a thin/empty result set forward as if complete. Evidence: [[Q17]].
- **K-B8 — Report refuted hypotheses and verified-sound guards, not just findings.** Honesty about what held up (and what you didn't cover) is part of the deliverable; it prevents the next run from re-litigating settled ground and marks true scope.

<!-- KERNEL:END -->

## DISPUTED / UNPROVEN (quarantined — not law)

- **D-1 — The knowledge-corpus/flywheel is a moat.** Demoted to unproven under a structural data-starvation headwind; anti-correlated with membrane quality. Do not operationalize as a rule; treat as a measured hypothesis only. Evidence: [[Q18]].
