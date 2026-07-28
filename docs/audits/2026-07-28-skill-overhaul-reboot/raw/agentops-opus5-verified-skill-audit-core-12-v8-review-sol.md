# Sol Independent Validation — Core-12 Skill Audit v8

**Date:** 2026-07-28  
**Decision:** **PASS**  
**Target:** `/tmp/agentops-opus5-verified-skill-audit-core-12-v8.md`  
**Mode:** fresh independent Sol validation  
**Nature:** binding acceptance review of the audit artifact, not an AgentOps `verdict.v3`

## Opening identity

```text
audit path       : /tmp/agentops-opus5-verified-skill-audit-core-12-v8.md
audit SHA-256    : 3f9d8051d691fb40885ca45586d6e6bfb03b4ae147545767185b6c14b423b392
audit lines      : 811
audit bytes      : 105572
prior review     : /tmp/agentops-opus5-verified-skill-audit-core-12-v7-review-sol.md
prior SHA-256    : 17ea51b4580e514f7276ad89602e47927e70c0d4ab478286231413c5c4565ba5
prior lines      : 387
prior bytes      : 22954
subject worktree : /Users/bo/dev/agentops-worktrees/skill-overhaul
subject branch   : codex/skill-overhaul-20260724
subject HEAD     : 0088c6e3824da201eabb1e751ac8e976599e0b5c
subject tree     : c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
subject status   : clean (0 paths)
validator context: sol_review_core12_audit_v8
```

The exact audit bytes, predecessor-review bytes, Git commit, Git tree, branch,
and clean worktree were verified before substantive review. The v8 audit and
the v7 Sol review were read in full. All fourteen v1–v7 audit/review artifacts
were found and their SHA-256 identities and line counts independently
reproduced.

## Executive outcome

v8 satisfies the v7 Sol review completely. It corrects the current-version
ledger, chain, predecessor scope, severity history, and provenance/seal without
changing the preserved technical audit:

1. The inherited ledger is correctly **23 corrections across six authoring
   rounds** (v2–v7). The v8 round adds CC1–CC5, making **28 corrections across
   seven rounds** in the complete artifact.
2. The v7 chain is correctly **seven audits and six prior Sol reviews**. With
   v8 and the v7 review included, the complete chain is **eight audits and seven
   prior Sol reviews**.
3. The formerly bare ten-artifact phrase is now scoped to the **ten pre-v6
   lineage artifacts**. The complete prior set is correctly **fourteen
   artifacts**: seven audits and seven Sol reviews, v1–v7.
4. The severity history now includes v7 and v8, both at
   **3 P0 / 11 P1 / 14 P2 = 28**.
5. The source-hash caption, checked/not-checked boundaries, and seal reach the
   correct review horizon and preserve the immediate v7 identities.

The v7 technical program independently reproduces: 49 canonical skills; 48
tracked core-owned files and 7,648 lines; zero mismatch across all 48 declared
SHA-256 rows and all twelve Git trees; 9 contract validators, 10 effective
Python processes, 6 suppressed, 4 unsuppressed, exactly one delegated; the
proof, writer, fallback, ADR-0016 layout, scenario, and per-skill findings; and
the ranked 3/11/14 program.

No finding warrants `REQUEST_CHANGES` or `NOT_PROVEN`.

## v7 finding closure

| Required correction | Result | Independent evidence |
|---|---|---|
| CC1 — ledger cardinality | **PASS** | X1–X5 = 5, Y1–Y8 = 8, Z1–Z3 = 3, AA1–AA3 = 3, BB1–BB4 = 4: **23 across six rounds** through v7. CC1–CC5 make **28 across seven** through v8. The old 19/5 statement appears only under explicit “Before v7” history. |
| CC2 — audit-chain cardinality | **PASS** | `v7 → … → v1` contains **7 audits / 6 prior reviews**; `v8 → … → v1` contains **8 audits / 7 prior reviews**. Both scopes are explicit in §13. |
| CC3 — predecessor scope | **PASS** | v1–v5 plus their reviews are correctly called **ten pre-v6 lineage artifacts**. Adding v6/v7 and their reviews yields the stated **fourteen prior artifacts**. |
| CC4 — latest severity history | **PASS** | §0 has v2–v8 columns; v7 and v8 are both 3/11/14 = 28. Direct row recount gives 3 P0 rows, 11 P1 rows, and 14 numbered P2 items. |
| CC5 — provenance and seal | **PASS** | The whole-file caption reaches v1→v7, the v6 review is present in the six-review table, the v7 review is handled explicitly afterward, §10 identifies all fourteen prior artifacts, and the seal binds v8 to the exact v7 audit/review pair. |

## Lineage identities

