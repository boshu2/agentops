# AgentOps — Corrected Deep Audit v8: Twelve Workflow/Support Skills

**Subject:** `/Users/bo/dev/agentops-worktrees/skill-overhaul`
**Branch:** `codex/skill-overhaul-20260724`
**HEAD (open and close):** `0088c6e3824da201eabb1e751ac8e976599e0b5c`
**Tree (open and close):** `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`
**Status:** clean — `git status --porcelain=v1` returned 0 lines at open and at close.
**Mode:** read-only. No repository file edited, no generation, no projection, no commit/merge/push/tag, **no productive or destructive skill executed**. **No PASS claimed.**
**Date:** 2026-07-28

**Skills (12):** bootstrap · codebase-recon · handoff · refactor · reverse-engineer · sbh · scaffold · shared · status · test · using-gc · workflow-builder

## Inputs, SHA-verified at open

| Input | SHA-256 | Verified |
|---|---|---|
| v6 audit | `0515700b6afda3fbad71b00ed971bce9357a5fcee94c488e80070c935cee61c9` | **match**; read in full (670 lines) |
| v6 Sol review | `a29ff4a4f1522705ff6b1c5a80f6251b4e9d98750ecf671018fd8f280734e78c` | **match**; read in full (552 lines) |
| **v7 audit** | `ec07309d243bf3e86319dc07d22da70eda2bb93a21c0f6732868fd80a200dca8` | **match**; 725 lines, 83,765 bytes; read in full |
| **v7 Sol review** | `9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9` | **match**; 438 lines, 21,232 bytes; read in full |

All four preserved unchanged. **v8 supersedes v7** as the corrected record. v3–v6 and their reviews remain preserved at their prior digests.

---

## 0A. v7 → v8 correction ledger

Sol's v7 review returned **REQUEST_CHANGES** on **three audit-document defects**, explicitly stating they "are audit-document defects, not newly discovered repository-skill findings" and that "the finding and severity totals do not change." All three are corrected below, each **independently re-derived from live source in this pass**.

| # | Sol v7 blocker | v7 said | v8 says | Basis re-derived this pass |
|---|---|---|---|---|
| **W1** | Witness E is not reconciled across all 27 rows | §3.8's Witness-E table classified only rows 25–27 and labelled rows 1–24 "**Not reached** / not applicable" | **§3.8's Witness-E table is rebuilt across all 27 rows.** Rows 1–24 executed *before* the row-26 failure and are directly classifiable, so the "not reached" phrasing is **withdrawn as false**. Exact state: present 1–8, 11, 13, 25; absent 9, 10, 12, 14–24; failed 26; not applicable 27. Arithmetic **11 + 14 + 1 + 1 = 27** | `reverse_engineer.py:1840-1875` read verbatim: `_ensure_dirs([council_dir])` at `:1846` precedes the row-25 renders at `:1850-1851`, and the row-26 `write_text` at `:1855` is the *last* statement before `return 0` at `:1873` |
| **W2** | Row 26's output condition is not exact | Row 26 listed only "the destination **does not already exist** (`:1854`)" as its condition | **Row 26 gains an internal three-way outcome table**, the same shape rows 18 and 24 already use. The parent-directory precondition is now explicit: destination absent **and** `.agents/learnings/` absent raises `FileNotFoundError` and produces **no** learning file | `:1853` builds the path, `:1854` is the `not …exists()` guard, `:1855` is the `write_text` that raises. **`.agents/learnings` appears exactly once in the whole script — at `:1853` — and is never created** |
| **W3** | The effects vocabulary-gap account omits handoff's temp-file lifecycle | §7.3's deletion and temporary-file-lifecycle rows named only `sbh` and `reverse-engineer` | **`handoff` is added to both §7.3 rows** with its exact conditional cleanup lifecycle, and §7.2 and §9.3 carry the same statement. **H6 remains the existing owner**; no finding is added and no severity total changes | `cli/cmd/ao/handoff.go:131` `os.CreateTemp(dir, ".handoff-*.tmp")`; `:135` `tmpName := tmp.Name()`; `:136` `defer func() { _ = os.Remove(tmpName) }()`; `:148` `os.Rename(tmpName, target)` |

**No finding changes.** The **13 P0 / 25 P1 / 34 P2 = 72** ledger, all twelve one-by-one intent and RPI-fit judgments, the exact 70-path owned inventory, the six C1 locations and C2/C3 taxonomy, the two effects rails, the seven split repair units, and every per-skill severity and repair are **preserved exactly** as Sol validated them at v7. Sol's own v7 review states the arithmetic "remains valid under v7's stated repair-unit convention" after these three corrections.

**One live fact sharpens W1 and W2 beyond what the review states.** `.agents/learnings/` is **absent at the bound tree** — `ls -d .agents/learnings` returns nothing at `0088c6e3…`. Witness E's precondition is therefore not an exotic setup; it is the **default state of a fresh checkout**. Any run of `reverse_engineer.py` from a repository root that has never had a learning written will take the `FileNotFoundError` branch. This strengthens **RE2** without changing its severity.

**RE8 stays blocked.** §3 is now exact on every witness and every row condition, but the matrix has been found incomplete or inexact in **four** consecutive revisions by four different mechanisms (§11.5). Publication must follow an independent review of *this* matrix, not this document's own assessment of it.

---

## 0. v6 → v7 correction ledger *(preserved)*

Sol's v6 review returned **REQUEST_CHANGES** on five audit-document defects, explicitly stating they "do not add or remove a repository skill finding, so the supported 72-row skill ledger remains unchanged." All five are corrected below, each **independently re-derived from source in this pass**.

| # | Sol blocker | v6 said | v7 says | Basis |
|---|---|---|---|---|
| **H1** | The matrix omits durable producers it claims to cover | §3.0 expressly included class-A reserved-lane writes in scope; §9.5 called the 25 rows "the complete durable inventory"; neither the two `.agents/council` writes nor the `.agents/learnings` write is a row | **Two new rows in a new §3.6.** Row **25** groups the vibe + postmortem council writes (`:1845-1851`, identical owner and condition). Row **26** is the conditional learning write (`:1853-1871`). **Matrix is now 27 rows**; old row 25 (Phase 2) becomes **27** | source read, §0.2 |
| **H2** | The runtime-witness reconciliation is not exact | Every witness omitted model-produced row 25 from both columns; the both-mode row named only 18 and 22–24 as absent and described the state as "the repo set and the binary set combined"; the repo row named row 19 twice | **§3.8 rebuilt as explicit per-witness tables covering all 27 rows each**, with an explicit `not applicable — model-produced Phase 2` classification for row 27. Both-mode translated exactly from Sol's numbering. The union phrasing and the duplicate row-19 mention are removed | Sol's exact witness, §0.3 |
| **H3** | Row 18 omits its no-archive durable branch | "Extracted archive tree under `<--local-clone-dir>/extracted/` (+ `PRIMARY.txt`)", outer condition only | **Row 18 gains an internal outcome table** (same shape as row 24): no openable ZIP candidate → `extract.NOOP.md` and immediate return, **no `PRIMARY.txt`, no `analysis_root` redirect**; candidate found → `zip@<offset>/…`, `manifest.json`, `PRIMARY.txt` | source read, §0.4 |
| **H4** | The row-6/row-20 pipeline fault examples are each insufficient | "helper renamed **or** `<tmp>/binary/` unwritable" as two standalone examples | Both standalone examples **removed**. The direct empty-evidence seam is retained as the valid test. The pipeline injection is restated as a **conjunction** that provably suppresses both evidence producers without aborting earlier | source read, §0.5 |
| **H5** | The scratch-by-class summary omits security-validation scratch | §0.3 covered clone/tmp, sitemap, CLI-help, binary analysis, archive index | **Security-validation scratch class added**: `validate_security_audit.sh:55-61` → `scan_secrets.sh:31-32` `mktemp -t re_rpi_secrets.XXXXXX` with EXIT-trap deletion. Still outside the durable producer rows | source read |
| **H6** | *(Sol, minor)* | Row 21 cited "write `:1400`" | `:1400` **binds** the path; `write_text` is at **`:1401`**. Corrected; the output identity `comparison-report.md` is unchanged | source read |

**No finding changes.** The 13 P0 / 25 P1 / 34 P2 = 72 ledger, all twelve intent and RPI-fit assessments, and every per-skill severity and repair are preserved exactly as Sol validated them. **C1/C2 membership and the RE1/RE2/RE4 interpretations are preserved unchanged** — promoting the class-A writes to matrix rows records *who produces them*; it does not soften that they are undeclared, land outside `--output-dir`, and occupy reserved judgment and Learn lanes.

**RE8 stays blocked.** §3 is materially more accurate than v6's, but the matrix has now been found incomplete in **three** consecutive revisions by three different mechanisms (§11.5). Publication must follow an independent review of *this* matrix, not this document's own assessment of it.

### 0.1 Row layout and renumbering map

Class-A writes execute at `:1845-1871`, after every other script producer including the security block, and before `return 0` at `:1873`. Model-produced Phase 2 stays last. New §3.6 therefore sits between security and Phase 2.

| v6 row | v7 row | Output |
|---|---|---|
| 1–24 | **1–24 unchanged** | unconditional, repository, binary, both-mode, security |
| — | **25 (new)** | `.agents/council/<date>-vibe-<slug>.md` + `<date>-postmortem-<slug>.md` |
| — | **26 (new)** | `.agents/learnings/<date>-<slug>-reverse-engineer.md` *(conditional)* |
| 25 | **27** | Phase-2 `steal-map.md` *(model-produced)* |

Section numbers shift: v6 §3.6 (Phase 2) → **§3.7**; v6 §3.7 (reconciliation) → **§3.8**; v6 §3.8 (clone-metadata) → **§3.9**. Every total and reference reconciled: §0, §3.0, §3.8, §9.5, §10, §11.4.

### 0.2 H1 re-derived from source

```python
    # 9) Reports (vibe-style + postmortem) + learning.                    # :1844
    council_dir = REPO_ROOT / ".agents" / "council"                        # :1845
    _ensure_dirs([council_dir])                                           # :1846   <- council only
    vibe_path = council_dir / f"{_today_ymd()}-vibe-{product_slug}.md"    # :1847
    post_path = council_dir / f"{_today_ymd()}-postmortem-{product_slug}.md"  # :1848

    _render_template(TEMPLATES_DIR / "vibe-report.md.tmpl", vibe_path, {...})      # :1850  -> row 25
    _render_template(TEMPLATES_DIR / "postmortem.md.tmpl", post_path, {...})       # :1851  -> row 25

    learning_path = REPO_ROOT / ".agents" / "learnings" / f"{_today_ymd()}-{product_slug}-reverse-engineer.md"  # :1853
    if not learning_path.exists():                                        # :1854   -> row 26 guard
        learning_path.write_text(                                         # :1855-1871
            ... canned, run-independent content ...
        )

    return 0                                                              # :1873
```

**Row 25 groups the two council writes** because owner and condition are identical — the same rule that groups row 5's four template renders and row 8's two inline statements. Both are `_render_template` calls, both unconditional at this point in `main()`, both under `council_dir`.

**Row 26 is separate** because its condition differs: it fires only when the destination does not already exist (`:1854`). It is also a different owner — an inline `write_text`, not `_render_template`.

**`REPO_ROOT = Path.cwd()`**, so both rows land wherever the caller stood, **outside `--output-dir`** — class A in §4. `_ensure_dirs` at `:1846` creates `council_dir` **only**, which is exactly **RE2**: on a cwd without `.agents/learnings/`, row 25 succeeds and row 26 raises `FileNotFoundError`.

**Preserved unchanged:** RE1 (undeclared reserved-lane writes, P0, C1+C2), RE2 (the crash, P0), RE4 (canned run-independent learning content in the Learn corpus, P0, C2), the C1 membership of `.agents/council` and `.agents/learnings`, and their C2 co-membership. Sol reproduced all three files outside `--output-dir` on a fresh both-mode run.

### 0.3 H2 — the exact both-mode translation

Sol's fresh both-mode witness, stated in **v6 numbering**:

```text
Present : 1–5, 7–9, 11, 13–17, 20, 21
Absent  : 6, 10, 12, 18, 19, 22–24
Not produced by script-only Phase 1 : 25
```

**Translated to v7 numbering** — rows 1–24 unchanged, old 25 → new 27, plus the two new class-A rows, which Sol observed present on that same run:

