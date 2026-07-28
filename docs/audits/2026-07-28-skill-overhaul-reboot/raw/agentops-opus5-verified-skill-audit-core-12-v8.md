# Verified Audit — the Twelve Core RPI Skills (v8, corrected)

**Subject:** `/Users/bo/dev/agentops-worktrees/skill-overhaul`
**Branch:** `codex/skill-overhaul-20260724`
**HEAD:** `0088c6e3824da201eabb1e751ac8e976599e0b5c`
**Tree:** `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`
**Status:** `git status --porcelain` → 0 lines at open and at close.
**Date:** 2026-07-28
**Mode:** read-only correction pass. No tracked file created, edited, or deleted. No generation, commit, merge, push, or tag. **No AgentOps semantic validation was performed and no verdict was minted; no PASS is claimed.**
**Skills:** rpi · plan · implement · validate · learn · scope · reality-check · research · premortem · postmortem · council · idea-genie — twelve of the **49-skill canonical corpus**.

## Input identities — **four** rows, all verified byte-exact at open

**Two immediate inputs** (the pair this revision corrects) and **two lineage artifacts** (v7's immediate inputs, retained here). The table is four rows, not two; "both" is withdrawn (BB1, retained).

| Input | SHA-256 | Lines |
|---|---|---|
| **IMMEDIATE 1/2** — `…-core-12-v7.md` (v7 — **this document's immediate input**) | `939ff8e5f35fdf0b31cf25c8ec9fe887bf0214049d8e9ab99c8c38aaa2b7317a` | 757 |
| **IMMEDIATE 2/2** — `…-core-12-v7-review-sol.md` (fresh Sol, **REQUEST_CHANGES**) | `17ea51b4580e514f7276ad89602e47927e70c0d4ab478286231413c5c4565ba5` | 387 |
| **LINEAGE 1/2** — `…-core-12-v6.md` (v6 — **v7's immediate input**) | `2d140dfeeac439fdbcf058af73fbee3cae8a5f9756655f268683de691b24bf7e` | 725 |
| **LINEAGE 2/2** — `…-core-12-v6-review-sol.md` (v6 Sol review — **the pair v7 was authored to correct**) | `111151202943927a80c09f6907ab2c0a969ab649d699a168ca064c12948cf27d` | 377 |

**All four read in full.** The two immediate inputs are the **v7 audit** and the **v7 Sol review**; the two lineage artifacts are the **v6 audit** and the **v6 Sol review**, which were v7's own immediate inputs. Each Sol review is an **advisory review, not an AgentOps semantic verdict**. Every correction was independently rechecked against live source before adoption.

**The v7 Sol review passed all four v6 blockers and the entire technical program.** It records `REQUEST_CHANGES` solely for current-version lineage and cardinality arithmetic in v7's correction ledger and seal, and states explicitly that those defects "do not change the 3 P0 / 11 P1 / 14 P2 technical program." v8 therefore corrects lineage arithmetic only and carries every technical finding forward unchanged.

**Preserved lineage (corrected at v6 — AA3; extended at CC3/CC5).** v5 wrongly presented v3 and the v3 review as its inputs. The exact chain is: **v8 supersedes v7** (`939ff8e5…`, 757) under the v7 Sol review (`17ea51b4…`, 387); **v7 superseded v6** (`2d140dfe…`, 725) under the v6 Sol review (`11115120…`, 377); **v6 superseded v5** (`78d7ceb5…`, 713) under the v5 Sol review (`b9dbc0b6…`, 423); **v5 was authored as the correction of v4** (`433a2c3e…`, 668) **and the v4 Sol review** (`3f0aa75e…`, 373) — that exact pair, not v3; v4 superseded v3 (`4278e7d9…`, 604) under the v3 review (`21c14eee…`, 232); v3 superseded v2 (`b7a1fe44…`, 571) under the v2 review (`0dbabf87…`, 313); v2 superseded v1 (`ddd048bd…`, 666) under the v1 review (`0480f406…`, 486). **Predecessor scope, stated exactly (CC3).** The **ten pre-v6 lineage artifacts** — five audits (v1–v5) and five Sol reviews — are preserved unchanged, and so are the v6 audit and review and the v7 audit and review. Counting the complete set, **fourteen prior audit/review artifacts** (seven audits, v1–v7, and seven Sol reviews, v1–v7) are preserved unchanged. The bare phrase "all ten predecessor artifacts" is withdrawn: it named only the pre-v6 subset, which stopped being the whole predecessor set at v6. v1's own upstream provenance stands: two prior advisory reports (`7fce5268…`, 412 lines; `7b128561…`, 177 lines), Fable-class despite their headings, treated as advisory evidence only, with their landing `d66f01d5…` confirmed an ancestor of this HEAD and `git diff d66f01d5 HEAD -- skills/{the twelve}` empty.

---

## 0. Correction ledger

**Ledger cardinality, stated with explicit scope (CC1).**

| Scope | Rounds | Corrections |
|---|---|---|
| **Inherited ledger through v7** | **six** authoring rounds — v2, v3, v4, v5, v6, v7 | **23** — X1–X5 (5) + Y1–Y8 (8) + Z1–Z3 (3) + AA1–AA3 (3) + BB1–BB4 (4) |
| **This v8 round** | one — v8 | **5** — CC1–CC5 |
| **Complete ledger including v8** | **seven** authoring rounds — v2 through v8 | **28** |

**The historical figure, retained only with its scope attached (CC1).** *Before v7*, the ledger was **nineteen corrections across five rounds**. That statement was correct for the v2–v6 span and is false for any later artifact; v7 carried it forward unscoped while its own paragraph enumerated BB1–BB4 and named six passes, which is the contradiction the v7 Sol review recorded. It appears here only as labelled history.

**X1–X5** (the v2 pass) and **Y1–Y5** (the v3 pass) are carried forward intact. **Y6–Y8** are the v4 pass. **Z1–Z3** are the v5 pass. **AA1–AA3** are the v6 pass. **BB1–BB4** are the v7 pass. **CC1–CC5** are this pass (v8). **Labelling convention, tightened at BB2 and retained:** every round is named by the version that authored it — the v2, v3, v4, v5, v6, v7, and v8 passes — and the bare phrase "this pass" is reserved **solely** for v8's own round. Each successive artifact re-points that phrase at itself and gives the prior owner an explicit version label, which is why BB1–BB4 are labelled **the v7 pass** throughout this document.

### Rounds 1–2 (carried forward, all re-confirmed by the v3 Sol review)

| # | Correction | Status |
|---|---|---|
| **X1** | P0 v3-vocabulary owner set = **root `AGENTS.md` + `docs/architecture/operating-loop.md`**; both unbound by the epoch-1 descriptor | **STANDS** (CONFIRMED ×2) |
| **X2** | Owned inventory **48 files / 7,648 lines** | **STANDS** (CONFIRMED ×2) |
| **X3** | **26 `@covered-by` references → 22 unique** targets; duplicates ×4 and ×2 | **STANDS** (CONFIRMED ×2) |
| **X4** | RPI proof-membership prose re-ranked **P1 → P2**; 0 of 12 disclose membership, no rule requires it | **STANDS** (CONFIRMED ×2) |
| **X5** | Three prior omitted P2s rolled in (reality-check, council, postmortem) | **STANDS** (CONFIRMED AS CORRECTED BY Y1) |
| **Y1** | Postmortem effect-honesty contradiction restored to §4.10, taxonomy N6, P2 item 14 | **STANDS** (CONFIRMED) |
| **Y2** | Ranked totals **3 P0 / 11 P1 / 14 P2 = 28** | **STANDS** (CONFIRMED — §7.1 = 3 rows, §7.2 = 11, §7.3 = 1–14) |
| **Y3** | Nine tracked `validate.sh`; **eight** lifecycle-negative-guard validators after excluding Validate's kernel contract validator; four inert / four sound | **STANDS** (CONFIRMED) |
| **Y4** | `.pyc` provenance | **PARTIALLY CORRECTED THIS ROUND — see Y6.** The mtime conclusion stands; the universal suppression claim does not |
| **Y5** | Seal narrowed to "No AgentOps semantic validation/verdict was performed" | **STANDS** (CONFIRMED) |

### Round 3 (the v4 pass)

| # | Correction | Source | Independent recheck at HEAD | Disposition |
|---|---|---|---|---|
| **Y6** | *(further corrected this round — see Z1: the replacement partition was itself wrong.)* **Withdraw the universal claim that every executed validator suppresses bytecode.** Replace with per-validator facts, the isolated witness, and the narrower mtime conclusion | Sol [P2] | Verified line by line across all nine contract validators (§9.1). `skills/plan/scripts/validate.sh:9` runs `python3 "$skill_dir/scripts/mint_intent.py" --help >/dev/null` with **no** `-B` and **no** `PYTHONDONTWRITEBYTECODE`; `mint_intent.py:13-16` `importlib`-loads the owned `skills/validate/scripts/kernel_v3.py`, so that import is what Python caches. **Witness re-executed from scratch at the v4 pass** (U8) | **ADOPTED** (§1, §9.1, §10, §11, §12) |
| **Y7** | **Recorder-path anchor corrected `skills/validate/SKILL.md:120` → `:119`** | Sol [P3] | `sed -n '118,121p'` shows the dangling reference on **line 119**: "external caller may run `scripts/record_proof_transition.py`. The recorder" | **ADOPTED** (§4.4, §7.2) |
| **Y8** | **Self-found: the `.pyc` inventory was under-counted.** v3 §9 named two `__pycache__` directories (validate, rpi) holding 7 files; the Sol review listed the same 7 | self-found at the v4 pass | Live, within the twelve: **10 `.pyc` files across 5 `__pycache__` directories** — `implement/scripts`, `plan/scripts`, `rpi/scripts`, **`rpi/tests`**, `validate/scripts`. Both prior counts missed `implement/scripts/freeze_candidate.cpython-314.pyc`, `plan/scripts/mint_intent.cpython-314.pyc`, and `rpi/tests/test_run_once.cpython-314.pyc` | **ADOPTED** (§9.1) |

### Round 4 (the v5 pass) — v4 → v5 correction ledger

| # | Correction | Source | Independent recheck at HEAD | Disposition |
|---|---|---|---|---|
| **Z1** | **Withdraw the `4 suppressed / 2 unsuppressed / 3 none` validator partition and the "Scope invokes no Python" claim.** Replace with an unambiguous effective-process census that counts delegated execution | Sol [P2] | `skills/scope/scripts/validate.sh:6` delegates to `heal.sh --check --strict`; `heal.sh:46` is an unsuppressed `python3` heredoc on that path. Re-derived the full census independently: **9 validators, 10 processes, 6 suppressed, 4 unsuppressed**. `heal.sh:95` verified **unreachable** from `--check` (gated by `MODE == fix`), which is why the total is 10 and not 11 | **ADOPTED** (§1, U9, §4.6, §9.1, §10, §11, §12) |
| **Z2** | **Carry the fresh isolated Scope witness** and preserve the narrower Plan conclusion | Sol [P2] | Re-executed at the v5 pass: `scope validate: PASS`, **42** isolated `.pyc`, **0** core-owned. Repository ten-file core cache unchanged before and after; all mtimes still predate the audit | **ADOPTED** (§9.1) |
| **Z3** | **Reconcile the checked/not-checked boundary.** v4 made a definitive per-validator claim while listing `heal.sh` as not read | Sol [P2] | `heal.sh` source read at the v5 pass (`:40-52`, `:88-100`); the "source not read" statement is removed from §11 and the read is recorded in §10 | **ADOPTED** (§10, §11) |

**Unchanged by this round:** every ranked finding, every count, and every mechanism. **3 P0 / 11 P1 / 14 P2 = 28** stands; the taxonomy, validator denominators, semantic-verdict wording, 49-skill corpus, 48-file/7,648-line inventory, scenario links, proof/writer/fallback/layout findings, the Plan isolated witness, the ten-file cache provenance, the `skills/validate/SKILL.md:119` anchor, and all twelve one-by-one analyses are carried forward intact. Z1–Z3 correct a census and a provenance boundary — not a mechanism.

### Round 5 (the v6 pass) — v5 → v6 correction ledger

| # | Correction | Source | Independent recheck at HEAD | Disposition |
|---|---|---|---|---|
| **AA1** | **Remove §4.6's stale "no Python invocation" claim for Scope.** v5 corrected the census in §1/U9/§9.1 but left the contradicting inventory sentence in place | Sol [P2] | `skills/scope/scripts/validate.sh:6` delegates to `heal.sh --check --strict`; `heal.sh:46` runs **one unsuppressed `python3` heredoc** on that path (`:95` gated by `MODE == fix`, unreachable). External-cache witness: **42 `.pyc` total, 0 core-owned** | **ADOPTED** (§4.6) |
| **AA2** | **Correct "two of the four unsuppressed processes are delegated" to one.** | Sol [P2] | Exactly one is delegated — Scope via `heal.sh:46`. Direct: Validate `validate.sh:14` (within its mixed path), Plan `validate.sh:9`, Research `validate.sh:21` | **ADOPTED** (§12) |
| **AA3** | **Restore the exact v4 lineage and provenance.** v5's input table, lineage paragraph, ledger header, round labels, and checked scope were copied from v4 and named v3 as the immediate input | Sol [P2] | v5's immediate inputs were **v4** (`433a2c3e…`, 668) and the **v4 Sol review** (`3f0aa75e…`, 373); the ledger holds X1–X5, Y1–Y8, Z1–Z3 = sixteen across four rounds before the v6 pass; all ten predecessor identities reproduce | **ADOPTED** (§Input identities, lineage, §0 header, round labels, §10, §11, §13) |

**Unchanged by this round:** the effective census (**9 validators, 10 processes, 6 suppressed, 4 unsuppressed**), **3 P0 / 11 P1 / 14 P2 = 28**, both isolated witnesses (Plan 64/1 core-owned; Scope 42/0), the ten-file cache provenance, Postmortem's contradiction, the taxonomy, validator denominators, the 49-skill corpus, 48 files / 7,648 lines, scenario links, proof/writer/fallback/layout findings, the `skills/validate/SKILL.md:119` anchor, and all twelve one-by-one analyses. AA1–AA3 correct internal consistency and provenance — not a mechanism, a severity, or a count.

### Round 6 (the v7 pass) — v6 → v7 correction ledger

Immediate inputs: the **v6 audit** (`2d140dfe…bf7e`, 725 lines) and the **v6 Sol review**
(`11115120…f27d`, 377 lines), both SHA-verified at open.

| # | Correction | Source | Independent recheck at HEAD | Disposition |
|---|---|---|---|---|
| **BB1** | **Input cardinality.** The table has **four** rows — two immediate inputs plus two restored lineage artifacts — but was headed "both verified" and followed by "Both read in full" | Sol [P2] | Table re-headed "**four** rows," each row now labelled `IMMEDIATE 1/2`, `IMMEDIATE 2/2`, `LINEAGE 1/2`, `LINEAGE 2/2`; the immediate pair is v6 + v6 review, the lineage pair is v5 + v5 review | **ADOPTED** (§Input identities) |
| **BB2** | **Round ownership.** v6 reserved "this pass" for AA1–AA3, yet Y6/Y8 (the v4 round), Z2/Z3 (the v5 round), the Round-3 witness heading, and five body/checked statements still said "this pass" | Sol [P2] | Every round is now named by its authoring version. Y6/Y8 and the Round-3 witnesses → **the v4 pass**; Z1/Z2/Z3 and the heal.sh read → **the v5 pass**; AA1–AA3 → **the v6 pass**. **Interpretation recorded:** the instruction reserved "this pass" for AA1–AA3, but this artifact is v7 — keeping it there would re-create the ambiguity being repaired, so "this pass" now means v7 alone and every historical round carries an explicit version label | **ADOPTED** (§0 rule, Y6, Y8, Z1–Z3, Round-3 and Round-5 headers, §9.1, §10, §11, §13) |
| **BB3** | **Whole-file lineage caption.** Said "unchanged across v1→v4 … all three Sol reviews" despite the restored five-audit/five-review chain | Sol [P2] | **At v7,** the caption was widened to **v1 → v6** over **all five prior Sol reviews**, with a table giving each review's exact identity and its reported 48-row check; Sol confirmed the restriction was not a checked-boundary limit, since all five report checking the full 48 rows. **CC5 extends that caption to v1 → v7 over six prior Sol reviews** — see §9.1 for the current wording | **ADOPTED** (§9.1 inventory caption) |
| **BB4** | **Learn inventory.** Said "No owned executable" while `skills/learn/scripts/validate.sh` is tracked mode `100755` | Sol [P3] | Live: `100755 blob 70341c13… skills/learn/scripts/validate.sh`, and **zero** `python3` tokens in it. Replaced with "**No owned functional implementation beyond the contract validator; the executable validator invokes no Python**," retaining the 100755 mode and the false-pass P0 | **ADOPTED** (§4.5) |

**Unchanged by this round:** the effective census (**9 validators, 10 processes, 6 suppressed,
4 unsuppressed, exactly one delegated**), **3 P0 / 11 P1 / 14 P2 = 28**, both isolated witnesses
(Plan 64/1 core-owned; Scope 42/0), the ten-file cache provenance, the taxonomy, validator
denominators, the 49-skill corpus, 48 files / 7,648 lines, scenario links, proof/writer/fallback/
layout findings, the `skills/validate/SKILL.md:119` anchor, and all twelve one-by-one analyses.
BB1–BB4 correct cardinality, round attribution, caption scope, and one inventory sentence — not a
mechanism, a severity, or a count.

### Round 7 (this pass — v8) — v7 → v8 correction ledger

Immediate inputs: the **v7 audit** (`939ff8e5…7317a`, 757 lines) and the **v7 Sol review**
(`17ea51b4…65ba5`, 387 lines), both SHA-verified at open. The v7 Sol review returned
`REQUEST_CHANGES` on **one [P2] finding** — current-version lineage and cardinality arithmetic —
after passing all four v6 blockers, the full technical program, and an independent replay of every
executable suite.

| # | Correction | Source | Independent recheck | Disposition |
|---|---|---|---|---|
| **CC1** | **Correction-ledger cardinality.** v7 line 31 said "nineteen corrections across five rounds" while the same paragraph enumerated BB1–BB4 and named six passes | Sol [P2] | Counted from v7's own labels: X1–X5 = 5, Y1–Y8 = 8, Z1–Z3 = 3, AA1–AA3 = 3, BB1–BB4 = 4 → **23 across six rounds (v2–v7)**. The nineteen/five figure is retained **only** with an explicit "before v7" scope | **ADOPTED** (§0 header) |
| **CC2** | **Audit-chain cardinality.** v7 line 741 displayed `v7 → v6 → v5 → v4 → v3 → v2 → v1` — seven versions — but called it "six audits and six Sol reviews" | Sol [P2] | The displayed v7 chain is **seven audits and six prior Sol reviews**. Including this v8 audit and the v7 Sol review, the lineage is **eight audits and seven prior Sol reviews**. Both counts are stated with their scope | **ADOPTED** (§13 seal, §Input identities) |
| **CC3** | **Predecessor scope.** v7 lines 25 and 703 called the v1–v5 set "all ten predecessor artifacts" although v6 and its review were predecessors of v7 | Sol [P2] | Restated as **ten pre-v6 lineage artifacts** for that subset, with the complete predecessor set counted at **fourteen prior audit/review artifacts** (v1–v7 audits and v1–v7 reviews) | **ADOPTED** (§Input identities lineage, §10) |
| **CC4** | **Latest severity column.** v7's historical severity table ended at bold **v6** although v7 independently recomputed the same rows | Sol [P2] | Table extended with **v7** and **v8** columns, both **3 / 11 / 14 = 28**, and captioned as history **through v8**. The totals are unchanged in every column since v3 | **ADOPTED** (§0 severity table) |
| **CC5** | **Provenance and seal extension, plus a whole-file numeric sweep.** v7's inventory caption, checked list, not-checked list, and seal all stopped at the v5 or v6 horizon | self-found under Sol's required sweep | Inventory caption now reads **v1 → v7** over **six prior Sol reviews**, with the v6 review's reported 48-row check added to the table; the checked list counts **fourteen** prior artifacts; both "all five prior Sol reviews" statements in §11 now read **six**; the seal is extended through the v7 Sol review and this v8 subject. Every numeric lineage, correction, and version claim in the file was swept for agreement | **ADOPTED** (§0, §9.1, §10, §11, §13) |

**Unchanged by this round:** every ranked finding and every technical count. The effective census
(**9 validators, 10 processes, 6 suppressed, 4 unsuppressed, exactly one delegated**),
**3 P0 / 11 P1 / 14 P2 = 28**, both isolated witnesses (Plan 64/1 core-owned; Scope 42/0), the
ten-file cache provenance, the taxonomy (A1–A30, C1–C10, R1–R3, S1–S3, N1–N6), validator
denominators, the **49-skill canonical corpus**, **48 files / 7,648 lines**, all 48 whole-file
hashes and all twelve Git trees, scenario links, proof/writer/fallback/layout findings, the
`skills/validate/SKILL.md:119` anchor, all executed witnesses, and all twelve one-by-one skill
analyses are carried forward intact. **CC1–CC5 correct lineage arithmetic and provenance scope
only — not a mechanism, a severity, a count, or a recommendation.**

### Two discrepancies between the task instruction and live source — recorded, not silently applied

| Instruction as stated | Live source | Resolution |
|---|---|---|
| "`skills/plan/scripts/validate.sh:9` runs `python3 skills/plan/scripts/kernel_v3.py --check`" | Line 9 is `python3 "$skill_dir/scripts/mint_intent.py" --help >/dev/null`. **Plan owns no `kernel_v3.py`** — the kernel is `skills/validate/scripts/kernel_v3.py`, reached by `importlib` from `mint_intent.py:13-16` | The **substance is correct and adopted** — line 9 invokes Python without suppression and can write an owned `.pyc`. The **command and path are corrected** to what the file actually contains. This also explains why the artifact is `kernel_v3.cpython-314.pyc` and not `mint_intent.cpython-314.pyc`: a script run as `__main__` is never cached; the *imported* kernel is |
| "Correct the recorder citation to `cli/cmd/ao/loop_context.go:119`, not line 120" | **`cli/cmd/ao/loop_context.go` does not exist anywhere in the tree** (`find . -name loop_context.go` → 0 matches). The v3 citation under correction is `skills/validate/SKILL.md:120`, and the Sol review's P3 names that same file | Applied as **`skills/validate/SKILL.md:120 → :119`**, which is the correction the Sol review actually requires and which live source confirms. The `loop_context.go` path is not adopted because no such file exists to cite |

### Severity totals — history through **v8**; unchanged, recomputed from rows (CC4)

| Rank | v2 | v3 | v4 | v5 | v6 | v7 | **v8** |
|---|---:|---:|---:|---:|---:|---:|---:|
| P0 | 3 | 3 | 3 | 3 | 3 | 3 | **3** |
| P1 | 11 | 11 | 11 | 11 | 11 | 11 | **11** |
| P2 | 13 | 14 | 14 | 14 | 14 | 14 | **14** |
| **Total ranked** | 27 | 28 | 28 | 28 | 28 | 28 | **28** |

The v7 column records the rows the v7 Sol review independently recomputed and confirmed
(3 P0 rows, 11 P1 rows, 14 numbered P2 items = 28). The v8 column is this pass's own recount from
§7.1–§7.3 and is identical. **Totals have been unchanged since v3.**

Y6–Y8 are provenance, anchor, and inventory corrections. Per the Sol review, neither finding "alter[s] a technical recommendation or ranked total." Adjudication taxonomy is likewise unchanged: **30** confirmed (A1–A30), **10** corrected (C1–C10), **3** rejected (R1–R3), **3** superseded (S1–S3), **6** new (N1–N6).

---

## 1. Method

Every canonical `SKILL.md` and every directly owned script, test, feature, fixture, schema, and validator for the twelve skills was read line by line at this HEAD — **48 files, 7,648 lines**. The live CLI owners and doctrine surfaces were read in full: `AGENTS.md`, `docs/architecture/operating-loop.md`, `docs/adr/ADR-0016-state-tiers.md`, `docs/contracts/proof-contracts/active.json`, the epoch-0b descriptor, the active epoch-1 descriptor, `scripts/.skill-python-grandfather`, and `scripts/check-scenario-coverage.sh`. Intent was reconstructed from executable behavior and declared contract first; `AGENTS.md` supplied boundary vocabulary, never intent. Generated routers, catalogs, and per-runtime images were not consulted for intent.

**Bytecode hygiene of the method itself (corrected by Y6).** This audit's witnesses executed nine `validate.sh` contract scripts. **Bytecode suppression is a property of each validator's own source, not of a caller-level environment this audit set** — no `PYTHONDONTWRITEBYTECODE=1` was exported around W4/W5/W6. The **effective** process census — counting delegated execution, not just literal tokens — is **9 validators running 10 Python processes: 6 suppressed, 4 unsuppressed** (§9.1). Scope invokes no Python directly, but `skills/scope/scripts/validate.sh:6` delegates to `skills/skill-builder/scripts/heal.sh --check --strict`, whose `:46` heredoc is an unsuppressed Python process on that exact check path (Z1). The repository's `.pyc` cache is therefore **not** provably untouched by construction; it is shown untouched by **measurement** — every core `.pyc` mtime predates this audit, before and after every witness (U6, U8). The one bytecode-capable path was proved capable in an isolated cache prefix that kept all output outside the subject.

Nine claims were settled by execution rather than reading (§2). Each correction round re-executed the decisive subset and re-derived every count it changed.

---

## 2. Executed witnesses

| # | Command | Result |
|---|---|---|
| **W1** | `scripts/check-scenario-coverage.sh --json` over all ten owned features | **pass:** rpi 4/4, plan 3/3, implement 4/4, validate 7/7, idea-genie 2/2, idea-challenge 2/2. **fail:** learn 0/2, research 0/3, premortem 0/2, postmortem 0/2 |
| **W2** | `bash skills/learn/scripts/validate.sh` | `learn skill contract: PASS`, rc=0 — while `skills/learn/SKILL.md:35` contains "emit a lifecycle receipt", which `validate.sh:6`'s `!`-negated grep exists to forbid |
| **W3** | Scratch probe: `set -euo pipefail` + `! grep -Eiq 'receipt' <file containing "receipt">` then `echo REACHED-PASS` | `REACHED-PASS`, `exit=0` — bash exempts `!`-inverted commands from `set -e` |
| **W4** | `validate.sh` for learn, scope, research, postmortem, premortem, plan, implement | all rc=0 PASS |
| **W5** | `bash skills/rpi/scripts/validate.sh` | `rpi skill contract: PASS`, rc=0 — genuinely runs the 13-test `unittest` suite |
| **W6** | `bash skills/validate/scripts/validate.sh` | `kernel-v3-corpus: PASS (43 cases, shared Python/Go)` → `validate skill contract: PASS`; worktree still 0 dirty |
| **W7** | Re-hash of every component in the active epoch-1 descriptor | 8 skill-owned bound components all **OK** (no drift); proof root intact |
| **W8** | `grep -cE '^\s*def test_'` | `test_kernel_v3.py` **27**, `test_validate.py` **16**, `test_run_once.py` **13**; CAS test is a real `def` at `test_kernel_v3.py:1120`; string `test_kernel_v3` **absent** from the active descriptor |
| **W9** | Existence sweep | zero schema hits for `reality-check-report` / `council-report` / `postmortem-report`; no repo-root `scripts/record_proof_transition.py`; learn's two cited verdict digests resolve **0 of 2**; `.agents/` top-level contains only `ao/`; `fm-ws-noncanonical-topdir` **not found** in `cli/` or `scripts/` |

**Round-1 (v2) witnesses:** V1 inventory (48 / 7,648) · V2 `@covered-by` census (26 / 22) · V3 root-contract vocabulary sweep · V4 live re-run of the learn false pass · V5 inert-negation census · V6 proof-membership disclosure sweep (0 of 12) · V7 descriptor parse (25 components, 8 skill-owned, 8 unique) · V8 grandfather census (24 pins, 4 in-core) · V9 doctrine hash recompute · V10 hash-row counts (48 each).

**Round-2 (v3) witnesses:** U1 postmortem effect-honesty citations · U2 v2 §4.10 omission scan · U3 nine-file `validate.sh` census · U4 Validate's kernel-contract-validator classification · U5 `PYTHONDONTWRITEBYTECODE` presence · U6 `.pyc` mtime provenance · U7 ranked-row recount.

**Round-3 (the v4 pass) witnesses:**

| # | Check | Result |
|---|---|---|
| **U8** | **Isolated bytecode-capability witness, re-executed from scratch.** `PFX=$(mktemp -d /tmp/pycache-witness.XXXXXX); env -u PYTHONDONTWRITEBYTECODE PYTHONPYCACHEPREFIX="$PFX" bash skills/plan/scripts/validate.sh` | `plan skill contract: PASS`, **rc=0**. 64 `.pyc` files were written into `$PFX`, of which **exactly one is skill-owned**: `<PFX>/…/skills/validate/scripts/kernel_v3.cpython-314.pyc`. The other 63 are Python 3.14 stdlib. **`git status --porcelain` → 0 before and after.** This proves *capability*, not that any audit witness altered the repository cache |
| **U9** | Per-validator **effective** Python-process census across all nine, counting delegated execution (§9.1) | suppressed-only: **rpi, implement** (2 processes) · mixed: **validate** (5 processes — 4 suppressed, `:14` heredoc unsuppressed) · unsuppressed-only: **plan, scope (delegated via `heal.sh:46`), research** (3 processes) · no Python: **learn, premortem, postmortem** · **totals: 9 validators, 10 processes, 6 suppressed, 4 unsuppressed** |
| **U10** | `skills/validate/scripts/validate.sh:14` heredoc content | `python3 - "$skill_dir" <<'PY'` — reads the program from stdin (so it is `__main__` and is never cached) and imports only stdlib plus `jsonschema`. **No skill-owned module is imported**, so this unsuppressed invocation writes no owned `.pyc` |
| **U11** | `skills/research/scripts/validate.sh:21` | `python3 -m json.tool "$skill_dir/schemas/findings.json" >/dev/null` — unsuppressed, stdlib only, no owned module imported |
| **U12** | `skills/validate/SKILL.md` lines 118–121 | The dangling recorder reference sits on **line 119** — "external caller may run `scripts/record_proof_transition.py`. The recorder" (Y7) |
| **U13** | Core-12 `.pyc` inventory, full enumeration before and after U8 | **10 files across 5 `__pycache__` directories**; every mtime **byte-identical before and after** the witness; newest is `rpi/tests/test_run_once.cpython-314.pyc` at `2026-07-27 23:08:30` — all predate the 2026-07-28 audit (Y8) |
| **U14** | `find . -name loop_context.go -not -path './.git/*'` | **0 matches** — the file named in the task instruction does not exist in this tree |

No tracked file was written by any witness. W5/W6 write only into system temp directories; U8 redirected all bytecode to an isolated `/tmp` cache prefix. The worktree was re-verified clean after every execution.

---

## 3. The proof-binding map (governs every recommendation below)

`docs/contracts/proof-contracts/active.json` (`25bc0adc…`) declares **epoch 1**, pointing at `docs/evidence/proof-epochs/epoch-1/subject-refreeze-candidate-descriptor.json` (`f6358e38…2340`). That descriptor binds **25 components**; **8** live inside four of the twelve skills (V7, re-confirmed by the v3 Sol review — 8 entries, 8 unique refs, all bytes and modes matching):

| Role | Bound file | Digest at HEAD |
|---|---|---|
| `validator-contract` | `skills/validate/SKILL.md` | OK |
| `validator-implementation` | `skills/validate/scripts/kernel_v3.py` | OK |
| `validator-cli` | `skills/validate/scripts/validate_v3.py` | OK |
| `qualification-corpus-runner` | `skills/validate/scripts/check_kernel_v3_corpus.py` | OK |
| `transition-recorder-implementation` (+ `transition_recorder`) | `skills/validate/scripts/record_proof_transition.py` | OK |
| `plan-intent-minter` | `skills/plan/scripts/mint_intent.py` | OK |
| `implement-candidate-freezer` | `skills/implement/scripts/freeze_candidate.py` | OK |
| `rpi-dispatcher` | `skills/rpi/scripts/run_once.py` | OK |

`kernel_v3.load_active_proof` (`kernel_v3.py:1149-1157`) re-hashes **every** component and compares mode on each validation, raising `TerminalValidation` on any byte or mode change. Editing or moving any of the eight **halts all judgment under epoch 1** until a recorded transition activates a new descriptor.

**Not bound at epoch 1:** `test_kernel_v3.py`, `test_validate.py`, `validate.py`, `check_contract_corpus.py`. The last two were bound under epoch-0b; epoch 1 replaced them. `validate.py`'s current bytes (`adafe4e1…`) still match the epoch-0b entry, but epoch-0b is not active.

**Ratchet interaction (V8).** `scripts/.skill-python-grandfather` holds exactly **24** pins. Four sit inside the twelve: `rpi/scripts/run_once.py`, `validate/scripts/{check_contract_corpus,test_validate,validate}.py`. **Seven** governed Python files under `skills/*/scripts/**` are unpinned — `plan/mint_intent.py`, `implement/freeze_candidate.py`, `validate/{kernel_v3,validate_v3,record_proof_transition,check_kernel_v3_corpus,test_kernel_v3}.py`. `skills/rpi/tests/test_run_once.py` is exempt as a class; `test_kernel_v3.py` is not, because it sits under `scripts/`. `--scope head` passes at this HEAD.

**The collision.** Six of those seven unpinned files are *also* bound proof components. ADR-0016 §3 says they must become Go and leave `skills/*/scripts`; the proof root says they cannot move without a transition. Any recommendation that says "delete the file and watch the ratchet go green" is, at this landing, an instruction to brick epoch-1 judgment.

---

## 4. Per-skill findings

### 4.1 rpi

**Owned (5 files, 693 lines):** `SKILL.md` (117), `references/rpi.feature` (31), `scripts/run_once.py` (150), `scripts/validate.sh` (12), `tests/test_run_once.py` (383).

`run_once.py` is the loop's executable law and is genuinely pure — no Git, no `ao`, no tracker. Plan's return must be exactly `{intent_ref, intent_digest, byte_length}` (`:64-69`); the ref must literally bind the minted digest (`:77-85`); the snapshot is re-verified through `kernel.consume_intent_snapshot` with exact byte-length equality (`:86-93`). `None` from Plan → `NOT_PLANNED`; `None` from Implement → `NOT_BUILT`. Validate's return must be PASS/FAIL/NOT_PROVEN, echo the same `intent_digest`, and supply all six durable identities (`:126-135`) or the invocation raises. Terminal reports are built only by `kernel.build_rpi_report_v2`.

**Placement** the one-shot dispatcher; the only orchestrator; self-limits to zero-or-one dispatch per phase. **Effects** `[dispatch_core_phases]` — honest. **Mutation boundary** none of its own. **Stop semantics** hard; `rpi-report.v2` cannot carry a next action — `require_exact_keys` (`kernel_v3.py:1728-1748`) rejects unknown fields. **Accidental authority** none.

**Prose vs tests.** W1 4/4 executed. W5 runs the real suite. All five `@covered-by` targets resolve. `test_opaque_correlation_bounds_are_enforced` (`:335`) exercises the 8-entry/256-char bounds; `test_serialized_remote_boundary_preserves_single_mint_identity` (`:221`) drives the real `mint_intent.py` across a JSON round-trip and asserts `mint.call_count == 1` while the living source is rewritten mid-flight.

**Defect — two inert negations.** `scripts/validate.sh:9-10` asserts absence of `Continuation envelope` and `repair revision per wave` via `! grep -Fq`; under `set -euo pipefail` a `!`-inverted command never triggers exit (W3, V5, U3).

**Improvements.**
- **P1 — convert the two inert negations to the sound form.** Owner `skills/rpi/scripts/validate.sh:9-10`. *Witness:* append `Continuation envelope` to a scratch copy of `SKILL.md` and point the validator at it — it exits 0 today.
- **P2 (re-ranked from P1 by X4) — proof-bound status is undocumented in the skill that owns the file.** Owner `skills/rpi/SKILL.md` (unbound — safe to edit). **Reason for the re-rank:** V6 shows **0 of 12** core `SKILL.md` files disclose membership — including the bound `skills/validate/SKILL.md` — with **no** rule requiring it. RPI is not an outlier and breaches no contract. Promote back to P1 only if the repository adopts a general disclosure rule. *Witness:* the descriptor lists `skills/rpi/scripts/run_once.py` while `grep -Fq 'epoch-1' skills/rpi/SKILL.md` fails.
- **P2 —** correlation bounds proven at test level (`:335`) and corpus level (`correlation.over-property-bound`). No action.

### 4.2 plan

**Owned (4 files, 190 lines):** `SKILL.md` (85), `references/plan.feature` (23), `scripts/mint_intent.py` (72), `scripts/validate.sh` (10).

`mint_intent.py` is a thin adapter over `kernel.mint_intent_snapshot`: mint exact bytes once, content-address under the intent dir, return only `{intent_ref, intent_digest, byte_length}` with `intent_ref = <normalized root>/<digest>.intent`. It reaches the kernel by `importlib` — `:13` `KERNEL_PATH = Path(__file__).parents[2] / "validate" / "scripts" / "kernel_v3.py"`, `:14-16` `spec_from_file_location` → `module_from_spec`. `--expected-digest` is plumbed to the kernel, which raises `TerminalValidation` on mismatch (`kernel_v3.py:486-489`). All shaping is agent-executed prose; the freeze runs through `validate_v3.py freeze-scope`.

**Placement** inside the loop; the only phase permitted to touch the living intent source and the only minter. **Effects** `update_intent_source` plus the mint write into the runtime-excluded intent store. **Mutation boundary** the caller's intent source, pre-freeze only; a post-freeze discovery stops the invocation (`SKILL.md:68-70`). **Stop semantics** typed proposed-amendment-and-stop (`SKILL.md:36-38`). **Accidental authority** none; `SKILL.md:55` explicitly forbids a plan packet or campaign graph.

**Defect A — inert negation.** `scripts/validate.sh:8` asserts `! grep -Fq 'plan-packet.v1'`.

**Defect B — the contract validator can write owned bytecode (new detail, Y6).** `scripts/validate.sh:9` runs `python3 "$skill_dir/scripts/mint_intent.py" --help >/dev/null` with **no** `-B` and **no** `PYTHONDONTWRITEBYTECODE`, unlike its rpi, implement, and validate siblings. Because `mint_intent.py` imports the owned kernel, running Plan's contract validator on a cold cache writes `skills/validate/scripts/kernel_v3.cpython-314.pyc` — a `__pycache__` entry inside a **proof-bound component's directory**. U8 reproduced this in an isolated cache prefix: `plan skill contract: PASS`, rc 0, exactly one skill-owned `.pyc` emitted. The artifact is gitignored and did not dirty the tree, but the asymmetry with the other three Python-invoking validators is unexplained.

**Improvements.**
- **P0 — the "promote to Go and delete" path would break the active proof root.** Owner: the Go-kernel promotion program + `docs/contracts/proof-contracts/`. `mint_intent.py` is simultaneously ADR-0016 debt and the bound `plan-intent-minter`. *Witness:* move it and call `kernel.load_active_proof(repo)` — `TerminalValidation: active proof component bytes or mode changed: skills/plan/scripts/mint_intent.py`; **all judgment halts**, the opposite of "the ratchet goes green".
- **P1 — the untested surface is the CLI, not the law.** Owner `skills/plan/tests/` (new; ratchet-exempt, unbound). Genuinely untested: `--expected-digest` refusal and the `argparse main()` path (`:41-68`). *Witness:* a CLI invocation with a wrong `--expected-digest` must exit 2 with `plan-mint-intent:` on stderr; nothing asserts it.
- **P1 — inert negation.** Owner `skills/plan/scripts/validate.sh:8`.
- **P2 — `produces: scope-index.v1` names a capability whose executable lives in validate.** Owner `skills/plan/SKILL.md:11-12`. *Witness:* `grep -rn 'freeze-scope\|freeze_scope_index' skills/plan/` returns nothing.
- **Hygiene note (not ranked) —** align `validate.sh:9` with its siblings by prefixing `PYTHONDONTWRITEBYTECODE=1`. This is bytecode hygiene, not a contract defect: the output is gitignored and the tree stays clean. Recorded here because Y6 withdrew the claim that suppression is universal.

### 4.3 implement

**Owned (4 files, 186 lines):** `SKILL.md` (78), `references/implement.feature` (25), `scripts/freeze_candidate.py` (72), `scripts/validate.sh` (11).

`freeze_candidate.py` consumes the pre-minted snapshot against the expected digest and builds `subject-manifest.v2` over declared observation roots. `derive_effect_receipt` marks coverage `COMPLETE` only when both manifests use exactly `REPOSITORY_OBSERVATION` and `COMPLETE_RUNTIME_EXCLUSIONS` (`kernel_v3.py:742-749`); `store_verdict_v3` demotes anything else to `NOT_PROVEN` (`:1609-1621`).

**Placement** inside the loop; sole owner of subject edits and factual receipts. **Effects** `modify_declared_subject`, `derive_subject_manifest` — accurate. **Mutation boundary** the subject bounded by frozen scope; observation explicitly *not* bounded by write scope (`SKILL.md:44-46`). **Freshness** any post-freeze mutation is terminal — proved by `test_candidate_mutation_after_freeze_is_terminal` and, more sharply, `test_candidate_mutation_between_validation_recomputations_is_terminal`. **Accidental authority** none; `SKILL.md:73-78` forbids commit, push, claim, close, release, land, reserve, retry, intent revision, semantic validation.

**Defect A — two inert negations** (`validate.sh:8-9`). The validator does **not** grep for commit/push/claim/close/retry at all — a claim the advisory reports made that the file does not support.

**Defect B — unverified kernel fallback.** `freeze_candidate.py:13-30` `find_kernel()`: if `../../validate/scripts/kernel_v3.py` is missing it walks every ancestor for `<ancestor>/agents/validator/skills/validate/scripts/kernel_v3.py` and executes whatever it finds, **with no digest check**. Neither `run_once.py` nor `mint_intent.py` has this fallback. It sits inside a bound component: the file whose bytes epoch 1 pins can be made to execute a kernel epoch 1 does not pin. Current direct and projected bytes are identical at `f7787f4505c6f49c77890411a49387a02beec7a267595e158af6e4184ca6ef70`, so the exposure is divergence/tampering, not a present mismatch.

**Improvements.**
- **P0 — same proof-root collision as plan.** *Witness:* moving it makes `load_active_proof` raise `TerminalValidation` naming `skills/implement/scripts/freeze_candidate.py`.
- **P1 — remove or digest-check the projected-kernel fallback.** Owner `freeze_candidate.py:13-30` — **transition-gated**; belongs in the next candidate descriptor, not a hotfix. *Witness:* stage a tree with the direct kernel absent and a divergent projected kernel; the bound component executes unverified code and exits 0.
- **P1 — inert negations.** Owner `skills/implement/scripts/validate.sh:8-9`.
- **P2 — adapter-level probe.** Owner `skills/implement/tests/` (new, ratchet-exempt, unbound).
- **P2 — `consumes: scope-index.v1` has no implement-owned reader.**

### 4.4 validate

**Owned (11 files, 5,352 lines):** `SKILL.md` (139), `references/validate.feature` (46), `scripts/validate.sh` (46), `kernel_v3.py` (2,030), `test_kernel_v3.py` (1,213), `validate.py` (586), `record_proof_transition.py` (321), `validate_v3.py` (285), `test_validate.py` (280), `check_kernel_v3_corpus.py` (259), `check_contract_corpus.py` (147). All read in full.

`kernel_v3.py` is the proof kernel: single-mint snapshots with filename-binds-digest enforcement (`:494-508`); `subject-manifest.v2` with the canonical COMPLETE rule; `scope-index.v1` freezing with unique criterion IDs and a hard refusal to absorb a required criterion into an exclusion (`:605-610`); `derive_effect_receipt` with typed change kinds including deletions and mode changes plus digest-nullability consistency (`:807-819`); `build_check_receipt`; `store_verdict_v3`; `load_active_proof` walking pointer → contract → components → recorder → corpus → transition → qualification-verdict.

`store_verdict_v3` is stricter than prose suggests: exactly six semantic keys (`:1658-1663`); every criterion evidence digest must be a member of supplied typed receipts (`:1664-1672`); `verify_manifest_v2(final, repository)` runs **twice** (`:1574`, `:1676`); incomplete coverage → `NOT_PROVEN` takes **precedence** over out-of-scope → `FAIL` (`if` at `:1609` / `elif` at `:1622`); a candidate touching `active.json` raises before any proof loads (`:1632-1633`).

**Placement** the fresh-Validate phase and the kernel authority plan/implement/rpi borrow. **Effects** `write_verdict_artifact`; real effects are atomic, fsync'd filesystem writes, no network, no Git. **Stop semantics** no WARN, confidence, disposition, next action, retry, or delivery state. **Accidental authority** none found across 5,352 lines.

**Validator note (Y3, refined by U9/U10).** `skills/validate/scripts/validate.sh` is the **kernel contract validator**, not a lifecycle-negative-guard validator: it runs `--help` smoke checks on `validate_v3.py` and `record_proof_transition.py`, a schema-conformance heredoc, `unittest discover` over `test_kernel_v3.py`, and `check_kernel_v3_corpus.py` — carrying **zero** `! grep` or `if grep …; then exit 1; fi` lifecycle assertions. It is the ninth tracked `validate.sh` and is excluded from the eight-validator negative-guard denominator. Of its **five** Python invocations, **four** set `PYTHONDONTWRITEBYTECODE=1` (`:12`, `:13`, `:44`, `:45`); the fifth (`:14`) is a stdin heredoc that imports only stdlib and `jsonschema`, so it caches nothing skill-owned.

**Prose vs tests.** W1 7/7. W6: the 27-test kernel suite plus the 43-case shared corpus execute green; `check_kernel_v3_corpus.py:210-218` fails closed if `go` leaves the required-consumer set; `check_contract_corpus.py` fails closed below 10 cases (`:116-118`), supports a required-schema mode (`:100`), mirrors Go's duplicate-key rejection (`:33-47`).

**Defect A — the v2 writer performs the exact act the skill's own validator greps to forbid.** `SKILL.md:106` says "Never re-snapshot intent during storage" and `validate.sh` greps that literal string; `validate.py store-verdict` (`:552-558`) reads `--intent-source` from a **living file** and calls `snapshot_intent(...)` before persisting `verdict.v2`. `test_validate.py:181` names the behavior deliberately.

**Defect B — dual path grammars.** `validate.py:38-46` `normalize_rel` rewrites `\`→`/` and strips `./`; `kernel_v3.py:180-199` rejects both outright.

**Defect C — dangling path citation inside a bound component (anchor corrected by Y7).** **`SKILL.md:119`** directs an external caller to `scripts/record_proof_transition.py`; no such repo-root file exists (W9). The real path is `skills/validate/scripts/record_proof_transition.py` — exactly what the descriptor binds. (v3 cited line 120; live source places the sentence on **119**, verified by U12.)

**Improvements.**
- **P0 — the largest ADR-0016 debt is also the proof root.** Five unpinned governed Python files here; four are bound epoch-1 components and `skills/validate/SKILL.md` is bound too. Go promotion is gated on a proof-root transition, not merely on writing Go.
- **P1 — v2 writer vs the skill's own grep-asserted invariant.** Owner `skills/validate/scripts/validate.py` — **not** an epoch-1 component, so the code-side fix needs **no transition**; only a `SKILL.md` wording change would. Prefer the unbound lever: gate v2 `store-verdict` behind an explicit legacy flag.
- **P1 — dangling recorder path.** Owner **`skills/validate/SKILL.md:119`** — **transition-gated**; queue into the next candidate descriptor.
- **P2 — `test_kernel_v3.py` relocation** is proof-unblocked (W8: the string is absent from the descriptor; the qualification corpus is `tests/fixtures/rpi-kernel-v3`). Moving it to `skills/validate/tests/` shrinks the governed set by one 49.8 KB file with no transition; the only coupling is `validate.sh`'s `unittest discover -s "$skill_dir/scripts"`.
- **P2 — record the dual path grammar**, preferably as a comment in the unbound `validate.py`.

### 4.5 learn

**Owned (3 files, 69 lines):** `SKILL.md` (52), `references/learn.feature` (10), `scripts/validate.sh` (7, tracked mode **100755** — an owned executable). **No owned functional implementation beyond the contract validator; the executable validator invokes no Python.** v6's "No owned executable" is withdrawn (BB4) — it contradicted the live Git mode. Learn's place in the effective census is unchanged: it remains in the **no-Python** class, and its live `! grep` false pass (P0) is unaffected.

Contract-only, declaring the off-path consumer role. Two disciplines carry real weight — **overweight failures** ("a `NOT_PROVEN` or `FAIL` verdict carries more teaching value than a PASS") and **provenance decay** ("every cited artifact must still resolve… Dead citations get pruned rather than paraphrased"). **Placement** outside the loop, strictly after it; `operating-loop.md:117-118` concurs. **Optionality** total. **Accidental authority** none.

**Defect A — the validator is live-false-passing (W2, W3, V4).** `validate.sh:6` is `! grep -Eiq 'receipt|plan_impact|next_action|retry|delivery|closure' "$skill_dir/SKILL.md"`. Line 35 contains "emit a lifecycle receipt". The grep matches; `!` inverts; bash exempts inverted commands from `set -e`; PASS, rc 0. This is the one case in the corpus where the forbidden token is **already present** and the guard is **already silent** — re-reproduced live in the v2 pass and again by the v3 Sol review.

**Defect B — the skill violates its own decay rule.** `SKILL.md:44-47` cites two verdict digests under `.agents/ao/verdicts/`; W9 resolves **0 of 2**.

**Defect C — v2 vocabulary.** `consumes: [verdict.v2]` and body prose, while the live writer emits `verdict.v3`.

**Improvements.**
- **P0 — the validator cannot fail.** Owner `skills/learn/scripts/validate.sh:6` (+ reword `SKILL.md:35`, e.g. "emit lifecycle bookkeeping", so the corrected guard passes for the right reason).
- **P1 — dead citations.** Owner `skills/learn/SKILL.md:44-47`.
- **P1 — v2/v3 vocabulary.** Owner `skills/learn/SKILL.md:8-9,30`.
- **P2 — no scenario coverage** (W1 `fail 0/2`).

### 4.6 scope

**Owned (2 files, 121 lines):** `SKILL.md` (107), `scripts/validate.sh` (14). No feature file; **no direct `python3` token in Scope's own validator — but Scope is not Python-free in effect** (see the bytecode note below).

Advisory write-scope reviewer feeding Plan. Two distinctive disciplines: the **axiom-kernel** derivation (every include/exclude pattern traces to exactly one named axiom; failure mode *vibes perimeter*) and the **byte-verified recovery ceremony** (restore then compare content hashes; failure mode *eyeballed restore*). **Effects** `[]` — correct, and response-only. **Mutation boundary** none (`SKILL.md:36-37`). **Accidental authority** none — and uniquely **the validator proves the negative correctly**: `validate.sh:8-12` uses `if rg -n '…'; then exit 1; fi`, immune to the `set -e` exemption, and delegates structural hygiene to `heal.sh --check --strict`. The strongest of the eight lifecycle-negative-guard validators. **Bytecode note (Z1, restated exactly at v6 — AA1):** that delegation *is* Scope's Python path. `skills/scope/scripts/validate.sh:6` runs `heal.sh --check --strict`, and `heal.sh:46` executes **one unsuppressed `python3` heredoc** on that exact `--check` path — no `-B`, no `PYTHONDONTWRITEBYTECODE`. Scope is therefore an **effective unsuppressed Python caller**, even though its own `validate.sh` contains no `python3` token; `heal.sh:95` is gated by `MODE == fix` and is unreachable here. The external-cache witness recorded **42 `.pyc` total and 0 core-owned** (§9.1). Any statement that Scope makes "no Python invocation" is withdrawn.

**Improvements.**
- **P2 — the YAML output block is prose-only.** Owner `skills/scope/scripts/` (new output validator).
- **P2 — no feature file.** Owner `skills/scope/references/` (new).

### 4.7 reality-check

**Owned (1 file, 65 lines):** `SKILL.md`. No scripts, schema, feature, or validator.

Three sharp disciplines: the **vision-coverage audit** (failure mode *built-world bias*), **frozen question variants** (*drifting rubric*), and the **ambition-escalation checkpoint**. **Effects** `write_advisory_gap_report`. **Stop** report and stop; `SKILL.md:63-65` denies creating work, scheduling, claiming, implementing, validating, retrying, delivering.

**Defect — dangling output contract.** `produces: [reality-check-report.v1]` / `output_contract: reality-check-report.v1` name a contract that exists nowhere (W9).

**Improvements.**
- **P1 — give the named contract a surface.** Owner `skills/reality-check/{schemas,scripts}/` (new). Copy `idea-genie/scripts/validate-output.sh` (jq) or `premortem/scripts/validate-output.sh` (python).
- **P2 — add `scripts/validate.sh`** (X5: verified absent; the skill holds exactly one tracked file).

### 4.8 research

**Owned (4 files, 266 lines):** `SKILL.md` (123), `references/research.feature` (18), `schemas/findings.json` (96), `scripts/validate.sh` (29).

Three disciplines: **commit-level citation** (*floating citation*), **capability-flag doneness** (*effort-shaped doneness*), and multi-report synthesis with a source ledger forbidding merged citations — "Reports that repeat one upstream source are agreement in wording, not independent corroboration." **Accidental authority** none: `validate.sh:23-27` uses the sound `if rg …; then exit 1; fi` form against retired lifecycle markers — the second-strongest anti-regression guard in the set.

**Bytecode note (Y6/U11).** `validate.sh:21` runs `python3 -m json.tool "$skill_dir/schemas/findings.json"` without suppression. It imports only stdlib and no skill-owned module, so it writes no owned `.pyc`. Recorded for completeness because Y6 withdrew the universal suppression claim.

**Improvements.**
- **P2 — `effects: []` versus declared `Write`.** Owner `skills/research/SKILL.md:14,16-17`. Same contract class as the Postmortem finding (N6).
- **P2 — no scenario coverage** (W1 `fail 0/3`).

### 4.9 premortem

**Owned (5 files, 221 lines):** `SKILL.md` (96), `references/premortem.feature` (12), `schemas/premortem-plan-review.v1.schema.json` (41), `scripts/validate.sh` (21), `scripts/validate-output.sh` (51). No Python invocation in the contract validator.

Two strategies that must not be homogenized into Plan: **adversarial defeat attempts** (*armchair pessimism*) and the **derivation-diff challenge** ("A challenger that critiques the handed plan is a yes-man with extra steps"). **Accidental authority** none — sound guard form. The output validator is real and strict: exact key set, 64-hex `intent_digest`, per-finding non-empty evidence, and **enforced author/judge distinctness**.

**Improvements.**
- **P1 — v2 vocabulary.** Owner `skills/premortem/SKILL.md:86` — "not `verdict.v2`" should read "no verdict of any version".
- **P2 — no scenario coverage** (W1 `fail 0/2`).

### 4.10 postmortem

**Owned (3 files, 141 lines):** `SKILL.md` (106), `references/postmortem.feature` (17), `scripts/validate.sh` (18). No Python invocation.

Core discipline: **correlation-to-cause discrimination** — promoting a claim requires a stated mechanism, discriminating evidence, and a counterfactual test; "Symptom disappearance after a change satisfies none of these on its own" (*post-hoc fix attribution*). `SKILL.md:50-51`'s honesty clause is grep-enforced. **Accidental authority** none — sound guard form.

**Effects — declared empty while the skill writes a durable artifact (N6, preserved from v3).** The contradiction is stated four times inside one file:

| Line | Content |
|---|---|
| `:10-11` | `produces:` / `- postmortem-report.md` |
| `:17` | `effects: []` |
| `:64-65` | "Emit a report containing supported claims, rejected claims, unknowns, evidence references, and suggested experiments. Stop." |
| `:91-95` | Artifact directory `.agents/postmortem/` · filename `YYYY-MM-DD-postmortem-<topic>.md` · Markdown serialization · validator command `bash skills/postmortem/scripts/validate.sh` |

This is the same contract class as Research's `effects: []`-versus-declared-`Write` (§4.8). It also matters structurally: **Defect A below depends on this write being real.** An audit that proves the ADR-0016 layout conflict and the loop-visible effect from the same artifact write cannot leave `effects: []` unchallenged.

**Defect A — artifact directory versus ADR-0016, with a loop consequence.** `SKILL.md:91` declares `.agents/postmortem/`. ADR-0016 §1 closes the scratch tier to `ao/`, `scratch/`, `projections/`. `kernel_v3.RUNTIME_EXCLUSIONS` does **not** cover `.agents/postmortem/`, and `COMPLETE` coverage requires exactly that exclusion set (`:742-749`). A postmortem written mid-experiment appears as a real changed path and, absent a scope class, becomes an `undeclared_path` → `FAIL` (`:1622-1631`).

**Defect B — the proposed enforcement does not exist.** ADR-0016 §1 names `fm-ws-noncanonical-topdir` as the enforcing `ao doctor` detector; W9 finds it nowhere in `cli/` or `scripts/`.

**Defect C — `output_contract` points at a feature file** (`SKILL.md:30`).

**Defect D — v2 vocabulary** (`consumes: [verdict.v2]`).

**Improvements.**
- **P1 — move the convention into the closed set.** Owner `skills/postmortem/SKILL.md:91-92`. **Acceptance-witness note:** the witness must distinguish *layout compliance* from *runtime exclusion* — moving the directory under `scratch/` repairs the ADR violation but, by design, mid-experiment visibility remains, because `RUNTIME_EXCLUSIONS` covers only `.git`, `.agents/ao/intents`, `.agents/ao/verdicts`, `.agents/ao/reports`.
- **P1 — the ADR's own enforcement is unbuilt.** Owner `cli/` (`ao doctor` detector, bead `age-state-tiers-operationalize-5mzlm.7`).
- **P2 (N6) — declare the report-write effect, or make the output response-only.** Owner `skills/postmortem/SKILL.md:17` (frontmatter `effects`), reconciled against `:10-11`, `:64-65`, and `:91-95`. Either declare a `write_advisory_postmortem_report`-class effect scoped to the artifact directory, or delete the artifact specification and make the output a response the caller persists. *Lethal witness:* a corpus check asserting that any skill whose `Output Specification` names an artifact directory **and** a filename convention declares a non-empty `metadata.effects` fails on postmortem today, while `scope` (response-only, `effects: []`) correctly passes.
- **P2 — `output_contract` names a feature file.**
- **P2 — v2 vocabulary.**
- **P2 — no scenario coverage** (W1 `fail 0/2`).

### 4.11 council

**Owned (1 file, 81 lines):** `SKILL.md`. No scripts, schema, feature, or validator.

Distinctive strategy content: **methodology-weighted agreement** ("A consensus claim must name at least two distinct methodologies among its supporting judges"; *echo consensus*), the **model-diversity axis** with `diversity_unsatisfied` disclosure and an explicit "never via `claude -p`", **fresh sessions per round**, and **synthesis-bucket completeness** ("a finding silently dropped from synthesis is majority laundering"). Explicitly subordinate as a Validate strategy (`SKILL.md:79-81`).

**Defect A — dangling output contract.** `council-report.v1` has zero schema hits (W9).
**Defect B — v2 vocabulary** (`SKILL.md:78`).

**Improvements.**
- **P1 — give `council-report.v1` a schema and validator.** Owner `skills/council/{schemas,scripts}/` (new). Copy `idea-genie/scripts/validate-challenge.sh`.
- **P2 — v2 vocabulary.**
- **P2 — no `scripts/validate.sh` and no feature file** (X5: verified — exactly one tracked file).

### 4.12 idea-genie

**Owned (5 files, 263 lines):** `SKILL.md` (124), `references/idea-genie.feature` (15), `references/idea-challenge.feature` (16), `scripts/validate-output.sh` (43), `scripts/validate-challenge.sh` (65).

Two modes: **elicit** (an empty `no-new-work` result is valid; quota-filling forbidden) and **duel** (sealed independent perspectives for one-way doors). Both outputs have **real** validators — the strongest output-contract enforcement among the advisory skills. `validate-challenge.sh` enforces an exact top-level key set, pins `handoff.owner == "plan"`, and for `one-way` requires sealed generation, ≥2 perspectives with unique IDs **and** unique context IDs, cross-reviews where reviewer ≠ subject, non-empty disagreements and refutations, and the **absence** of `requires_ntm`. **Accidental authority** none — the exact-key-set check refuses a `readiness` field structurally. These two are **output** validators, not lifecycle contract validators; idea-genie owns no `scripts/validate.sh` and is outside the nine-file denominator (Y3). Four of the eight tests in `tests/scripts/agentops-native-skills.bats` are idea-genie's; all pass.

**Defect — artifact directory.** `SKILL.md:98` declares `.agents/ideas/<run-id>/`; same ADR-0016 conflict and loop consequence as postmortem.

**Improvements.**
- **P1 — move the convention into the closed set.** Owner `skills/idea-genie/SKILL.md:98,104` **and** `tests/scripts/agentops-native-skills.bats` fixtures (which embed `.agents/ideas/run-1`, `run-2`). Validator logic needs no change. Same layout-vs-exclusion witness note as postmortem P1.
- **P2 — shared context-isolation reference** linked from both council and idea-genie; the strategies stay distinct, only the isolation checklist is shared.

---

## 5. Cross-skill RPI map

```text
  outside the loop (pre)          ── the one-shot loop ──          outside the loop (post)
  ─────────────────────           ───────────────────────          ───────────────────────
  research      (evidence) ┐
  idea-genie    (options)  │                                        learn      (verdict
  reality-check (gaps)     ├──▶  plan ──▶ implement ──▶ validate                collections)
  scope         (boundary) ┘       ▲          │            │        postmortem (causal
                                   │          │            │                    questions)
  premortem (plan challenge) ──────┘          │            │
  council   (judgment strategy) ──────────────┼────────────┘
                                              │
                        rpi dispatches each phase at most once ──▶ rpi-report.v2
```

**Required core (4):** rpi, plan, implement, validate — laws executable (`run_once.py` + `kernel_v3.py`), each owning at least one bound epoch-1 component.
**Pre-loop advisory (4):** scope, reality-check, research, idea-genie.
**Strategies (2):** premortem (frozen-plan challenge), council (independent judgment, subordinate to the one accountable fresh validator).
**Post-loop consumers (2):** learn, postmortem — both off-path; `operating-loop.md:110-118` concurs.

**Authority sweep.** Across all twelve, no skill holds retry, queue, budget, campaign, Git, closure, release, or delivery authority, and only validate writes a verdict. **That conclusion survives the full read of all 7,648 owned lines** and is the strongest thing this corpus has going for it.

**Enforcement tiers — precise denominator (Y3).** The twelve own **nine** tracked `scripts/validate.sh` files. One of the nine — Validate's — is the **kernel contract validator** (zero lifecycle-negative assertions) and is excluded. Of the remaining **eight lifecycle-negative-guard validators**:

| Form | Count | Files |
|---|---:|---|
| Structurally inert (`! grep` under `set -e`) | **4** | rpi (2 assertions), plan (1), implement (2), learn (1) — **6 assertions total** |
| Sound (`if grep/rg …; then exit 1; fi`) | **4** | scope, research, premortem, postmortem |

Executable behavioral proof: rpi (13 tests), validate (27 + 16 tests + 43-case cross-language corpus), idea-genie and premortem (real **output** validators, distinct from the nine contract validators). No `scripts/validate.sh` at all: reality-check, council, idea-genie.

**The `.agents/` interaction.** Only `.git`, `.agents/ao/intents`, `.agents/ao/verdicts`, `.agents/ao/reports` are runtime-excluded. Every other `.agents/` path — including the proposed `scratch/` and `projections/` tiers and the declared `.agents/postmortem/` and `.agents/ideas/` — is observed by `subject-manifest.v2`. Any skill writing under `.agents/` during an experiment is loop-visible by construction, **which is precisely why postmortem's `effects: []` (§4.10) is a contradiction and not a nitpick.**

---

## 6. Adjudication of the advisory findings

**CONFIRMED (30).** A1 rpi `run_once.py` grandfathered **and** bound · A2 correlation-bound test exists · A3 plan `produces: scope-index.v1`, freezer in validate's CLI · A4 implement `consumes: scope-index.v1`, no owned reader · A5 implement adapter probe is hygiene (P2) · A6 `validate_v3.py record-check` present · A7 recorder verifies membership/subject/pointer three times, CAS-locked, content-addressed, pointer swapped last · A8 `store_verdict_v3` six keys / receipt-subset evidence / double manifest verify · A9 incomplete-coverage demotion precedes out-of-scope FAIL · A10 43-case corpus with a live required `go` consumer · A11 `check_contract_corpus.py` fails closed under 10 cases · A12 dual path grammars · A13 v2 writer contradicts the legacy sentence · A14 learn `consumes: verdict.v2` · A15 learn violates its own decay rule · A16 scope has no output validator · A17 `reality-check-report.v1` dangling · A18 reality-check has no `validate.sh` · A19 research `effects: []` vs declared `Write` · A20 premortem v2 wording · A21 postmortem/idea-genie dirs conflict with ADR-0016 · A22 postmortem `consumes: verdict.v2` · A23 `council-report.v1` has no schema · A24 council v2 wording · **A25 `operating-loop.md` is one version behind the executable** — *repair implication expanded by X1 to include root `AGENTS.md`* · A26 validator strength is bimodal — *denominator made precise by Y3* · A27 test count 27+16 = 43 · A28 the CAS test is real · A29 four features carry no tags · A30 deliberate strategy differences preserved.

**CORRECTED (10).**

| # | Advisory claim | Correction |
|---|---|---|
| C1 | plan/implement P0 witness = "ratchet green with the file absent" | Both are bound epoch-1 components; deleting or moving either raises `TerminalValidation` and halts judgment. Both P0s stand **as transition-gated**; readiness is a recorded transition, not an absent file |
| C2 | "five unpinned… `kernel_v3` + recorder are bound" | The bound set is larger: `kernel_v3`, `validate_v3`, `check_kernel_v3_corpus`, `record_proof_transition`, **and `skills/validate/SKILL.md`**. Of the five unpinned files, **four** are bound |
| C3 | `test_kernel_v3.py` "is descriptor-referenced" | **Refuted** (W8). No transition needed; only `validate.sh`'s discover path couples. Downgraded to P2 |
| C4 | plan "double-mint / expected_digest / packet shape tested only indirectly" | **Overstated.** `test_run_once.py:17-24` imports the real adapter; `:221-307` asserts `mint.call_count == 1` and exact-byte survival; `:196-219` tests packet shape. Genuinely untested: `--expected-digest` refusal and `main()` |
| C5 | "five validators actively grep-reject that vocabulary" | **Overstated.** Six `!`-negated assertions across rpi ×2, plan ×1, implement ×2, learn ×1 are inert (W3, V5); implement's validator does not grep for commit/push/claim/close/retry at all |
| C6 | v2 writer fix is "transition-gated" | **Sharper and less gated.** It re-snapshots a living intent source (`validate.py:554-558`) — the exact act `SKILL.md:106` forbids — and `validate.py` is **not** bound, so the code fix needs no transition |
| C7 | postmortem/idea-genie witness = the `fm-ws-noncanonical-topdir` detector | **Unrunnable** (W9). Findings stand on ADR-0016 §1; witnesses replaced; the missing detector is its own P1 |
| C8 | framed purely as a layout violation | **Strengthened.** Outside `RUNTIME_EXCLUSIONS`, so a mid-experiment write is loop-visible and forces `FAIL` absent a scope class |
| C9 | idea-genie coverage proven by "the eight bats tests" | The bats file holds 8 tests **total**; only **4** are idea-genie's |
| C10 | "every named test was verified to exist" | True but weaker than it sounds: `check-scenario-coverage.sh:91` matches by `grep -qF` **substring**. **Corrected by X3:** **26 references** resolve to **22 unique** real declarations — duplicates ×4 and ×2 |

**REJECTED (3).** R1 research `schema_version` is already pinned (`findings.json:66-70` `enum: [1]`, required at `:94`) · R2 premortem author/judge distinctness is already enforced in `validate-output.sh` · R3 the "27+17 = 44" correction was itself wrong; W8 confirms 27+16 = **43**.

**SUPERSEDED (3).** S1 plan/implement "no behavioral probe" → re-scoped by C4 to the CLI surfaces only · S2 `test_kernel_v3.py` "same transition discipline" → C3 · S3 "three bound components" → §3: eight skill-owned components across four skills.

**NEW (6).** N1 six inert `!`-negated assertions across four of the eight lifecycle-negative-guard validators, **learn already false-passing** (P0 learn / P1 rpi, plan, implement) · N2 dangling recorder path inside a bound component, **`skills/validate/SKILL.md:119`** (P1, transition-gated) · N3 unverified projected-kernel fallback in a bound component (P1, transition-gated) · N4 `fm-ws-noncanonical-topdir` does not exist, so ADR-0016 §1 is inert prose (P1) · N5 postmortem `output_contract` points at a `.feature` file (P2) · N6 postmortem declares `effects: []` while producing, emitting, and fully specifying `postmortem-report.md` (P2; raised in v1 prose, first ranked in v3).

---

## 7. Re-ranked improvements — 3 P0 · 11 P1 · 14 P2 = 28

### 7.1 P0 — act before integration to main (3 rows)

| Owner | Change | Lethal acceptance witness |
|---|---|---|
| `skills/learn/scripts/validate.sh:6` (+ reword `SKILL.md:35`) | Replace the inert `! grep` with `if grep …; then exit 1; fi` | W2/V4: the validator exits 0 while `grep -nEi 'receipt' skills/learn/SKILL.md` returns line 35 |
| Go-kernel promotion program + `docs/contracts/proof-contracts/` | Treat the seven unpinned governed Python files as a **transition-gated** program: six are bound epoch-1 components (`mint_intent`, `freeze_candidate`, `kernel_v3`, `validate_v3`, `record_proof_transition`, `check_kernel_v3_corpus`); only `test_kernel_v3.py` can move freely | Move any bound file and call `kernel.load_active_proof(repo)` — it raises `TerminalValidation: active proof component bytes or mode changed: <path>`, halting all judgment |
| **`AGENTS.md` + `docs/architecture/operating-loop.md`** *(owner set expanded by X1)* | Rewrite both to v3 vocabulary with a short v2-legacy note. **Both are unbound by the epoch-1 descriptor — a cheap repair surface, not a transition-gated one.** The repair corrects *vocabulary*; any change to what either contract *grants* is a separate, explicitly authorized change | `AGENTS.md:55` says fresh Validate "writes one `verdict.v2`"; `operating-loop.md:12` says "-> durable verdict.v2" (also `:10` `subject-manifest.v1`, `:70` `validate.py`), while the executable loop emits `subject-manifest.v2`, `scope-index.v1`, `check-receipt.v1`, `effect-receipt.v1`, `verdict.v3`, `rpi-report.v2` via `validate_v3.py`. **A docs witness must fail while *either* file still says fresh Validate writes `verdict.v2`** |

### 7.2 P1 — before the next proof epoch or the next release (11 rows)

| Owner | Change | Lethal acceptance witness |
|---|---|---|
| `skills/{rpi,plan,implement}/scripts/validate.sh` | Convert five inert `!` negations to the sound form | Append the forbidden phrase to a scratch copy of the SKILL.md; the validator exits 0 today |
| `skills/validate/scripts/validate.py` (**unbound — no transition**) | Gate the v2 `store-verdict` behind an explicit legacy flag | `validate.py store-verdict --intent-source <living file>` succeeds and snapshots a mutable source, contradicting `SKILL.md:106` |
| **`skills/validate/SKILL.md:119`** (**transition-gated**) | Correct the recorder path to `skills/validate/scripts/record_proof_transition.py` | `test -f scripts/record_proof_transition.py` fails |
| `skills/implement/scripts/freeze_candidate.py:13-30` (**transition-gated**) | Remove or digest-check the projected-kernel fallback | Stage a tree with the direct kernel absent and a divergent projected kernel; the bound component executes unverified code and exits 0 |
| `skills/reality-check/{schemas,scripts}/` (new) | Give `reality-check-report.v1` a schema + validator | `grep -rl reality-check-report schemas/ skills/*/schemas/` returns nothing |
| `skills/council/{schemas,scripts}/` (new) | Give `council-report.v1` a schema + validator | `grep -rl council-report …` returns nothing; a consensus block citing one methodology is accepted |
| `skills/postmortem/SKILL.md:91-92` | Move the artifact convention into the ADR-0016 closed set | `.agents/` holds only `ao/`; a mid-experiment write lands in `undeclared_paths` → `FAIL`. **Witness must separate layout compliance from runtime exclusion** |
| `skills/idea-genie/SKILL.md:98,104` + `tests/scripts/agentops-native-skills.bats` fixtures | Same move for `.agents/ideas/<run-id>/` | Same as above, same separation |
| `cli/` (`ao doctor`) | Build the `fm-ws-noncanonical-topdir` detector (bead `.7`) | `grep -rn fm-ws-noncanonical-topdir cli/ scripts/` returns nothing |
| `skills/plan/tests/` (new, ratchet-exempt) | Cover `--expected-digest` refusal and the CLI `main()` path | No test drives `mint_intent.py`'s `main()` |
| `skills/{learn,premortem,postmortem,council}/SKILL.md` | One coordinated v2→v3 vocabulary pass (**downstream of the P0 root-contract repair**) | Each names `verdict.v2` while validate produces `verdict.v3` |

### 7.3 P2 (14 items)

1. `test_kernel_v3.py` relocation — proof-unblocked; only `validate.sh`'s discover path couples
2. scope output-shape validator
3. scope feature file (none exists)
4. research `effects` declaration vs declared `Write`
5. scenario-coverage tags for learn / research / premortem / postmortem
6. postmortem `output_contract` pointing at a `.feature` file (N5)
7. implement adapter-level probe
8. plan's `produces: scope-index.v1` ownership note
9. implement's unread `consumes: scope-index.v1`
10. shared context-isolation reference for council + idea-genie
11. RPI proof-membership prose *(re-ranked from P1 by X4)*
12. reality-check `scripts/validate.sh` *(rolled in by X5)*
13. council `scripts/validate.sh` + feature file *(rolled in by X5)*
14. postmortem: declare the report-write effect, or make the output response-only *(N6, rolled in by Y1)*

Also standing, unranked: validate's dual-path-grammar note (prefer a comment in the unbound `validate.py`); and the Plan bytecode-hygiene note (§4.2), which is gitignored output, not a contract defect.

---

## 8. Contradictions and open decisions

1. **The canonical contracts are one version behind the executable — and there are two of them.** `AGENTS.md:55` and `operating-loop.md:10,12,64,70` speak v1/v2 while the loop runs the v3 chain and the four core `SKILL.md` files are v3-native. The *invariants* still hold, so this is vocabulary drift, not semantic conflict. But `AGENTS.md` is the repository operating contract, and its own source-precedence rule puts live behavior above narrative docs, so **both** documents are currently outranked by the thing they govern. **The repair corrects vocabulary only.**
2. **The loop's own mechanism is its own ADR-0016 debt, and the two rules collide.** Six files must become Go and leave `skills/*/scripts` (ADR-0016 §3); the same six cannot change bytes without a recorded transition. **Open:** one epoch-2 descriptor carrying all six promotions, or a sequence of narrower epochs?
3. **`skills/validate/SKILL.md` is both a contract and a frozen artifact.** Every wording fix to the most-read contract in the repo costs a transition — including the one-line recorder-path repair at `:119`. **Open:** is `validator-contract` the right binding granularity, or should the bound artifact be a narrower normative extract?
4. **v2 is simultaneously "immutable legacy read format" and a live writer.** **Open:** demote behind a legacy flag, or restate the sentence? The code lever is unbound and therefore cheaper.
5. **ADR-0016's closed set is unenforced.** The named detector does not exist and two skills declare directories outside the set. The ADR's own Amendment warns that an unenforced rule decays into inert prose — that has now happened to §1 as it previously had to §3.
6. **Negative assertions are a systemic fail-open.** Six assertions across **four of the eight lifecycle-negative-guard validators** cannot fail; the correct idiom already exists in the other four. **Open:** fix in place, or add a repo-level check rejecting `! grep` in any `skills/*/scripts/validate.sh`?
7. **`check-scenario-coverage.sh` matches test names by substring** (`grep -qF`) and under `--run` filters bats by regex. **This weakens every coverage claim in the corpus, including the four passing ones — and X3's 26→22 correction does not touch it.**
8. **Should skills disclose active-proof membership at all?** X4 lowers the RPI item to P2 because **no** skill discloses it (0 of 12) and **no** contract requires it. **Open:** adopt a general disclosure rule — promoting this back to P1 across four skills — or leave the descriptor as the sole authority?
9. **Is `metadata.effects` a binding declaration or decorative?** N6 (postmortem) and A19 (research) are the same class. **Open:** enforce a corpus rule tying `Output Specification` artifact declarations to non-empty `effects`, or accept that `effects` describes only loop-visible mutation and document that narrower meaning.
10. **Should contract validators be uniform about bytecode? (new, Y6).** Four of the nine invoke Python with `PYTHONDONTWRITEBYTECODE=1`; two invoke it without; three invoke none. Only Plan's unsuppressed path imports a skill-owned module, and it writes into a **proof-bound component's** `__pycache__`. The output is gitignored and harmless today. **Open:** standardize suppression across all Python-invoking validators, or record deliberately that bytecode caching is out of scope for contract validation?
11. **Deliberate differences preserved.** scope's axiom-kernel · premortem's derivation-diff and defeat attempts · postmortem's three-element causation · council's methodology weighting and echo-consensus guard · idea-genie's sealed duel · research's capability-flag doneness · reality-check's frozen question variants · learn's overweight-failures rule. Nothing here proposes merging any of them into the core.

---

## 9. Source identities

**Landing**

```
worktree : /Users/bo/dev/agentops-worktrees/skill-overhaul
branch   : codex/skill-overhaul-20260724
HEAD     : 0088c6e3824da201eabb1e751ac8e976599e0b5c
tree     : c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
dirty    : 0 paths (verified at open, after every witness, and at close)
```

**Per-skill tracked inventory — 48 files / 7,648 lines (X2)**

| Skill | Files | Lines | Git tree |
|---|---:|---:|---|
| rpi | 5 | 693 | `58efc95769dd6dc70995e94731ee30c30464ce8e` |
| plan | 4 | 190 | `50a478e2b95e109f862f3b9ea6c51d95d8ce1f9f` |
| implement | 4 | 186 | `e6a319a38555ecb78cf890bec2ee8ce5b5e547be` |
| validate | 11 | 5,352 | `23a4303663d2ffd60dc6da3e6d8862c94efc95e8` |
| learn | 3 | 69 | `0463fe95ec7f419f225bd3811706f445e28a898f` |
| scope | 2 | 121 | `03d17ca4c602aa36b8cae8246e296b592000bbc6` |
| reality-check | 1 | 65 | `7ab92b5c7cf61cbe655d5aa0e4abaad2246fe1a7` |
| research | 4 | 266 | `060952784a32cf276e28ca068854a6e8c1eb226d` |
| premortem | 5 | 221 | `d92c541d09ed63b88b4a461d0212b8a09ceb1baa` |
| postmortem | 3 | 141 | `5e677d72e6dabfd6fa424b7ab604e1d0b8c1d14e` |
| council | 1 | 81 | `08202d5d0286d12e34dea189f282deb37ede8c76` |
| idea-genie | 5 | 263 | `c4b22e9fa5a1e4a6472440e135332f4b0be0ad3e` |
| **Total** | **48** | **7,648** | — |

### 9.1 Bytecode hygiene — corrected (Y6, Y8)

**Per-validator facts, verified line by line (U9–U11).** Suppression is a property of each validator's own source; this audit set no caller-level `PYTHONDONTWRITEBYTECODE`.

| Validator | Python invocations | Suppressed | Imports a skill-owned module? | Can write an owned `.pyc`? |
|---|---:|---:|---|---|
| `skills/rpi/scripts/validate.sh:11` | 1 | **1** | yes (test suite) | no — suppressed |
| `skills/implement/scripts/validate.sh:10` | 1 | **1** | yes (`freeze_candidate.py`) | no — suppressed |
| `skills/validate/scripts/validate.sh:12,13,44,45` | 5 | **4** | yes | no — the four owned-module calls are suppressed |
| `skills/validate/scripts/validate.sh:14` | *(the 5th)* | **0** | **no** — stdin heredoc, stdlib + `jsonschema` only | no — nothing owned is imported |
| **`skills/plan/scripts/validate.sh:9`** | 1 | **0** | **yes** — `mint_intent.py:13-16` `importlib`-loads `skills/validate/scripts/kernel_v3.py` | **YES** |
| `skills/research/scripts/validate.sh:21` | 1 | **0** | no — `python3 -m json.tool`, stdlib only | no |
| **`skills/scope/scripts/validate.sh:6` → `heal.sh:46`** | **1 (delegated)** | **0** | **no** — stdin heredoc; stdlib + PyYAML only | **no** — 0 core-owned `.pyc` measured |
| learn · premortem · postmortem | 0 | — | — | no |

**The withdrawn claim (v3).** v3 §9 said "Every executed validator suppresses bytecode." **That is false** and stays withdrawn.

**The corrected replacement census (Z1).** v4 replaced it with a mutually exclusive **4 suppressed / 2 unsuppressed / 3 none** *validator* partition. That partition is also wrong, on two independent counts, and is withdrawn:

1. **It cannot be a validator partition at all,** because `validate` is **mixed** — 4 suppressed calls and 1 unsuppressed heredoc in the same validator. A validator-level bucket cannot hold it.
2. **It omitted Scope's delegated Python.** `skills/scope/scripts/validate.sh:6` runs `bash "$REPO_ROOT/skills/skill-builder/scripts/heal.sh" --check --strict "$SKILL_DIR"`, and `heal.sh:46` is `python3 - "$REPO_ROOT" "${normalized[@]}" <<'PY'` — **no `-B`, no `PYTHONDONTWRITEBYTECODE`**, on the `--check` path Scope actually invokes. The literal token is absent from Scope's own `validate.sh`, which is exactly why a lexical search missed it.

**The unambiguous effective-process census** — counting processes, not validators, and counting delegated execution:

| Class | Validators | Count | Python processes |
|---|---|---:|---:|
| Suppressed only | rpi, implement | 2 | 2 (2 suppressed) |
| **Mixed** suppressed + unsuppressed | validate | 1 | 5 (**4 suppressed, 1 unsuppressed**) |
| Unsuppressed only | plan, **scope — delegated via `heal.sh`**, research | 3 | 3 (3 unsuppressed) |
| No Python | learn, premortem, postmortem | 3 | 0 |
| **Total** | | **9** | **10 — 6 suppressed, 4 unsuppressed** |

**Reachability was checked, not assumed.** `heal.sh` contains a *second* `python3` call at `:95` (`generate-skill-mesh.py`), but it is guarded by `if [[ "$MODE" == fix ]]`. Scope invokes `--check --strict`, so `MODE=check` and `:95` is **unreachable** from this path. It is therefore excluded, which is why the total is **10** processes and not 11. A first derivation at the v5 pass counted it and had to be corrected — the same class of error as the defect being repaired.

**If a lexical census is wanted instead,** it must be labelled as such and never conflated with effective execution: counting only literal `python3` tokens inside each `validate.sh` gives **5 validators with a direct call and 4 without**. That is a statement about *text*, not about *what runs*.

**Isolated capability witness (U8).** `PFX=$(mktemp -d); env -u PYTHONDONTWRITEBYTECODE PYTHONPYCACHEPREFIX="$PFX" bash skills/plan/scripts/validate.sh` → `plan skill contract: PASS`, rc 0, and exactly one skill-owned artifact among 64 emitted:

```
<PFX>/…/skills/validate/scripts/kernel_v3.cpython-314.pyc
```

`git status --porcelain` returned **0** before and after. **This proves capability, not that any audit witness altered the repository cache.**

**Isolated Scope witness (Z1, fresh at the v5 pass).** The same method applied to Scope's delegated path:

```
PFX=$(mktemp -d); env -u PYTHONDONTWRITEBYTECODE PYTHONPYCACHEPREFIX="$PFX" \
  bash skills/scope/scripts/validate.sh

scope validate: PASS
isolated .pyc total : 42
core-12-owned .pyc  : 0
```

The 42 entries are stdlib and PyYAML cache files. **This does not disturb the narrower conclusion that Plan is the only unsuppressed core-validator path shown to import a core-owned module and create a core-owned `.pyc`.** Scope's delegated process is unsuppressed but imports nothing core-owned. The repository's ten-file core cache was re-inventoried before and after and was **unchanged**, and every mtime still predates the audit.

**Repository cache — full inventory (Y8 corrects v3's count).** Within the twelve, **10 `.pyc` files across 5 `__pycache__` directories**, all gitignored and excluded from the hash table below. v3 named two directories / 7 files; the v3 Sol review listed the same 7. The three missed entries are marked ★.

```
2026-07-24 17:28:28  skills/implement/scripts/__pycache__/freeze_candidate.cpython-314.pyc   ★
2026-07-27 22:56:26  skills/plan/scripts/__pycache__/mint_intent.cpython-314.pyc             ★
2026-07-27 22:56:26  skills/rpi/scripts/__pycache__/run_once.cpython-314.pyc
2026-07-27 23:08:30  skills/rpi/tests/__pycache__/test_run_once.cpython-314.pyc              ★
2026-07-24 17:28:28  skills/validate/scripts/__pycache__/check_kernel_v3_corpus.cpython-314.pyc
2026-07-24 18:38:40  skills/validate/scripts/__pycache__/kernel_v3.cpython-314.pyc
2026-07-24 17:28:28  skills/validate/scripts/__pycache__/record_proof_transition.cpython-314.pyc
2026-07-24 17:28:28  skills/validate/scripts/__pycache__/test_kernel_v3.cpython-314.pyc
2026-07-24 15:08:20  skills/validate/scripts/__pycache__/validate.cpython-314.pyc
2026-07-24 17:28:28  skills/validate/scripts/__pycache__/validate_v3.cpython-314.pyc
```

**The conclusion that survives.** Every mtime above predates the 2026-07-28 audit — the newest is `2026-07-27 23:08:30` — and every one was **byte-identical before and after** the U8 witness (U13). So: *the observed repository `.pyc` files predate this audit and were not produced by its witnesses.* That is a **measured** conclusion, not a structural guarantee.

**Doctrine and proof surfaces (SHA-256, recomputed in the v2 pass — V9)**

```
1437d9021bb8ccdb15c47659c9d4eca27553aec6018b368434955a1776fd2bab  AGENTS.md
8c7f36a1c9b6c37a938f8d7ab2e01a59dbb1da978a9a3a3805e884d2b357bc5c  docs/architecture/operating-loop.md
14dba837f6e5946b9f02a2a60bb409c682c35b865f53602199092723ba974aec  docs/adr/ADR-0016-state-tiers.md
25bc0adcf9ab9d64a088a0580b8693a3721f8a363d758d3bcb748f511892a1f3  docs/contracts/proof-contracts/active.json
f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340  docs/evidence/proof-epochs/epoch-1/subject-refreeze-candidate-descriptor.json
859e691122caf6b260d922ec36f66d1c627feb9fab4d33b2dbb78d5a1c9fd7f3  scripts/.skill-python-grandfather
```

**Whole-file SHA-256 — all 48 owned files** (unchanged across **v1 → v7**; every row independently re-confirmed by **all six prior Sol reviews**, each of which reports checking the full 48-row set — caption widened at BB3 and extended to the v6 review at CC5)

| Prior Sol review | Reported whole-file check |
|---|---|
| v1 review (`0480f406…`, 486) | all 48 reported SHA-256 values match |
| v2 review (`0dbabf87…`, 313) | all 48 whole-file rows reproduce |
| v3 review (`21c14eee…`, 232) | all 48 report SHA rows reproduce |
| v4 review (`3f0aa75e…`, 373) | all 48 rows pass |
| v5 review (`b9dbc0b6…`, 423) | all 48 rows match live bytes |
| v6 review (`11115120…`, 377) | all 48 rows reproduce against live bytes |

The v7 Sol review (`17ea51b4…`, 387) likewise reports "all 48 audit SHA-256 rows match live bytes;
path sets are identical." **Six prior reviews plus the v7 review therefore cover the full 48-row set
seven times over**; the pre-BB3 caption understated it, and the pre-CC5 caption stopped one review
short.

```
d6ee5d652ad56a9e7a47dc3544698e22f2c7cc2ff81ce5cd99a3786ab227f0f4  skills/council/SKILL.md
a19e458f9563170b32f8ddc3a9a941e4568725b89e87e7670dac800be06ad196  skills/idea-genie/SKILL.md
eff400dca97610d45e198bbc17c506787c17df33fd28343fa6257fc7b23823f7  skills/idea-genie/references/idea-challenge.feature
35735854034bedc1e34fdb1f993d594d9c8932090edee8071fda28d3971c6dac  skills/idea-genie/references/idea-genie.feature
0acb03d48d0cee648ffbbb702d490dcb90807b089372b8626f62a78fe8587716  skills/idea-genie/scripts/validate-challenge.sh
22f868af8d38500539296f048aad50c068fea21a938e7155a09d1fe3ea73afb9  skills/idea-genie/scripts/validate-output.sh
bcbb2156409af426769e77dc90aaeed43371dfd54f4ae886caac2b8f49359e8b  skills/implement/SKILL.md
90a737a9aaba69ee32397c07a5a023ba7b4dab96bc515879f72a2ad087e6eff1  skills/implement/references/implement.feature
51988cc25b183a365e5c5c9fcbe66df9014a08760f4a8187d7e3d41b25c99dc1  skills/implement/scripts/freeze_candidate.py
9c42441842179430c24ad8944b4a566a7d801fc092f8c2fdffd66ef1ee59cea1  skills/implement/scripts/validate.sh
dca69a9c7c0bb3959232be7abb2338deb8f52e0820f17ba6612a4b78e5ac4baa  skills/learn/SKILL.md
9f39c2737512e99736cae0af88a63c54f34ddab41ae91d7ced647e7f133fa7c2  skills/learn/references/learn.feature
7e81f71d079c84f95386d5254eba19ad3f118cf1ff123568d9f9550139234925  skills/learn/scripts/validate.sh
c1808f7d874e85a0a7923f83c38254dab874a27dc0bbf600095316b3c26f2431  skills/plan/SKILL.md
f52ca8501a53933077c6e779c49e402422cc87468f7b210cfcc4068bd208d23b  skills/plan/references/plan.feature
6bb36c50ad87f4ae0eccee8c6a40256ef37429c4452a13cd7d41a0b0d56b11bc  skills/plan/scripts/mint_intent.py
457be15c622db62e1a0c80eadfff9dee47ed93ccc3320bf7b4e411d8aee42888  skills/plan/scripts/validate.sh
8012357dab12d6ee45dbde163098c6f58fbbcd082e000e0641fe6e254ba89ec9  skills/postmortem/SKILL.md
3f7f218f6ef8754a206adc10ba329adff6d019328ce56724b9954feda03587b6  skills/postmortem/references/postmortem.feature
668bbe7b72d2c94eaffecec6546b5da0a8b2c6785fd46743be71e8ded4568e61  skills/postmortem/scripts/validate.sh
9450d2775f3d15a211decb0ebfd4f2a1b6180b8b400740239304568ff5bd7fdc  skills/premortem/SKILL.md
ed54ec3f6135bfc2f27f306eb3729b98e0bf64ad727b9a544550023a0010497c  skills/premortem/references/premortem.feature
a8de64b7d7654d2f73ad45943483b27ff582911edd6c8754a5fe76b0e4306651  skills/premortem/schemas/premortem-plan-review.v1.schema.json
7c59b3a35b60490e77d49a14877aba922176b6f746e386d1c1755d3e954f375d  skills/premortem/scripts/validate-output.sh
d61331f97014c0a97269ddbedf48eec44d3befa37dcb8f03f2406007ff84672e  skills/premortem/scripts/validate.sh
cf49935936a78a1eecf3610386f22a79f74d14a4678a03562802ce096a23f13a  skills/reality-check/SKILL.md
4c6e53a12dad694130179e673df544eec1de37994a430543ade95f2582e5594b  skills/research/SKILL.md
51844890fbf04fc849dba643065b13b0ecd72be563f8a3b338621ab1047513ae  skills/research/references/research.feature
fdda96ebf5ea0e412d8c58f6937abe1cba629590bc8ac0b39c531936f68b75e7  skills/research/schemas/findings.json
6e58f100379f98672bb233f8c3dfef1aef3c616905d56bff92da0d73c39b386e  skills/research/scripts/validate.sh
ec58369b6fcf525ef0fd0c5d25cb2a596642daac61ef9891944ec75195f6909b  skills/rpi/SKILL.md
76ef7786081067a2f4833cf725089f18fa85b026170f1a99f10d3d76d1225f6d  skills/rpi/references/rpi.feature
57fb4e491216adc75e15fabc3b117d498b1ff4601edb5f2773e619ce82f253be  skills/rpi/scripts/run_once.py
ecb6d1cb426fbfb735c784aac1f7cb26f4fbac40200b79f7212842ff7215fee0  skills/rpi/scripts/validate.sh
aadbb9dc229be51dbb6a9576ebefe1a9af2c0faa34a9d79d8b29cec5e4634b9a  skills/rpi/tests/test_run_once.py
ff943beb2b55b9a0116759c739916fa540bf5eade66fe952a015479a11e527e0  skills/scope/SKILL.md
6b63e81b02972cdc1391a5195b229ab81b5eb5fa62585299a7e57b785d327991  skills/scope/scripts/validate.sh
8ecc121b6d7a63946a903d062f6913a8e32663ecb1418f193002490946e9158d  skills/validate/SKILL.md
bc9ab49fb81c4cd8c94313307da59c35df4eb12691c0da5a5e36cb16d140f127  skills/validate/references/validate.feature
b173fab958006f38d56950e72ac571d18eda2bbd085d92a665acc9feaa3083ce  skills/validate/scripts/check_contract_corpus.py
173e862c1a65e5ff2b0a6b3ba36d7eb21a53e3bc2ef9fc0990ae11adb5dc9a73  skills/validate/scripts/check_kernel_v3_corpus.py
f7787f4505c6f49c77890411a49387a02beec7a267595e158af6e4184ca6ef70  skills/validate/scripts/kernel_v3.py
49f22c1af70f8a0de09c44bd132e74d22f053d0b9a5f353f98bdc8082c0c5e58  skills/validate/scripts/record_proof_transition.py
a9642b27a4ae4da474a874a93a5faba403bf7d6299b85c82c96efe3a8104ff4e  skills/validate/scripts/test_kernel_v3.py
8e1b289cba8664bbd1662bbc41fb8bfe0b1741559b487a745535f4796cc8ac17  skills/validate/scripts/test_validate.py
adafe4e127f9ddeb534c633a2cb4c2321b47e8a6a6f3656a0872076427f72247  skills/validate/scripts/validate.py
550f89058d490e3c14e8c0ce25eb70a5cdc2326ccc9c170658238bc9fc8a4db7  skills/validate/scripts/validate.sh
055c0f4b9afe9934dbd8948115323a5ff2706aae809888e75efeacb5fdff0841  skills/validate/scripts/validate_v3.py
```

---

## 10. Checked

- **All 48 owned files, line by line, in full (7,648 lines):** every `SKILL.md`, every `references/*.feature`, every `schemas/*.json`, every `scripts/*.sh` and `scripts/*.py`, both test suites under `skills/validate/scripts/`, and `skills/rpi/tests/test_run_once.py`. `kernel_v3.py` (2,030) and `test_kernel_v3.py` (1,213) each read across contiguous ranges covering every line.
- **Doctrine, in full:** `AGENTS.md`, `docs/architecture/operating-loop.md`, `docs/adr/ADR-0016-state-tiers.md`, `scripts/.skill-python-grandfather`, `scripts/check-scenario-coverage.sh`.
- **Proof surfaces:** `active.json`, the epoch-0b descriptor, the active epoch-1 descriptor (25 components; 8 skill-owned, 8 unique), and a re-hash of every bound component.
- **All fourteen prior audit/review artifacts in full (CC3):** v1 (666), first Sol review (486), v2 (571), second Sol review (313), v3 (604), third Sol review (232), v4 (668), fourth Sol review (373), v5 (713), fifth Sol review (423), **v6 (725), sixth Sol review (377), v7 (757), seventh Sol review (387)** — every declared digest independently reproduced. The first ten of these are the **ten pre-v6 lineage artifacts**; v5 omitted v4 and the v4 review from that subset and they were restored at AA3. The v7 Sol review independently reproduced all twelve v1–v6 identities and line counts.
- **Executed:** W1–W9, V1–V10, U1–U7, and **U8–U14 at the v4 pass** — the isolated bytecode-capability witness with an external cache prefix, the nine-validator Python/suppression census, the `validate.sh:14` heredoc import analysis, the `research:21` stdlib-only confirmation, the `SKILL.md:119` anchor, the full 10-file `.pyc` inventory before and after the witness, and the `loop_context.go` absence check.
- **Bytecode provenance, specifically:** which validators suppress and which do not (§9.1), that only Plan's unsuppressed path imports a skill-owned module, that the witness artifact was `kernel_v3.cpython-314.pyc` and nothing else skill-owned, and that all ten repository `.pyc` mtimes are unchanged and predate the audit.
- **`skills/skill-builder/scripts/heal.sh` source, read at the v5 pass (Z1):** the `--check`-path heredoc at `:46`, its `set +e` guard at `:45`, and the `if [[ "$MODE" == fix ]]` gate at `:93-98` that makes the second `python3` call at `:95` unreachable from `--check`.
- **Fresh isolated Scope bytecode witness (Z1):** `scope validate: PASS`, 42 isolated `.pyc`, **0** core-owned; repository ten-file core cache re-inventoried unchanged before and after.
- Landing identity and cleanliness verified at open, after every witness, and at close.
- **This v8 pass specifically:** the **v7 audit** (`939ff8e5…7317a`, 757 lines) and the **v7 Sol review** (`17ea51b4…65ba5`, 387 lines) read in full and SHA-verified at open; the repository HEAD `0088c6e3…`, tree `c0c43eef…`, branch, and clean status re-verified at open and again at close; every numeric lineage, correction-count, round-count, version-count, and predecessor-count claim in this document swept and reconciled (CC5); the 48 whole-file hash rows, all twelve Git-tree rows, and all twelve one-by-one skill sections carried forward **byte-for-byte from v7** and verified identical after transcription.

## 11. Not checked

- **Go-side internals.** `cli/internal/verdictcheck/*_test.go` and the Go readers bound as epoch-1 components were verified to exist and to be required by `check_kernel_v3_corpus.py`; their bodies were not read. All six prior Sol reviews report the Go package tests passing, and the v7 Sol review independently reran the focused Go verdict checker uncached; **that result is carried from those reviews, not reproduced here.**
- **`scripts/check-skill-python-ratchet.sh`, `scripts/check-proof-contract.py`, `scripts/bootstrap-proof-transition{,-v2}.py`.** Existence, wiring, and role verified from ADR-0016, the descriptor, and the validators that invoke them; source not read. **`skills/skill-builder/scripts/heal.sh` is no longer in this list — its source was read at the v5 pass** for the delegated Scope path (`:40-52` heredoc, `:88-100` mode gating); see §9.1/Z1. **No Sol review's ratchet- or proof-checker run was reproduced here.**
- **`tests/fixtures/rpi-kernel-v3/corpus.json` contents** (43 cases confirmed by count and required-ID set; individual payloads not read), `tests/fixtures/verdict-contract/cases`, and the 23-case legacy v2 contract corpus the third Sol review reports.
- **`tests/scripts/agentops-native-skills.bats`** — read in a prior pass at this same HEAD, four idea-genie tests verified by name; not re-read line by line here.
- **Whether any *historical* `.pyc` in the repository was produced by Plan's unsuppressed path.** U8 proves the path is capable; it does not attribute any specific existing cache file to it. `plan/scripts/__pycache__/mint_intent.cpython-314.pyc` and `rpi/tests/__pycache__/test_run_once.cpython-314.pyc` share the 2026-07-27 22:56–23:08 window, which is consistent with an earlier unsuppressed run but **is not proof of one** — mtime correlation is not causation, and no such attribution is claimed.
- **The other ~37 skills** of the 49-skill canonical corpus. Overlap claims are bounded to these twelve plus the siblings their frontmatter names.
- **Generated projections** (`skills-codex/**`, `images/**`, routers, catalogs) — deliberately unconsulted for intent. The first Sol review's `DRIFT: images/gemini/skills/skill-builder` observation is carried as residual risk, **not reproduced here**.
- **Non-Darwin platforms;** fsync durability and `fcntl` locking under other operating systems.
- **Whether the proposed changes would pass this repo's full CI.** No gate beyond W1–W6 was run; no P0/P1/P2 was implemented or simulated.
- **Git history / provenance** of the twelve skills. Every claim is about the tree at `0088c6e3`.
- **All six prior Sol reviews' mutation witnesses** (archived false-pass injections, bound-mode change, divergent projected-kernel execution, conditional `.agents/` effect receipts) were **not re-executed** here; the underlying static facts each depends on were independently re-verified.
- **The v8 pass did not re-execute the witness suite.** W1–W9, V1–V10, U1–U14, both isolated bytecode witnesses, the proof re-hash, the ratchet, the scenario matrix, and every test suite are **carried from the v7 audit**, where they were executed, and were **independently replayed by the v7 Sol review** (13 RPI / 27 v3 / 16 v2 / 43 shared / 23 legacy tests, Go verdict checker uncached, 8 native-skills Bats, proof checker at epoch 1 with 25 components, ratchet at 24 pins). v8 re-derived no executable result of its own; its corrections are confined to lineage arithmetic and provenance scope, and it changed no ranked finding.
- **No AgentOps semantic validation was performed and no verdict was minted; no PASS is claimed.** Nine `validate.sh` contract scripts plus one isolated capability witness were executed as read-only evidence gathering; that is script execution, not semantic acceptance. No `verdict.v3`, transition, source edit, generation, commit, merge, push, or release action was performed.

## 12. Residual risk

1. **The corrections do not change any mechanism.** All 30 confirmed findings, 10 corrections, 3 rejections, 3 supersessions, and 6 new findings survive intact. X1–X5 and Y1–Y8 correct owner scope, arithmetic, wording, provenance, anchors, and rank — not substance; Z1–Z3, AA1–AA3, BB1–BB4, and CC1–CC5 likewise correct census, provenance, cardinality, and lineage arithmetic only. **Twenty-eight corrections across seven authoring rounds have changed no mechanism, no severity, and no ranked total since v3.**
2. **The learn false pass is live right now.** Until it is fixed, **one of the eight lifecycle-negative-guard validators** reports PASS on bytes it exists to reject.
3. **Bytecode hygiene is measured, not guaranteed (Y6, refined by Z1).** **Exactly one** of the four unsuppressed processes is reached by *delegation* — Scope's, through `heal.sh:46`. The other three are **direct**: Validate's heredoc (`validate.sh:14`, direct within its mixed validator path), Plan's `mint_intent.py --help` (`validate.sh:9`), and Research's `python3 -m json.tool` (`validate.sh:21`). v5's "two of the four" is corrected (AA2). The lesson is unchanged: a census must follow calls, not tokens. The repository cache is shown untouched by mtime comparison before and after every witness — not by construction. **Plan's contract validator remains capable of writing an owned `.pyc` into a proof-bound component's `__pycache__` directory** whenever it runs on a cold cache without suppression. The artifact is gitignored and does not dirty the tree or alter any bound component's *tracked* bytes, but any future audit that asserts cache immutability must re-measure rather than assume.
4. **The `check-scenario-coverage.sh` substring weakness is untouched by X3.**
5. **A complete Go promotion must coordinate the six bound production moves with a valid next proof epoch.** All correction passes proved the fail-closed gate statically; no transition publication was simulated.
6. **Postmortem and Idea Genie cause undeclared effects only if their artifact writes fall between the frozen manifests.** Their paths violate ADR-0016's closed layout regardless — and moving them under `scratch/` repairs the layout without removing mid-experiment visibility.
7. **The projected-kernel fallback risk is divergence/tampering, not a known current byte mismatch** — current direct and projected kernels are identical at `f7787f45…6ef70`.
8. **Repository-wide generated-image currency is not claimed.**
9. **`effects` semantics remain undecided** (open decision 9). N6 and A19 are ranked P2 on the assumption that `effects` is a binding declaration; if the repository scopes `effects` to loop-visible mutation only, both become documentation clarifications, and that decision should be recorded before either is implemented.

---

## 13. Seal

This is an immutable correction artifact, written exactly once to `/tmp/agentops-opus5-verified-skill-audit-core-12-v8.md`. It supersedes `/tmp/agentops-opus5-verified-skill-audit-core-12-v7.md` (`939ff8e5…7317a`, 757 lines), which is preserved unchanged, and adopts the five lineage/cardinality corrections required by `/tmp/agentops-opus5-verified-skill-audit-core-12-v7-review-sol.md` (`17ea51b4…65ba5`, 387 lines), likewise preserved unchanged.

**Chain cardinality, stated with explicit scope (CC2).** The chain named in v7 — **v7 → v6 → v5 → v4 → v3 → v2 → v1** — is **seven audits and six prior Sol reviews**. v7 displayed those seven versions while calling them "six audits and six Sol reviews," which is the arithmetic the v7 Sol review rejected. Including this artifact, the complete chain is **v8 → v7 → v6 → v5 → v4 → v3 → v2 → v1**: **eight audits and seven prior Sol reviews**, all preserved unchanged.

**v7 was itself authored as the correction of v6** (`2d140dfe…bf7e`, 725 lines) **and the v6 Sol review** (`11115120…f27d`, 377 lines) — both preserved unchanged. **v6 was authored as the correction of v5** (`78d7ceb5…001c`, 713 lines) **and the v5 Sol review** (`b9dbc0b6…ab72`, 423 lines) — both preserved unchanged. **v5 was authored as the correction of v4** (`433a2c3e…1686`, 668 lines) **and the v4 Sol review** (`3f0aa75e…dc8e`, 373 lines) — both preserved unchanged. v3 (`4278e7d9…c9b8`) and its review (`21c14eee…f11e`) are preserved unchanged. v1 (`ddd048bd…`), the first Sol review (`0480f406…`), v2 (`b7a1fe44…`), and the second Sol review (`0dbabf87…`) are all preserved unchanged.

**Whole-file numeric sweep (CC5).** Every lineage, correction-count, round-count, version-count, and predecessor-count claim in this document was swept and reconciled: the ledger is **23 across six rounds through v7**, **28 across seven rounds including v8**; the chain is **seven audits / six prior reviews through v7**, **eight audits / seven prior reviews including v8**; the predecessor set is **ten pre-v6 lineage artifacts** within **fourteen prior audit/review artifacts**; the whole-file caption spans **v1 → v7** over **six prior Sol reviews**; and the severity table runs **v2 → v8** at a constant **3 / 11 / 14 = 28**.

**Two task-instruction paths were not adopted as written** because live source contradicts them — the Plan validator's line 9 invokes `mint_intent.py --help`, not `kernel_v3.py --check`, and `cli/cmd/ao/loop_context.go` does not exist in this tree. Both substantive corrections were applied against the real citations (§0).

**Closing landing identity — re-read immediately before sealing:**

```
worktree : /Users/bo/dev/agentops-worktrees/skill-overhaul
branch   : codex/skill-overhaul-20260724
HEAD     : 0088c6e3824da201eabb1e751ac8e976599e0b5c
tree     : c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
status   : clean (0 paths)
```

Opening and closing identity are byte-identical, and the ten repository `.pyc` mtimes are unchanged. No repository file was edited, generated, committed, merged, or pushed by this pass (v8), nor by any prior pass in this lineage. **No AgentOps semantic validation/verdict was performed and no PASS is claimed** — nine `validate.sh` contract scripts and one isolated capability witness were executed as read-only evidence, which is script execution, not semantic acceptance.

The whole-file SHA-256 and line count are computed after sealing and reported out of band, because embedding a file's own digest would change the bytes being identified.