| Version | Audit SHA-256 / lines | Sol-review SHA-256 / lines |
|---|---|---|
| v1 | `ddd048bd91b8f36125003a331350059e51a83bc4926c003e497cd4fa8f329835` / 666 | `0480f406bc9def0384995842337db16fb2d047dbc5b1b593e03fe32a7fb15f85` / 486 |
| v2 | `b7a1fe4441aa4ba4664dd27fde2421bfc44898011cb14c1825437c3737116c68` / 571 | `0dbabf878e487b088db24de99a6f2c426ac4e2f1e07caa8568ac00d48e270143` / 313 |
| v3 | `4278e7d9fcf547f650f9d4b24514078f134b603dd249666cdc3be805c382c9b8` / 604 | `21c14eeecd229f88f674fcbba69a9df5dab539b7126a0dd70e820a8ef95ef11e` / 232 |
| v4 | `433a2c3eca2038c8264366e7596fbc813aa9fea86c52657d7359cbb54a7f1686` / 668 | `3f0aa75ec8c6ce4f575844703c8a711165e4de1f67e6e0e24de56fa0fcafdc8e` / 373 |
| v5 | `78d7ceb54c7aaa63b01ce4f77f8c3850b31d6577a5a06f385c7d82909461001c` / 713 | `b9dbc0b6d39f6dc0bfeb15340ea87a5c84a61368334d2a7c1da9b4ca9ae4ab72` / 423 |
| v6 | `2d140dfeeac439fdbcf058af73fbee3cae8a5f9756655f268683de691b24bf7e` / 725 | `111151202943927a80c09f6907ab2c0a969ab649d699a168ca064c12948cf27d` / 377 |
| v7 | `939ff8e5f35fdf0b31cf25c8ec9fe887bf0214049d8e9ab99c8c38aaa2b7317a` / 757 | `17ea51b4580e514f7276ad89602e47927e70c0d4ab478286231413c5c4565ba5` / 387 |

The two upstream advisory reports also reproduce:
`7fce526856412aa13ba900936d4e6abaa39f785e90546f3555f70a8a55b66f9d`
at 412 lines and
`7b128561b36a89de2062995867370f30c29beb840d14570d2a94466ddfb186ac`
at 177 lines. Landing `d66f01d5` is an ancestor of the reviewed HEAD and
`git diff d66f01d5 HEAD` across the twelve skill directories contains zero
paths.

## Technical criterion matrix

| Criterion | Result | Independent evidence |
|---|---|---|
| Stable subject/repository identity | **PASS** | Exact v8/prior hashes, lines, bytes, HEAD, tree, branch, and clean status reproduce at open and close. |
| Canonical and owned inventory | **PASS** | `skills/*/SKILL.md` count is 49. Git-derived core set is 48 paths and 7,648 lines. |
| Whole-file identities | **PASS** | Parsed all 48 declared hash rows from v8 and compared them to live bytes: 0 mismatches, 0 missing, 0 extra. |
| Per-skill tree identities | **PASS** | All twelve declared file counts, line counts, and Git trees reproduce exactly. |
| Doctrine/proof identities | **PASS** | All six declared SHA-256 values for `AGENTS.md`, operating-loop doctrine, ADR-0016, active pointer, epoch-1 descriptor, and grandfather snapshot reproduce. |
| Validator denominator and guards | **PASS** | Nine tracked core `scripts/validate.sh`; Validate is the non-negative-guard kernel validator; the remaining eight split 4 inert / 4 sound, with six inert `! grep` assertions. |
| Effective Python census | **PASS** | 10 effective processes: 6 suppressed, 4 unsuppressed; Scope’s one line-46 `heal.sh` process is the only delegated process, and line 95 is fix-only. |
| Scenario matrix and links | **PASS** | Exact pass/fail matrix reproduces; 26 `@covered-by` references resolve to 22 unique targets, with duplicate multiplicities ×4 and ×2. |
| Proof binding and transition constraint | **PASS** | Epoch-1 proof checker returns PASS with 25 components. Eight rows/eight unique refs are core-owned. Eleven governed core Python files split 4 pinned / 7 unpinned; six unpinned production files are proof-bound. |
| Writer distinction | **PASS** | v3 writer re-verifies the final manifest twice, gives incomplete coverage precedence, checks typed-receipt membership, and accepts exactly six semantic draft keys. The callable v2 writer still snapshots a living `--intent-source`; Go is read/check only. |
| Projected fallback | **PASS** | Implement walks to a projected kernel without digest-checking selection if the direct kernel is absent. Current direct/projected bytes are both `f7787f4505c6f49c77890411a49387a02beec7a267595e158af6e4184ca6ef70`. |
| ADR-0016 layout/visibility | **PASS** | Closed top-level set is `ao/`, `scratch/`, `projections/`; the named doctor detector is absent. Postmortem and Idea Genie declare outside paths, and v3 runtime exclusions omit them. |
| Missing contracts/citations | **PASS** | Zero schema hits for Reality Check, Council, or Postmortem report names; recorder root path and `loop_context.go` absent; both Learn verdict prefixes unresolved. |
| Ranked program and taxonomy | **PASS** | Mechanical recount: 3 P0, 11 P1, 14 P2 = 28. A1–A30, C1–C10, R1–R3, S1–S3, and N1–N6 are complete with no gaps. |