```text
Present        : 1–5, 7–9, 11, 13–17, 20, 21, 25, 26
Absent         : 6, 10, 12, 18, 19, 22–24
Not applicable : 27   (model-produced Phase 2)
```

Row arithmetic: 16 + 2 present, 8 absent, 1 not applicable = **27**. Sol's discriminators were direct: row 6 append no, row 10 no, row 12 no, row 19 no, row 20 yes, row 21 yes.

**Two v6 phrasings withdrawn.** "The repo set and the binary set combined" is not conditionally valid — row 20 sets `wrote_cli = True`, so the repo witness's row-6 append does **not** survive in both mode. And the repo reconciliation named row 19 twice (once inside the `14–20` range and again separately); the duplicate is removed.

Full four-table reconciliation in §3.8.

### 0.4 H3 re-derived from source

`extract_embedded_archives.py`, two mutually exclusive outcomes:

| Branch | Lines | Durable outputs |
|---|---|---|
| **No openable ZIP candidate** | `if not cands:` `:79`; write `:80-83`; `return 0` `:84` | `<extract_root>/extract.NOOP.md` only. **No `PRIMARY.txt`**, so the caller's `if primary.exists():` at `:1680` is false and **`analysis_root` is not redirected** |
| **Candidate found** | `best = sorted(...)` `:86`; `dest` `:87-88`; `extractall` `:92`; manifest `:96-106`; pointer `:109` | `<extract_root>/zip@<offset>/…` (extracted tree), `zip@<offset>/manifest.json`, and `<extract_root>/PRIMARY.txt` — the last **redirects `analysis_root`** at `:1681` |

Sol's fresh bounded helper probe against `/usr/bin/true` produced exactly `extract.NOOP.md` and no `PRIMARY.txt`. Row 18 keeps one invocation row (the helper is invoked once, `check=True`) with an internal outcome table, the same shape as row 24.

### 0.5 H4 — the corrected negative-branch prescription

Three source facts, all re-read this pass:

| Fact | Site | Consequence |
|---|---|---|
| `capture_cli_help.sh` creates `cli-help-tree.txt` before touching the target | `capture_cli_help.sh:45` | on a normal run the help tree always exists |
| The helper is invoked **only if present**, `check=False` | `:1634` `capture_script.exists()`, `:1636-1639` | removing it skips the call and does not abort |
| `strings.head.txt` is written **inside** a `command -v strings` guard | `analyze_binary.sh:169-171` | with `strings` absent the redirection never runs, so the file is **not** created — and the script still exits 0, satisfying `check=True` |

**Why each of v6's standalone examples fails, confirming Sol:**

- *Rename the capture helper alone* — `analyze_binary.sh:171` still writes `strings.head.txt` on any host that has `strings`, so `_write_binary_cli_surface_spec` takes its strings branch and row 20 still writes.
- *Make `<tmp>/binary/` unwritable* — `analyze_binary.sh` is invoked with **`check=True`** at `:1621-1628` (after `_ensure_dirs([tmp_dir / "binary"])` at `:1619`), so the run aborts before control ever reaches the CLI-selection block at `:1751-1775`. Row 20 and row 6 are never evaluated.

**Corrected prescription — the direct seam is the valid test; pipeline injection must be a conjunction.**

> **Primary (preferred).** Call `_write_binary_cli_surface_spec(output_dir, tmp_dir, product_name=…, date=…)` directly, with `<tmp>/binary/` existing and empty. Assert `return False` and no `spec-cli-surface.md`. Sol executed exactly this and observed `return=False`, `spec_exists=False`.
>
> **Pipeline injection, if the row-6 interaction must be driven end to end.** The injected environment must **independently suppress both evidence producers while leaving every earlier `check=True` step succeeding**. One configuration provably satisfies this: **`capture_cli_help.sh` absent from the package** (skipped at `:1635`, no help tree) **and `strings` absent from `PATH`** (guard at `analyze_binary.sh:169` false, no `strings.head.txt`, and the script still exits 0 so `:1628` is satisfied). Then assert `spec-cli-surface.md` is absent and the `## CLI Surface … _Omitted_` note appends to `spec-code-map.md`.

Neither "helper renamed" nor "scratch unwritable" appears in v7 as a standalone example.

### 0.6 Scratch by class — complete, with H5 added

§3 enumerates **durable output producers**. Operational scratch (§4 class B) is **summarized by class here, not enumerated row by row**:

| Class | Representative sites |
|---|---|
| Clone / work root | `local_clone_dir`, default `.tmp/<product_slug>` (`:1509`); `extracted/` under it is row 18 |
| Main-script tmp root | `tmp_dir = <repo>/.tmp/reverse-engineer-<product_slug>` (`:1513`) |
| Sitemap fetch scratch | `<tmp>/<slug>-sitemap.xml` (`:1564`, via `fetch_url.py` `write_bytes`), `<tmp>/<slug>-sitemap-paths.txt` (`:1567-1569`) |
| CLI-help helper scratch | `capture_cli_help.sh:39-48` — `cli-help-tree.txt`, `cli-commands.txt`, `.seen-command-paths.tmp`, `.visited-prefixes.tmp`, under `<tmp>/binary/` |
| Binary analysis scratch | `analyze_binary.sh` — `strings.head.txt` (`:171`), `strings.ai-hits.txt` (`:173`/`:175`), `disassembly.head.txt` (`:181`/`:183`), plus a `mktemp` removed by an EXIT trap (`:46-48`) |
| Archive index scratch | `list_embedded_archives.py --out-json` → `<tmp>/binary/embedded-archives.json` (`:1654`); its `--out-index-md` sibling is durable row 16 |
| **Security-validation scratch (H5)** | `validate_security_audit.sh:55-61` resolves and invokes the copied `scan-secrets.sh`; `scan_secrets.sh:31-32` — `TMP="$(mktemp -t re_rpi_secrets.XXXXXX)"` with `trap 'rm -f "$TMP"' EXIT` |

Several scratch files are *read* by durable producers — `cli-commands.txt` feeds row 19, `strings.head.txt` feeds rows 17 and 20 — so they appear in row conditions without being rows themselves.

---

## 1. Finding ledger and totals — preserved, Sol-validated three times

### 1.1 The repair-unit rule

> **One finding = one independently repairable defect. Two defects are separate if either can be fixed without deciding the other.**

| Bundle | Split into | Rationale |
|---|---|---|
| **H1** | `H1a` missing `type` · `H1b` missing `consumed` · `H1c` fractional-second `id` vs whole-second pattern | Sol's Draft 2020-12 check reproduced exactly three errors, three times. |
| **RF1** | `RF1a` commit authority · `RF1b` automatic-revert authority · `RF1c` stale feature-header `consumes`/`produces` | Removing the commit grant does not repair the revert grant or the header drift. |
| **SC1** | `SC1a` "Step 5: Initial Commit" · `SC1b` "Next steps:" continuation | Two separate blocks in one file; either can be deleted alone. |
| **SC5** | `SC5a` exemplar-selection proof gap · `SC5b` one-way-sync proof gap | Two independent disciplines, two independent witnesses. |
| **T4** | `T4a` oracle-strength · `T4b` mutation-kill · `T4c` harness-health floors | Three separately witnessable doctrines. |
| **U1** | `U1a` dispatch-once · `U1b` one-wake-maximum | Separate fixtures; they fail independently. |
| **H5** | `H5a` stale `ao handoff` command name · `H5b` nonexistent session-start hook | Two unrelated stale references in one description. |

**Severity note on RF1c.** `RF1a`/`RF1b` are authority grants (**P0**, class C3). `RF1c` is metadata drift with no authority effect and is ranked **P1**.

### 1.2 Explicit finding ledger

| Skill | P0 | P1 | P2 | Row IDs |
|---|---:|---:|---:|---|
| bootstrap | 0 | 0 | 3 | B1, B2, B3 |
| codebase-recon | 0 | 2 | 3 | CR1, CR2 / CR3, CR4, CR5 |
| handoff | **3** | 1 | **5** | H1a, H1b, H1c / H2 / H3, H4, H5a, H5b, H6 |
| refactor | **3** | **4** | 1 | RF1a, RF1b, RF2 / RF1c, RF3, RF4, RF5 / RF6 |
| reverse-engineer | 4 | 6 | 7 | RE1–RE4 / RE5–RE10 / RE11–RE17 |
| sbh | 0 | 2 | 1 | S1, S2 / S3 |
| scaffold | **3** | 1 | **3** | SC1a, SC1b, SC2 / SC3 / SC4, SC5a, SC5b |
| shared | 0 | 0 | 1 | SH1 |
| status | 0 | 1 | 3 | ST1 / ST2, ST3, ST4 |
| test | 0 | **6** | 3 | T1, T2, T3, T4a, T4b, T4c / T5, T6, T7 |
| using-gc | 0 | **2** | 2 | U1a, U1b / U2, U3 |
| workflow-builder | 0 | 0 | 2 | W1, W2 |
| **Total** | **13** | **25** | **34** | **72 rows** |

Sol recomputed `13/25/34 = 72` at v4, v5, and v6. **`9 / 21 / 32` remains withdrawn.** The H1–H6 corrections add no finding and remove none.

---

## 2. Confirmed corrections preserved

**2.1 Inventory.** Exactly **70** tracked owned files. Per-skill: `2, 3, 2, 3, 40, 1, 5, 1, 2, 9, 1, 1`. reverse-engineer's 40 = 18 scripts + 14 templates + 4 fixtures + `SKILL.md` + `.gitignore` + `agents/openai.yaml` + `references/reverse-engineer.feature`. Sol's fresh v6 comparison found **`missing=0`, `extra=0`, `mismatch=0`** against all 70 v2 whole-file digests, with ordered path-list SHA-256 `e78d803549b6c6c16e21ae5117a54958286e4131881d85a94c5e66c99d14ba19`.

**2.2 Two effects rails.** Legacy `metadata.effects` is an unconstrained **string array** — no enum, no scope/authorization/cleanup/receipt. Structured `metadata.contract_v3.effects` is a **separate shadow object rail** with a 12-member `kind` enum, declared by exactly one skill at this subject (`skill-builder`, outside these twelve). `operate_gas_city` is legal legacy metadata; no live enum gate rejects it.

**2.3 Bootstrap P0 refuted.** Validate creates the intent store on first write (`skills/validate/scripts/validate.py:427`); `ao status` appends both store paths to `Checked` before reading and treats `os.IsNotExist` as empty, never `Unavailable`. The proposed witness passes today and is therefore not lethal.

**2.4 Handoff — exactly three direct failures.** Missing `type`; missing `consumed`; `id` formatted `20060102T150405.000000000Z` (`cli/cmd/ao/handoff.go:70`) against `^handoff-[0-9]{8}T[0-9]{6}Z$`. Sol's fresh v6 dry run produced `handoff-20260728T092537.691971000Z` and validation returned exactly three errors.

**2.5 Six noncanonical directories.** `.agents/recon`, `.agents/handoff`, `.agents/research`, `.agents/council`, `.agents/learnings`, `.agents/tests`. ADR-0016's closed set is `ao/`, `scratch/`, `projections/`. The detector `fm-ws-noncanonical-topdir` is absent from executable source, while `cli/internal/initapp/initapp.go:28` **creates `.agents/handoff` itself**. Accepted-contract/source defects, **not currently-executing detector failures**.

**2.6 Reserved lane vs scratch vs work root.** Three distinct classes, preserved in §4. **The class-A writes are now also matrix rows 25 and 26** (§0.2); their §4 disposition is unchanged.

**2.7 SBH context withdrawal.** `sbh`, `shared`, and `using-gc` all lack `context:`; `using-gc` is execution-tier; the frontmatter schema neither requires nor declares `context`.

**2.8 Status test provenance.** `TestLoadLoopEvidence_NoArtifactsIsExplicit` at `cli/internal/statusapp/statusapp_test.go:59`; Sol's fresh focused test passed at v5 and v6.

**2.9 Scenario execution.** Reproduced by Sol at v4, v5, and v6:

```text
codebase-recon    rc=0  pass 2/2 errors=0
refactor          rc=1  fail 0/4 errors=4
scaffold          rc=1  fail 0/4 errors=4
test              rc=1  fail 0/3 errors=3
reverse-engineer  rc=1  fail 0/3 errors=3
```

