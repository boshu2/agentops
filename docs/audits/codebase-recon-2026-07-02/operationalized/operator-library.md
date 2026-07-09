# Operator Library — executable operators distilled from the recon

> Each operator card: **TRIGGER** (when to fire) · **ACTION** (what to do) · **AVOIDS** (the failure mode) · **EVIDENCE** (quote-bank anchors) · **PROMPT MODULE** (paste-ready) · **MECHANIZABLE-AS** (the concrete gate check / code change that turns the rule from advice into enforcement — this column is what Stage 2/3 consume).
> Format is marker-bounded for deterministic parsing by `validate-operators.py`.

<!-- OPERATORS:START -->

## OP-1 — Fail-closed-on-unknown guard
- **TRIGGER:** reviewing or writing any gate/validation path that can return `Unknown`/`Skip`/`nil-verdict` while a `Blocking` flag is set.
- **ACTION:** ensure "could not determine" on a blocking check routes to the failing exit path, not to pass. Treat missing script / launch error / unreadable input as FAIL for blocking checks.
- **AVOIDS:** a silent fail-open in the release authority (a blocking check that can't run, passing). [[Q4]] is the live instance.
- **EVIDENCE:** [[Q3]], [[Q4]], [[Q15]].
- **PROMPT MODULE:** "For every gate verdict path, enumerate the non-PASS statuses. For each, ask: if this check is Blocking, does this status contribute to a non-zero exit? If any Blocking+non-determinable status yields exit 0, it is a fail-open — make it fail-closed."
- **MECHANIZABLE-AS:** (a) fix `report.ExitCode()`/`isBlockingFail` so `Unknown` on a `Blocking` check → exit 1; (b) new native gate check `gate.no-fail-open-on-unknown` that greps for `GateStatus{Unknown,Skip}` returns reachable by a blocking check without escalation. **→ audit A1 / Pattern 4.**

## OP-2 — Verify-against-git, not docs
- **TRIGGER:** any claim that something "changed / was resolved / regressed / is legacy / is live."
- **ACTION:** confirm with `git show <base>:<file>`, `git log -S <symbol>`, `git merge-base --is-ancestor <commit> <base>` before writing the claim.
- **AVOIDS:** repeating a stale doc claim (06-24 called `ao rpi` "compiled legacy" 5 days after it was removed). [[Q11]]
- **EVIDENCE:** [[Q11]], [[Q12]].
- **PROMPT MODULE:** "Before asserting X changed, name the commit. Run `git log -S` for the symbol and `git merge-base --is-ancestor` against the compared base. If you can't cite a commit, downgrade the claim to 'unverified'."
- **MECHANIZABLE-AS:** a recon-checklist item; optionally a `docs.recon-claims-cite-commits` lint on audit artifacts.

## OP-3 — Regression-vs-lens classifier
- **TRIGGER:** a finding appears "new" relative to a prior run.
- **ACTION:** check whether the *code* is new (blame/`-S` the relevant line to the prior base). If unchanged, label "latent — surfaced by rotated lens," not "regression."
- **AVOIDS:** false-alarm regressions and mis-attributed blame. [[Q12]]
- **EVIDENCE:** [[Q12]], [[Q4]].
- **PROMPT MODULE:** "Diff findings by verifying the underlying code against the prior base commit. Partition into {regression, latent-missed, resolved, lens-difference, environmental}. Only 'regression' means the code got worse."

## OP-4 — False-green test detector
- **TRIGGER:** citing a green test suite as proof a behavior is enforced.
- **ACTION:** trace the assertion to shipping code. If the test re-implements the logic locally (a `simulate*` helper) or asserts against a hand-built fixture the production writer can't emit, it is false-green.
- **AVOIDS:** concluding "STRONG / enforced" from tests that exercise no shipping code (06-24's `safety` verdict). [[Q13]]
- **EVIDENCE:** [[Q13]], `.claude/rules/go.md` fixture-fidelity.
- **PROMPT MODULE:** "For each test backing a security/enforcement claim, name the shipping function it drives. If it drives a `simulate*`/re-implementation or a synthetic fixture, flag false-green and re-point it at the real path."
- **MECHANIZABLE-AS:** rewrite `safety/doc.go` to hookless reality + delete/repoint the mirror tests. **→ audit A6.**

## OP-5 — Gate-coverage self-audit
- **TRIGGER:** relying on a drift/lint gate (regex or allowlist) to catch a class.
- **ACTION:** enumerate the class's members and confirm the gate's match set covers all of them; add missing members.
- **AVOIDS:** a blind-spot gate that stays green while the drift it names grows (retired-tech regex omitting `ao rpi|orchestrate|evolve`). [[Q14]]
- **EVIDENCE:** [[Q14]].
- **PROMPT MODULE:** "List every term the gate is supposed to catch. Grep the gate's own pattern for each. Any missing term is a silent hole — add it, then sweep for newly-caught violations."
- **MECHANIZABLE-AS:** add `\bao (rpi|orchestrate|evolve|flywheel|corpus|loop|tick)\b` to `check-docs-no-retired-tech.sh:43`, then sweep ~60 docs. **→ audit A5/A4.**

## OP-6 — Exit-code contract completeness
- **TRIGGER:** adding/reviewing a command that returns a typed exit code, or reviewing `ao capabilities`.
- **ACTION:** ensure every emitted code (3–10, HARDEN/REDO/BLOCKED/…) is enumerated in the published machine contract.
- **AVOIDS:** an agent that can't interpret codes 3–10 because the contract only lists {0,1,2}. [[Q1]]
- **EVIDENCE:** [[Q1]].
- **MECHANIZABLE-AS:** feed per-command exit codes into `ao capabilities` from the typed-error defs. **→ audit A7.**

## OP-7 — Finish-adoption over re-extraction
- **TRIGGER:** proposing to "extract a helper" for a duplicated primitive (atomic write, hash chain, jsonl append).
- **ACTION:** first check whether the canonical helper already exists; if so, the work is migration + closing the last gap (e.g. parent-dir fsync), not a new extraction.
- **AVOIDS:** the overclaim this very run made about Pattern 7 (`storage.AtomicWriteFile` already existed). [[Q8]]
- **EVIDENCE:** [[Q8]].
- **MECHANIZABLE-AS:** migrate private writers onto `storage.AtomicWriteFile`; add parent-dir fsync; fix `pool.atomicMove` no-fsync. **→ Pattern 7 / 06-24 P1.**

## OP-8 — Extract genuinely-duplicated tamper-evidence
- **TRIGGER:** a security-critical primitive (hash chain) hand-rolled in ≥3 packages with no shared helper.
- **ACTION:** extract one tested core; carry the dual gosec/semgrep suppressions for the SHA-1 git-object variant.
- **AVOIDS:** 5× divergent hash-chain implementations drifting apart. [[Q2]]
- **EVIDENCE:** [[Q2]], `.claude/rules/go.md` gosec/semgrep dual-annotation.
- **MECHANIZABLE-AS:** new `cli/internal/hashchain` package (`Seal`, `VerifyChain`, `AppendIdempotent`). **→ Pattern 1.**

## OP-9 — Fail-loud on partial fan-out
- **TRIGGER:** orchestrating a multi-agent run whose result set feeds a synthesizer.
- **ACTION:** assert k == N agents landed; on k < N, surface "RUN INCOMPLETE — k/N" instead of synthesizing a thin set as if complete.
- **AVOIDS:** a silently-empty recon (the prior 06-24 zero-report run). [[Q17]]
- **EVIDENCE:** [[Q17]], MEMORY note `workflow-tool-agent-failure-handling`.
- **PROMPT MODULE:** "After a parallel fan-out, count answered vs intended. Log dropped agents by id. If any blocking result is missing, fail-closed — do not let optimism fill the gap."

<!-- OPERATORS:END -->

## Mechanizable backlog (feeds Stage 2 idea-wizard → Stage 3 rpi)

| ID | Operator | Concrete change | Maps to | Leverage |
|----|----------|-----------------|---------|----------|
| M1 | OP-1 | `report.ExitCode()`: `Unknown` on Blocking → fail; + `gate.no-fail-open-on-unknown` check | audit A1 | **P0 — fixes core membrane hole** |
| M2 | OP-5 | Retired-tech gate regex + doc sweep | audit A5/A4 | **P1 — self-catching, one-line + sweep** |
| M3 | OP-4 | Rewrite `safety/doc.go`; repoint false-green tests | audit A6 | P1 |
| M4 | OP-6 | Per-command exit codes in `ao capabilities` | audit A7 | P2 |
| M5 | OP-7 | Finish atomic-write adoption + parent-dir fsync + `pool.atomicMove` fsync | Pattern 7 / P1 | P2 (crash-safety) |
| M6 | OP-8 | Extract `cli/internal/hashchain` | Pattern 1 | P2 (dedup security-critical) |
| M7 | OP-9 | Fail-loud partial-fanout guard for recon workflow | Q17 | P3 (process) |
| M8 | OP-2/OP-3/OP-4/OP-5 | Fold the recon operators into the `codebase-audit` / `review` skill so the next run applies them by default | K-B | P3 (compounds method) |