## Effective Python-process census

| Validator | Processes | Suppressed | Unsuppressed | Delegation |
|---|---:|---:|---:|---|
| RPI | 1 | 1 | 0 | direct |
| Plan | 1 | 0 | 1 | direct |
| Implement | 1 | 1 | 0 | direct |
| Validate | 5 | 4 | 1 | direct |
| Learn | 0 | 0 | 0 | — |
| Scope | 1 | 0 | 1 | **delegated through `heal.sh:46`** |
| Research | 1 | 0 | 1 | direct |
| Premortem | 0 | 0 | 0 | — |
| Postmortem | 0 | 0 | 0 | — |
| **Total** | **10** | **6** | **4** | **one delegated** |

The second `heal.sh` Python command at line 95 is gated by `MODE == fix`;
Scope invokes `--check --strict`, so it is unreachable from the validated path.

## Replayed witnesses

| Probe | Independent result |
|---|---|
| Nine contract validators | all rc 0 |
| RPI suite | 13 PASS |
| Validate v3 suite | 27 PASS |
| Validate v2 suite | 16 PASS |
| Shared kernel corpus | 43 PASS |
| Legacy v2/schema corpus | 23 PASS with schema required |
| Go verdict checker | PASS with `-count=1` |
| Native-skills Bats | 8 PASS; first four are Idea Genie |
| Proof checker | PASS, epoch 1, 25 components |
| Python ratchet | PASS, 24 pins |
| Scenario matrix | RPI 4/4, Plan 3/3, Implement 4/4, Validate 7/7, Idea Genie 2/2, Idea Challenge 2/2; Learn 0/2, Research 0/3, Premortem 0/2, Postmortem 0/2 |
| Learn false pass | line 35 contains `receipt`; validator still returns PASS, rc 0; isolated `! grep` control reaches the PASS path |
| Plan wrong expected digest | rc 2 with exact-digest mismatch |
| Plan isolated bytecode | 64 `.pyc`, exactly one core-owned: Validate `kernel_v3.cpython-314.pyc` |
| Scope isolated bytecode | 42 `.pyc`, zero core-owned |
| Repository bytecode cache | 10 files in 5 directories; path/mtime/size/content snapshot digest remained `9dddeb36cdcde7ef95f90bb28d72c7cbe451d2676eed0ac69ddcfa244dd71616` before and after both isolated witnesses |
| Proof components | all 25 descriptor digests and modes match; eight core refs are unique and valid |
| Whole-file/tree inventory | 48/48 file hashes and 12/12 trees match |

The isolated witnesses wrote only under disposable `/tmp` cache prefixes,
which were removed after measurement. The repository stayed clean.

## Per-skill judgment

| Skill | Result | Intent/RPI/control-flow adjudication |
|---|---|---|
| RPI | **PASS** | Correct one-shot Plan → Implement → fresh Validate dispatcher. Live code enforces exact three-field intent identity, ref/digest/length binding, terminal NOT_PLANNED/NOT_BUILT, semantic status set, and six durable validation identities. Two inert guards remain valid P1 findings. |
| Plan | **PASS** | Correct sole pre-freeze intent shaper/minter. Live adapter delegates one exact mint to the v3 kernel; wrong `--expected-digest` returns rc 2. Proof-transition P0, CLI/guard P1, and scope-index ownership P2 stand. |
| Implement | **PASS** | Correct sole subject editor/factual receipt phase. It consumes the exact snapshot and builds a v2 manifest. The proof collision, unchecked projected fallback, inert guards, and adapter/scope-index seams are accurately ranked. |
| Validate | **PASS** | Correct sole fresh semantic judge and v3 writer. Complete-coverage precedence, exact draft shape, typed receipt evidence, proof loading, double final-manifest verification, callable v2 living-source writer, dual path grammar, and dangling line-119 recorder path all reproduce. |
| Learn | **PASS** | Correct optional post-loop consumer. Its mode-100755 contract validator invokes no Python but false-passes on the already-present forbidden token; two citations are dead and v2 vocabulary remains. |
| Scope | **PASS** | Correct response-only advisory boundary reviewer. Its lifecycle guard is sound; effective execution delegates one unsuppressed Python heredoc at `heal.sh:46`, while the fix-only line 95 is unreachable. |
| Reality Check | **PASS** | Correct pre-Plan evidence-gap strategy. `reality-check-report.v1` has no schema/validator surface; the P1/P2 seams stand. |
| Research | **PASS** | Correct bounded evidence supplier. Schema version is pinned to enum `[1]`; guard is sound; unsuppressed `json.tool` imports no core module. Effects and scenario P2 findings stand. |
| Premortem | **PASS** | Correct optional frozen-plan challenge. Live output validator enforces exact fields, digest shape, evidence, and distinct author/judge identities. P1 vocabulary and P2 coverage remain accurate. |
| Postmortem | **PASS** | Correct optional post-verdict causal analysis. The report-write/`effects: []` contradiction, outside-closed-set path, loop visibility, missing detector, feature-file output contract, vocabulary, and coverage findings reproduce. |
| Council | **PASS** | Correct methodology-weighted optional strategy subordinate to one accountable Validate context. Named contract remains absent and v2 vocabulary remains. |
| Idea Genie | **PASS** | Correct pre-Plan elicitation/duel strategy. Live validators and four related Bats cases enforce sealed route behavior, exact Plan handoff, identity separation, dissent/refutation, and no extra authority field. Layout/shared-reference findings stand. |