Plus `skills/scaffold/scripts/validate.sh` PASS and `skills/test/scripts/validate.sh` PASS 5/5 — both false-negative boundaries, not proofs.

**2.10 Python ratchet.** `--scope head` passes with 24 grandfathered files; reverse-engineer's eight execution-path `.py` files are in that set. Shrink-only.

**2.11 Security gate.** Fresh `--security-audit` runs render exactly seven Markdown files and three copied executable helpers; `findings.md` retains four `_TBD_` fields and `validate-security-audit.sh --no-sbom` returns 0. RE3 is real.

**2.12 Gas City evidence.** Local primary source at tag `v1.3.5` (`aec4c52ef7649e37a6c4795f1b177bce0401d5ea`) confirms the single-item idempotent early return and the batch no-nudge condition; upstream issue #4586 describes the v1.3.5 inherited-rig demand-spawn failure. Carried from the v4 Sol primary-source check; that provenance is disclosed.

**2.13 Both-mode exercised.** Sol's v5 and v6 both-mode probes completed rc=0 and produced `comparison-report.md` plus the combined outputs. Row 21's residual is discharged.

**2.14 SBOM Go side branch now exercised.** Sol's v6 review added a fallback probe with `go.mod` present and a fake failing `go`, producing `sbom.go-mod.modules.json` and `go-list.stderr`. That residual is discharged; only the successful real-`syft` path remains source-derived.

---

## 3. The reverse-engineer durable-output producer matrix — **27 rows**

### 3.0 What this matrix covers, and what it does not

**Covers.** Every identified **durable output producer** — a write that survives the run, whether it lands in `--output-dir` (rows 1–24) or, for the class-A reserved-lane writes, outside it (rows 25–26) — plus the model-produced Phase-2 artifact (row 27).

**Does not cover.** Operational scratch (§4 class B), **summarized by class in §0.6, not enumerated row by row**. Several scratch files are read by durable producers and therefore appear inside row *conditions* without being rows.

**Not called exhaustive.** The matrix has been found incomplete in three consecutive revisions by three different mechanisms (§11.5). It is called complete for the durable producers identified to date, with the residual named in §11.4, and **RE8's publication recommendation remains blocked pending independent review of this version**.

Rows are grouped **only** where the condition *and* the producing owner are identical.

### 3.1 Unconditional on any successful run — rows 1–8

| # | Output | Condition | Producer (file:line) |
|---|---|---|---|
| 1 | `docs-features.txt` | **File unconditional; content conditional across three mutually exclusive branches.** (a) sitemap form when `--docs-sitemap-url` is supplied and the fetch succeeds — **this branch sits outside every mode guard and runs in binary mode too**; (b) repo-scan form when `mode ∈ {repo,both}` **and** `analysis_root.exists()`, writing an empty file when no slugs match; (c) empty file otherwise | (a) `:1562-1582`; (b) `:1585-1605`; (c) `:1607` |
| 2 | `feature-inventory.md` | none | `:1683-1697` → `generate_feature_inventory_md.py`, `check=True` |
| 3 | `feature-registry.yaml` *(**initial** scaffold)* | none. **May be rewritten in place by row 19** in binary/both mode | `:1699-1715` → `scaffold_feature_registry.py`, `check=True`; path bound at `:1700` |
| 4 | `feature-catalog.md` | none | `:1727-1738` → `generate_feature_catalog_md.py`, `check=True` |
| 5 | `spec-architecture.md`, `spec-code-map.md`, `spec-clone-vs-use.md`, `spec-clone-mvp.md` | none — **one template-render loop, identical condition and owner** | `:1740-1748` `_render_template` |
| 6 | `spec-code-map.md` *(append)* | **neither row 10 nor row 20 wrote** (`not wrote_cli` at `:1769`) — appends a `## CLI Surface … _Omitted_` note to the row-5 file | `:1769-1775` |
| 7 | `validate-feature-registry.py` (wrapper) | none | `:1801` → `_write_wrapper_validate_feature_registry` `:1405-1408` |
| 8 | `analysis-root/` (dir) + `analysis-root-path.txt` | none — **two inline statements, identical condition and owner** | `:1803-1804` |

### 3.2 Repository-mode producers — rows 9–13

| # | Output | Condition | Producer |
|---|---|---|---|
| 9 | `clone-metadata.json` | `mode ∈ {repo,both}` **and any first `--upstream-repo` clone** — **not** gated on `--upstream-ref` | `:1521` guard; `:1541-1543` write **outside** the `if args.upstream_ref:` block |
| 10 | `spec-cli-surface.md` *(**repository** producer)* | `mode ∈ {repo,both}` **and** `analysis_root.exists()` **and** a Node, Python, or Go CLI is detected; returns `False` otherwise. **On `False`, row 20 may still produce the same filename** | `:1752-1759` → `_write_cli_surface_spec` `:662-831`; early returns `:679`, `:686`; writes `:723-724` (Python/Go) or `:830-831` (Node) |
| 11 | `spec-artifact-surface.md` | `mode ∈ {repo,both}` — **always written when called**; content degrades to one of three stub forms | `:1778-1779` → `_write_artifact_surface_spec` `:834`; stubs `:853`, `:861`, `:870` |
| 12 | `artifact-registry.json` | `mode ∈ {repo,both}` **and all three**: `analysis_root.exists()`, a detected Node CLI package, `<pkg>/templates/manifests/` present | `:955`; three early returns at `:851-853`, `:859-861`, `:868-870` |
| 13 | `contracts/repo-contract.json` | `mode ∈ {repo,both}`; none beyond mode | `:1788-1789` → `_write_repo_contract_json` `:1138`; dir `:1159`, write `:1274` |

### 3.3 Binary-mode producers — rows 14–20

| # | Output | Condition | Producer |
|---|---|---|---|
| 14 | `binary-analysis.md` | `mode ∈ {binary,both}` (requires `--authorized` **and** `--binary-path`, both hard-die) **and** the tmp artifact exists after `analyze_binary.sh` | `:1610-1631`; die at `:1612`, `:1614`; helper `_run` **`check=True`** at `:1621-1628`; copy `:1629-1631` |
| 15 | `cli-help-tree.txt`, `cli-commands.txt` *(copied to output)* | `capture_cli_help.sh` **exists** (`:1635`); run **best-effort (`check=False`)**; each file copied only if produced — **identical condition and owner, so one row.** The helper creates both in `<tmp>/binary/` **unconditionally** at `capture_cli_help.sh:45-46` (§0.5) | `:1634-1646` |
| 16 | `binary-embedded-archives.md` | `mode ∈ {binary,both}`; `list_embedded_archives.py` invoked with this path as a **required `--out-index-md` output**, `check=True`. Its `--out-json` sibling lands in `<tmp>` and is scratch (§0.6) | `:1648-1659` |
| 17 | `binary-symbols.txt` | `mode ∈ {binary,both}` *(inherited from the `:1718` call guard)* **and** `<tmp>/binary/strings.head.txt` exists **and** the destination does not already exist | `:468-471`, inside `_enrich_registry_with_binary_evidence` — **same owner and same invocation as row 19** |
| 18 | See the two-outcome table below | `mode ∈ {binary,both}` **and not** `--no-materialize-archives` (default is to materialize); conflicting flags hard-die at `:1663` | block `:1665-1681`; helper `check=True` at `:1668-1678` → `extract_embedded_archives.py` |
| 19 | `feature-registry.yaml` *(**binary enrichment rewrite**, in place over row 3)* | `mode ∈ {binary,both}` (`:1718`) **and** nonempty command groups parsed from `<tmp>/binary/cli-commands.txt` (`:474-485`) **and** registry parse succeeds (`:531-532`) **and** the existing registry has no populated group notes (`:537-540`). Output carries `evidence_source: 'binary --help + string extraction'` (`:559`) | `:1718-1725` → `_enrich_registry_with_binary_evidence` `:449-579`; three no-rewrite returns at `:485`, `:532`, `:540`; write `:578`, `return True` `:579` |
| 20 | `spec-cli-surface.md` *(**binary fallback** producer)* | `not wrote_cli` — row 10 did not write — **and** `mode ∈ {binary,both}` **and** at least one of `<tmp>/binary/cli-help-tree.txt` or `<tmp>/binary/strings.head.txt` exists. Content conditional on which | `:1761-1767` → `_write_binary_cli_surface_spec` `:582-659`; **negative return `:655` — source-derived but normally dominated, §0.5**; write `:658-659` |

**Row 18 — two mutually exclusive internal outcomes (H3).**

| Archive result | Durable outputs | Downstream effect |
|---|---|---|
| **No openable ZIP candidate** (`:79`) | `<extract_root>/extract.NOOP.md` (`:80-83`), then `return 0` (`:84`) | **No `PRIMARY.txt`** ⇒ `if primary.exists():` at `:1680` is false ⇒ **`analysis_root` is not redirected** |
| **Candidate found** (`:86`) | `<extract_root>/zip@<offset>/…` extracted tree (`:87-92`), `zip@<offset>/manifest.json` (`:96-106`), `<extract_root>/PRIMARY.txt` (`:109`) | `PRIMARY.txt` **redirects `analysis_root`** at `:1681` |

Sol's fresh bounded helper probe read `/usr/bin/true` and produced exactly `extract.NOOP.md`, with no `PRIMARY.txt` — the first branch. The candidate branch is source-derived; no archive was materialized by either party.

**Rows 17 and 19 share one owner and one invocation** (`:1719`) with different conditions — Sol's isolated probes fired each without the other (`rewrite_only`: return True, evidence_source True, binary_symbols False; `symbols_only`: return False, evidence_source False, binary_symbols True). Two conditions, two rows.

**Rows 10 and 20 emit the same filename** and are mutually exclusive within a run. A matrix row is a producer under a condition, not a filename.

### 3.4 Both-mode producer — row 21

| # | Output | Condition | Producer |
|---|---|---|---|
| 21 | **`comparison-report.md`** | `mode == "both"` exactly | `:1796-1797` → `_write_comparison_report` `:1281`; path bound `:1400`, **`write_text` `:1401`** (H6) |

### 3.5 Security-mode producers — rows 22–24

| # | Output | Condition | Producer |
|---|---|---|---|
| 22 | Seven Markdown artifacts under `security/`: `threat-model.md`, `attack-surface.md`, `dataflow.md`, `crypto-review.md`, `authn-authz.md`, `findings.md`, `reproducibility.md` | `--security-audit` — **one render loop, identical condition and owner** | `:1825-1834` `_render_template` over `templates/security/*.tmpl` |
| 23 | Three copied helpers: `validate-security-audit.sh`, `scan-secrets.sh`, `generate-sbom.sh` (source names use `_`, destinations use `-`), each `chmod 0o755` | `--security-audit` only — **copied regardless of `--sbom`** | `:1836` → `_copy_security_validators` `:1454-1467` |
| 24 | See the three-outcome table below | `--security-audit` **and** `--sbom`; generator invoked `check=False` | `:1838-1839` → `generate_sbom.sh` (54 lines) |

**Row 24 — three outcomes, not exclusive alternatives.**

| `syft` state | Files produced | Script exit |
|---|---|---|
| **absent** (`:15` false) | `sbom.NOOP.md` (`:36-43`) and `dep-risk-report.md` in **No-Op form** (`:45-53`). **No SPDX/stderr pair.** | 0 |
| **present and succeeds** (`:17` true) | `sbom.spdx.json` and `syft.stderr` from the `:17` redirection, then `dep-risk-report.md` in **syft form** (`:18-24`), then `exit 0` (`:25`). **No `sbom.NOOP.md`; no Go side files.** | 0 |
| **present and fails** (`:17` false) | `sbom.spdx.json` and `syft.stderr` **remain** — the shell created them at `:17` before executing `syft`. Fallback continues and **additionally** produces `sbom.NOOP.md` **and** `dep-risk-report.md` in No-Op form. | 0 |

**Go side files, orthogonal.** Whenever the fallback is reached (rows 1 and 3), if `$ROOT/go.mod` exists **and** `go` is on `PATH`, the script additionally creates `sbom.go-mod.modules.json` and `go-list.stderr` (`:30-34`, `|| true`). Sol's v6 fallback probe with `go.mod` and a fake failing `go` produced exactly those two files.

**Two mechanisms.** (i) The shell opens and truncates both redirection targets at `:17` before executing `syft`, so the pair exists whenever `syft` is present regardless of exit status. (ii) `set -euo pipefail` (`:2`) does not abort on a failing command in an `if` condition, so a nonzero `syft` falls through to `:30`; the script has no trailing `exit`, so its status is the final `cat`'s — **0**. A failed-`syft`/successful-fallback run therefore belongs **inside** the successful-mode inventory.

### 3.6 Reserved-lane producers, outside `--output-dir` — rows 25–26 *(new, H1)*

Both are rooted at `REPO_ROOT = Path.cwd()` and land wherever the caller stood. They are **class A** in §4, **C1 + C2** in §5, and they sustain **RE1, RE2, and RE4** — recording their producers here does not soften any of that.

| # | Output | Condition | Producer |
|---|---|---|---|
| 25 | `.agents/council/<date>-vibe-<slug>.md` **and** `.agents/council/<date>-postmortem-<slug>.md` | Every run that reaches report generation — **no mode or flag guard**. **Grouped: identical owner and condition**, the same rule that groups row 5 | `council_dir` `:1845`, `_ensure_dirs` `:1846`, paths `:1847-1848`, two `_render_template` calls `:1850-1851` |
| 26 | `.agents/learnings/<date>-<slug>-reverse-engineer.md` | **Three-way — see the outcome table below.** `:1853` builds the path, `:1854` guards on `not …exists()`, `:1855` writes | `:1853-1871`, inline `write_text` with canned run-independent content (**RE4**) |

**Row 26 internal outcomes — the live control flow is three-way** *(same shape as rows 18 and 24; corrects Sol v7 W2)*:

| Live state at `:1854` | Result | Exit |
|---|---|---|
| Destination **already exists** | Guard is false. **No write.** The run continues to `return 0` at `:1873` | rc=0 |
| Destination **absent** and `.agents/learnings/` **exists** | `write_text` at `:1855` succeeds — **the learning file is written** | rc=0 |
| Destination **absent** and parent `.agents/learnings/` **absent** | `write_text` at `:1855` raises **`FileNotFoundError`**. **No learning file is produced**, and the process aborts | rc=1 |

**The parent directory is never created.** `_ensure_dirs` at `:1846` covers `council_dir` only, and the string `learnings` occurs **exactly once in the entire script — at `:1853`**, as a path component. That is **RE2**, unchanged in severity.

**The third row is the default state of a fresh checkout.** `.agents/learnings/` does not exist at the bound tree, so a first run from an untouched repository root takes the `FileNotFoundError` branch. Row 25's council writes precede it and are protected by `_ensure_dirs`, which is exactly why Witness E shows rows 1–25 complete and only row 26 failed.

### 3.7 Phase-2 output — row 27, model-produced

| # | Output | Condition | Producer |
|---|---|---|---|
| 27 | `steal-map.md` | Phase 2 of the skill contract; **authored by the model, not by `reverse_engineer.py`** | `SKILL.md:60` |

**Every script-only Phase-1 witness classifies this row as *not applicable*, never as absent** — no invocation of `reverse_engineer.py` can produce or fail to produce it.

### 3.8 Reconciliation against Sol's runtime witnesses — all 27 rows, per witness

Each table is computed independently from the witness's own observations, not inferred as a union of other witnesses.

**Witness A — repository mode**, local empty Git origin, no `--upstream-ref`, no authorization, `--security-audit`, no `--sbom`. rc=0.

| State | Rows | Basis |
|---|---|---|
| **Present** | 1, 2, 3 *(initial form)*, 4, 5, **6** *(append — `wrote_cli` stayed False)*, 7, 8, 9 *(`"upstream_ref": null`)*, 11, 13, 22, 23, **25, 26** | observed output set, security directory, and the three class-A files |
| **Absent** | 10 *(no CLI detected)*, 12 *(no Node CLI package)*, 14, 15, 16, 17, 18, 19, 20 *(not binary mode)*, 21 *(not `both`)*, 24 *(no `--sbom`)* | row 19 named **once** |
| **Not applicable** | 27 — model-produced Phase 2 | |

**Witness B — binary mode**, `/usr/bin/true`, materialization disabled, authorized. 17 top-level `--output-dir` entries. rc=0.

| State | Rows | Basis |
|---|---|---|
| **Present** | 1, 2, 3 *(initial form — no rewrite)*, 4, 5, 7, 8, 14, 15, 16, 17, **20**, **25, 26** | `spec-cli-surface.md` present via the binary fallback |
| **Absent** | **6** *(row 20 set `wrote_cli`, so no append)*, 9 *(no `--upstream-repo`)*, 10, 11, 12, 13 *(repo-only)*, 18 *(materialization disabled)*, **19** *(empty `cli-commands.txt` → `:484-485` returned without rewriting)*, 21, 22, 23, 24 | |
| **Not applicable** | 27 | |

The 17-entry count is `--output-dir` contents; rows 25–26 write three files **outside** it and do not change that count.

**Witness C — binary mode**, authorized Docker CLI, materialization disabled. 28 command rows. rc=0.

| State | Rows | Basis |
|---|---|---|
| **Present** | as Witness B **plus 19** — registry rewritten to the binary-evidence form carrying `evidence_source: 'binary --help + string extraction'` | row 3's file, row 19's content |
| **Absent** | as Witness B, minus 19 | |
| **Not applicable** | 27 | |