Across the twelve, no inspected skill grants retry, queue, budget, campaign,
Git, closure, release, or delivery authority. Validate remains the sole
semantic verdict writer.

## Findings

None. The v7 review’s sole P2 blocker is fully corrected, and no new
audit-integrity defect was found.

## Checked

- Exact v8 audit identity, line count, byte count, and full text.
- Exact v7 Sol-review identity, line count, byte count, and full text.
- Exact HEAD, tree, branch, opening/closing cleanliness.
- SHA-256 and line counts for all fourteen v1–v7 audit/review artifacts and the
  two upstream advisory reports.
- The complete v7→v8 diff; technical sections, 48 hash rows, and 12 tree rows
  are unchanged outside the CC correction/provenance projections.
- All twelve live `SKILL.md` contracts in full.
- RPI dispatcher, Plan minter, Implement freezer, every core contract
  validator, delegated `heal.sh` control path, Idea Genie validators,
  Premortem output validator, recorder, v3 CLI, and risk-bearing v2/v3 writer,
  proof, manifest, effect, and report ranges.
- All 48 live path identities, all twelve per-skill file/line/tree totals, all
  six doctrine/proof hashes, canonical-skill count, correction arithmetic,
  ranked arithmetic, and taxonomy sequences.
- All replayed witnesses listed above, including isolated cache witnesses and
  bytecode census.

## Not checked

- Full repository CI, all Go packages, regeneration gates, generated
  projections, or generated-image currency.
- Every line of all 48 owned files. All twelve contracts, every validator, core
  adapters, and the risk-bearing implementation ranges were read; complete
  owned-file identity was established by hash.
- Every payload in the 43-case shared corpus or 23-case legacy corpus.
- A destructive mutation of a bound component, a published proof transition,
  a divergent projected-kernel execution, or a conditional `.agents/` effect
  mutation. Their live binding/control-flow premises were checked.
- Historical causation of the ten existing repository `.pyc` files.
- Non-Darwin behavior, injected OS-level durability failures, or races beyond
  the focused suites.
- The audit author’s internal reading/execution history. Immutable bytes, exact
  lineage artifacts, live source, and independently reproduced results are the
  evidence.
- Semantic acceptance of a software change. No `verdict.v3`, proof transition,
  repository edit, generation, commit, merge, push, tag, or release was
  performed.

## Residual risk

1. Learn’s validator remains live-false-passing because `! grep` is exempt from
   `set -e`.
2. Scenario-name linkage remains substring-based.
3. Plan can create a core-owned kernel cache when run unsuppressed on a cold
   cache; repository-cache immutability is measured, not structural.
4. The projected fallback is currently byte-equal but is not digest-checked at
   selection time.
5. Six proof-bound production Python moves require a valid proof transition.
6. Postmortem and Idea Genie writes are loop-visible only when they occur
   between manifests, but their declared paths violate ADR-0016 regardless.
7. `effects` semantics remain an open contract question for Research and
   Postmortem.

These are preserved subject findings, not defects in v8’s correction.

## Closing identity and seal

Immediately before sealing:

```text
audit SHA-256    : 3f9d8051d691fb40885ca45586d6e6bfb03b4ae147545767185b6c14b423b392
audit lines      : 811
audit bytes      : 105572
prior SHA-256    : 17ea51b4580e514f7276ad89602e47927e70c0d4ab478286231413c5c4565ba5
prior lines      : 387
prior bytes      : 22954
subject HEAD     : 0088c6e3824da201eabb1e751ac8e976599e0b5c
subject tree     : c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
subject status   : clean (0 paths)
```

The opening and closing subject identities are byte-identical. The audit and
repository remained unchanged throughout validation.

This review is sealed at publication. Its SHA-256, line count, and byte count
are computed after sealing and reported out of band because embedding its own
digest would change the bytes being identified.