**Witness D — both mode**, local empty Git origin cloned on first use, `/usr/bin/true`, authorized, materialization disabled, no `--security-audit`. 21 top-level `--output-dir` entries plus all three class-A files. rc=0. *(Sol's exact result, translated per §0.3.)*

| State | Rows |
|---|---|
| **Present** | 1, 2, 3, 4, 5, 7, 8, 9, 11, 13, 14, 15, 16, 17, **20**, **21**, **25, 26** |
| **Absent** | **6**, **10**, **12**, 18, **19**, 22, 23, 24 |
| **Not applicable** | 27 — model-produced Phase 2 |

Sol's discriminators, verbatim: row 6 append no · row 10 repository CLI no · row 12 artifact registry no · row 19 registry rewrite no · row 20 binary CLI yes · row 21 comparison yes. Arithmetic: 18 present + 8 absent + 1 not applicable = **27**.

**Witness E — missing-learnings crash**, repo mode from a cwd without `.agents/learnings/`, no `--upstream-repo`, no `--security-audit`. **rc=1**, `FileNotFoundError` at `:1855`. *(Rebuilt across all 27 rows; corrects Sol v7 W1.)*

| State | Rows | Count | Basis |
|---|---|---:|---|
| **Present** | 1, 2, 3, 4, 5, 6 *(append — `wrote_cli` stayed False)*, 7, 8, 11, 13, **25** | **11** | the unconditional successful-run set, the two repo-mode producers that fire without a CLI or Node package, and both council reports — all written **before** the failure |
| **Absent** | 9 *(no `--upstream-repo`)*, 10 *(no CLI detected)*, 12 *(no Node CLI package)*, 14, 15, 16, 17, 18, 19, 20 *(not binary mode)*, 21 *(not `both`)*, 22, 23 *(no `--security-audit`)*, 24 *(no `--sbom`)* | **14** | mode and flag guards, not the crash — each would be absent on a successful run with the same invocation |
| **Failed** | **26** — destination absent **and** parent `.agents/learnings/` absent → `FileNotFoundError` at `:1855` | **1** | `_ensure_dirs` at `:1846` covers `council_dir` only (**RE2**) |
| **Not applicable** | 27 — model-produced Phase 2 | **1** | no invocation of `reverse_engineer.py` can produce or fail to produce it |

**Arithmetic: 11 present + 14 absent + 1 failed + 1 not applicable = 27.**

**Rows 1–24 were reached.** The failure is the *last* durable statement in the script — `:1855`, immediately before `return 0` at `:1873`. Every script-owned producer ahead of it had already run to completion. v7's "**Not reached** / not applicable" phrasing for rows 1–24 was therefore **false** and is withdrawn: those rows are directly classifiable, and their absences are ordinary mode/flag guards rather than consequences of the crash.

**The distinction matters for RE2's severity.** If rows 1–24 were genuinely unreached, the crash would be an early abort that costs the caller the whole run. It is not: the caller loses **only** the learning file, after every other durable output has landed — including both class-A council reports written outside `--output-dir`. The crash is late, narrow, and silent about everything it already wrote.

This witness exists only because rows 25 and 26 are separate rows: it is the direct evidence that they have different conditions and different failure modes.

**Two structural discriminators, both load-bearing.**

- **Row 6 discriminates rows 10/20.** Witness A kept `wrote_cli` False, so `:1769` appended and no `spec-cli-surface.md` exists. Witnesses B/C/D produced the spec and did **not** append. A single collapsed row cannot produce both behaviors — and this is why "the repo set and the binary set combined" was an invalid description of Witness D.
- **Row 19 is invisible to file listings.** Witnesses B and C produce identical top-level entry lists and differ only in the *content* of `feature-registry.yaml`.

### 3.9 Preserved: the clone-metadata mismatch is a three-way contradiction

- **Code (`:1528-1543`).** The `if args.upstream_ref:` guard covers only `git fetch` + `checkout FETCH_HEAD`. The `clone_meta` dict and its `write_text` sit **outside** that guard, so the file is written on any first `--upstream-repo` clone. Sol's local-origin witness produced `clone-metadata.json` with `"upstream_ref": null`.
- **`SKILL.md:97`** — "…`clone-metadata.json` **only when `--upstream-ref` is supplied**." **Contradicts the code.**
- **`SKILL.md:176`** (Troubleshooting) — "No `clone-metadata.json` | `--upstream-repo` not passed." **Matches the code**, and therefore contradicts `:97`.

**Finding RE9 [P1].** Owner: `SKILL.md:97` (and `:176` for consistency), or `reverse_engineer.py:1528-1543` if ref-gating is the intent.

---

## 4. Three distinct out-of-`--output-dir` path classes *(preserved)*

| Class | Paths | Effect | Scope | Output contract | Disposition |
|---|---|---|---|---|---|
| **A. Reserved-lane writes** | `.agents/council/<date>-vibe-*.md`, `-postmortem-*.md` (**row 25**, `:1845-1851`); `.agents/learnings/*.md` (**row 26**, `:1853-1871`) | `filesystem.write` | Rooted at `REPO_ROOT = Path.cwd()` — lands wherever the caller stood, **outside `--output-dir`** | **Undeclared.** Absent from the skill's Output Specification; the lanes belong to judgment and Learn | **Sustains P0** (RE1, RE2, RE4) |
| **B. Operational scratch** | `.tmp/…` — clone root, main tmp root, sitemap, CLI-help helper, binary analysis, archive index, **security validation** (enumerated by class in §0.6) | `filesystem.write` | Ephemeral working area | Not an output artifact; **deliberately not enumerated as matrix rows** | **Not a P0.** P2 at most |
| **C. Explicit work root** | `--local-clone-dir`, incl. `extracted/` (row 18) | `filesystem.write` | **Caller-supplied and explicitly named** | Declared by flag | **Not a defect.** RE16's unbounded-decompression concern is a resource defect, not a scope one |

**Being a matrix row does not change class A's disposition.** Rows 25 and 26 record *who writes them*; §4 records *that they escape the declared output contract*, and RE1/RE2/RE4 remain P0.

---

## 5. Normalized authority taxonomy, with complete C1 membership *(preserved)*

| Class | Definition | Members |
|---|---|---|
| **C1 — Closed-layout state fault** | The writer mints a top-level `.agents/` directory outside ADR-0016's closed set. A *location* fault. | **All six noncanonical locations:** `codebase-recon` `.agents/recon` · `handoff` `.agents/handoff` · `reverse-engineer` `.agents/research` · `test` `.agents/tests` · **`reverse-engineer` `.agents/council`** · **`reverse-engineer` `.agents/learnings`** |
| **C2 — Core/judgment-lane claim fault** | An artifact asserts or occupies a lane the core loop reserves — judgment or Learn — without independent judgment. Content fault, **always co-located with a C1 here**. | `reverse-engineer` only: mechanical `vibe-report`/`postmortem` into `.agents/council` (**row 25**); run-independent canned `.agents/learnings/*.md` (**row 26**) |
| **C3 — Prohibited side authority** | The skill grants Git, subject-mutation, or continuation authority its own contract denies. Does **not** write a verdict and does **not** occupy a core phase artifact. | `refactor` (RF1a commit, RF1b auto-revert, RF2 internal contradiction); `scaffold` (SC1a commit, SC1b continuation) |

Sol confirmed all six C1 locations and the C2 co-membership at v4, v5, and v6. `schemas/handoff.v1.schema.json`'s `consumed*` and `$defs.rpi` fields belong to **none** of the three — a field is data, not exercised authority (H2). **Nothing in the twelve writes a verdict or owns a core phase.**

---

## 6. The SBH context finding stays withdrawn *(preserved)*

| Skill | `context:` | tier |
|---|---|---|
| `sbh` | **absent** | `execution` |
| `shared` | **absent** | `library` |
| `using-gc` | **absent** | **`execution`** |
| other nine | present | — |

`using-gc` is execution-tier and lacks `context:`, so both the uniqueness claim and its witness fail. `schemas/skill-frontmatter.v2.schema.json` has `required: [name, description, hexagonal_role, practices, metadata]` and does not declare `context` as a property at all. **The P2 and its witness remain withdrawn.**

---

## 7. Effects under one declared vocabulary *(preserved)*

### 7.1 The declared vocabulary

```text
filesystem.read   filesystem.write   process.start
network.read      network.write      environment.read
environment.write clock.read         credential.switch
external.mutate   runtime.session    host.configure
```

**`filesystem.delete` is NOT one of the 12 v3 kinds.** Deletion is recorded in the **vocabulary-gap table** (§7.3) rather than silently mixed into a v3-vocabulary list.

**Severity rule.** A skill declaring `metadata.effects: []` while performing real effects carries **P2**. It carries **P1** when the undeclared surface includes host mutation, network access, or execution of a caller-supplied target. Any non-empty legacy string exempts the skill.
Applied: **P1** for `sbh` and `reverse-engineer`. **P2** for `bootstrap`, `codebase-recon`, `handoff`, `refactor`, `scaffold`, `status`, `test`, `workflow-builder`. **Exempt:** `using-gc`. **Correct as declared:** `shared`.

### 7.2 Real effects in v3 vocabulary

| Skill | v3 kinds | Declared |
|---|---|---|
| bootstrap | `filesystem.read`, `filesystem.write` | `[]` |
| codebase-recon | `filesystem.read`, `filesystem.write`, `process.start` | `[]` |
| **handoff** | `filesystem.read`, `filesystem.write` (**including a conditional temp-file create/delete lifecycle — see §7.3**), `clock.read` (`handoff.go:67`), `process.start` (`:112`), **`environment.read`** — `gitDiscoveryEnv()` reads `os.Environ()` **twice** (`cli/cmd/ao/git_read.go:18,19`) | `[]` |
| refactor | `filesystem.read`, `filesystem.write`, `process.start` | `[]` |
| **reverse-engineer** | `filesystem.read`, `filesystem.write` (**including outside `--output-dir` — rows 25, 26**), `process.start` (incl. **the caller-supplied target binary**, recursive `--help` to depth 3), `network.read` (`git clone`, `urllib` via `fetch_url.py`), `clock.read` (`_today_ymd()`), **`environment.read`** (`command -v` probes for `strings`, `rg`, `syft`, `go`, `otool`/`objdump`, `timeout`/`gtimeout`) | `[]` |
| **sbh** | `filesystem.read`, `filesystem.write`, `process.start`, `host.configure`, `external.mutate` | `[]` |
| scaffold | `filesystem.read`, `filesystem.write`, `process.start` | `[]` |
| shared | none | `[]` — **correct** |
| status | `filesystem.read`, `clock.read` | `[]` — while the Go contract declares `EffectFilesystem \| EffectClock` |
| test | `filesystem.read`, `filesystem.write`, `process.start` | `[]` |
| **using-gc** | `process.start` (`SKILL.md:38,40,55,63,65,76,77`), `filesystem.read`, `runtime.session`, `external.mutate`, **`host.configure` (conditional)** — `:78-83` instructs adding exact-path `trust_level` entries to `~/.codex/config.toml` | `[operate_gas_city]` |
| workflow-builder | `filesystem.write` | `[]` |

### 7.3 Vocabulary-gap table — real behavior with no v3 kind

| Behavior | Skill(s) | Evidence | Nearest v3 kind | Gap |
|---|---|---|---|---|
| **File deletion** | `sbh`; `reverse-engineer` (two `mktemp`/EXIT-trap sites); **`handoff`** *(conditional cleanup)* | `analyze_binary.sh:46-48`; `scan_secrets.sh:31-32`; **`cli/cmd/ao/handoff.go:136`** — `defer func() { _ = os.Remove(tmpName) }()` | `filesystem.write` | **No `filesystem.delete` kind exists.** Handoff's deletion is *conditional*: on a successful `os.Rename` at `:148` the temp path is already gone and the deferred call is a discarded no-op; on any pre-rename or rename failure it **actually deletes** the temporary file. The v3 rail cannot express "deletes only on the failure path." |
| **Temporary-file lifecycle** | `reverse-engineer`; **`handoff`** | §0.6's seven scratch classes; **`handoff.go:131`** `os.CreateTemp(dir, ".handoff-*.tmp")` → `:135` `tmpName := tmp.Name()` → `:136` deferred removal → `:148` `os.Rename(tmpName, target)` | `filesystem.write` | Creation outside the declared output dir plus trap-based removal has no distinct kind. **Handoff's variant is narrower and safer** — the temp file is created *inside* the destination directory `.agents/handoff` so the rename is same-filesystem and atomic — but it is still a create-then-conditionally-delete lifecycle that one undifferentiated `filesystem.write` cannot describe. |
| **Writes outside the declared output root** | `reverse-engineer` | **rows 25, 26** | `filesystem.write` | The vocabulary cannot distinguish a write inside the caller's declared output contract from one that escapes it — which is why RE1 needs prose to state what the effect declaration cannot. |
| **Archive extraction of untrusted payloads** | `reverse-engineer` | row 18, `extract_embedded_archives.py:92` | `filesystem.write` | Decompression of attacker-controlled input is a materially different risk from an ordinary write. |
| **In-place rewrite of a declared output** | `reverse-engineer` | **row 19**, `:578` | `filesystem.write` | A second producer replacing an earlier producer's durable artifact is indistinguishable from the first write — which is how row 19 escaped two revisions of the matrix. |
| **Tool/PATH probing** | `reverse-engineer` | `analyze_binary.sh:169`, `capture_cli_help.sh:31-35`, `generate_sbom.sh:15,31` | `environment.read` | v3 has no way to say *which* tools, so a reader cannot tell that behavior degrades silently when one is absent — **exactly the surface rows 18, 20, and 24 depend on**. |

**Corpus-level consequence.** `sbh` (host mutation and deletion) and `reverse-engineer` (network, target execution, extraction, out-of-root writes) declare the same effect surface as `status` (pure read). That is a taxonomy failure on the live rail.

**Deletion is not silently subsumed.** This audit records cleanup deletion in the gap table rather than folding it into `filesystem.write`, and applies that rule **consistently to all three skills** that perform it — `sbh`, `reverse-engineer`, and now `handoff`. The alternative Sol offered — explicitly defining cleanup deletion as subsumed by `filesystem.write` — is rejected because it would erase the distinction between handoff's failure-path-only cleanup and `sbh`'s intentional destructive deletion, which is precisely the distinction the corpus most needs. **This changes no finding and no severity**: `handoff`'s empty legacy rail is already owned by **H6 [P2]**, and no new finding is created.

---

## 8. Evidence provenance *(preserved)*

**8.1** `TestLoadLoopEvidence_NoArtifactsIsExplicit` lives at `cli/internal/statusapp/statusapp_test.go:59`, not in `statusapp.go`.

**8.2** Scenario coverage was executed with `--run --json`, not carried; the five results are in §2.9 and Sol reproduced them at v4, v5, and v6. Correct JSON keys: `result`, `scenarios_total`, `scenarios_covered`, `errors`.

---

## 9. Per-skill deep sections

Sol assessed all twelve at v6: eleven **PASS**, and reverse-engineer **PARTIAL** — repository ledger 4/6/7 supported, RE8's publication repair blocked.

### 9.1 bootstrap — 2 files

**Intent.** A never-overwrite, ask-before-inventing document seeder. `SKILL.md:38-40`: "a setup step that can only add is idempotent by construction." Failure mode *scaffold sprawl*.
**RPI fit.** Outside the core; a session/pre-loop initializer. Performs no Plan/Implement/Validate move; `Non-goals` excludes running RPI. Sol: supported.
**In / out / effects / authority.** In: caller target dir + explicitly requested artifacts. Out: created files, `.agents/ao/verdicts/sha256/`, created/skipped/failed report. Effects: `filesystem.read`, `filesystem.write`; declared `[]`. Authority clean — no work ownership, Git, closure, verdict, or retry.
**Owned sources examined.** Both tracked files in full; `initapp.go` created-directory set; `statusapp.go` missing-store semantics.
**Strengths.** Create-only seeding; the idempotence-by-construction argument stated as the reason, not the rule.
**Findings.** **B1 [P2]** legacy-empty-rail. **B2 [P2]** three-way `bootstrap` naming overlap (skill · `ao init` · `ao session bootstrap`) with no cross-reference in any of the three. **B3 [P2]** no feature file, validator, or test. **Withdrawn:** the missing-intent-store P0 (§2.3).
**Repairs.** P2 — decide the effects rail then declare truthfully (B1); add the three-way distinction sentence (B2); add a never-overwrite fixture: pre-create `PRODUCT.md` with known bytes, run the procedure, assert byte-identity (B3).
**Ledger.** B1, B2, B3 → 0/0/3.
**Checked / not_checked.** Checked: both owned files, the two Go owners. Not checked: no bootstrap run performed by either party.
**Residual risk.** Low — prose-only, but the mutation surface is create-only.

### 9.2 codebase-recon — 3 files

**Intent.** A *falsifiable* repository model: every material claim typed `fact | inference | unknown`, cited, mechanically re-checkable; a second run proves a verified delta.
**RPI fit.** Outside the core, upstream of it — an evidence producer feeding the caller or Plan. Routes binding PASS/FAIL to `validate`. Sol: supported.
**In / out / effects / authority.** In: repository at a recorded commit, optional prior pack. Out: `.agents/recon/<run-id>/codebase-recon.{json,md}`. Effects: `filesystem.read`, `filesystem.write`, `process.start`; declared `[]`. Authority: the model case — produces evidence and stops.
**Owned sources examined.** All 3 tracked files in full; scenario coverage re-executed.
**Strengths.** The fact/inference/unknown taxonomy; prior-pack delta discipline; required non-empty `commit`. **The only one of the twelve with a real artifact validator and passing scenario coverage — 2/2 with `--run`.**
**Findings.** **CR1 [P1]** the validator resolves evidence against the wrong repository: `repo_root` derives from the script's own location (`scripts/validate-output.sh:10-11`); no `--repo-root`. **CR2 [P1]** symlinked invocation breaks root resolution — logical `pwd` through `~/.claude/skills/codebase-recon` yields `.claude`; needs `pwd -P`. **CR3 [P2]** the `file:line` floor is prose-only; a directory passes because resolution uses `-e`. **CR4 [P2, C1]** `.agents/recon`. **CR5 [P2]** legacy-empty-rail.
**Repairs.** P1 — add `--repo-root` and default to it (CR1); use `pwd -P` (CR2). P2 — decide whether the manifest validator owns the stricter floor (CR3); record CR4 against the ADR-0016 bead; declare effects (CR5).
**Witnesses.** A manifest whose evidence path exists only in the target repo → validator reports missing evidence. Invocation through the symlinked path → spurious failure. A manifest citing `cli` (a directory) as `fact` evidence passes today.
**Ledger.** CR1, CR2 / CR3, CR4, CR5 → 0/2/3.
**Checked / not_checked.** Checked: all 3 owned files; `--run --json` executed. Not checked: behavior against a non-AgentOps target repository.
**Residual risk.** Low-moderate — the validator is real but mis-rooted, so it can be green on a target it never resolved.

### 9.3 handoff — 2 files (+ live Go owners + `handoff.v1`)

**Intent.** A verifiable end-state artifact: exact paths and facts the next context can act on without trusting the author's memory. Failure mode *optimistic closure*.
**RPI fit.** Outside the core — a session-boundary support surface. No phase, no ownership, no verdict, no continuation choice. Sol: supported.
**In / out / effects / authority.** Writer: `ao session handoff … [--dry-run]` → `.agents/handoff/handoff-<RFC3339Nano>.json`, atomic temp+rename — `os.CreateTemp(dir, ".handoff-*.tmp")` at `handoff.go:131`, `tmpName` captured at `:135`, `defer func() { _ = os.Remove(tmpName) }()` at `:136`, `os.Rename(tmpName, target)` at `:148`. **The deferred removal is conditional cleanup**: after a successful rename the temp path no longer exists and the call is a discarded no-op; on any pre-rename or rename failure it deletes the temporary file. The temp file is created *inside* the destination directory, so the rename is same-filesystem and atomic. This lifecycle is recorded in the §7.3 vocabulary-gap table because v3 has no `filesystem.delete` kind — **it is owned by the existing H6 and adds no finding**. Reader: `ao session rehydrate` selects the lexically newest file, **pure read**. Effects: `filesystem.read`, `filesystem.write`, `clock.read` (`handoff.go:67`), `process.start` (`:112`), **`environment.read`** (`git_read.go:18,19`); declared `[]`. Authority correct in code — **no consumption marking anywhere in the Go**.
**Owned sources examined.** Both owned files; `handoff.go`, `sessionapp.go`, `initapp.go`, `git_read.go:17-33`; the full `handoff.v1` schema including `$defs.rpi`.
**Strengths.** `SKILL.md:50-52` separates the human Markdown artifact from the JSON CLI boundary — two artifacts, two audiences, one honest contract.
**Findings — H1 split three ways per §1.1.** **H1a [P0]** writer omits the schema-required `type`. **H1b [P0]** writer omits the schema-required `consumed`. **H1c [P0]** `id` formatted `20060102T150405.000000000Z` (`handoff.go:70`) violates `^handoff-[0-9]{8}T[0-9]{6}Z$`. **H2 [P1]** schema drift: `consumed*` plus `$defs.rpi` is stale against the operating-loop contract — **class none**, data shape, not exercised authority. **H3 [P2]** archived `tests/claude-code/test-handoff-skill.sh` is stale; runner opt-in, exits `SKIPPED` unless `AGENTOPS_ENABLE_CLAUDE_CODE_FUNCTIONAL_TESTS=1`. **H4 [P2, C1]** `.agents/handoff` — `initapp.go:28` creates it, so this is product behavior. **H5a [P2]** schema description names a nonexistent `ao handoff` command. **H5b [P2]** it also names a session-start hook that does not exist. **H6 [P2]** legacy-empty-rail. **Refuted:** the Markdown-reader "incompatibility."
**Repairs.** P0 — pick one reconciliation direction, then each of H1a/H1b/H1c is independently applicable. P1 — retire or re-scope the lifecycle block (H2). P2 — fix or delete the archived test; record H4; correct the two stale references.
**Witnesses.** `ao session handoff "x" --dry-run` validated against `schemas/handoff.v1.schema.json` → **exactly 3 errors**, reproduced verbatim by Sol at v4, v5, and v6.
**Ledger.** H1a, H1b, H1c / H2 / H3, H4, H5a, H5b, H6 → 3/1/5.
**Checked / not_checked.** Checked: owned files, four Go owners, the schema, the three-error witness, **and `handoff.go:125-152` in full — the complete temp-create/defer-remove/rename lifecycle**. Not checked: Go suites not executed this pass; **no handoff write was executed in this pass, so the failure-path deletion is source-established, not observed**.
**Residual risk.** Moderate — every artifact the shipped command writes fails its own active schema.

### 9.4 refactor — 3 files

**Intent.** One bounded behavior-preserving transformation where "behavior-preserving" is *executed, not asserted*.
**RPI fit.** Outside the core, an optional Implement method. Produces a diff and evidence; does not validate. Sol: supported.
**In / out / effects / authority.** In: caller-selected transformation + named preserved behavior. Out: diff summary, commands run, results, explicit not-checked list. Effects: `filesystem.read`, `filesystem.write`, `process.start`; declared `[]`. **Authority: class C3 leaks in owned artifacts.**
**Owned sources examined.** All 3 owned files in full; `SKILL.md:47-49` and `:74-75` verbatim; the feature file; coverage re-executed 0/4.
**Strengths.** Baseline/after neutrality (golden-output hashing, the "no quietly vanished red" rule); explicit not-checked surface; seam probes in disposable isolation with a hard stop at two.
**Findings — RF1 split three ways per §1.1.** **RF1a [P0, C3]** `references/refactor.feature` grants **commit**, contradicting `SKILL.md:47-49`. **RF1b [P0, C3]** the same feature grants **automatic revert**. **RF1c [P1]** the feature header declares `consumes: complexity / produces: git-changes` against the real frontmatter. **RF2 [P0, C3]** **internal** contradiction: `SKILL.md:74-75` says a hash mismatch is "to explain **or revert**" while `:47-49` says the skill does not revert. **RF3 [P1]** dangling `/complexity` route. **RF4 [P1]** no scenario coverage — 0/4. **RF5 [P1]** neutrality gates have no behavioral witness. **RF6 [P2]** legacy-empty-rail.
**Repairs.** P0 — resolve RF2 **first**, since `SKILL.md` governs the feature rewrite; then delete the commit grant (RF1a) and the revert grant (RF1b). Sol: "precedence-first repair ordering is sound." P1 — correct the header (RF1c); remove or implement `/complexity` (RF3); rewrite the four scenarios with `@covered-by` (RF4); build a neutrality harness (RF5).
**Witnesses.** `grep -n 'revert' skills/refactor/SKILL.md` returns **both** `:48` and `:75`. Coverage → `fail 0/4`. A fixture where a refactor deletes a test file — no new red, one silently vanished red — is accepted by every check in the repo today.
**Ledger.** RF1a, RF1b, RF2 / RF1c, RF3, RF4, RF5 / RF6 → 3/4/1.
**Checked / not_checked.** Checked: all 3 owned files; coverage executed. Not checked: no refactor executed.
**Residual risk.** High for authority — the canonical skill and its owned feature both grant powers the contract denies, and nothing enforces precedence.

### 9.5 reverse-engineer — 40 files

**Intent.** Two separable things: a mechanically verifiable teardown (Phase 1, script-produced) and a steal-map (Phase 2, **model-produced**), with the vocabulary `have | gap | steal | park | reject` and one-way-door adoptions routed to Plan.
**RPI fit.** Outside the core — external-system research feeding Plan. Sol: supported.
**In / out / effects / authority.** In: `product_name`, `--mode`, `--upstream-repo`, `--upstream-ref`, `--binary-path`, `--authorized`, `--security-audit`, `--sbom`, `--docs-sitemap-url`, `--local-clone-dir`, `--output-dir`, materialization flags. **Out: the 27-row durable-output producer matrix (§3) — rows 1–24 inside `--output-dir`, rows 25–26 in reserved lanes outside it, and model-produced `steal-map.md` as row 27. Operational scratch is summarized by class in §0.6 and is deliberately not enumerated.** Effects: §7.2 + the §7.3 gaps. **Authority: the only skill reaching class C2.**
**Owned sources examined.** All 40 owned files (inherited full read). This pass read the class-A block `:1843-1873` in full, `extract_embedded_archives.py:70-112`, `analyze_binary.sh:1-12,160-190` and its tail, `capture_cli_help.sh:33-52,268-280`, the capture invocation `:1634-1648`, the analyze invocation `:1616-1632`, the extraction call site `:1665-1682`, `validate_security_audit.sh:50-65`, `scan_secrets.sh:25-40`, and the comparison write `:1398-1403`. Prior passes covered the registry producers, both CLI-spec producers, and `generate_sbom.sh` in full.
**Strengths.** Caller-authorization language; the clone/use distinction; one-way-door decisions handed to Plan; `--upstream-ref` pinning recording the resolved SHA; the deliberately date-free `contracts/repo-contract.json`.
**Findings.** **RE1 [P0, C1+C2]** undeclared reserved-lane writes rooted at `Path.cwd()` — **now matrix rows 25 and 26**; class A per §4. **RE2 [P0]** hard crash without `.agents/learnings/`: `_ensure_dirs` at `:1846` covers `council_dir` only — the row-25/row-26 split is exactly this fault line (Witness E). **RE3 [P0]** the security gate certifies placeholders: `validate_security_audit.sh:46-53` greps only `^Evidence:` / `^(Fix|Remediation):`, which the template supplies as literal `_TBD_`. **RE4 [P0, C2]** row 26's canned content is run-independent fabricated material in the Learn corpus. **RE5 [P1]** legacy-empty-rail, P1 under the rule. **RE6 [P1]** repo cloning is not authorization-gated; `:1612` is binary-mode only. **RE7 [P1]** archive materialization defaults on (`:1665`) despite the "index only" guardrail. **RE8 [P1]** Output Specification incomplete and mis-conditioned — replaced by §3, **publication still blocked**. **RE9 [P1]** clone-metadata three-way contradiction (§3.9). **RE10 [P1]** `scripts/validate.sh` is `py_compile` over 8 files only. **RE11 [P2, C1]** `.agents/research`. **RE12 [P2]** machine-absolute path baked into the durable wrapper (`:1420-1422`). **RE13 [P2]** degenerate golden fixture. **RE14 [P2]** network-dependent regression test. **RE15 [P2]** dead `spec-cli-surface.md.tmpl`. **RE16 [P2]** unbounded ZIP decompression (row 18's candidate branch, `:92`). **RE17 [P2]** gemini projection ships instructions without `scripts/`.
**Repairs.** P0 — confine rows 25–26 under `--output-dir` and delete the canned learning (RE1, RE4); create `learnings_dir` in `_ensure_dirs` or drop the write (RE2); reject `_TBD_` in the security gate (RE3). P1 — gate repo cloning behind `--authorized` or narrow the guardrail sentence (RE6); flip the materialization default or state it (RE7); **for RE8, do not publish §3 until an independent reviewer has checked *this* version** — the corrected specification must carry rows 10/20 as two producers of one filename, rows 3/19 as a write and a rewrite-in-place of one file, rows 25/26 as reserved-lane producers outside the declared output root, rows 18 and 24 as internal outcome tables, and an explicit statement that scratch is out of scope; fix `SKILL.md:97` (RE9); make `validate.sh` run a hermetic behavioral subset (RE10).
**Witnesses.** Repo mode from a cwd without `.agents/learnings/` → both council reports written, then `FileNotFoundError` (Witness E; Sol reproduced at v4, v5, and v6). Render `findings.md.tmpl` unmodified and run the copied validator → exits 0 on an audit with zero real findings. `--mode=repo --upstream-repo=<any>` succeeds with no authorization flag. RE9's `file://` local-origin witness. **For row 19:** assert on registry *content*, not the directory listing — a target with a real subcommand tree must yield `evidence_source: 'binary --help + string extraction'`, a target without one must leave the initial scaffold. **For row 18:** a binary with no openable ZIP must produce `extract.NOOP.md` and no `PRIMARY.txt`. **For row 20's negative branch:** the direct empty-evidence call (§0.5); if driven end to end, the injection must remove `capture_cli_help.sh` **and** `strings` from `PATH` together.
**Ledger.** RE1–RE4 / RE5–RE10 / RE11–RE17 → 4/6/7. Sol validated all 17 at v4, v5, and v6.
**Checked / not_checked.** Checked: **every durable output producer now in §3, including the two class-A rows**, with all citations re-derived; the seven scratch classes identified and summarized (§0.6); Sol's five runtime witnesses reconciled row-by-row across all 27 rows (§3.8). Not checked: **no reverse-engineer execution in this pass or in v4's, v5's, or v6's** — §3 is derived from source reading plus Sol's independently executed witnesses.
**Residual risk.** **Highest of the twelve.** Undeclared network + target execution + extraction, reserved-lane writes, and a security gate that certifies emptiness.
**Go note.** All 8 execution-path `.py` are grandfathered; the ratchet is green at head scope. Every P0 is fixable in place without a port.

### 9.6 sbh — 1 file

**Intent.** An irreversibility-ordered host-remediation specialist: status/dry-run → ballast release → cleanup of unprotected files → emergency deletion, never skipping forward while a more reversible step remains. Failure mode *wrong-mount relief*.
**RPI fit.** Outside the core — an optional host adapter. Touches no RPI artifact. Sol: supported.
**In / out / effects / authority.** In: a constrained mount + one explicit caller authorization. Out: mount, free bytes, pressure state, dry-run candidates, authorization used, exact command and exit code, bytes reclaimed, protection vetoes, checked/not-checked. Effects: §7.2 **plus deletion, which has no v3 kind** (§7.3); declared `[]`. **Widest mutation boundary of the twelve, the least declared.**
**Owned sources examined.** The single owned file in full; `docs/SKILL-API.md:147-154`.
**Strengths.** "Urgency raises the stakes of an irreversible mistake; it never lowers the authorization bar" — the clearest authorization discipline in the twelve, attached to the most destructive surface.
**Findings.** **S1 [P1]** legacy-empty-rail, P1 under the rule (host mutation + deletion). **S2 [P1]** the irreversibility ordering has **no executable witness**. **S3 [P2]** external-binary contract unpinned. **Withdrawn (§6):** the "unique missing `context:`" P2. **Refuted:** `user-invocable: false` does not contradict its triggers.
**Repairs.** P1 — declare the destructive surface truthfully, naming deletion explicitly since the v3 vocabulary cannot (S1); add a `.feature` + fixture rejecting a transcript that jumps to `emergency --yes` (S2). P2 — pin the SBH version (S3).
**Witnesses.** A fixture transcript beginning with `emergency --yes` is indistinguishable from a compliant run under every artifact in the repo today.
**Ledger.** S1, S2 / S3 → 0/2/1.
**Checked / not_checked.** Checked: the owned file; the schema's `context` handling. Not checked: **no live SBH behavior by either party.**
**Residual risk.** High in principle, low in practice at this subject — nothing in the repo can execute it, so the undeclared authority is latent.

### 9.7 scaffold — 5 files

**Intent.** Stamp one bounded scaffold and verify it once. Two differentiating disciplines: *clone a proven exemplar* and *one source of truth, one-way sync*.
**RPI fit.** Outside the core, an optional Implement method. No lifecycle authority. Sol: supported.
**In / out / effects / authority.** In: target root, language/component/CI mode, name. Out: created files, selected build/test/lint commands, exit codes, checks not run. Effects: `filesystem.read`, `filesystem.write`, `process.start`; declared `[]`. **Authority: class C3 leak in an owned reference.**
**Owned sources examined.** All 5 owned files in full; `references/generic-templates.md:189-199,353-356`; `scripts/validate.sh:5`; coverage re-executed 0/4.
**Strengths.** Exact declared write scope; no overwrite without authorization; "the result contains no verdict, lifecycle state, retry instruction, or next action."
**Findings — SC1 and SC5 split per §1.1.** **SC1a [P0, C3]** `generic-templates.md:189-199` "Step 5: Initial Commit" instructs a git commit, denied by `SKILL.md:34,79,90`. **SC1b [P0, C3]** `:353-356` prints a "Next steps:" continuation block, separately denied. **SC2 [P0]** the validator is structurally blind: `scripts/validate.sh:5` binds `SKILL="$SKILL_DIR/SKILL.md"` and every check targets that one file; **references are never scanned**. **SC3 [P1]** no scenario coverage — 0/4. **SC4 [P2]** legacy-empty-rail. **SC5a [P2]** exemplar-selection discipline has no witness. **SC5b [P2]** one-way-sync discipline has no witness. **Revised:** "`/scaffold` nonexistent" is refuted; the 0/4 fact stands.
**Repairs.** P0 — delete Step 5 (SC1a); delete the "Next steps" block (SC1b); extend the forbidden-token sweep across `references/**` (SC2). P1 — add `@covered-by` tags (SC3). P2 — declare effects; witness the two disciplines separately.
**Witnesses.** `validate.sh` → `scaffold contract: PASS` (rc=0) while `grep -n 'Initial Commit' skills/scaffold/references/generic-templates.md` → line **189**. Sol reproduced this blindness at v5 and v6.
**Ledger.** SC1a, SC1b, SC2 / SC3 / SC4, SC5a, SC5b → 3/1/3.
**Checked / not_checked.** Checked: all 5 owned files; validator and coverage executed. Not checked: no scaffold generation performed.
**Residual risk.** Moderate-high — a real authority grant behind a gate that structurally cannot see it.

### 9.8 shared — 1 file

**Intent.** A just-in-time context library that is explicitly *not* permission: "a reference read only when a consuming skill needs it cannot silently become a dependency." Failure mode *reference promotion*.
**RPI fit.** Outside the core — reference policy. "The core loop has no hard dependency on this library." Sol: supported.
**In / out / effects / authority.** None. `effects: []` is **correct**. Authority: the only skill whose entire purpose is *preventing* accidental authority — "Treat runtime and factory state as adapter evidence; never translate it into core Plan, Candidate, RPI, or verdict state."
**Owned sources examined.** The single owned file in full; `metadata.dependencies` semantics.
**Strengths.** Context is not permission; the precedence rule that source skill contracts and executable behavior outrank shared prose — the rule this entire audit operates under.
**Findings.** **SH1 [P2]** plural `produces` wording could be clarified. **Refuted:** "empty library" — the canonical `SKILL.md` **is** the policy document; "bootstrap consumes an empty supplier" — `consumes` identifies content, hard dependencies live in `metadata.dependencies`. **Note:** lacks `context:` (§6); not a finding.
**Repairs.** P2 — optionally clarify the plural wording. Nothing else is supported.
**Ledger.** SH1 → 0/0/1.
**Checked / not_checked.** Checked: the owned file in full. Not checked: nothing material outstanding.
**Residual risk.** Very low.

### 9.9 status — 2 files (+ live Go owners)

**Intent.** A snapshot in which every line traces to an artifact on disk right now. Failure mode *recency-as-activity*.
**RPI fit.** Outside the core — a read-only session/evidence view. No queues, priority, claiming, next action, repair, retry governance, or state change. Sol: supported.
**In / out / effects / authority.** In: cwd. Out: text or JSON with `intent_artifacts`, `verdict_artifacts`, `latest_kind`, `last_evidence_at`, `last_evidence_age`, `state`, `checked`, `corrupt`, `unavailable`, `not_checked`. Effects: `filesystem.read`, `clock.read` — declared correctly in the **Go** contract; frontmatter says `[]`. Authority: none — `state` is literally `<kind>_is_latest_evidence`, never a phase name.
**Owned sources examined.** Both owned files; `statusapp.go` including the missing-store path; `statusapp_test.go:59`.
**Strengths.** **The strongest content-identity discipline in the twelve** — artifact names must be valid content-addressed digests, intents are re-hashed against their filename, verdicts are version-dispatched, and anything failing lands in `corrupt` rather than the count.
**Findings.** **ST1 [P1]** prose overclaims live output: `SKILL.md:34-36` claims subject-manifest reporting plus per-artifact digests and timestamps; the command emits two counts plus one newest-mtime and hard-codes "caller-supplied subject manifests" into `NotChecked`. **ST2 [P2]** legacy-empty-rail, contradicting the command's own Go contract. **ST3 [P2]** empty `references/` held open by `.gitkeep`. **ST4 [P2]** `model: haiku` unexplained.
**Repairs.** P1 — delete the overclaimed output language or implement it (ST1). P2 — mirror the Go effect declaration once the rail is decided (ST2).
**Witnesses.** Sol's fresh HEAD-built `ao status --json` on an empty store exposed exactly `checked, intent_artifacts, not_checked, state, verdict_artifacts`, with `state: "no_evidence"`. No digest or per-artifact timestamp field exists.
**Ledger.** ST1 / ST2, ST3, ST4 → 0/1/3.
**Checked / not_checked.** Checked: both owned files, `statusapp.go`, `statusapp_test.go:59`. Not checked: Go suites not executed this pass.
**Residual risk.** Low — the implementation is more conservative than the prose.

### 9.10 test — 9 files

**Intent.** Produce *real* tests and reproducible evidence, never a plan.
**RPI fit.** Outside the core, an optional Implement method whose output is the primary evidence path into Validate. Correctly worded: "**Supply** that evidence to Validate." Sol: supported.
**In / out / effects / authority.** In: `--mode` (`generate|coverage|tdd|strategy`), `--scope`, `--min-coverage`, `--dry-run`, optional caller `.feature`. Out: `.agents/tests/{coverage-raw.txt, coverage-func.txt|coverage.json, gaps.md, summary.md, tdd-log.md, strategy.md}` plus test files in language-native locations. Effects: `filesystem.read`, `filesystem.write`, `process.start`; declared `[]`. Authority: clean separation from Validate; never issues a verdict.
**Owned sources examined.** All 9 owned files in full; both checks re-executed.
**Strengths.** Three disciplines that are the best statements of their kind in this corpus: the **oracle-strength hierarchy**, the **mutation-kill proof** (*immortal test*), and the **harness health floors** (*dead harness* — "its green is decoration").
**Findings — T4 split three ways per §1.1.** **T1 [P1]** the skill mandates `check-scenario-coverage.sh --run` as its proof mechanism and its own feature fails **0/3**; the checker is not wired as a corpus gate. **T2 [P1]** `scripts/validate.sh` proves only that five words/shapes exist in `SKILL.md` prose (5/5 pass). **T3 [P1]** `produces: [result.json]` unbacked by the Output Specification and by the tree. **T4a [P1]** oracle-strength hierarchy has no behavioral witness. **T4b [P1]** mutation-kill proof has none. **T4c [P1]** harness-health floors have none. **T5 [P2, C1]** `.agents/tests`. **T6 [P2]** `golden-artifacts.md` and `golden-artifact-strategy.md` overlap substantially. **T7 [P2]** legacy-empty-rail.
**Repairs.** P1 — wire the checker or stop mandating it (T1); replace prose greps with fixture-driven checks (T2); correct `produces` (T3); build three separate witnesses (T4a, T4b, T4c). P2 — record T5; merge or disambiguate the golden references (T6).
**Witnesses.** `check-scenario-coverage.sh --run --json skills/test/references/test.feature` → `fail 0/3`. Delete the entire Workflow section from `SKILL.md` while leaving the words "tdd" and "coverage" — `validate.sh` stays green.
**Ledger.** T1, T2, T3, T4a, T4b, T4c / T5, T6, T7 → 0/6/3.
**Checked / not_checked.** Checked: all 9 owned files; both checks executed. Not checked: no test generation performed.
**Residual risk.** Moderate — the corpus's best testing doctrine is entirely unenforced, including against itself.

### 9.11 using-gc — 1 file

**Intent.** Orchestrate a standing Mayor session as a dispatch shepherd through two doors, and read completion from bead/verdict state rather than pane prose.
**RPI fit.** Outside the core — an optional, explicitly caller-selected runtime adapter, dispositioned `keep_optional_adapter`, aligned with ADR-0015. Sol: supported; GC close is not AgentOps completion.
**In / out / effects / authority.** In: an explicitly selected city, caller-authored source beads, bead ids. Out: runtime evidence per supplied packet. Effects: §7.2 including **conditional `host.configure`** (`:78-83`). Declared `[operate_gas_city]` — **the only non-empty `effects` array of the twelve**, and a legal legacy string. **Authority correctly refused, and the model statement of the twelve** — "A GC `close` is **not** an AgentOps completion. A fresh GC judge may supply evidence to Validate; only Validate writes the verdict."
**Owned sources examined.** The single owned file in full, including `:78-83` verbatim.
**Strengths.** GC close ≠ AgentOps completion; no hidden retries; the **liveness truth stack** ("when the roster says active but the pane is wedged, the pane wins"); the **stall protocol**; hand the Mayor **bead ids only, never prose**; "explicit selection only — an available substrate is not a selected one."
**Findings — U1 split two ways per §1.1.** **U1a [P1]** the dispatch-once discipline has no AgentOps-owned behavioral witness. **U1b [P1]** the one-wake-maximum discipline has none either. **U2 [P2]** version-pinned external claims with no local guard. **U3 [P2]** `tmux -L <socket>` appears at three call sites without naming that the socket comes from `mayor status`. **Refuted:** "ad-hoc effect token fails the v3 enum" — legacy `metadata.effects` accepts arbitrary nonempty strings.
**Repairs.** P1 — add a `.feature` + `@covered-by` for "re-dispatch of an `in_progress` bead is a no-op" (U1a) and a separate one for "one wake maximum, then stop" (U1b). P2 — add a version/compatibility probe (U2); name the socket source once (U3). Also declare the conditional host configuration.
**Ledger.** U1a, U1b / U2, U3 → 0/2/2.
**Checked / not_checked.** Checked: the owned file in full. Not checked: **no live Gas City behavior by any party.** The v1.3.5 and issue-#4586 facts are carried from the v4 Sol primary-source check; that provenance is disclosed.
**Residual risk.** Moderate and external — correctness depends on an upstream this repo cannot observe.

### 9.12 workflow-builder — 1 file

**Intent.** Author thin, at-most-once dispatch adapters and keep them thin: "a workflow that cannot retry or select work cannot compound a failure, so the worst case is one reported error per operation." Failure mode *framework gravity*.
**RPI fit.** Outside the core — a meta/authoring tool. It builds optional one-shot adapters; it is not itself a loop move or an adapter. Sol: supported.
**In / out / effects / authority.** In: explicitly supplied inputs, executors, write scopes, outputs. Out: a runnable script plus a dry-run or fixture demonstrating exact dispatch count and failure reporting. Effects: `filesystem.write`; declared `[]`. **Authority: the most complete non-authority enumeration of the twelve.**
**Owned sources examined.** The single owned file in full.
**Strengths.** The caller-proven-disjoint write-scope rule for parallel operations, which correctly restates `AGENTS.md` §Concurrency at the adapter layer; at-most-once dispatch stated as the reason failures cannot compound.
**Findings.** **W1 [P2]** legacy-empty-rail. **W2 [P2]** the at-most-once invariant has no witness. **Refuted:** "missing self-exemplar" — `SKILL.md:62-63` requires a fixture in each *generated* workflow; "silent about AgentOps workflow internals" — a deliberate generic target-runtime adapter.
**Repairs.** P2 — declare the write (W1); add an at-most-once witness (W2).
**Ledger.** W1, W2 → 0/0/2.
**Checked / not_checked.** Checked: the owned file in full. Not checked: no workflow authored or executed.
**Residual risk.** Low.

---

## 10. Checked / not_checked

### Checked
- Both inputs read in full and SHA-verified at open; digests unchanged at close.
- Exact HEAD/tree/clean status at open and close.
- **H1 re-derived**: the class-A block `:1843-1873` in full — `council_dir` `:1845`, `_ensure_dirs` `:1846` *(council only)*, paths `:1847-1848`, both `_render_template` calls `:1850-1851`, the learning path `:1853`, its existence guard `:1854`, its inline write `:1855-1871`, and `return 0` `:1873`.
- **H2**: witnesses **A-D** reconciled across all 27 rows (§3.8), with row 27 classified `not applicable` in every script-only witness and the both-mode table translated exactly from Sol's numbering. **v7's claim that all *five* witnesses were reconciled row-by-row was false** - Witness E classified only rows 25-27 and mislabelled rows 1-24 "not reached." Corrected in this pass; see **W1** below.
- **W1 re-derived (v8)**: `reverse_engineer.py:1840-1875` read verbatim. `_ensure_dirs([council_dir])` at `:1846` precedes the row-25 renders at `:1850-1851`; the row-26 `write_text` at `:1855` is the **last durable statement** before `return 0` at `:1873`. Rows 1-24 therefore execute ahead of the failure and are directly classifiable. Witness E's full state recorded: **11 present + 14 absent + 1 failed + 1 not applicable = 27**.
- **W2 re-derived (v8)**: row 26's three-way live control flow - `:1853` path construction, `:1854` `not ...exists()` guard, `:1855` `write_text`. **`grep -n learnings` over the entire script returns exactly one hit, `:1853`**, proving the parent is never created (**RE2**). **`ls -d .agents/learnings` at the bound tree returns nothing**, so the `FileNotFoundError` branch is the default state of a fresh checkout.
- **W3 re-derived (v8)**: `cli/cmd/ao/handoff.go:125-152` read in full - `os.CreateTemp` `:131`, `tmpName := tmp.Name()` `:135`, deferred `os.Remove(tmpName)` `:136`, write `:137`, `Sync` `:141`, `Close` `:145`, `os.Rename(tmpName, target)` `:148`, `return target, nil` `:151`.
- **v3 vocabulary re-confirmed (v8)**: `grep -c "filesystem.delete" schemas/skill-contract.v3.schema.json` -> **0**. The deletion gap is live, not carried.
- **Whole-file sweep (v8)**: every Witness-E reference, every row-26 condition statement, every handoff temp/effect statement, and every severity and count claim in this document re-read and reconciled against the corrections above.
- **H3 re-derived**: `extract_embedded_archives.py` — `if not cands:` `:79`, NOOP write `:80-83`, `return 0` `:84`, candidate path `:86-109`; caller `:1665-1681` including the `PRIMARY.txt` check at `:1680` and the `analysis_root` redirect at `:1681`.
- **H4 re-derived**: `capture_cli_help.sh:45` unconditional creation; `:1634-1639` `capture_script.exists()` guard with `check=False`; `analyze_binary.sh:169-171` `command -v strings` guard around the redirection; `analyze_binary.sh:2` `set -euo pipefail` and its exit-0 behavior without `strings`; `:1619-1628` `_ensure_dirs` + `check=True`.
- **H5 re-derived**: `validate_security_audit.sh:55-61` scanner resolution and invocation; `scan_secrets.sh:31-32` `mktemp -t re_rpi_secrets.XXXXXX` with EXIT trap.
- **H6**: `comparison-report.md` bound at `:1400`, written at `:1401`.
- Per-skill tracked-path counts and the 70 total: `2,3,2,3,40,1,5,1,2,9,1,1`.
- v2–v6 confirmed classes preserved (§2), each independently confirmed by Sol.

### Not checked
- **No semantic verdict.** This is an audit correction, not a `verdict.v2`, and **no PASS is claimed**.
- **No reverse-engineer execution in this pass.** §3 is derived from source reading plus Sol's independently executed witnesses. No network, clone, binary execution, archive extraction, SBOM generation, or fuzzing; no bytecode created.
- **No skill was run, no projection generated, no generator/formatter/sync executed.**
- **No live `sbh` behavior**; **no live Gas City behavior** (v1.3.5 facts carried from the v4 Sol check).
- **No Go test suite executed** this pass; **no `ao` subcommand was run** this pass.
- The 70-file digest appendix was **not re-derived** here — preserved on Sol's independent v6 confirmation (`missing=0, extra=0, mismatch=0`).
- **Row 18's candidate branch** (archive actually materialized) has no witness by either party — only the `extract.NOOP.md` branch was observed. No archive was materialized anywhere.
- **Row 20's negative branch** has been exercised only as a **direct call** (Sol: `return=False`, `spec_exists=False`). The end-to-end conjunction described in §0.5 was **not** executed by either party.
- **Row 24's successful real-`syft` path** is source-derived; Sol's success probe used a local `true` as a fake `syft`. The Go side branch is now witnessed via a fake failing `go`.
- Generated projections were not used as authority.
- The other 37 skills are out of scope; no proposed repair was implemented or CI-tested.
- **Pre-existing untracked `__pycache__/` directories** from earlier passes remain; they are gitignored, keep porcelain at 0, and this pass created none.
- **No witness was re-executed in this pass.** Witnesses A-E are carried from Sol's independent v7 reproduction. This pass re-derived their *classifications* from source control flow; it did not re-run `reverse_engineer.py` in any mode.
- **No handoff write was executed.** The failure-path deletion at `handoff.go:136` is **source-established, not observed** - no run was induced to fail between `os.CreateTemp` and `os.Rename`.
- **The 72-row ledger was not recomputed from scratch** in this pass. It is preserved on Sol's independent recomputation at v7 (`P0=13, P1=25, P2=34, total=72`), which this pass does not disturb.

---

## 11. Residual risk

1. **Contract-migration ambiguity remains the largest standing risk.** Legacy string metadata, the optional shadow v3 rail, accepted-but-unoperationalized ADR-0016 state policy, and live writer/doctor behavior are not one coherent executable contract.
2. **ADR-0016's closed layout is accepted but inert.** The detector is unimplemented and `ao init` itself mints `.agents/handoff`. Recording the six conflicts is correct; treating them as live gate failures would repeat v1's error.
3. **The v3 effect vocabulary cannot express deletion, rewrite-in-place, or escape from the declared output root.** §7.3 names all three. The third is why RE1 needs prose to state what the effect declaration structurally cannot.
4. **The matrix is complete for the durable producers identified to date, and is not proven exhaustive.** Three residuals remain, all narrower than v6's: **row 18's candidate branch** (no archive was materialized by either party), **row 20's negative branch end to end** (only the direct call was executed), and **row 24's successful real-`syft` path** (probed with a fake). Row 21 and the Go side branch are now discharged. A reviewer evaluating RE8 should verify the three remaining items first.
5. **Four producer-shape errors have now survived into successive revisions, and the failure mode has shifted every time.** v3 collapsed the CLI-spec producers into a catch-all. v4 kept them collapsed while *describing* the second producer in prose. v5 separated them but missed a producer that **rewrites a file another producer already created** — invisible to directory-listing reconciliation. v6 added that row and still omitted **producers writing outside `--output-dir`**, because the reconciliation only ever listed the output directory. The generalizable rule, now stated as a checklist for any completeness claim: enumerate producers by **walking the call sequence**, then check the result against (a) which files exist, (b) what those files contain, and (c) **every root written to, not only the declared one**. Each successive miss was in a blind spot the previous method could not see.
6. **The finding count is a convention, not a measurement.** 13/25/34 follows from the stated repair-unit rule applied to explicit rows. A reader who bundles H1 back into one finding gets 11 P0. Sol recomputed and confirmed the arithmetic at v4, v5, and v6.
7. **A fifth failure mode appeared at v7: an over-claimed reconciliation.** v3-v6 each missed a *producer*; v7 found every producer but **asserted witness coverage it did not have**, labelling 24 executed rows "not reached." That is a different class of error - not an incomplete enumeration but an unearned completeness claim. The checklist in item 5 must therefore be extended: after enumerating producers, **state per witness which rows it actually classifies, and never let a summary sentence assert coverage a table does not show**.
8. **This document is unvalidated, and RE8 depends on that.** §3 must not be published as the Output Specification on this document's own assessment of its completeness — that assessment has now been wrong four times - three times on completeness, once on witness coverage.

---

## 12. Closing subject state

```text
HEAD   : 0088c6e3824da201eabb1e751ac8e976599e0b5c   (unchanged)
tree   : c0c43eefb8042af5a6a7877c0f7f0de80149ffc6   (unchanged)
status : clean — git status --porcelain=v1 returned 0 lines
```

**Repository identity re-verified at close**, after every correction in this document was written: HEAD `0088c6e3824da201eabb1e751ac8e976599e0b5c`, tree `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`, `git status --porcelain=v1` -> 0 lines. Identical to open.

No repository file was created, edited, or deleted. No generation, projection, commit, merge, push, tag, or verdict. No skill was executed - productive, destructive, or otherwise. All executed checks were read-only source reads, `git ls-tree`, `ls`, and `grep`/`sed` inspection; scratch work stayed outside the worktree. **All four inputs preserved unchanged at their stated digests.**

**This document remains advisory and binds no semantic verdict.** It is not a `verdict.v2` or `verdict.v3`, claims no PASS, and confers no implementation authority.

*Written once, atomically, to `/tmp/agentops-opus5-verified-skill-audit-workflow-12-v8.md`. Its whole-file SHA-256, byte count, and line count are computed after the final byte and supplied alongside - a file cannot contain its own digest.*

**Full provenance chain, all preserved unchanged at their stated digests:**

| Artifact | SHA-256 |
|---|---|
| `...-v3.md` | `59c87b63...` |
| `...-v3-review-sol.md` | `abff70ea...` |
| `...-v4.md` | `b93f36c8...` |
| `...-v4-review-sol.md` | `c2e3a0e5...` |
| `...-v5.md` | `71f060d9...` |
| `...-v5-review-sol.md` | `f4cc66ae...` |
| `...-v6.md` | `0515700b6afda3fbad71b00ed971bce9357a5fcee94c488e80070c935cee61c9` |
| `...-v6-review-sol.md` | `a29ff4a4f1522705ff6b1c5a80f6251b4e9d98750ecf671018fd8f280734e78c` |
| **`...-v7.md`** | **`ec07309d243bf3e86319dc07d22da70eda2bb93a21c0f6732868fd80a200dca8`** |
| **`...-v7-review-sol.md`** | **`9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9`** |
| **`...-v8.md`** | **this document - digest reported alongside** |
