# Opus 5 — adapters-13 deep audit, v14 (provenance-order reseal: RC-3 repaired; no technical content changed)

**Skills:** `account-rotation, agent-mail, agent-native, agy-native, cass, cc-hooks, codex-exec, converter, dcg, ms, ntm, rch, swarm`

| | |
|---|---|
| **Landing** | `/Users/bo/dev/agentops-worktrees/skill-overhaul` |
| **Branch** | `codex/skill-overhaul-20260724` |
| **Opening HEAD / tree / status** | `0088c6e3824da201eabb1e751ac8e976599e0b5c` / `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6` / clean, 0 porcelain |
| **Closing HEAD / tree / status** | identical — verified in §12 |
| **Mutations to the landing** | none. No edit, commit, merge, push, staged change, or generated artifact |
| **Date** | 2026-07-28 |

## Inputs read in full and SHA-verified

| Artifact | Declared SHA-256 | Verified | Lines |
|---|---|---|---|
| **v8 audit `…-adapters-13-v8.md` — direct input to v9** | `dcce5130dc6621133d21bb3c8868fb676af745605ba05ab5f6f9dd9d15b1152a` | **match** | 797 |
| **v8 Sol review `…-v8-review-sol.md` — direct input to v9** | `e2476f12ef1333ea2acb85d5b8044ef200db3cc2a74be5394881fc65c65b7d88` | **match** | 528 |
| v7 audit `…-adapters-13-v7.md` — **the direct input to v8** | `eb3fedd825ba8ee2e87f4e276cbb3affb6dd6e5ce69dd4454eee6873ef025ac1` | **match** | 775 |
| v7 Sol review `…-v7-review-sol.md` — **the direct input to v8** | `e69e9d296c6ba024948705323529b5cb343a4d550683e80635b299de129d6920` | **match** | 464 |
| v6 audit `…-adapters-13-v6.md` — **the direct input to v7, and the pass whose Trials A/B/C produced the 123 cass command executions cited throughout** | `307bea30d4a5cc1a8c70b260a44df61bd5c8f375a3864f891f2dd9466ae1cbcb` | **match** | 730 |
| v6 Sol review `…-v6-review-sol.md` — **the direct input to v7** | `adb147b732b82e2d9dd39c121868acd14c4755ce223146e2ce64b7f1f14766b0` | **match** | 419 |
| v5 audit `…-adapters-13-v5.md` (history) | `a02c387623f4ddd9509d65fb6ba630ac4c4deafb1afd312a10e1cf2e157540bb` | **match** | 724 |
| v5 Sol review `…-v5-review-sol.md` (history) | `71b1f36ae588a985eb0c5a1394c458807d3f9f7bc0c6cac1e0b261f8bd2f3998` | **match** | 384 |

All eight digests above were recomputed at v10 against the files on disk and all eight matched. **None was accepted on any predecessor's word.** The eight split into two provenance classes, and v9's blanket phrase "the values v8 declared" was false for two of them:

| Class | Count | Artifacts | Who declared the value |
|---|---:|---|---|
| **Declared by the v8 audit** | **6** | v7 audit `eb3fedd8…`, v7 Sol review `e69e9d29…`, v6 audit `307bea30…`, v6 Sol review `adb147b7…`, v5 audit `a02c3876…`, v5 Sol review `71b1f36a…` | each value appears verbatim in the v8 audit and was matched against the file on disk |
| **v9/v10 direct-input identities, independently computed** | **2** | v8 audit `dcce5130…152a`, v8 Sol review `e2476f12…5d88` | **neither appears anywhere in the v8 audit** — verified by exact search, 0 occurrences each. A file cannot declare its own whole-file digest before it is written, and it cannot declare the digest of a review authored after it. These two were supplied as direct inputs and computed independently. |

All predecessors are preserved unchanged. **No PASS is claimed. This is advisory evidence, not a `verdict.v2`.**

---

## 0.0 v13 → v14 — provenance-order reseal *(v14, current)*

**v14 repairs exactly one report-integrity defect and changes no technical content whatsoever.**
It is the response to the fresh v13 Sol review
(`86e8b566998972f9e9c0277036ab9463e119d341cc9ba469b8216727f2290571`, 171 lines / 13,619 bytes,
mode `0444`), whose disposition is **`REQUEST_CHANGES` solely on RC-3**. That review records that
the two v12-raised defects are repaired and that **every technical criterion reproduces** — all
thirteen analyses, the v6 attribution and arithmetic for the CASS trials, the 78-file inventory,
and the `5 / 34 / 39 = 78` finding ledger.

- **RC-3.** v13's closing provenance paragraph was introduced as *"Provenance chain, newest
  first"* but did not present one. It opened at **v12 → v11**, inserted a sentence about **v13**
  mid-paragraph, then resumed at **v11 → v10** — an order of v12, v13, v11 — and it never opened
  with the then-current relation **v13 → v12**. The label and the structure disagreed.

**The repair.** The paragraph is rewritten so the literal sequence is newest-first and complete:
**v14 → v13 → v12 → v11 → v10 → v9 → v8 → v7 → v6 → v5**, opening with the current relation and
pinning the exact digest, extent, and mode of both v13 and the v13 Sol review. Nothing else in the
paragraph's factual content is altered; the earlier links carry their existing citations.

**Nothing else changed.** All **13 per-skill technical judgments**, the exact **CASS v6 execution
attribution**, the owned-file inventory, and the **5 P0 / 34 P1 / 39 P2 = 78** finding ledger are
byte-preserved from v13. No finding, severity, count, owner classification, projection result,
evidence item, limitation, or correction-history row moved.

**What v14 did not do.** v14 **executed no technical witness, issued no cass command, and ran no
repository evidence of any kind** — not even the three read-only Git queries v12 recorded. It read
v13 and the v13 Sol review, verified both digests and extents, and rewrote one paragraph. It
therefore **inherits, and does not re-establish, every technical criterion**; those remain
attributed to the passes that executed them and to the reviews that reproduced them.

---

## 0.1 v12 → v13 — report-integrity seal *(history: the v13 revision)*

**v13 is a two-defect report-integrity seal.** It repairs exactly the two items the v12 Sol review raised and advances current metadata. *(v13 also declared itself the final revision of this lineage. The fresh v13 Sol review then raised **RC-3**, a third report-integrity defect, so that claim did not hold and **v14** — this document — is the response. See §0.0.)* **v13 re-ran no technical witness and no CASS evidence**, and edited no repository, Git, or predecessor artifact.

- **RC-1.** v12's *Checked — added at v12* block said v11's byte count and digest "reproduced the values supplied to **this pass**" and that v11's line count "was computed by **this pass**" — an operative current-pass label that falsified v12's own exclusive whole-file sweep result. **v13 states these as `supplied to v12` and `computed at v12`**, which is what actually happened, and the sweep claim is thereby exact.
- **RC-2.** v12's §1.6 (the v5 → v6 history block) carried stale bookkeeping: it asserted a **current (v11)** revision, pointed at §1.0 as that revision's corrections, and referred to itself as "**this §1.5 block**" while physically sitting in §1.6. **v13 rewrites the passage to versioned history only** — every generation named by version, the §1.6 self-reference corrected, and no current-revision claim of any kind.

Nothing else changed. All **13 per-skill technical judgments**, the exact **CASS v6 execution attribution**, the owned-file inventory, and the **5 P0 / 34 P1 / 39 P2 = 78** ledger are preserved byte-for-byte.

---

## 0.2 v11 → v12 — report-integrity reseal *(history: the v11 → v12 revision)*

**Scope of the v12 pass, stated as history.** v12 was a **report-integrity reseal**. It
corrects point-of-use provenance defects in v11's own text and changes **nothing** technical: no
count, finding ID, severity, per-skill analysis, evidence item, limitation, inventory figure, owner
classification, projection result, or correction-history row differs from v11. **No repository
witness was re-run and no cass command was issued.** No repository file, Git state, predecessor
artifact, or review was edited.

**Direct inputs, SHA-verified at open:**

| Input | SHA-256 | Extent |
|---|---|---|
| **v13 audit** `…-adapters-13-v13.md` — **direct input to v14** | `1d1709a3b5be5557e51776e94a0c11612028cf406cc16b74d8116ce8b335d3f5` | **1,146 lines / 138,200 bytes**, mode `0400` |
| **v13 Sol review** `…-v13-review-sol.md` — **the correction authority for v14** | `86e8b566998972f9e9c0277036ab9463e119d341cc9ba469b8216727f2290571` | 171 lines / 13,619 bytes, mode `0444` |
| v12 audit `…-adapters-13-v12.md` *(lineage; direct input to v13)* | `74224c9f64b4050632395a91cdd23e6a032a5aaccdb1e690efcdac17e06c83d5` | 1,133 lines / 136,158 bytes |
| v12 Sol review `…-v12-review-sol.md` *(lineage; the correction authority for v13)* | `dc5cb34fa623b44f67ea7bcd688c0d0679626c86869b8bd94e83d17d12a2ac53` | 180 lines / 13,175 bytes |
| v11 audit `…-adapters-13-v11.md` *(lineage; direct input to v12)* | `c4ee8551cebfc321f91fe2a84b9e65d4d2385722e58773fe127e863eca38cec2` | 1,032 lines / 127,877 bytes |
| v10 Sol review `…-v10-review-sol.md` *(lineage; direct input to v11 and v12)* | `f13581e4ad14974fe2c84a4a0b3f5e8a27562352d57da071b71574762373eaf0` | 223 lines / 15,574 bytes |

The two v14 inputs were read in full and both digests and extents were recomputed and matched at open. **v14 is the response to a binding fresh review** — the v13 Sol review — not an author-lane initiative. *(The lineage note that there is no Sol review of v11 remains true and is retained in §0.2's history.)*

**Defects corrected, each at its point of use:**

| # | Defect in v11 | v11 said | v12 says |
|---|---|---|---|
| **R-1** | The atomic-write footer named the wrong output file — an artifact cannot truthfully name a predecessor as its own write target | *"Written once, atomically, to `…-adapters-13-v10.md`"* | Names `…-adapters-13-v12.md`, the file actually written, and adds byte count to the post-write reporting list |
| **R-2** | The close-state prose attributed a snapshot to the wrong pass and compared against a predecessor's opening snapshot | *"Re-verified at the close of **v9** …"*; *"Identical to **v9's own opening snapshot** …"*; *"**v9 ran no witness** …"* | States **exactly** what the v12 pass measured: one read-only `git rev-parse HEAD`, `git rev-parse HEAD^{tree}`, and `git status --porcelain`, and nothing more. Earlier snapshots are attributed to v8, v9, v10, and v11 as **their** measurements |
| **R-3** | The provenance chain began at v11 and could not carry v11's post-write identity, which did not exist when v11 was written | *"This **v11** supersedes the **v10 audit** …"* | Begins with **v12 superseding v11** at v11's exact post-write digest and extent — `c4ee8551…cec2`, 1,032 lines / 127,877 bytes — computed after v11 was sealed |
| **R-4** | Point-of-use provenance still attached predecessor work to an unnamed "this" | *"direct input to **this v9**"* ×2; *"found by **this pass's** own end-to-end sweep"*; *"read for new evidence in **this pass**"*; *"**This revision** is wording and structure only"* | Every site names its version: `v9`, `the v11 pass`, `at v11`. The phrase "this pass" now appears operatively **only** in this §0.1, where it means v12 |
| **R-5** | One operative incidence adjective survived outside CASS prose | *"the **common** `rm -rf \"$dir\"` idiom"* (§10.9 DCG) | *"the **unresolved-variable** `rm -rf \"$dir\"` form"* — a structural description with no frequency content |
| **R-6** | The closing disclaimer named v11 | *"v11 has not been independently reviewed"* | Names **v12**, and states plainly that v12 claims no `verdict.v2`, no PASS, and no validation, and that it **inherits but does not re-establish** the technical criteria the v10 Sol review passed |

**Preserved from v11 without change.** Every CASS datum and its v6 attribution; the exact-count and
no-frequency corrections the v10 Sol review required; the `CASS-2` refutation and uncounted
disposition; the `5 P0 / 34 P1 / 39 P2 = 78` ledger; the 78-file / 14,015-line / 466,080-byte
inventory with per-skill vector `1,4,3,1,24,15,1,4,9,4,1,8,3`; the six live owners and their line
counts; the selected AGY surface; `CV-8`; the installed DCG matrix; Codex projection ownership; all
thirteen one-by-one intent/RPI/live-behavior judgments; and every §1 correction-history row.

### 0.2 v12 whole-file sweep — method and result

Run over the **final v12 bytes**, not a predecessor:

| Sweep | Result |
|---|---|
| Operative `this pass` / `this revision` / `current pass` / `fresh` attributing predecessor work to v12 | **zero**. The phrase appears operatively only in §0.1 (meaning v12); every other occurrence is version-named or inside a labelled rejected quotation |
| Operative qualitative incidence claim (`most`, `several`, `often`, `usually`, `rare`, `common`, `frequent`, …) | **zero**. Surviving hits are the sweep vocabulary itself, labelled rejected predecessor quotations, explicit negations (*"no proportion, majority, modal value, or rate"*), and three non-incidence superlatives ranked over the named thirteen-skill corpus — the four nonoperative classes enumerated in §1.1 |
| Current-version self-label attributing predecessor work to v12 | **zero** |
| Markdown table structure — header, separator, and adjacent rows with nothing interposed | **zero orphan rows, zero header/separator pairs without rows** |
| Headings or prose stranded inside a table block | **zero** |

---

## 1. Correction ledger

### 1.0 v10 → v11 — **history**: the corrections made in the **v11** revision

Direct inputs: the **v10 audit** (`5724141e32759c0ee5f4fea802c3fb95cf3216be9f64f6cf7b19b4296eba63b0`,
907 lines / 115,140 bytes) and the **v10 Sol review**
(`f13581e4ad14974fe2c84a4a0b3f5e8a27562352d57da071b71574762373eaf0`, 223 lines / 15,574 bytes),
both SHA-verified at open and again immediately before this file was written.

**The v10 Sol review returned `REQUEST_CHANGES` on exactly two report-level defects** and stated
that neither repair "changes a CASS datum, source trial, finding ID or severity, inventory count,
owner classification, projection result, or per-skill judgment." **This v11 revision executed no
command trial and produced no new receipt.** Every CASS number below is the number v10 already
carried, which is the number the **v6 authoring pass** recorded.

| # | v10 Sol defect | v10 said | v11 says | Basis re-derived in **this (v11)** pass |
|---|---|---|---|---|
| **RC-1** | Operative qualitative CASS incidence wording survived the v10 sweep at lines 250, 252, and 306, contradicting v10's own lines 63, 834, and 861 | "of the 37 differing pairs: **most** differed in `budget.elapsed_ms` alone"; "**several** differed in all four"; "**most** cross-form pairs differed in **one** field" | Every operative incidence word is replaced by the **exact recorded counts** where the retained evidence derives them, and by an **existential statement plus an explicit non-retention disclosure** where it does not. The retained Trial B record supports exactly: **3 of 40 byte-identical**, **37 of 40 differing**, and the exact field shape of **pair 11**. The per-pair field-difference distribution across the other 36 differing pairs **was not retained by the v6 authoring pass and is not reconstructible**, so no proportion over them is asserted. Two further hedges — "occurred in **some** recorded samples" and "appeared in **some** recorded samples" — are likewise replaced by `3 of 40` / `37 of 40`. The v7→v8 row's "**Several** CASS summaries" is replaced by the exact enumerated count | Widened vocabulary swept over the **final v11 bytes**, not a predecessor (§1.1). Trial B's retained record is §2.2's fenced block: three named byte-identical pairs (6, 33, 38) and pair 11's exact three-field shape |
| **RC-2** | The current-versus-history split was internally false: line 40 headed the v8→v9 block "in **this** revision"; line 45 continued "**This revision**"; line 133 said "**This revision's** own corrections are RC-1 and RC-2 in §1.0"; and the new v10 ledger was inserted **between** the v8→v9 table's separator and its rows, so neither rendered as one table | see the three cited sites, and the split table at v10 lines 50–51 / 69–70 | **Every historical block now names its originating version explicitly.** §1.0 is this v11 ledger; §1.2 is v10's; §1.3 is v9's; §1.4 is v9's sweep; §1.5 is v8's; §1.6 is v6's; §1.7 is the v3→v5 chain. The phrase "this revision" / "this pass" appears in an operative sentence **only** where it means v11, and every historical occurrence is either renamed to its version or quoted inside a labelled withdrawn quotation. **Each table is contiguous** — header, separator, and rows adjacent with nothing interposed — and the current ledger is structurally outside every historical table | Mechanical table-structure scan over the final v11 bytes: every header+separator has adjacent rows, and no orphan row exists (§1.1) |

**Nothing else changed.** Every CASS datum and source trial, the v6 attribution of Trials A/B/C, the
`CASS-2` refutation and its uncounted disposition, the `5 P0 / 34 P1 / 39 P2 = 78` ledger, the
78-file / 14,015-line / 466,080-byte inventory with per-skill vector `1,4,3,1,24,15,1,4,9,4,1,8,3`,
the six live owners and their line counts, the selected AGY surface, `CV-8`, the installed DCG
matrix, Codex projection ownership, and all thirteen one-by-one intent/RPI/live-behavior judgments
are **preserved exactly** as the v10 Sol review passed them.

---

### 1.1 Whole-file sweep executed for **v11** — method, result, and the allowed quotations

**Method.** The widened vocabulary was run with `re.finditer` over **the final v11 bytes**, not
over a predecessor. Each pattern is matched standalone with non-letter boundaries on both sides, so
`most` matches on its own and is not hidden inside `mostly`:

```text
most · several · many · few · some · routinely · readily · sometimes · often · frequently ·
typically · usually · rarely · occasionally · commonly · generally · normally · mostly ·
seldom · regularly · ordinarily · habitually · customarily · as a rule · more often ·
most of the time · majority · minority · predominantly · largely · in general ·
tends to · tend to · almost always · nearly always
```

A second scan covered `this revision` / `this pass`, and a third checked every Markdown table for a
contiguous header, separator, and at least one adjacent row with no interposed content.

**Result — operative CASS incidence claims: zero.** Every hit in the final file falls into exactly
one of four nonoperative classes, enumerated here so a reader can reproduce the triage rather than
trust it:

| Class | Why it is nonoperative | Sites |
|---|---|---|
| **A — the sweep vocabulary itself** | The pattern list above and the v9 pattern list in §1.6 are *the search terms*. They assert nothing about any sample. | §1.1 vocabulary block; §1.4 pattern list |
| **B — labelled withdrawn quotation, version-attributed** | Text quoted from a named predecessor inside a correction row whose `said` column is explicitly the rejected wording. Each is attributed to the version that wrote it and is marked as replaced. | §1.0 RC-1 quoting **v10**'s "most"/"several"; §1.3 RC-1 quoting **v8**'s "readily"/"sometimes"; §1.2 RC-1 quoting **v9**'s "fails routinely"; §1.5 SC-1 quoting **v7**; the checked-list entry re-deriving the v9 "routinely" site |
| **C — non-incidence superlative over a named finite set** | `the most precise …`, `the most disciplined …`, `the most reusable …` rank one item against an explicitly bounded comparison set (this thirteen-skill corpus). A superlative over a named set is an ordering claim, not a frequency, rate, proportion, or likelihood claim about repeated trials. No CASS sentence uses one. | three per-skill "Strong." sentences in §10 |
| **D — regex literal** | `sometimes` occurring inside a quoted regular expression in the v9 method description. | §1.4 |

**No class-A/B/C/D hit is a CASS incidence claim, and no CASS sentence anywhere in this file
contains an incidence word.** The three v10 sites the v10 Sol review named — its lines 250, 252,
and 306 — are replaced with exact counts or with existential statements plus an explicit
non-retention disclosure (§1.0 RC-1). Three further hedges v10 carried — "occurred in **some** recorded samples", "appeared in
**some** recorded samples", and "occurred in **some** recorded samples and not in others" — are
replaced with the exact `3 of 40` / `37 of 40` counts. **The third was not named by the v10 Sol
review**; it sits in the §9.7 refuted-claims table and was found by the **v11** pass's own
end-to-end sweep of the final bytes.

**What the retained CASS evidence does and does not support.** The v6 authoring pass retained, for
Trial B, exactly: **40 matched cross-form pairs**, **3 byte-identical** (pairs 6, 33, 38, each with
its digest), **37 differing**, and **the exact field shape of pair 11**. It did **not** retain the
per-pair field-difference distribution across the other 36 differing pairs. This file therefore
asserts existence — *at least one differing pair differed in `budget.elapsed_ms` alone; at least one
differed in all four* — and **no proportion, majority, modal value, or rate over the 37**. That
limit is a property of the retained record, not a hedge.

**Result — current-versus-history attribution, as measured by the v11 pass.** At v11, `this
revision` / `this pass` appeared in an operative sentence only in §1.0, where it then meant
**v11**. *(Superseded at v12: §1.0 is now labelled history, and the only operative occurrence in
this file is in §0.1, where it means **v12**.)* Every historical block names its originating
version in its own heading: §1.2 → v10, §1.3 → v9, §1.4 → v9's sweep, §1.5 → v8, §1.6 → v6,
§1.7 → the v3→v5 chain. Remaining occurrences of the phrase are inside class-B withdrawn
quotations, where the quoted text is exactly the wording being rejected and the row names whose
wording it was.

**Result — table structure.** Every Markdown table in the final file has a header, a separator, and
at least one adjacent row, with nothing interposed; there are **zero orphan rows** and **zero
header/separator pairs without rows**. The current v10 → v11 ledger (§1.0) is a structurally
separate section that precedes every historical ledger; no current-correction heading sits inside a
historical table.

---

### 1.2 v9 → v10 — **history**: the two corrections made in the **v10** revision

The v9 Sol review (`47a290749a409c1d57690e4ce0fe054917966e633db7b92ac214d4e7dae00a82`, 301 lines / 20,108 bytes) returned **REQUEST_CHANGES** on **two narrow report-level defects** and stated explicitly that neither repair "changes a CASS datum, `CASS-2`'s refuted/uncounted disposition, an ID, a severity, an inventory count, an owner classification, or a per-skill judgment." Both were corrected in v10, each independently re-derived in the **v10** pass.

| # | v9 Sol defect | v9 said | v10 said | Independent basis, re-derived in the **v10** pass |
|---|---|---|---|---|
| **RC-1** | A qualitative frequency claim survived at line 259 | "byte identity is achievable, has been observed by both parties, **and also fails routinely**" | The sentence now states **only the observed sample facts** and the exact recorded counts (Trial B 3/40 equal, 37/40 differing; Trial A 40 distinct), and explicitly asserts **no frequency, rate, typicality, or likelihood** | `sed -n 259p` on the v9 subject reproduced the exact sentence in the v10 pass. The word conflicted with v9's own line 45 (which lists *frequency* among the rejected claim classes) and line 824 ("they do not establish a frequency"). |
| **RC-2** | Line 27 attributed all eight digests to "the values v8 declared"; the closing line generalized over the whole chain | "all eight matched the values v8 declared"; "**No pass in this chain** is described as having executed work it inherited" | Line 27 now **splits the eight by declarer** — six declared in the v8 audit, two independently computed direct-input identities. The closing sentence is **narrowed to this v10 summary**, with predecessor misattributions retained only as labelled correction history | Exact search of the v8 audit: `dcce5130…152a` → **0 occurrences**; `e2476f12…5d88` → **0 occurrences**; the other six → **1 occurrence each**. The v8 audit's own on-disk digest is `dcce5130…152a`, confirming it cannot contain that value. |

**Nothing else changed.** Every validated count, the CASS sample-scoping, the v6 provenance attribution of Trials A/B/C, the `CASS-2` refutation and its uncounted status, the `5 P0 / 34 P1 / 39 P2 = 78` ledger, the 78-file / 14,015-line / 466,080-byte inventory with per-skill vector `1,4,3,1,24,15,1,4,9,4,1,8,3`, the six live owners and their line counts, the selected AGY surface, `CV-8`, the installed DCG matrix, Codex projection ownership, and all thirteen one-by-one intent/RPI/live-behavior analyses are **preserved exactly** as the v9 review validated them.

**Whole-file frequency sweep, re-run at v10.** Scanning for `routinely`, `readily`, `sometimes`, `often`, `frequently`, `typically`, `usually`, `rarely`, `occasionally`, `commonly`, `generally`, `normally`, `mostly`, `seldom`, `regularly`, `ordinarily`, `habitually`, `customarily`, `as a rule`, `more often`, and `most of the time` over the v9 subject returned exactly three hits: `routinely` at line 259 (**the live claim, now removed**), `readily` at line 45 (**inside the marked RC-1 historical quotation**), and `sometimes` at line 55 (**a literal regex pattern in the sweep list**). Only the first was operative. **No qualitative frequency word survives in any current v10 assertion**; the two remaining occurrences are explicitly labelled historical quotation and pattern-list text, which requirement 3 permits.

**Whole-file provenance-source sweep, re-run at v10.** Every sentence claiming who declared a digest was re-read. Line 27 was the only over-attribution; the closing chain sentence was the only over-generalization. Both are corrected above.

### 1.3 v8 → v9 — **history**: the two corrections made in the **v9** revision

Direct inputs: the **v8 audit** (`dcce5130…152a`, 797 lines) and the **v8 Sol review**
(`e2476f12…5d88`, 528 lines), both SHA-verified at open.

**The v9 revision executed no command trials and produced no new receipt.** It was a wording-and-
provenance correction over v8's text. Every CASS number below is the number v8 already carried;
only its attribution and the scope of the sentence around it changed. Nothing was re-run in order
to relabel provenance.

| # | Sol v8 finding | v8 said | v9 said | Basis, as recorded by the v9 pass |
|---|---|---|---|---|
| **RC-1** | Surviving CASS statements still asserted a mechanism, a required condition, an exhaustive field set, or a frequency, rather than what was observed | "byte identity … is **contingent on timing**, not impossible. It **occurs when** all four volatile fields coincide **and fails otherwise**"; "Byte identity therefore **requires** a **conjunction**"; "Both components occur **readily** … their conjunction occurs **sometimes**"; "**Structurally equivalent** after normalizing volatile timing and age fields"; §10.5 improvement (8) instructing a check to normalize exactly the four fields | Every such statement is restated as an observation over the recorded samples: **both raw classes occurred**; in every recorded sample **raw equality coincided with equality of all four observed volatile fields**, and raw inequality coincided with a difference in at least one of them; elapsed time is an **observed correlate**, not an established exclusive cause; **no exhaustive volatile-field set, immutable output shape, frequency, fixed digest, or causal necessity is asserted**. The word "invariant," which v8 had narrowed rather than dropped, no longer appears in any CASS sentence at all. Improvement (8) now requires normalizing **at least** the four observed fields, comparing **matched live state**, and **failing or reporting** any newly differing field or shape rather than silently widening the normalization | v8 review RC-1 |
| **RC-2** | v8 labelled inherited evidence as newly executed — "this pass," "fresh" — although the 123-command structured trial belongs to the v6 authoring pass | "Evidence executed for **this pass**"; "123 **fresh** command executions"; "**this pass**" as the observer column of the two-class table; "this pass's **123 status command executions**"; "**This pass:** 3 of 40 cross-form pairs"; "123 command executions **this pass**" | The 123-execution trial (Trials A/B/C = 40 + 80 + 3) is attributed throughout to the **v6 authoring pass**, carried forward unchanged through v7, v8, and now v9. No "this pass" or "fresh" label survives on it. Because **v9 issued no cass command**, no CASS evidence anywhere in this file is labelled as v9's | v8 review RC-2, **independently confirmed against the v6 source**: v6 §2.2 records Trials A, B, and C at its lines 96, 107, and 121, and the three byte-identical Trial B digests `773e9c09…`, `f7c79a19…`, `ac9a8f1d…` at its lines 112–114. v6 miscounted those same executions as "83 fresh invocations"; v7 recounted them to 123 without re-running them |

### 1.4 Whole-file sweep executed for **v9** — method and result *(history)*

**Method.** After assembling the complete v9 text, two regular-expression families were run over
every line of the file with `re.finditer` and line-numbered, then every hit was triaged by hand
against its sentence; no pattern was narrowed to quiet a hit.

1. **Causal / exhaustive / frequency wording**, 21 patterns — `contingent on timing`,
   `requires a\b`, `fails otherwise`, `occurs when`, `\breadily\b`, `occurs sometimes`,
   `[Ss]tructurally equivalent`, `regardless of form`, `any two samples`, `\balways\b`,
   `\bnever\b`, `\bimpossible\b`, `\bcannot\b`, `\bexhaustive\b`, `\binvariant\b`,
   `\bthe rate\b`, `guarantee[sd]?\b`, `prove[sn]?\b`, `because of`, `caused by`, `\bdue to\b`.
2. **Inherited-evidence provenance**, 6 patterns — `this pass`, `This pass`, `\bfresh\b`,
   `\bFresh\b`, `newly executed`, `executed for`.

Because §1.1 necessarily quotes the very tokens it sweeps for, the counts below are taken over the
whole file **excluding §1.1 itself** — a region fixed by its own headings, so the count does not
depend on the length of this paragraph.

**Result.** The two families returned **79** and **20** hits over the swept region. Every hit
was triaged against its sentence, and none required an edit beyond the ones already listed in
§1.0.

- ***Causal / exhaustive / frequency — 79 hits.*** **25** are verbatim quotations of wording this
  revision or a predecessor *removed*, held inside the §1.0–§1.4 correction ledgers and §12's
  residual note 2 so a reader can still see what was withdrawn. **6** are the scope disclaimers
  themselves — sentences whose entire function is to deny an exhaustive field set, a timeless
  shape, a frequency, or a sole cause. **47** belong to findings with no CASS content (`RCH-A`'s
  "always do, never ask", swarm's "isolation proven, not inferred from paths", the converter's
  "cannot run at all", ADR-0016's "Python never ships in skills", the seal's note that a file
  cannot contain its own digest, and similar). **1** is the phrase *fresh-validator* in §10.5's
  strength note. **No surviving sentence asserts a CASS mechanism, required condition, exhaustive
  volatile-field set, fixed output shape, frequency, or exclusive cause outside a marked
  quotation.**
- ***Inherited-evidence provenance — 20 hits.*** **15** are quotations of the labels RC-2 removed,
  carried in the ledger rows and in §2.2's account of v6's own "83 fresh invocations" miscount.
  **3** are `fresh scratch directories` and the `<fresh-scratch>` path placeholder inside a
  recorded command line — directory hygiene, not an evidence label. **2** are the AgentOps
  subject-matter terms *fresh validation* (`AN-2`, §9.2 row 6) and *fresh-validator* (§10.5).
  **No occurrence of "this pass" or "fresh" now attaches to the 123-command CASS trial or to any
  other inherited evidence, and no evidence anywhere in this file is attributed to v9 or v10.**

### 1.5 v7 → v8 — **history**: the two corrections made in the **v8** revision

*Recorded as history.* v8's direct inputs were the **v7 audit** (`eb3fedd8…5ac1`, 775 lines) and the **v7 Sol review**
(`e69e9d29…6920`, 464 lines), both SHA-verified at the v8 open. v7's direct inputs were the **v6 audit**
(`307bea30…1cbcb`, 730 lines) and the **v6 Sol review** (`adb147b7…66b0`). **The SC-1 and SC-2 rows below are the v7 → v8 corrections and are likewise history.**

| # | Sol v7 finding | v7 said | v8 says | Basis |
|---|---|---|---|---|
| **SC-1** | Six enumerated CASS summaries outside the qualified §2.2 restated the conclusion as an unscoped structural or causal property | "contingent on timing, **not on the command form**"; the fixed digest/494 paths as "the property worth relying on"; "the same four fields **regardless of form**"; "the difference set between **any two samples**"; "pass or fail depending on **timing alone**"; "**The invariant.** … digest `65ae94e3…` and 494 set-equal leaf paths" | Every occurrence rewritten to evidence-bounded wording: **every sampled matched cross-form pair normalized equal**; **no form-specific difference was observed in that sample**; the four fields are the **tested normalization for the recorded samples**, not an exhaustive volatile set; elapsed time is an **observed correlate**, not an established sole cause; **both raw classes occurred** and raw equality is **contingent/unstable**; the digest and path counts are **recorded sample evidence**. **"Invariant" is now reserved for sampled matched-live-state structural equivalence** and never for a fixed digest, path count, field set, or causal exclusion | v7 review RC-1; that review's own Trial C observed **565 leaf paths** at the same cass 0.6.19, which is why a fixed shape is not carried as a structural property |
| **SC-2** | v7 retained stale v6 provenance | Input table named only the v5 audit/review; §1.1 was headed "v5 → v6 — the single correction in this revision"; the seal said the file was written to the **v6** path | Input table now names the **v7 audit/review as this revision's direct inputs** and the **v6 audit/review as v7's**; §1.1 and the RC-1/RC-2 rows are relabelled **history**; this §1.0 carries the current v7 → v8 corrections; the seal names the **v8** path and the predecessor chain includes the v6 and v7 audits and reviews | v7 review RC-2 |

**Unchanged by the v8 revision:** the 123-execution accounting and the exact per-trial comparison
populations (A: 780 unordered + 39 consecutive; B: 40 matched; C: 3 normalized samples, with no
combined total); `CASS-2` refuted and uncounted; 78 IDs at 5/34/39; 78 files / 14,015 lines /
466,080 bytes; the six owner classifications; the AGY selected-surface facts; `CV-8`; the DCG
matrix; every one-by-one per-skill section; and checked/not_checked. SC-1 and SC-2 correct wording
scope and provenance — not a mechanism, a severity, or a count.

### 1.6 v5 → v6 — **history**: the correction made in the **v6** revision

*Recorded as history.* Sol's v5 review returned **REQUEST_CHANGES** on exactly one item: v5's `D2b` promoted a three-sample observation into a universal. Sol produced counterexamples, they were reproduced independently at the v6 pass, and the universal was withdrawn there. **The RC-1 and RC-2 rows below are the v6 → v7 corrections and are likewise history.** The v7 → v8 corrections, SC-1 and SC-2, are recorded in §1.5; the v9 → v10 corrections in §1.2; the v8 → v9 corrections in §1.3; and the v10 → v11 corrections in §1.0. **This §1.6 block records only the v5 → v6 correction and makes no claim about any later revision.**

| # | Sol v5 finding | v5 said | v6 says | Basis |
|---|---|---|---|---|
| **RC-1** *(v7)* | v6's CASS execution accounting was false | "83 fresh invocations" (§2.2 header, and at the checked, coverage, residual, and seal citations); "80-invocation trial"; "80 pairwise comparisons"; "123 comparisons across both parties" | **123 command executions** — Trial A 40, Trial B 80 (40 pairs × 2), Trial C 3. Trials A+B = 40 calls + 40 pairs = **120 executions**. Comparison populations stated separately per trial: A = 780 unordered + 39 consecutive; B = 40 matched; C = 3 normalized samples. The "80 pairwise" and cross-party "123 comparisons" figures are withdrawn as underivable | recomputed from the trial definitions in §2.2 |
| **RC-2** *(v7)* | Finite sampled evidence was phrased as an exhaustive property | "the difference set … is **always** a subset"; "depends **only** on elapsed wall-clock time"; "the command form has **no observable effect on the output at all**"; a fixed normalized digest presented as invariant | Every claim scoped to the sampled matched live state: **every sampled matched cross-form pair normalized equal and no form-specific difference was observed in that sample**; raw equality remains contingent with both classes preserved; the four-field normalization is the **tested method**, not proof that only those fields can vary; elapsed time is an **observed correlate**, not the only possible cause; `65ae94e3…` and 494 paths are **sample evidence**, not timeless structure | §2.2, §11 refuted row 6, §12 |
| **F1** | `D2b` overclaims that byte identity is impossible | "Byte-identity was **never an achievable standard**: the same form **is not** byte-identical to itself across two sequential calls" (§1 D2b row) | **Both raw classes — byte-identical and differing — occurred in the recorded samples, so `D2b`'s universal denial is not supported.** In every recorded sample raw equality coincided with equality of all four observed volatile fields, and raw inequality coincided with a difference in at least one of them; no exhaustive field set, immutable output shape, frequency, or exclusive cause was established. Observed in both classes by both parties (§2.2) | Trials A+B, executed at the **v6 authoring pass**: 40 same-form calls + 40 cross-form pairs = **120 command executions** |
| **F1a** | The "superset" rationale is not general | "The same form differs from itself in a **superset** of the fields the two forms differ in" (§2.2) | **In every sampled comparison, the difference set fell within the same four volatile fields, in both same-form and cross-form samples**; the subset that appeared correlated with elapsed wall-clock time between the calls rather than with the command form. The superset relation was an artifact of v5's three samples. *Scope: sampled populations below, not an exhaustive property.* | Trial A: 780 unordered + 39 consecutive comparisons; Trial B: 40 matched comparisons |
| **F1b** | The universal is repeated in residual risk | "**No two `cass status` invocations can ever be byte-identical**, so the original evidence sentence could not have been true under any pairing" (§12) | Rewritten: v4's citation of byte identity and v5's universal denial were **both** methodology errors; the durable rule is narrower than either (§12 residual risk 2) | same |
| **F1c** | A self-referential phrase claim was false | "The phrase 'byte-identical' appears nowhere in v5" (§2.2 conclusion) — it occurred at four places in v5 | Removed. v6 makes **no claim about its own text**; it states only that byte identity is not the basis of the `CASS-2` refutation | Sol's count, confirmed |

**What survives unchanged.** The substantive `CASS-2` refutation, the four-field normalization as the **tested** comparison method, and the sampled structural-equivalence conclusion. The shared normalized digest `65ae94e3…` and the 494 set-equal leaf paths survive as **recorded sample evidence** — not as a fixed shape (SC-1). Sol marked each **PASS**. `CASS-2` remains refuted and remains **uncounted** — it is refuted-table row 6, never a ledger finding. **No finding, severity, or total changes.**

### 1.7 v3 → v4 → v5 corrections, all Sol-verified, preserved *(history)*

| # | Correction | Status at v6 |
|---|---|---|
| C1–C10 | v4's ledger reconstruction: 78 canonical IDs; `AN-4`/`AN-5` ledgered; `RCH-5` assigned; S3 narrowed to four members; refuted rows corrected to six; DCG N4 narrowed to the observed 0.5.6 matrix; six live owners; 520/718 line counts; 11 live AGY references of 12 raw | **PASS** — Sol reparsed the ledger to exactly 5 P0 / 34 P1 / 39 P2 = 78 unique IDs, 20 aliases uncounted |
| **D1** | `CV-8` Bash 3.2 impact: `local -n` fails, `set -euo pipefail` aborts at line 283 with **exit 2**, **zero output files**, output directory never created | **PASS** — Sol reproduced rc 2, 0 files, sentinel preserved, Bash 5.3 contrast 5 files |
| **D1b** | The failure occurs during parsing (`collect_files`, called at 304–305), far before the destructive clean at 631 and `mkdir -p` at 633, so a pre-existing output directory is not deleted | **PASS** — Sol's sentinel witness confirmed |
| **D2** | `CASS-2` refutation rests on structural equivalence after normalizing four volatile fields, not on raw digest comparison | **PASS** |
| **D2b** | *(the universal)* | **WITHDRAWN — see §1.3 F1** |

### Verified totals, carried forward unchanged

```text
P0 =  5    P1 = 34    P2 = 39    TOTAL = 78 unique canonical IDs
```

Sol independently reparsed this ledger at v5 and reported "P0 5 rows, 5 unique IDs / P1 34 rows, 34 unique IDs / P2 39 rows, 39 unique IDs / total 78 rows, 78 unique IDs," with none of the 20 absorbed aliases appearing as a counted ID, S3 naming exactly four skills, and the refutation table holding six rows. The v6 correction touches none of it.

### Preserved from v5 unchanged (Sol verified each)

78 files / 14,015 lines / 466,080 bytes and per-skill path/line counts; the explicit ledger, aliases, S3 narrowing and refuted rows; `CV-8`'s corrected Bash 3.2 behavior; the DCG 0.5.6 temp-root matrix; **six** live Go/shell owners; `codex-exec.sh` = **520** lines and `convert.sh` = **718** lines; **11** semantic AGY references from 12 raw grep hits; canonical placements and RPI fit for all 13; ADR-0016 application and the 24-pin shrink-only ratchet; RCH authority inversion; swarm lexical-isolation defects; converter deletion and line-deletion findings; projection ownership and 13/13 parity; and all test, witness, and improvement content.

---

## 2. Evidence base, and which pass executed it

Read-only with respect to the subject. All witnesses ran in fresh scratch directories outside the repository; the landing was verified clean after every step. **v9 executed none of these witnesses.** They are inherited from the passes named against each one and are carried forward unchanged; §2.2's 123 cass executions belong to the **v6 authoring pass**.

### 2.1 `CV-8` — preserved, Sol-reproduced

Unchanged from v5 and independently reproduced by Sol at the same subject.

```text
$ /bin/bash --version
GNU bash, version 3.2.57(1)-release (arm64-apple-darwin25)

$ /bin/bash skills/converter/scripts/convert.sh skills/converter codex <fresh-scratch>/out
exit: 2            stdout: (empty)
stderr: convert.sh: line 283: local: -n: invalid option
        local: usage: local name[=value] ...
output directory exists afterwards: NO        files created: 0

$ bash --version   # PATH bash, contrast
GNU bash, version 5.3.15(1)-release (aarch64-apple-darwin25.4.0)
exit: 0            files created: 5

# pre-existing output directory holding a sentinel
exit: 2            SENTINEL.txt survives: YES        files in pre/: 1
```

Static ordering confirms it: `set -euo pipefail` at line 5; `local -n` at 283–284 inside `collect_files()`, first called from `parse_bundle()` at 304–305; `rm -rf "$output_dir"` at 631 and `mkdir -p` at 633 inside `write_output()`. Parsing precedes writing, so the run dies before either. The package declares **no** bash-version precondition anywhere under `skills/converter/**`. `CV-8` is retained at **P1** as a portability/availability defect, not a data-loss claim.

### 2.2 `CASS-2` and the withdrawn `D2b` universal — 123 command executions, **executed at the v6 authoring pass**

Installed `cass 0.6.19`. All executions are read-only status queries.

**Execution and comparison accounting (corrected at v7 — RC-1).** v6 labelled this section
"83 fresh invocations." That figure was wrong: it counted each cross-form *pair* as one execution.
The exact accounting is:

```text
Trial A   40 same-form calls                      =  40 command executions
Trial B   40 matched cross-form pairs (2 calls ea.) =  80 command executions
Trial C    3 calls                                =   3 command executions
                                                    ---------------------
                                                     123 command executions
```

Trials A + B together are **40 same-form calls plus 40 cross-form pairs = 120 command executions**;
v6's "80-invocation trial" conflated pairs with executions.

**Comparison populations are separate from executions and are not interchangeable.** Each trial
supports a different comparison set, and there is no single combined comparison total:

| Trial | Executions | Comparison population |
|---|---:|---|
| A | 40 | **780** unordered pairs (`C(40,2)`) and **39** consecutive pairs |
| B | 80 | **40** matched cross-form comparisons (one per pair) |
| C | 3 | 3 normalized samples compared for digest and leaf-path equality |

v6's "80 pairwise comparisons" appears in no trial and is withdrawn, as is the cross-party
"123 comparisons" total in §12 — 123 is the **v6 pass's** execution count, not a comparison count,
and comparison populations from different trials and different parties are not additive.

**Trial A — same form, 40 sequential `cass status --json`.**

```text
outputs                     : 40
unique whole-file digests   : 40        (no collision in this window)
budget.elapsed_ms values    : 30 unique of 40; repeats {244:4, 214:2, 238:2,
                              248:2, 233:2, 253:2, 225:2, 245:2}
unordered pairs agreeing on budget.elapsed_ms : 13 of 780
consecutive pairs differing ONLY in budget.elapsed_ms : 18 of 39
```

**Trial B — cross form, 40 sequential `(--json, --robot-format json)` pairs.**

```text
byte-identical pairs : 3 of 40

  pair  6  773e9c0936e7fa816715aa9c9567c56711e8deb963a91ac6a3dcb0527f0be0f6
  pair 33  f7c79a1975958ad318e0b30612b3709e898b0c2da88a8a1a652cc89102aab9be
  pair 38  ac9a8f1d85539f3b07d3c68cb03b7e37b66746929aab773124b3405c8be9aada

of the 37 differing pairs, the v6 authoring pass retained the exact
field shape of one pair only:
  pair 11  differed in age_seconds / generated_at / timestamp,
           with budget.elapsed_ms agreeing
at least one differing pair differed in budget.elapsed_ms alone;
at least one differing pair differed in all four observed fields.
The per-pair field-difference distribution across the remaining 36
differing pairs was NOT retained and is not reconstructible, so no
proportion, majority, or rate over them is asserted.
```

**Trial C — three samples, normalization re-check.**

```text
raw digests            : three distinct values
normalized digests     : all three ->
    65ae94e3fdfa4420bc43eed0a6a234b2bdae0df06dbe3dff4b692c2e9f4b0b41
parsed leaf paths      : 494 / 494 / 494, sets equal
```

**Both observed classes, preserved side by side.**

| Class | Observed by | Evidence |
|---|---|---|
| **Byte-identical** | **v6 authoring pass** | Trial B pairs 6, 33, 38 — three cross-form pairs, digests above |
| | **Sol (v5 review)** | same-form: 40 invocations, 37 unique digests, calls 35/36/37 sharing `29b2d058c1ff7bf1706c80c399c6df11b350d72a8b03e4445b4e5dfe73b40a25`; cross-form: pair 15 byte-identical at `36e70f12d805bc8ea28beee70cb599f76370423d83e4cd46fc5076fa2d752916` |
| **Differing** | **v6 authoring pass** | Trial A — 40 unique digests over 40 invocations; Trial B — 37 of 40 pairs differed |
| | **Sol (v5 review)** | three-sample run crossing a one-second boundary: all four volatile fields differed |
| | **v5's original samples** | A1/B1/A2 all raw-distinct; A1–B1 differed in two fields, A1–A2 in four |

**Observed co-occurrence — how the two classes appeared.** The four volatile fields *observed* split into two granularity classes:

- **One-second granularity:** `age_seconds` (two sites), `generated_at`, `_meta.timestamp`. In every recorded sample, two calls inside the same wall-clock second agreed on all of them.
- **Millisecond granularity:** `budget.elapsed_ms` — the command's own measured duration. In the recorded samples this was the residual difference: in **18 of 39** consecutive same-form pairs it was the *only* field that differed.

In the recorded samples, raw byte identity coincided with agreement across **both** granularity classes at once: the two calls fell in the same wall-clock second *and* the command's measured duration matched. Each class was observed agreeing on its own — 13 of 780 unordered same-form pairs agreed on `elapsed_ms`, and 18 of 39 consecutive pairs agreed on everything else — and both were observed agreeing together in 3 of 40 Trial B cross-form pairs. Trial A's window recorded none; Trial B's recorded three; Sol's trials recorded two. *These are co-occurrence counts within the recorded samples. They are not a derived rate, a necessary condition, or an exclusive cause, and the four fields are the volatile fields observed, not an established exhaustive set.*

**The corrected property, stated exactly as far as the evidence supports and no further.**

> Raw byte identity between two `cass status` JSON outputs is **contingent and unstable**. Elapsed wall-clock time between calls was the **observed correlate** in the recorded samples; no form-specific difference was observed in any sampled matched cross-form pair. That is a statement about the sampled population — **not** a universal exclusion of the command form as a possible cause. Both raw classes occurred in the recorded samples: byte-identical pairs were observed by both parties, and differing pairs were also observed. The exact recorded counts are v6 Trial B's **3/40 equal** and **37/40 differing** matched cross-form pairs, and Trial A's **40 distinct** raw outputs. **No frequency, rate, typicality, or likelihood is asserted** — the samples establish that both outcomes occurred, and nothing about their relative incidence. It is therefore **not** the correctness criterion and must not be used as an equivalence test. *Scope: sampled matched live state at cass 0.6.19 on this host; not a universal causal or structural claim.*
>
> **Across the sampled comparison populations** — Trial A's 780 unordered and 39 consecutive
> comparisons, Trial B's 40 matched cross-form comparisons, and Trial C's 3 normalized samples —
> every observed difference fell within the same four volatile fields (`age_seconds`,
> `budget.elapsed_ms`, `generated_at`, `_meta.timestamp`), in both same-form and cross-form
> samples. No non-volatile field differed in any sampled comparison by either party. **These four
> fields are the tested normalization method, not a proof that only these fields can ever vary**
> — §12 records that the volatile set is not established as exhaustive.
>
> Elapsed wall-clock time between calls was an **observed correlate** of which subset appeared.
> It is not established as the only possible cause.
>
> **Every sampled matched cross-form pair normalized equal, and no form-specific difference was
> observed in that sample.** After normalizing those four fields the sampled outputs collapsed to
> digest `65ae94e3…` with 494 set-equal leaf paths. **That digest and path count are recorded
> sample evidence at cass 0.6.19 on this host — not a timeless structural constant.** A separate
> sequential sweep observed a second, transient normalized structure (516 leaf paths) that did not
> recur, which is why this audit relies on sampled matched-live-state equivalence rather
> than a fixed digest.

**Why the corollary is the stronger basis for the refutation.** `CASS-2` alleged that `--robot-format json` diverges from `--json`. The evidence says that **in every sampled matched cross-form pair, no form-specific difference was observed** — the differences that did appear were drawn from the same four observed volatile fields whether the comparison was same-form or cross-form. That refutes the finding more directly than any digest comparison, and it does not depend on whether a particular pair happened to collide. **It is a statement about the sampled population, not a universal claim that the command form can never have an observable effect.**

**On v5's "superset" claim.** v5 observed that its same-form pair differed in four fields while its cross-form pair differed in two, and generalized. Trial B refutes the generalization on its retained record: of 40 matched cross-form pairs, **3 differed in no field**, **37 differed in at least one**, and **pair 11 differed in three fields not including `budget.elapsed_ms`** — a shape v5's superset relation cannot produce. One counterexample is sufficient to refute a universal, and pair 11 is that counterexample. **No proportion over the 37 is asserted**, because the per-pair field-difference distribution was not retained (§2.2). Subsets vary in both directions; the superset relation was a three-sample artifact.

**Normalization stability — as sample evidence, not as structure.** The digest `65ae94e3fdfa4420bc43eed0a6a234b2bdae0df06dbe3dff4b692c2e9f4b0b41` reproduced across **three independent sessions** — v5's authoring pass, Sol's v5 review, and the v6 authoring pass — as did the 494 leaf-path count. **That digest and path count are recorded sample evidence, not a timeless shape.** The v7 review independently observed a **565-leaf-path** live status shape in its own Trial C at the same cass 0.6.19, which is why the digest and path count are carried as properties of the sampled live state rather than as constants. **The property worth relying on is narrower — structural equivalence of sampled matched cross-form pairs after the tested normalization.** *Scope: sampled matched live state at cass 0.6.19 on this host; not a universal causal or structural claim.*

### 2.3 Preserved-fact re-confirmation

| Check | Result | Sol v5 |
|---|---|---|
| `wc -l scripts/lib/codex-exec.sh` | **520** | confirmed |
| `wc -l skills/converter/scripts/convert.sh` | **718** | confirmed |
| per-skill `git ls-tree` path counts | `1,4,3,1,24,15,1,4,9,4,1,8,3` = **78** | confirmed |
| per-skill line totals summed | **14,015** (466,080 bytes) | confirmed |
| grandfathered Python pins | **24**, exactly four in this scope | confirmed |
| AGY paths | **12 raw / 11 live** after the §3 filter | confirmed, occurrence counts matched |
| `dcg --version` and temp-root matrix | **0.5.6**, six-case matrix in §10.9 | reproduced exactly |
| landing at open and close | `0088c6e38…` / `c0c43eef…` / clean, 0 porcelain | confirmed both ends |

**A live instance of N5/N12, carried forward.** In v4's pass the dcg guard blocked a probe whose *quoted loop data* contained destructive-looking fragments, on a command that deletes nothing (`core.filesystem:rm-rf-general` class). Per the dcg skill's own response protocol no override was requested; the probe was re-expressed with the operator constructed from bytes and completed. This is direct evidence for **N5** and **N12**, and evidence that the dcg response protocol works as written.

---

## 3. AGY reference count — 11 live of 12 raw

Counting rule, applied uniformly: `N` = case-insensitive occurrences of the token `agy` in the file. A path counts as a **live** reference only if at least one occurrence is an AGY-semantic construct (`agy-native`, `agy -p`, `AGY`, an `agy`-bearing installer/path segment, or a named reviewer/membrane arm) — not merely the byte sequence.

| # | Path | `agy` | Live? | What the occurrences are |
|---|---|---:|---|---|
| — | `cli/testdata/transcripts/real-2.4mb.jsonl` | 6 | **NO — filtered** | base64 noise (`…DagyFrYWf…`, `…SagyAATJ4…`) and bead ids `bd-agye.json` / `be-agye.json`; **0** AGY-semantic tokens |
| 1 | `tests/scripts/reviewer-adapters.bats` | 39 | yes | the AGY adapter suite (7 AGY cases) |
| 2 | `scripts/lib/codex-exec.sh` | 24 | yes | the live adapter; `agy -p`, ROUTINE-tier ruling |
| 3 | `evals/membrane/README.md` | 4 | yes | AGY as membrane arm B; `--membrane-cmd 'agy -p "$1"'` |
| 4 | `scripts/lint/codex-cross-runtime-skills.txt` | 3 | yes | `agy-native`, `AGY` |
| 5 | `scripts/skill-eval.sh` | 2 | yes | `agy` as a named quorum reviewer |
| 6 | `scripts/ci-local-release.sh` | 2 | yes | `install-(agy\|claude\|codex\|opencode).sh` allowlist |
| 7 | `tests/scripts/ci-local-release.bats` | 2 | yes | `scripts/install-agy.sh` fixture |
| 8 | `tests/scripts/cross-runtime-hook-baseline.bats` | 2 | yes | test **"accepts AGY top-level command and conversation-id fields"** — locks AGY's `conversation_id` + top-level `command` shape against Codex's `thread_id` + `tool_input.command_line` |
| 9 | `scripts/.preamble-grandfather` | 1 | yes | line 164 → `scripts/install-agy.sh` |
| 10 | `tests/install/test-install-smoke.sh` | 1 | yes | `"scripts/install-agy.sh"` |
| 11 | `tests/scripts/skill-eval.bats` | 1 | yes | `quorum: agy` |

Sol independently reproduced 12 raw / 11 semantic paths and matched the per-file occurrence counts, including 39 and 24. **The orphan refutation stands** — it rests on the live adapter, 7 passing AGY tests, eval documentation, and installer/lint/hook surfaces, not on a raw count.

---

## 4. Live Go and shell owners — **six**, classified

The canonical `SKILL.md` files are the **skill source contract**. The surfaces below are live implementations and consumers. None is a canonical skill source; none may be edited as an architecture source.

| # | Live owner | Lines | Classification | Authority relationship |
|---|---|---:|---|---|
| 1 | `scripts/lib/codex-exec.sh` | **520** | Conforming implementation, one T7a conflict | Implements the one-shot shape both `codex-exec/SKILL.md` and `agy-native/SKILL.md` describe: executes once, records output, classifies MISSING/TIMEOUT/ECHO/GENUINE-NONZERO, and "owns no reviewer selection, retry, validation, admission, merge, or continuation decision." De-facto owner of both skills' behavior; neither skill cites it. **Conflict: LIVE-1.** |
| 2 | `cli/internal/runtimecmd/runtimecmd.go` | 85 | Lower-level, conforming, partial | Pure argv builder. Direct Codex invocation only; **fail-closes on LAW 0** — never emits `-p`/`--print`, returns `ErrClaudeHeadlessProhibited` for any `claude` binary including `env -i claude` and absolute paths. Grants no authority the skills lack. |
| 3 | `cli/internal/eval/runtime.go` | 815 | Higher-level composition | Owns subprocess execution and cleanup for the eval runtime. `runLiveRuntimeWithAttempts` loops with per-attempt `context.WithTimeout`. **That retry is eval-layer composition and is not evidence the codex-exec one-shot port retries.** |
| 4 | `cli/internal/adapters/eval/scenario_ab_runtime.go` | 198 | Higher-level consumer | One-shot A/B consumer. `RunCodexExec` uses `--dangerously-bypass-approvals-and-sandbox`; `sandboxedCodexCmd` "deliberately refuses empty deny lists." The treatment arm's isolation posture needs an explicit T7a judgment it has not received. |
| 5 | `cli/internal/adapters/eval/scenario_ab_agentic.go` | 242 | Higher-level consumer, out of port scope | `for turn := 0; turn < agenticMaxTurns` (8), executing model-emitted shell in an isolated workspace. Multi-turn agentic execution is **not** the codex-exec one-shot port. |
| 6 | `scripts/ms-reindex.sh` | 301 | Go-owner candidate, misplaced | `ms_serve_pids()` discovers by `grep -F "$SERVE_PATTERN"` over a `ps` snapshot then TERM/KILLs every match — substring identity, not executable identity. Also removes locks, rebuilds state, probes a server, interpolates `MS_REINDEX_PROBE_QUERY` into JSON unescaped, emits verification facts. Under ADR-0016 this is Go mechanism, not skill glue. |

Sol reproduced all six line counts and confirmed the classifications from source: "one-shot adapter plus unbounded timeout fallback; LAW-0 argv builder; higher-level retry owner; one-shot sandboxed consumer; eight-turn agentic consumer; and substring-based MS process discovery/kill mechanism."

**Consequence.** CE-3 ("codex-exec owns no validator or test") is **false as a repository claim** — `codex-exec-lib.bats` (9) and `reviewer-adapters.bats` (16) are live behavior locks. Restated as **S3**: state the proof owner; do not infer absence from package-local inventory.

**LIVE-1 [P1]** — `codex_exec_timeout_cmd` prefers `timeout`, falls back to `gtimeout`, and per its own comment degrades to running the reviewer **with no timeout** when neither exists. T7a requires typed deadlines and forces `NOT_PROVEN` when cleanup or effects are unobservable; a silent unbounded degradation is the opposite. Sol's independently executed witness: with neither binary on `PATH`, `codex_exec_timeout_cmd 300` returned zero bytes.

**LIVE-2 [P2]** — the AGY branch runs `--sandbox --dangerously-skip-permissions`, documented in-file as an accepted ROUTINE/fallback-tier posture per an "A7 bench ruling." Reasoned and recorded, but no canonical skill states it, so a reader of `agy-native/SKILL.md` cannot discover the posture they get.

---

## 5. Canonical placement matrix — preserved

| Skill | Canonical target placement | Tranche |
|---|---|---|
| `account-rotation` | support; cross_cutting, **standalone** | T7b |
| `agent-mail` | runtime; runtime_transport | T7a |
| `agent-native` | runtime; runtime_transport | T7a |
| `agy-native` | runtime; runtime_transport | T7a |
| `cass` | evidence; plan_input, **post_verdict** | T4 |
| `cc-hooks` | support; cross_cutting | T7b |
| `codex-exec` | runtime; runtime_transport | T7a |
| `converter` | implementation; implement_method | T5 |
| `dcg` | support; cross_cutting | T7b |
| `ms` | support; cross_cutting, **standalone** | T7b |
| `ntm` | runtime; runtime_transport | T7a |
| `rch` | **runtime; implement_method, cross_cutting** | T7a |
| `swarm` | runtime; runtime_transport | T7a |

Sol marked every placement and RPI-fit reconstruction **Supported** across all 13.

**T7a acceptance mapped to evidence** (7 skills: agent-mail, agent-native, agy-native, codex-exec, ntm, rch, swarm):

| Clause | Met | Gap |
|---|---|---|
| 1 unavailable behavior explicit | codex-exec/agy via `CODEX_EXEC_MISSING=2` (tested) | `ntm`, `agent-mail`, `rch`, `swarm`, `agent-native` state none |
| 2 typed deadlines/cleanup | codex-exec/agy: timeout + `_cleanup` array | **LIVE-1** breaks the deadline guarantee; `ntm`/`agent-mail`/`swarm` type nothing |
| 3 isolation **proven, not inferred from paths** | none | **swarm proves only lexical prefixes — SW-1 and SW-3 both reproduce** |
| 4 no state bleed | **all 7 hold** — none writes `verdict.v2`, none claims phase authority | — |
| 5 distinct capability | codex-exec / agy-native / ntm / swarm genuinely distinct | agy-native's distinctness asserted in prose the live adapter does not reflect |
| 6 `NOT_PROVEN` on unobservable | `rch` has the vocabulary | `rch doctor --json` returns `success: true` with the daemon down (N11) — inverted |

Clause 4 is uniform good news. **Clause 3 is the systemic failure.**

---

## 6. ADR-0016 — preserved

`docs/adr/ADR-0016-state-tiers.md` §3: *"Python never ships in skills."* Mechanism/trust/receipts → Go; know-how → skills + references; glue is POSIX `sh` only. The 2026-07-25 amendment makes it mechanical: gate `skill.python-ratchet`, blocking, shrink-only; `skills/*/tests/**` exempt as a class; allowlist growth rejected.

Executed: **PASS, 24 grandfathered files remain.** Four fall in this set:

| File | Status |
|---|---|
| `skills/agent-native/scripts/fake_model_runner.py` | grandfathered execution-path debt |
| `skills/cass/scripts/prompt_miner.py` | grandfathered execution-path debt |
| `skills/ms/scripts/mcp-search.py` | grandfathered execution-path debt |
| `skills/swarm/scripts/dispatch_once.py` | grandfathered execution-path debt |

`skills/swarm/tests/test_dispatch_once.py` is **test-path, exempt as a class — not debt**; Sol confirmed it remains outside the governed execution path. These four are **GOV-1 [P1]**: accepted legacy debt with a migration obligation, not a current gate failure and not an open policy question. `fake_model_runner.py` is a test fixture under `scripts/`; moving it to `tests/` discharges its debt at zero architectural cost.

**Two v2 remediations remain withdrawn as architecturally wrong:** embedding `ms-reindex.sh` in the skill package (it is Go mechanism → route to an `ao` subcommand), and treating the cc-hooks shell engine and converter's rewriter as end-state packaging.

---

## 7. RCH authority inversion — preserved (P0)

`skills/rch/SKILL.md:29-32` and `:53-55` require explicit caller authority for destructive cleanup, worker deployment, daemon configuration, and remote mutation.

`references/FAIL_OPEN.md` §**"Don't Ask the Human"** says the opposite — *"always do, never ask"* — then enumerates `rch daemon start`; removing `${XDG_RUNTIME_DIR:-/tmp}/rch/hook_autostart.cooldown`; `rch workers probe --all` plus "fix what's fixable"; raising `total_slots`; `rch workers sync-toolchain --all`; and

> `ssh ubuntu@<host> 'sudo chown -R ubuntu:ubuntu /data/projects/<repo> && sudo chmod 775 /data/projects/<repo>'` … "This is a known recovery; don't escalate."

`references/RECOVERY_PLAYBOOKS.md:19`: **"Don't ask the human if the fix is in the playbook."**

**RCH-A [P0].** Reachable, remote, privileged, irreversible in the general case, instructed unconditionally by a shipped reference. Sol: "directly supported by the owned 'always do, never ask' remote/privileged instructions." v2's praise that `RECOVERY_PLAYBOOKS.md` "gates every destructive step behind explicit authorization" is false and stays withdrawn.

---

## 8. Swarm isolation — preserved (SW-3 at P1)

`dispatch_once._overlap` normalizes with `PurePosixPath` and compares **lexical prefixes only**. Executed on this tree:

```text
_overlap("alias", "src/auth")    -> False    # symlinked workspace: same files, judged disjoint
_overlap("src/Auth", "src/auth") -> False    # case-insensitive fs: same files, judged disjoint
```

Both packets admit, both dispatch, and the failure is **silent**. Sol's independently executed witnesses showed the host treats each pair as the same path, then observed `_overlap=False` and both packets dispatched. The governing contract is explicit: "Disjoint file paths are necessary but not sufficient," and T7a clause 3 requires isolation **proven, not inferred from paths**. **SW-1 and SW-3 are both P1.** `SW-2` (`write_scope.exclude` silently ignored by `_includes()`) remains a separate narrower **P2**.

---

## 9. The finding ledger — mechanically reconstructable

**Counting rule, stated once and applied literally.** One explicit row per canonical finding ID. Exactly one severity per row. No unexpanded ranges. A systemic ID absorbs its per-skill aliases, which are listed in `absorbs` and are **never counted**. Totals are the row counts of these three tables and nothing else.

### 9.1 P0 — 5 rows

| # | ID | Skill | Systemic? | Finding |
|---:|---|---|---|---|
| 1 | `CV-1` | converter | no | `rm -rf "$output_dir"` uncontained over a caller-supplied path; destroys the source package (W3: 4→2 files, exit 0) |
| 2 | `N1` | converter | no | `codex_rewrite_text` **deletes** whole lines matching runtime tokens, applied to every copied `.md`; parity checks only entry names (W1) |
| 3 | `CH-1` | cc-hooks | no | Three-way default-on / inert / ships-by-DEFAULT contradiction across `SKILL.md:279`, `:34`/`:170`, `GUARDRAIL-VALUE-PROOF.md` |
| 4 | `AM-1` | agent-mail | no | Untyped admin/disaster-recovery surface — `clear-and-reset-everything --force --no-archive` plus `install_precommit_guard` — with no typed mode or caller-authority boundary |
| 5 | `RCH-A` | rch | no | Owned references instruct "always do, never ask" for daemon start, cooldown removal, fleet toolchain sync, and remote `sudo chown -R`/`chmod`, against SKILL.md's explicit-authority requirement |

### 9.2 P1 — 34 rows

| # | ID | Skill(s) | Systemic? | Finding | Disposition |
|---:|---|---|---|---|---|
| 1 | `S1` | 10 skills | **YES** — absorbs `AR-1`, `AM-2`, `CASS-1`, `CH-4`, `CE-1`, `CV-3`, `DCG-4`, `NTM-1`, `RCH-1`, `MS-1a` | `metadata.effects: []` false against observed mutation | CONFIRMED |
| 2 | `MS-1` | ms | no | `scripts/validate.sh:12-14` mechanically asserts the false `effects: []` | CONFIRMED |
| 3 | `CH-2` | cc-hooks | no | Default cohort asserts host-global deny over explicit `_beads` paths in unrelated repositories | NARROWED |
| 4 | `SW-1` | swarm | no | Case-varying scopes admitted as disjoint on a case-insensitive filesystem | CONFIRMED |
| 5 | `SW-3` | swarm | no | Symlink-aliased scopes admitted as disjoint; reachable | PROMOTED P2→P1 |
| 6 | `AN-2` | agent-native | no | Fixture cannot bind fresh validation: accepts equal author/validator context IDs, carries no intent or subject digest, may degrade validator→author model, still emits a freshness attestation | CONFIRMED; Sol reproduced exactly |
| 7 | `AN-3` | agent-native | no | `diversity_unsatisfied` routed to a sidecar because the challenge validator forbids unknown top-level keys | CONFIRMED |
| 8 | `N2` | converter | no | Not the shipped-projection owner; two independent rewriters, no parity test | REVISED P0→P1 |
| 9 | `S2` | cass, cc-hooks, dcg, ms | **YES** — absorbs `CASS-3`, `DCG-3`, `MS-2` | `output_contract` absent from frontmatter; v2 schema cannot catch it | CONFIRMED |
| 10 | `S3` | **4 skills** — account-rotation, agent-mail, ntm, rch | **YES** — absorbs `NTM-2`, `AR-2`, `AM-5`, `RCH-2`; **retires `CE-3` as stated** | Proof layer thin — stated per actual proof owner, not "owns no validator" | NARROWED in v4; Sol-verified |
| 11 | `S4` | agent-native, codex-exec, ntm | **YES** — absorbs `CE-2`, `NTM-3` | Cross-adapter model-dispatch contract privately owned by one consumer skill | CONFIRMED |
| 12 | `AGY-R` | agy-native | no | Canonical skill not reconciled with its live root adapter, tests, or eval use | REPLACES refuted S5/AGY-1 |
| 13 | `AGY-2` | agy-native | no | Prescribes live discovery, names no discovery command | CONFIRMED |
| 14 | `LIVE-1` | codex-exec, agy-native | no | `codex_exec_timeout_cmd` degrades to unbounded execution when neither `timeout` nor `gtimeout` exists | CONFIRMED; Sol-reproduced |
| 15 | `DCG-1` | dcg | no | `SKILL.md:125` false — `rm -rf ./build` is blocked; same false claim in `COMMANDS.md` and `cc-hooks/DCG-RCH.md` | CONFIRMED |
| 16 | `N3` | dcg | no | Rule IDs, `explain`/`packs` format, version, hook path diverge from the installed binary; wrong rule ID makes the documented allowlist fix a silent no-op | CONFIRMED |
| 17 | `N4` | dcg | no | Temp-root carve-out is narrower than documented: at 0.5.6 it recognizes literal `/tmp`, `/private/tmp`, and the `$TMPDIR` form, but not arbitrary unresolved variables (`$LAB`, `$output_dir`) or relative paths (`./build`) | NARROWED in v4; Sol reproduced the exact matrix |
| 18 | `N5` | dcg | no | `bash -c` / inline-fragment false-positive surface undocumented | CONFIRMED, re-witnessed (§2.3) |
| 19 | `DCG-U` | dcg | no | Upstream identity and version unresolved across three inconsistent install routes | REVISED; supersedes DCG-5; Sol confirmed the upstream separation |
| 20 | `N6` | cass | no | Stale `skill.spec.json`; `PROMPTS.md` anchor to a removed section | CONFIRMED |
| 21 | `N7` | cass | no | `casr` folded-in vs "invoke the standalone skill"; malformed `../..casr/SKILL.md`; no `skills/casr` exists | CONFIRMED |
| 22 | `N8` | cass | no | `quick_analysis.sh:64` runs `cass index --json` unbounded, violating `SKILL.md:140` | CONFIRMED |
| 23 | `CASS-M` | cass | no | `SKILL.md:230` "None mutate state without explicit confirmation" vs `recover.sh` autonomous index refresh, `doctor --fix`, force-rebuild | CONFIRMED |
| 24 | `N9` | converter | no | `skill-bundle-schema.md` contradicts the code on frontmatter parsing and on both live adapter behaviors | CONFIRMED |
| 25 | `CV-2` | converter | no | Validator vacuous — "2 passed, 0 failed" while the skill can delete its own source | CONFIRMED |
| 26 | `CV-7` | converter | no | `collect_files()` non-recursive vs `copy_passthrough_resources()` recursive (W2) | CONFIRMED |
| 27 | `CV-8` | converter | no | `local -n` (lines 283–284) is a Bash ≥4.3 nameref; on `/bin/bash` 3.2.57 it is an invalid option and `set -euo pipefail` aborts the run with exit 2. The package documents no bash-version precondition, and the script's own documented `bash …` invocation resolves via `PATH`, so on a stock macOS the converter cannot run at all | CORRECTED at v5; Sol-verified |
| 28 | `N10` | rch | no | Six dangling sibling-doc references, all on the remediation path | CONFIRMED |
| 29 | `N11` | rch | no | `rch doctor --json` → `success: true`, `failed: 0` with the daemon down, while `rch check` correctly exits 2 | CONFIRMED; Sol reproduced at rch 1.0.26 |
| 30 | `N12` | cc-hooks | no | Pure predicates fire on quoted data; no false-positive test despite a pre-registered CUT criterion requiring one | CONFIRMED, re-witnessed (§2.3) |
| 31 | `MS-3` | ms | no | Requires a private operator checkout on a named branch while shipping canonically to three runtimes | CONFIRMED |
| 32 | `MS-4` | ms | no | Reindex mechanism unreachable from a copied install and misplaced — belongs in Go, not the package | REVISED remediation |
| 33 | `CH-3` | cc-hooks, cass | **YES** — absorbs `C3b` | `skill.spec.json` hard-dependency divergence from frontmatter — a class, not a one-off | CONFIRMED |
| 34 | `GOV-1` | agent-native, cass, ms, swarm | **YES** — no aliases | Four execution-path Python files are grandfathered ADR-0016 debt under a shrink-only ratchet | REFUTES v2 open decision 8 |

### 9.3 P2 — 39 rows

| # | ID | Skill | Finding |
|---:|---|---|---|
| 1 | `N13` | cc-hooks | `PATH`-clobbering hooks recipe in `PATTERNS.md` |
| 2 | `N14` | cass | Always-failing "universal extractor" in `SESSION_FORMATS.md` |
| 3 | `N15` | cass | `.summary` self-contradiction one section apart in `RECOVERY.md` |
| 4 | `N16` | cass, rch, dcg | Doc/live version drift with no freshness gate |
| 5 | `N17` | cass, ms, account-rotation, cc-hooks | Operator-private paths baked into shipped skills |
| 6 | `N18` | agent-mail | Retired `uv run python -m mcp_agent_mail.cli` entrypoint in `RECOVERY.md` |
| 7 | `N19` | cass | `TMPDIR` shadowing + temp-dir leak in `multi_machine_search.sh` |
| 8 | `N20` | converter | Cursor budget counts characters, not bytes |
| 9 | `CV-4` | converter | Degenerate triggers |
| 10 | `CV-5` | converter | Single-skill output path — fix by refusal, not silent `/$BUNDLE_NAME` rewrite |
| 11 | `CV-9` | converter | Unquoted `description:` producing invalid Cursor YAML |
| 12 | `CV-10` | converter | Codex-specific rewrites applied to Cursor and `test` targets |
| 13 | `AR-3` | account-rotation | Operator-private `claude-acct` route |
| 14 | `AR-4` | account-rotation | Both-tools-absent behavior undefined |
| 15 | `AM-3` | agent-mail | Advisory-vs-protective reservation semantics stated two ways |
| 16 | `AM-4` | agent-mail | `force_release_file_reservation` with no authority precondition |
| 17 | `AM-6` | agent-mail | `consumes: [task-intent]` on a transport |
| 18 | `AGY-3` | agy-native | AGY-absent path unspecified |
| 19 | `AGY-4` | agy-native | Thinnest kernel in the set for a full validator role |
| 20 | `LIVE-2` | agy-native | `--dangerously-skip-permissions` posture undisclosed in the canonical skill |
| 21 | `CE-4` | codex-exec | `consumes: []` while requiring a caller packet |
| 22 | `CE-5` | codex-exec | Two overlapping trigger vocabularies |
| 23 | `DCG-6` | dcg | Degenerate trigger |
| 24 | `DCG-7` | dcg | Fail-open-on-timeout with no caller-visible signal contract |
| 25 | `MS-5` | ms | MCP feedback tool exposed against the CLI-only rule |
| 26 | `NTM-4` | ntm | `consumes: [task-intent]` on a transport |
| 27 | `NTM-5` | ntm | External-doc delegation with no version pin |
| 28 | `RCH-3` | rch | Seven references with no authority ordering |
| 29 | `RCH-4` | rch | `not_proven` reused without a no-verdict-weight disclaimer |
| 30 | `SW-2` | swarm | `write_scope.exclude` silently ignored by `_includes()` |
| 31 | `SW-4` | swarm | Transitive executor effects undeclared |
| 32 | `CASS-4` | cass | `SELF-TEST.md` stale title |
| 33 | `CH-5` | cc-hooks | Stale `skill.spec.json` section references |
| 34 | `CH-6` | cc-hooks | Shipped guard message names the operator's factory repo |
| 35 | `CH-7` | cc-hooks | `installed-skill-edit-guard.sh` calls `jq` with no preflight — fails open by accident |
| 36 | `CH-8` | cc-hooks | Package conflates advisory documentation with running code |
| 37 | `AN-4` | agent-native | Wall-clock `utc_now()` makes fixture output non-reproducible |
| 38 | `AN-5` | agent-native | `tier: meta` against an undefined tier vocabulary |
| 39 | `RCH-5` | rch | Internal tracker id `bd-w5r9` leaked into a shipped reference |

### 9.4 Totals, derived from rows only

```text
P0 rows  =  5
P1 rows  = 34
P2 rows  = 39
--------------
TOTAL    = 78 canonical findings
```

Sol reparsed this ledger independently at v5 and reported exactly `P0 5 / P1 34 / P2 39 / total 78 rows, 78 unique IDs`, with the 20 absorbed aliases uncounted, S3 naming exactly four skills, six refuted rows, and `AN-4`/`AN-5`/`RCH-5` present in the 39-row P2 table. **The v6 correction changes no row.**

### 9.5 Absorbed aliases — 20, never counted

`S1` ← `AR-1`, `AM-2`, `CASS-1`, `CH-4`, `CE-1`, `CV-3`, `DCG-4`, `NTM-1`, `RCH-1`, `MS-1a` (10)
`S2` ← `CASS-3`, `DCG-3`, `MS-2` (3)
`S3` ← `NTM-2`, `AR-2`, `AM-5`, `RCH-2` (4)
`S4` ← `CE-2`, `NTM-3` (2)
`CH-3` ← `C3b` (1)

**One naming artifact disclosed:** `MS-1a` appears only inside `S1`'s absorbs list and is defined nowhere else. It denotes the ms *instance* of the false-effects declaration, distinct from `MS-1` (the validator that *enforces* the falsehood). The distinction is real; the naming is poor. Recorded rather than silently renumbered.

### 9.6 Withdrawals and severity moves — no row-count effect

| Item | Action | Counted? |
|---|---|---|
| `S5` / `AGY-1` | refuted; **replaced** by `AGY-R` | no — replaced 1-for-1 |
| `CE-3` | retired as stated; absorbed into `S3`'s restatement | no |
| **`CASS-2`** | **refuted — every sampled matched cross-form pair of `--json` and `--robot-format json` normalized equal after the tested four-field normalization, and no form-specific difference was observed in that sample; differences that did appear fell within those four fields in both same-form and cross-form comparisons (§2.2). *Sampled matched live state; the four fields are the tested normalization for the recorded samples, not an exhaustive volatile set.*** | no |
| `DCG-5` | superseded by `DCG-U` | no — superseded 1-for-1 |
| `N2` | severity P0→P1 | yes, once, in P1 |
| `SW-3` | severity P2→P1 | yes, once, in P1 |

### 9.7 Refuted outright — **6** claim rows

| # | Claim | Basis |
|---:|---|---|
| 1 | `S5`/`AGY-1` "agy-native is orphaned, zero tests, zero evals" | **11 live reference files** (§3); 7 AGY tests pass |
| 2 | `CE-3` "codex-exec has no validator or test" as a repository claim | `codex-exec-lib.bats` 9/9, `reviewer-adapters.bats` 16/16 |
| 3 | v2's praise that `RECOVERY_PLAYBOOKS.md` "gates every destructive step behind explicit authorization" | Its line 19 says the opposite (§7) |
| 4 | v2 §7.6 witness sentence implying `git add somefile` blocks | Regex requires literal `_beads`; verified NO_MATCH |
| 5 | v2 open decision 8 "no consulted contract forbids shipped skill Python" | ADR-0016 §3 forbids it; ratchet enforces it (§6) |
| 6 | `CASS-2` `--robot-format json` divergence | **Every sampled matched cross-form pair was structurally equal after normalizing the four observed volatile timing and age fields** on cass 0.6.19: normalizing `age_seconds`, `budget.elapsed_ms`, `generated_at`, `_meta.timestamp` collapses both forms to digest `65ae94e3…` with 494 set-equal leaf paths, reproduced across three independent sessions. Decisively, **all 40 recorded Trial B matched cross-form pairs normalized equal, with no form-specific difference observed in that sample**; differences that appeared fell within those same four volatile fields in both same-form and cross-form comparisons. *Scope: sampled matched live state; the digest and 494-path count are sample evidence, not timeless structure.* Raw byte identity **occurred in 3 of the 40 recorded Trial B matched pairs and not in the other 37**, and both raw classes occurred in the recorded same-form and cross-form trials (§2.2), which is why raw digest equality is not used here as the equivalence criterion. *No exclusive cause for the raw split is claimed.* |

---

## 10. Per-skill findings

Each: intent · RPI placement · I/O/effects · authority · strengths · exact defects (reconciled to §9) · ranked improvements · lethal witness · checked/not_checked · residual risk · direct-to-Go. Sol marked all 13 intent and RPI-fit reconstructions **Supported**.

### 10.1 `account-rotation` — T7b, support; cross_cutting, **standalone**

**Intent.** One caller-selected credential switch verified through the *target runtime*, not by inspecting credential bytes — a live process holds tokens in memory and authenticates as the old account after a byte-perfect file swap. Named failure: **stale-process identity**.
**I/O/effects.** In: host, family, requested account. Out: seven fields incl. observed identity and new-process-required. Effects: host credential state — declared `[]`.
**Authority.** Correctly minimal; disclaims restart, resume, pane selection, repo state, next action.
**Strong.** The identity definition — the only skill naming a verification method *because* the obvious one is wrong. Sol: "target-runtime identity is the right stale-process boundary."
**Defects (§9).** `S1` [P1] · `S3` [P1] · `AR-3`, `AR-4` [P2] · `N17` [P2]. Target obligations not discharged: `standalone` behavior, before/after identity plus **cleanup receipt**, and **partial-rotation** proof are absent.
**Improvements.** (1) declare effects; (2) emit the before/after identity + cleanup receipt the target requires; (3) specify both-tools-absent and partial-rotation paths; (4) document a consumer route or mark operator-only.
**Witness (hermetic).** With a fake credential tool on `PATH` and both real tools absent, the skill must report absence as a disclosed fact and stop — never fall back to byte comparison — and all seven output fields must be present. No credential touched.
**checked** — `SKILL.md` in full (50 lines, the whole package); frontmatter sweep; tool presence. **not_checked** — any `claude-acct`/`caam` execution.
**Residual risk.** *Medium.* Smallest surface; zero automated checks on the only credential-touching skill.
**Direct-to-Go.** Identity capture and the cleanup receipt are receipt-writing mechanism → `ao`. The skill retains the know-how.

### 10.2 `agent-mail` — T7a, runtime; runtime_transport

**Intent.** Messages, ACKs, identities, and **advisory** file reservations among explicitly coordinated writers; reservations bind only because every writer registers against the same absolute project path.
**I/O/effects.** In: absolute path, identities, thread, participants, paths, exclusivity, TTL. Out: ids, conflicts, timestamps, degraded surfaces. Effects: durable AM rows **plus a git pre-commit hook and a full destructive reset** — declared `[]`.
**Authority.** Asserted narrowly, **not enforced**, over a live surface exposing `release`, `verify`, `ci`, `guard`, `migrate`, `clear-and-reset-everything`.
**Strong.** `WORKFLOWS.md` — the best boundary-discipline document among the adapters: a reply is not a verdict; a handoff carries no ownership; transport errors never become retry, queue, lifecycle, or andon state.
**Defects (§9).** `AM-1` [P0] · `S1`, `S3` [P1] · `AM-3`, `AM-4`, `AM-6`, `N18` [P2]. Sol: "`AM-1` at P0 is justified by the reachable reset/hook-admin surface without a typed authority mode." T7a gaps: identity/reservation/ACK/TTL/conflict/degraded results untyped; capability-unavailable behavior unstated; cleanup unobservable → clause 6 says `NOT_PROVEN`.
**Improvements.** (1) split typed coordination modes from an explicitly caller-authorized admin/DR mode; (2) declare effects; (3) type the six result shapes; (4) state "advisory" in the same sentence as the collision guarantee; (5) rewrite `RECOVERY.md` onto the `am` CLI.
**Witness (hermetic).** A text/schema gate asserting `clear-and-reset-everything` and `install_precommit_guard` appear only inside a block naming explicit caller authority, and that every documented operation carries a typed result name. Fails today. No AM state touched.
**checked** — `SKILL.md`, `TOOLS.md`, `WORKFLOWS.md`, `RECOVERY.md` in full (4 paths, 829 lines); three read-only `--help` probes. **not_checked** — register, reserve, send, guard install, reset.
**Residual risk.** *High.* Largest gap in the set between an exemplary written boundary and an unbounded live tool surface.
**Direct-to-Go.** Destructive-reset and hook-installer paths are enforcement + user-file mutation → `ao` with typed authorization.

### 10.3 `agent-native` — T7a, runtime; runtime_transport

**Intent.** Operate caller-selected sessions as four explicit roles without the runtime becoming lifecycle authority.
**I/O/effects.** In: explicit packet, role, workspace, context identity, evidence destination. Out: provider state, transcripts, artifacts, terminal status. Effects: `[manage_runtime_sessions]` — **truthful**.
**Authority.** Correct, including refusal to convert provider retries/reconnects/idle states into Plan, Candidate, or verdict state. `Contract` item 6: a validator session may supply judgment; only Validate writes `verdict.v2`.
**Strong.** The intervention ladder scored by evidence and reversibility, with a real deadline semantic: past terminal status or the caller's window, "further intervention manufactures noise, not evidence."
**Defects (§9).**
- **`AN-2` [P1].** `fake_model_runner.validate_cross` accepts `--author-context-id` and `--validator-context-id` **with no equality rejection**, builds an evidence object with **no intent digest and no subject manifest digest**, may set `validator_model = author_model` on unsatisfied diversity, and still emits `freshness_attestation`. Sol's independently executed witness: "`validate-cross` accepted `X`/`X`, degraded AGY to Codex, emitted freshness, omitted intent and subject digests, rc 0."
- `AN-3` [P1] sidecar routing around the challenge validator · `S4` [P1] hosts the contract two suppliers cite · `GOV-1` [P1] · `AN-4` [P2] wall-clock `utc_now()` · `AN-5` [P2] `tier: meta` against an undefined vocabulary.
**Improvements.** (1) reject equal context IDs and require intent+subject digests before emitting any freshness attestation; (2) admit `diversity_unsatisfied` into the challenge schema; (3) move `model-dispatch.md` to `docs/contracts/`; (4) accept `--generated-at`/`SOURCE_DATE_EPOCH`; (5) **move the fixture to `tests/`**, discharging its ADR-0016 debt.
**Witness (hermetic).** Invoke `fake_model_runner.py validate_cross` with `--author-context-id X --validator-context-id X` into a `mktemp -d`; it must exit nonzero and write no evidence. Today it writes a freshness attestation.
**checked** — `SKILL.md`, `model-dispatch.md`, `fake_model_runner.py` in full (3 paths, 373 lines). **not_checked** — `tests/integration/test_multi_model_dispatch.bats`.
**Residual risk.** *Medium.* A fixture that can emit an unbindable freshness attestation is a proof-integrity risk.
**Direct-to-Go.** Freshness/context-identity verification is receipt mechanism → `ao`; the fixture becomes a test double, not an evidence producer.

### 10.4 `agy-native` — T7a, runtime; runtime_transport

**Intent.** Use AGY only on explicit caller selection, discovering its live command surface first — "a remembered flag is a guess, while a freshly listed one is evidence." Named failure: **wrapper drift**.
**I/O/effects.** In: explicit packet, workspace. Out: AGY run evidence, conversation identity, artifact refs. Effects: `[start_agy_session]` — **truthful**.
**Authority.** Correct: validators read-only, AGY plugin/memory/permission/retry state stays out of AgentOps phase state, `claude -p` floor restated.
**Strong.** Discover-before-acting is the correct opposite of `codex-exec`'s fixed procedure and the reason is written down. The live adapter is genuinely sophisticated — file-PATH-first delivery designs out the giant-inline drop class; the sentinel nonce plus packet-containment check means a `VERDICT:`-bearing packet echo is classified ECHO and the marker cannot veto it. Sol independently confirmed the transport, echo containment, sentinel/packet handling, flags, and missing-input exit behavior.
**Defects (§9).** `AGY-R` [P1] · `AGY-2` [P1] · `LIVE-1` [P1] · `AGY-3`, `AGY-4`, `LIVE-2` [P2].
**Improvements.** (1) reconcile `SKILL.md` with `scripts/lib/codex-exec.sh` (**520 lines**) — name the adapter, the tier ruling, the permission posture; (2) name the concrete discovery commands; (3) make the missing-timeout case explicit; (4) specify AGY-absent behavior; (5) declare the receipt fields `model-dispatch.md` requires.
**Witness (hermetic).** Extend `reviewer-adapters.bats` with a case where `PATH` contains neither `timeout` nor `gtimeout`: the adapter must refuse or emit an explicit unbounded-execution disclosure. Today it silently runs unbounded — Sol's independently executed run returned zero bytes from the wrapper.
**checked** — `SKILL.md` in full (1 path, 50 lines); `codex-exec.sh` contract header, timeout helper, AGY branch; `reviewer-adapters.bats` 16/16; **11 live AGY reference paths enumerated, classified, and filtered (§3)**. **not_checked** — any real `agy` invocation.
**Residual risk.** *Medium.* Reachable, but the canonical skill does not describe the adapter a caller actually gets.
**Direct-to-Go.** Execution/classification mechanism (timeout, echo detection, exit-code taxonomy) is a Go candidate; the skill keeps the discovery doctrine.

### 10.5 `cass` — T4, evidence; plan_input, **post_verdict**

**Intent.** Mine prior sessions as evidence into Plan, on the thesis that repeated prompts are working prompts.
**I/O/effects.** In: query, workspace key, limits, fields. Out: hits with `source_path`+line, aggregations, freshness. Effects: index rebuild, `doctor --fix`, ~90MB model download, ssh/rsync fan-out, `pages encrypt` — declared `[]`.
**Authority.** Well drawn: derived index data is the agent's; source session files are user data and are never touched.
**Strong.** Three doctrines worth protecting: **stale ≠ broken**; **lesson decay + failure overweight**; **authoritative-fallback** (exit 124 is "not observed", never "zero hits"). `recover.sh` is the best-engineered script in the thirteen packages. Sol: "its stale-vs-broken and provenance doctrine fits Plan input, while the post-verdict seam is absent as reported."
**Defects (§9).** `S1`, `S2` [P1] · `N6`, `N7`, `N8` [P1] · `CASS-M` [P1] · `GOV-1` [P1] · `N14`, `N15`, `N16`, `N17`, `N19`, `CASS-4` [P2]. Target gap: the `post_verdict` seam is entirely absent from the skill.
**Refuted here.** `CASS-2` — see §9.7 row 6 and §2.2. **Every sampled matched cross-form pair of `--robot-format json` and `--json` normalized equal** after the tested four-field normalization, and **no form-specific difference was observed in that sample**; across the recorded comparison populations every difference that appeared fell within those same four fields, in both same-form and cross-form comparisons. Raw byte identity between two status calls **appeared in 3 of the 40 recorded Trial B matched pairs and not in the other 37** — the v6 authoring pass recorded 3 byte-identical cross-form pairs out of 40 and zero collisions across 40 same-form executions; Sol recorded collisions in both of its earlier trials and none in the 40 matched pairs of its later one. Both raw classes occurred, which is why raw digest equality is not used here as the equivalence criterion. *Scope: sampled matched live state at cass 0.6.19 on this host; not a universal causal or structural claim.*
**Improvements.** (1) declare effects, separating local-index from remote-ssh; (2) resolve `CASS-M`; (3) add `output_contract`; (4) bound `quick_analysis.sh`'s index call; (5) regenerate or delete `skill.spec.json`; (6) resolve `casr`; (7) add the post-verdict seam; (8) **any check that compares two status invocations must normalize *at least* the four observed volatile fields — `age_seconds`, `budget.elapsed_ms`, `generated_at`, `_meta.timestamp` — compare *matched live state*, and *fail or report* any field or shape that differs after that normalization rather than silently widening the normalization to absorb it. Treat the four as the fields observed to date, not as a closed set. A raw digest comparison passed or failed in the recorded samples in step with elapsed time — an **observed correlate, not an established sole cause** — which makes a raw comparison a flaky test rather than a correctness check.**
**Witness (hermetic).** A gate asserting every `cass index` invocation under `skills/cass/scripts/**` is wrapped in `timeout` — passes for `recover.sh`, fails today for `quick_analysis.sh:64`.
**checked** — all 24 paths / 4,500 lines in full; read-only live probes plus the **v6 authoring pass's 123 status command executions** (Trials A/B/C: 40 + 80 + 3) across three trials with field-level diffs, normalization, digest comparison, and leaf-path equality (§2.2). **not_checked** — `index`, `doctor --fix`, `sources sync`, `models install`, `pages encrypt`, ssh fan-out.
**Residual risk.** *Medium.* Doctrine excellent; risk is stale metadata, one unguarded headline script, and an owned no-mutation claim its own recovery script violates.
**Direct-to-Go.** `prompt_miner.py` is grandfathered debt with an `ao` destination; recovery orchestration is mechanism → Go.

### 10.6 `cc-hooks` — T7b, support; cross_cutting

**Intent.** Two things in one package: a hook-authoring reference, and a **shipped, default-on admission-control engine** over a policies-as-data registry.
**I/O/effects.** In: PreToolUse JSON on stdin. Out: exit 0 silent / exit 2 + route / `permissionDecision:"ask"`. Effects: writes `~/.claude/hooks/aop/*`, rewrites `~/.claude/settings.json` (with backup), appends hashed telemetry, writes session sentinels — declared `[]`.
**Authority.** Weakest point, executed not theoretical.
**Strong.** 28/28 green: fail-open on missing `jq`/registry; silent on malformed stdin; unit-separator field joining because `@tsv` mangles regex backslashes; per-session deny dedup; SHA-256-hashed telemetry storing no raw path; waiver expiry; schema-enforced predicate discipline with a real `allOf/if/then`. `GUARDRAIL-VALUE-PROOF.md` is the strongest methodological artifact in the thirteen.
**Defects (§9).** `CH-1` [P0] · `CH-2`, `CH-3`, `N12`, `S1`, `S2` [P1] · `N13`, `N17`, `CH-5`, `CH-6`, `CH-7`, `CH-8` [P2]. Sol: "`CH-1` at P0 is directly visible between lines 34/170 and line 279." Target gap: "prove safe removal and required cleanup" — no uninstall path specified or tested.
**Improvements.** (1) reconcile the default-on doctrine across all three documents; (2) move repo-internal `_beads`/`ledger.jsonl` policies out of the default user-scope cohort; (3) declare effects; (4) add a false-positive test — **§2.3's blocked probe is a ready-made fixture**; (5) specify and test hook removal/cleanup.
**Witness (hermetic).** Install the default cohort into a `mktemp -d` `HOME`, then in an unrelated non-AgentOps repo containing a `_beads/` directory run the **legitimate** `git add _beads/notes.md`; assert it is **not** blocked. Today it is — Sol's independently executed run returned rc 2 with the private-ledger block, from a contained HOME that received the dispatcher plus two settings matchers.
**checked** — all 15 paths / 3,057 lines in full; `lint-policies.sh` + 28 bats executed; installer executed under a contained `HOME`; the `_beads` regex read and evaluated directly. **not_checked** — installing into the real `~/.claude/settings.json`; long-run telemetry (needs N≥30 sessions).
**Residual risk.** *High.* A well-built enforcement layer pointed at repo-specific paths, on by default at user scope, is where a correct implementation produces a wrong outcome.
**Direct-to-Go.** Installer, dispatcher, policy engine, telemetry, and uninstall path are enforcement + user-file mutation + receipts → `ao`. The shell engine is not an end-state packaging precedent.

### 10.7 `codex-exec` — T7a, runtime; runtime_transport

**Intent.** One prompt, one process, one captured artifact — "when nothing loops, every byte of output traces to exactly one invocation." Named failure: **stdin hang**.
**I/O/effects.** In: prompt, workspace root, sandbox tier. Out: exit status + artifact. Effects: process execution at a declared tier incl. `workspace-write` and optionally network — declared `[]`.
**Authority.** Explicit and correct: "A nonzero process exit is runtime evidence, not a semantic verdict."
**Strong.** `SKILL.md:74-80` is the most precise fresh-validator statement in the corpus: acceptance digest, exact subject manifest digest, author context ID, evidence, required checked/not-checked report, and a validator context ID that must differ before `PASS` is possible.
**Defects (§9).** `S1`, `S4`, `LIVE-1` [P1] · `CE-4`, `CE-5` [P2]. **`CE-3` is retired as stated** — Sol: "correctly retired because two suites hold 25 behavior tests." T7a gaps: wall-clock/output bounds and process-tree cleanup are implemented in the shell library but typed in neither the skill nor a result schema; cancellation unspecified.
**Improvements.** (1) sandbox-tiered effects checkable against the invoked `-s` flag; (2) cite `scripts/lib/codex-exec.sh` (**520 lines**) as the implementation and `codex-exec-lib.bats` as its lock; (3) fix `LIVE-1` — fail closed or explicitly disclose the missing timeout; (4) type the run result; (5) fix `consumes`; (6) collapse to one trigger source.
**Witness (hermetic).** A stub `codex` that blocks on open stdin, invoked through the documented shape: must terminate within budget and classify STALL-TIMEOUT (124). The suite already proves this for the AGY adapter; the codex path needs the same case.
**checked** — `SKILL.md` in full (1 path, 80 lines); `codex-exec.sh`; `codex-exec-lib.bats` 9/9; `reviewer-adapters.bats` 16/16; **six** live owners classified (§4). **not_checked** — any `codex exec` invocation.
**Residual risk.** *Low-medium.* The proof layer exists and is green; the exposure is the untyped result and `LIVE-1`.
**Direct-to-Go.** `runtimecmd.go` already holds argv-building with LAW 0 fail-closed. Timeout/classification/cleanup should follow it into Go.

### 10.8 `converter` — T5, implementation; implement_method

**Intent as written.** Parse a canonical skill into a universal SkillBundle, render per target, bundle arbitrates.
**Intent as reconstructed.** An ad-hoc export tool writing to `.agents/converter/`, **not** the owner of the shipped Codex projection (`N2`), whose own schema document misdescribes both live targets (`N9`). Sol: "T5 implementation method, but not the shipped Codex-projection owner."
**I/O/effects.** In: skill dir, target, optional output dir, `--codex-layout`. Out: `SKILL.md`+`prompt.md` / `<name>.mdc` / `bundle.md`. Effects: recursive delete of the output dir, file writes, `rsync` — declared `[]`.
**Authority.** Under Implement authority, which makes an unbounded delete over caller paths a scope violation by construction.
**Strong.** The clean-write rationale and the projection-editing failure mode are correct. Passthrough parity checking exists at all.
**Defects (§9).** `CV-1`, `N1` [P0] · `N2`, `CV-2`, `CV-7`, `CV-8`, `N9`, `S1` [P1] · `CV-4`, `CV-5`, `CV-9`, `CV-10`, `N20` [P2]. Sol reproduced both P0 witnesses: a semantic `NEVER use SendMessage...` line disappeared from Codex output; a contained 4-file fixture became 2 files at rc 0.

**`CV-8`, as corrected at v5 and Sol-verified.** `local -n` at lines 283–284 is a Bash ≥4.3 nameref inside `collect_files()`. On `/bin/bash` 3.2.57 it is rejected as an invalid option, and because line 5 sets `set -euo pipefail` the run aborts with **exit 2**, **zero files created**, and the output directory never created. `collect_files()` is first called from `parse_bundle()` at lines 304–305, whereas `rm -rf "$output_dir"` is line 631 and `mkdir -p` line 633 — both inside `write_output()`. Parsing therefore fails before writing begins, and a **pre-existing output directory is not deleted** (sentinel witness). The same invocation on bash 5.3.15 exits 0 and writes 5 files. The defect is a genuine **P1** because the package declares **no** bash-version precondition anywhere, the shebang is `#!/usr/bin/env bash`, and the script's own usage line instructs `bash skills/converter/scripts/convert.sh …` — so on a stock macOS the converter is unusable, and the only diagnostic is a raw interpreter error naming a line number. It is a portability/availability defect, **not** a data-loss claim.

**Improvements.** (1) refuse an output dir that is the source, an ancestor, the repo root, or outside an allowlisted root — **by refusal, not by silently appending `/$BUNDLE_NAME`**; (2) rewrite instead of delete, and make parity content-aware; (3) state the projection-ownership boundary; (4) replace the validator; (5) make `collect_files` recursive or declare the limit; (6) **for `CV-8`, either drop the nameref or add an explicit `BASH_VERSINFO` preflight failing with "requires bash ≥ 4.3, found $BASH_VERSION"** — the current behavior already fails closed, so this is a diagnosability and portability fix.
**Witness (hermetic).** In a `mktemp -d` lab: (a) `convert.sh <fixture> codex <fixture>` must exit nonzero and delete nothing; (b) a fixture reference containing `NEVER use SendMessage…` must survive projection byte-for-byte or the run must fail; (c) for `CV-8`, `/bin/bash convert.sh <fixture> codex <tmp>/out` must fail with a version-precondition message, not `local: -n: invalid option`. **Never run any of these against a live tree.**
**checked** — all 4 paths / 989 lines in full, including all **718** lines of `convert.sh`; validator executed (2 passed / 0 failed); W1–W3 in containment; the `CV-8` witness executed three ways; codex-twin bytes compared. **not_checked** — the >100 KiB non-ASCII Cursor overflow (reasoned, not triggered).
**Residual risk.** *High.* The only skill that can destroy a source tree, and the only one whose stated purpose does not match what ships. `CV-8` does not contribute to that risk — it fails closed — but it does mean the tool is unrunnable on a stock macOS.
**Direct-to-Go.** A 718-line parser/rewriter/destructive projector is not thin argument plumbing. Projection is mechanism → Go, and `codex-sync.sh` already owns the shipped path. Porting also removes the bash-version class entirely.

### 10.9 `dcg` — T7b, support; cross_cutting

**Intent.** Not a guard implementation — a **response protocol** for when a guard fires: blocks are checkpoints; find a safe alternative before mentioning override; never request, generate, or run an allow-once bypass.
**I/O/effects.** In: blocked command + matched rule. Out: block, rule, risk, reversible alternative, validation. Effects: writes `.dcg.toml` / `.dcg/allowlist.toml` on explicit request — declared `[]`.
**Strong.** The risk-tiered approval table and the **approval-laundering** failure mode. `PHILOSOPHY.md`'s asymmetry framing. **And the protocol works** — it fired during v4's pass and again since (§2.3); the documented sequence was followed without requesting an override, and the work completed via a narrower alternative.

**`N4`, at installed `dcg 0.5.6`** — matrix reproduced exactly by Sol:

| Command target | Result |
|---|---|
| `/tmp/agentops-dcg-probe` | **allow** |
| `/private/tmp/x` | **allow** |
| `$TMPDIR/x` | **allow** |
| `"$LAB"` | **deny** — `core.filesystem:rm-rf-general` |
| `"$output_dir"` | **deny** — `core.filesystem:rm-rf-general` |
| `./build` | **deny** — `core.filesystem:rm-rf-general` |

The temp-root carve-out recognizes **literal temp roots (`/tmp`, `/private/tmp`) and the recognized `$TMPDIR` variable form**. It does **not** extend to **arbitrary unresolved variables** (`$LAB`, `$output_dir`) or to **relative paths** (`./build`). Both earlier formulations — "only literal" and "variable blocked" — remain withdrawn. The surviving defect: the package documents context-awareness as a general path-semantics property without disclosing that recognition is limited to specific temp forms, so the unresolved-variable `rm -rf "$dir"` form loses the carve-out.

**`DCG-U` — version/upstream truth, three claim classes kept distinct.**
1. **Installed host** (`dcg 0.5.6`): the probe matrix above; `rm -rf ./build` blocked; `core.filesystem:rm-rf-general`, not `…-dangerous`.
2. **Current upstream** (`github.com/Dicklesworthstone/destructive_command_guard`, not the validator's `github.com/anthropics/…`): six-digit numeric allow-once codes and a bounded evaluation policy in which elapsed analysis time becomes explicit review/block rather than silent allow. Sol read the primary upstream documentation and confirmed both the identity and the policy.
3. **Package documentation** (unversioned): claims v0.8.2, four-hex allow-once codes, legacy hook paths, silent fail-open timeout.

An unversioned document cannot be adjudicated by a single installed binary. **Every dcg claim must name its class.**

**Defects (§9).** `DCG-1`, `N3`, `N4`, `N5`, `DCG-U`, `S1`, `S2` [P1] · `DCG-6`, `DCG-7`, `N16` [P2].
**Improvements.** (1) correct the context-awareness claims to the measured matrix above, naming the recognized temp forms explicitly; (2) re-derive every rule ID, sample, and version from a **pinned supported version**; (3) document the `bash -c` / inline-fragment scanning cost as a known false-positive surface with the safe-alternative pattern; (4) add `output_contract` and typed config-write effects; (5) pin one install route to the correct upstream and preserve the human-only override boundary.
**Witness (hermetic).** A probe asserting every rule ID quoted in the package appears in live `dcg packs --verbose`, **labelled with the binary version it ran against**. Fails today on `rm-rf-dangerous`.
**checked** — all 9 paths / 1,707 lines in full; `validate-dcg.sh` exit 0; the six-case probe matrix. **not_checked** — network install routes; `allow-once` (human-only); `scan install-pre-commit` (writes a git hook); destructive probes.
**Residual risk.** *High for documentation, low for behavior.* The binary is conservative; the reference corpus is the least reliable in the set, and the wrong rule ID makes the documented false-positive fix a silent no-op.
**Direct-to-Go.** Typed config writes are mechanism → `ao`; the response protocol is exactly the know-how that belongs in a skill.

### 10.10 `ms` — T7b, support; cross_cutting, **standalone**

**Intent.** Retrieval only, with a hard split: MCP for search and load, CLI for feedback and outcomes, because only CLI writes are verified to land.
**I/O/effects.** In: query or skill id. Out: search JSON / full `SKILL.md` text. Effects: spawns a disposable MCP server; writes feedback/outcome rows to a live DB; rebuilds a local index — declared `[]`, **enforced by the skill's own validator**.
**Strong.** `mcp-search.py` is the best-engineered code in the set: fail-closed on every transport and protocol error, strict payload validation (`count` must equal `len(results)`), a private process group with escalating SIGTERM→SIGKILL reap in a `finally`, 30s default timeout — and a bats test proving the reap by asserting the mock server PID is gone.
**Defects (§9).** `MS-1`, `MS-3`, `MS-4`, `S1`, `S2`, `GOV-1` [P1] · `MS-5`, `N17` [P2]. Target gap: "split retrieval from write/admin modes and type authorization, effects, and cleanup" — "retrieval-only" prose does not discharge this.
**Additional live-owner defects** (from `scripts/ms-reindex.sh`, 301 lines, §4 row 6): substring process discovery then TERM/KILL of every match; unescaped `MS_REINDEX_PROBE_QUERY` interpolation into JSON; an embedded Python verifier. Sol independently confirmed the validator's false-effects assertion, the private-checkout assumption, and the repo-root script placement.
**Improvements.** (1) invert the validator to require accurate effects; (2) split retrieval / write / admin into typed modes with authorization; (3) **route the reindex mechanism into `ao`** and require executable identity, not a command-line substring; (4) add `output_contract`; (5) resolve the private-checkout question.
**Witness (hermetic).** `MS-1`: the corrected validator must **fail at HEAD** and pass only after `SKILL.md` declares real effects — a mutation test on a `mktemp -d` copy, never the live tree. Plus a hostile fixture proving process selection requires executable identity.
**checked** — all 4 paths / 506 lines in full; validator + 6 bats executed; `ms --version`/`config` read-only; `ms-reindex.sh` discovery and sweep sections read. **not_checked** — `ms feedback add`, `outcome`, `index`, `ms-reindex.sh` execution.
**Residual risk.** *Medium-high.* Behaviorally the safest retrieval path, but the reindex mechanism can kill an unrelated process and the skill cannot be satisfied by a consumer.
**Direct-to-Go.** `mcp-search.py` is grandfathered debt with an `ao` destination. `ms-reindex.sh` is the clearest Go mechanism candidate in the thirteen.

### 10.11 `ntm` — T7a, runtime; runtime_transport

**Intent.** Host explicit agent roles in persistent panes as **transport, not lifecycle controller**.
**I/O/effects.** In: session, pane, role, workdir, command. Out: identifiers, exact command, timestamps, exit status, robot state, transcript refs, degraded surfaces, observed/unobserved effects. Effects: creates/selects panes, sends commands — declared `[]`.
**Authority.** Among the cleanest: never start or probe merely because installed; dispatch each command once; pane roles grant no ownership, admission, Git, release, or delivery authority.
**Strong.** The **liveness truth-stack**, ordered strongest-first: artifacts on disk → transcript growth → robot state → bare process existence, with "a successful prompt send sits below all of these and proves nothing about work." Paired with **kill-the-witness**. Sol: "operational evidence, not lifecycle state." The most reusable operational doctrine in the corpus.
**Defects (§9).** `S1`, `S3`, `S4` [P1] · `NTM-4`, `NTM-5` [P2]. T7a gaps: typed roles/commands/scopes; deadlines; observation windows; bounded robot evidence; capability-unavailable behavior; cancellation; cleanup — none typed, though `--robot-capabilities` proves the discovery surface is real.
**Improvements.** (1) declare effects; (2) type the seven T7a obligations, starting with observation window and bounded robot output; (3) add a truth-stack probe; (4) point at the relocated contract; (5) fix `consumes`. Keep the truth-stack doctrine in the skill.
**Witness (hermetic).** A fake `ntm` on `PATH` whose robot state reports "busy" while no artifact appears and no transcript grows: the procedure must classify the pane **not live**. Pair it with a bounded-output case — robot state exceeding the declared cap must truncate with a disclosed marker.
**checked** — `SKILL.md` in full (1 path, 106 lines); `--robot-capabilities` read-only. **not_checked** — creating a session or pane, sending any command.
**Residual risk.** *Low-medium.* Doctrine strong, discovery surface real; nothing mechanical enforces either.
**Direct-to-Go.** Pane/session lifecycle and bounded evidence capture are mechanism → `ao`; the truth-stack is know-how and stays.

### 10.12 `rch` — T7a, **runtime; implement_method, cross_cutting**

**Intent.** Offload one explicit compilation command or inspect the remote compiler path, and report. Named failure: **green-local blindness**.
**RPI placement — corrected in v3, preserved.** rch participates in producing the subject (it compiles it), placing it under Implement authority for that seam. Sol: "compilation participates in candidate production."
**I/O/effects.** In: one caller-authorized command or diagnostic. Out: status (`remote` | `local_fallback` | `failed` | `not_proven`), commands, exit codes, worker, summary, checked/not-checked. Effects: daemon restarts, worker deployment, remote mutation — declared `[]`.
**Authority.** **Contradicted by its own references — §7.** The kernel's stop rule ("Do not include a next action") remains the plainest in the set, which is what makes the reference-level inversion so consequential.
**Strong.** `MACHINE_INTROSPECTION.md` states its jq paths "were verified against live output, not assumed." `FAIL_OPEN.md`'s failure-taxonomy sections remain genuinely excellent — **that praise is scoped to the taxonomy, not to "Don't Ask the Human."**
**Defects (§9).** `RCH-A` [P0] · `N10`, `N11`, `S1`, `S3` [P1] · `RCH-3`, `RCH-4`, `N16`, `RCH-5` [P2]. T7a gaps: bounded remote execution, cancellation, and cleanup untyped; clause 6 actively inverted by `N11` — Sol reproduced at `rch 1.0.26`: `rch check` rc 2 while `doctor --json` rc 0 with `success:true`, `failed: 0`, and a daemon warning.
**Improvements.** (1) **resolve `RCH-A`** — either the kernel's authority requirement governs and the references are rewritten, or the skill declares an explicit autonomous-remediation envelope with a named blast-radius ceiling; (2) state that `rch check` exit status adjudicates readiness and `doctor.success` does not; (3) write the six missing docs or inline them; (4) declare effects, separating read-only diagnosis from remote mutation; (5) add the no-verdict-weight disclaimer to `not_proven`; (6) strip `bd-w5r9`; (7) move remote mechanism to Go.
**Witness (hermetic).** §7's text gate over `skills/rch/references/**` — every destructive-class command must sit inside a block naming explicit caller authority. Fails today. Plus, for `N11`: a fixture asserting that with the daemon down the skill reports `not_proven`/`failed` and never a green reading of `doctor.success`.
**checked** — all 8 paths / 1,516 lines in full; `FAIL_OPEN.md` "Don't Ask the Human" and `RECOVERY_PLAYBOOKS.md:19` read verbatim; `rch --version`, `rch check` (exit 2), `rch doctor --json` (exit 0, `success:true`) read-only. **not_checked** — `rch exec`, `daemon start/restart`, `workers probe/sync-toolchain/deploy-binary`, `fleet deploy`, any ssh or remote permission mutation.
**Residual risk.** **High.** The reference corpus is technically credible *and* instructs unattended remote `sudo chown -R` on a shared worker. Credibility is what makes it likely to be followed.
**Direct-to-Go.** Remote execution bounds, cleanup, and receipts are mechanism → `ao`. The authority envelope must be typed and enforced in Go, not asserted in prose a sibling reference contradicts.

### 10.13 `swarm` — T7a, runtime; runtime_transport

**Intent.** Expose exactly one port: `dispatch_once(explicit_disjoint_packets, executor) -> per-packet candidate | evidence | error`. Named failure: **partial-batch launch**.
**RPI placement.** The optional factory port — the only skill whose contract is also a first-class repo contract (`orchestration-ports.md`), and the two agree verbatim. Sol: "validates the full explicit batch and owns no work selection/retry/verdict state."
**I/O/effects.** In: complete packets with proven-disjoint write scopes, plus an executor. Out: per-packet result or factual error, input order preserved. Effects: `[invoke_selected_executor]` — truthful, though it understates transitive blast radius.
**Authority.** Comprehensive and correct: no selection, packet creation, scheduling, queue, ownership, retry, validation, integration, closure, Git, or delivery.
**Strong.** The most disciplined implementation in the set: rejects absolute paths and `..`, rejects duplicate `packet_id`, treats a bare `.` prefix as conservatively overlapping, uses `zip(..., strict=True)`, returns executor exceptions as factual per-packet errors with **no retry**. The batch-before-dispatch guarantee is proven by a test asserting `calls == 0`.
**Defects (§9).** `SW-1`, `SW-3`, `GOV-1` [P1] · `SW-2`, `SW-4` [P2]. **T7a clause 3 is the headline:** isolation must be proven, not inferred from paths — swarm proves only lexical prefixes, and generated surfaces, shared resources, external effects, and failure cleanup are not isolated at all.
**Improvements.** (1) resolve paths against a canonicalized workspace root before comparison, or declare and enforce a no-symlink precondition; (2) case-fold conditionally on detected filesystem case-sensitivity; (3) honor `exclude` or raise on its presence; (4) extend admission beyond paths to shared resources and external effects; (5) state that packet effects are the caller's to declare; (6) move admission mechanism to Go.
**Witness (hermetic).** Two additions to the existing suite, both in `mktemp -d`: `test_case_differing_scopes_are_not_assumed_disjoint` and `test_symlinked_scopes_are_not_assumed_disjoint` — each must raise `ValueError("write scopes overlap")` **and** assert `calls == 0`. Both fail today; the existing 6-test suite covers neither. Sol reproduced both failures with independently executed witnesses that first showed the host treats each pair as the same path.
**checked** — all 3 paths / 252 lines in full; 6/6 executed; both `_overlap` probes executed; host case-insensitivity proved. **not_checked** — behavior under a real executor (by design, caller-supplied); real multi-host dispatch.
**Residual risk.** **High.** Two independent, reachable defeats of the sole invariant, both silent — corrupted parallel work, not an error.
**Direct-to-Go.** `dispatch_once.py` is grandfathered debt with an `ao` destination. Path canonicalization and resource-isolation proof are exactly the verification ADR-0016 assigns to Go.

---

## 11. Preserved 78-file inventory

Re-derived from `git ls-tree` at this same HEAD in an earlier pass of this audit line and carried forward unchanged, **matching the v2 manifest byte for byte**; v9 did not re-run the enumeration. Sol's independent enumeration reproduced 13 skills / 78 files / 14,015 lines / 466,080 bytes with matching per-skill totals.

| Skill | Paths | Lines | | Skill | Paths | Lines |
|---|---:|---:|---|---|---:|---:|
| account-rotation | 1 | 50 | | dcg | 9 | 1,707 |
| agent-mail | 4 | 829 | | ms | 4 | 506 |
| agent-native | 3 | 373 | | ntm | 1 | 106 |
| agy-native | 1 | 50 | | rch | 8 | 1,516 |
| cass | 24 | 4,500 | | swarm | 3 | 252 |
| cc-hooks | 15 | 3,057 | | | | |
| codex-exec | 1 | 80 | | **TOTAL** | **78** | **14,015** |
| converter | 4 | 989 | | | | |

Counts `1,4,3,1,24,15,1,4,9,4,1,8,3`. The only on-disk file outside the set is `skills/swarm/scripts/__pycache__/dispatch_once.cpython-314.pyc` — untracked, gitignored, not canonical, not a finding.

**Coverage honesty.** All 78 owned files were read in full across v1+v2 and that coverage is inherited. v3 additionally read the plan matrix, T7a/T7b acceptance, ADR-0016 §3 + amendment, ports contracts, `codex-exec.sh`, `reviewer-adapters.bats`, the Go owners, `ms-reindex.sh`, the grandfather, the `_beads` rule, the rch reference ranges, and `fake_model_runner.py:155-195`. v4 measured the two corrected line counts, the AGY grep and filter, the per-skill path/line table, the grandfather membership, and the dcg probe matrix. v5 executed the three-run Bash 3.2 converter witness and the three-sample cass comparison. **v6 additionally executed** the cass trial set now correctly accounted at **123 command executions** (§2.2) and re-confirmed every preserved headline count. Independently, Sol read all 13 canonical `SKILL.md` files in full and every finding-bearing source range at v5.

---

## 12. Global checked / not_checked, and closing state

### Checked — added at **v12**

- Both direct inputs SHA-verified at open and read in full: the **v11 audit**
  (`c4ee8551…cec2`, 1,032 lines / 127,877 bytes) and the **v10 Sol review**
  (`f13581e4…eaf0`, 223 lines / 15,574 bytes). v11's byte count and digest reproduced the values
  supplied to v12; **v11's exact line count, 1,032, was computed at v12.**
- **One read-only repository check**, performed once immediately before the single write:
  `git rev-parse HEAD` → `0088c6e3824da201eabb1e751ac8e976599e0b5c`;
  `git rev-parse HEAD^{tree}` → `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`;
  `git status --porcelain` → 0 rows. **That is the entirety of v12's repository interaction.**
- Whole-file sweeps over the **final v12 bytes** for operative provenance phrases, current-version
  self-labels, qualitative incidence vocabulary, Markdown table structure, and headings stranded
  inside table blocks (§0.2).
- Every v11 footer and close-state site named in §0.1's defect table, located mechanically by line
  and corrected at its point of use.

### Not checked — added at **v12**

- **No repository witness, gate, validator, suite, generator, projection, or skill command was
  executed.** Beyond the three read-only Git queries above, v12 executed nothing against the
  repository.
- **No cass command was issued and no CASS population was created.** Every CASS number is the
  number the **v6 authoring pass** recorded.
- **No technical claim was re-derived.** v12 inherits v11's technical content wholesale; it did not
  re-verify a count, a digest, an inventory figure, an owner classification, or a per-skill
  judgment. A technical error present in v11 is present here.
- **No predecessor artifact or review was read for new evidence** beyond the two direct inputs.
- **v12 has not been independently reviewed, and no PASS, verdict, or validation is claimed.**

### Checked — added at **v11** *(history)*

- Both direct inputs SHA-verified at open and again immediately before the single write: the **v10
  audit** (`5724141e…63b0`, 907 lines / 115,140 bytes) and the **v10 Sol review**
  (`f13581e4…eaf0`, 223 lines / 15,574 bytes). Both read in full.
- Repository identity at open and close: HEAD `0088c6e3824da201eabb1e751ac8e976599e0b5c`, tree
  `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`, zero porcelain rows.
- **The widened incidence vocabulary run over the final v11 bytes**, with every hit triaged into
  classes A–D and each class enumerated by site (§1.1).
- **The three v10 sites the review named plus three it did not** — v10 lines 250, 252, and 306,
  plus three "some recorded samples" hedges at v10 lines 593, 662, and inside the §9.7 refuted-claims
  table — all located mechanically by an end-to-end sweep of the final bytes, and all corrected.
- **The retained Trial B record re-read** to establish exactly which counts the evidence derives:
  40 matched pairs, 3 byte-identical with digests, 37 differing, pair 11's exact three-field shape,
  and no retained per-pair distribution over the other 36.
- **Every `this revision` / `this pass` site** re-read and either version-named or confined to a
  labelled withdrawn quotation.
- **Mechanical Markdown table-structure scan** over the final v11 bytes: every header+separator has
  adjacent rows; zero orphan rows.
- Section renumbering verified non-colliding: §1.0 current, §1.2–§1.6 history, each version-named.

### Not checked — added at **v11** *(history)*

- **No CASS command was executed and no new population was produced.** Every CASS number is the
  number the **v6 authoring pass** recorded, carried unchanged through v7, v8, v9, and v10.
- **The unretained per-pair field-difference distribution was not reconstructed.** It is not
  derivable from the retained record, which is why §2.2 now states existence rather than proportion.
- No repository file, projection, Git state, external service, or remote was read for new evidence
  **at v11** beyond that pass's identity confirmation; every technical fact is carried from v10,
  which the v10 Sol review passed on all technical criteria.
- No finding, severity, count, owner classification, projection result, inventory figure, or
  per-skill judgment was revisited **at v11**. That revision was wording and structure only.

### Checked

**Added at v10:**

- Exact v9 subject identity: SHA-256 `8ea3ebdaa729aadf8c09e1484603598ba2c17209cd32af7808c7adb6f32f8183`, 859 lines, 107,112 bytes — recomputed and matched.
- Exact v9 Sol review identity: SHA-256 `47a290749a409c1d57690e4ce0fe054917966e633db7b92ac214d4e7dae00a82`, 301 lines, 20,108 bytes — recomputed and matched; **read in full**.
- Repository identity at open and close: HEAD `0088c6e3824da201eabb1e751ac8e976599e0b5c`, tree `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`, zero porcelain rows.
- **RC-1 site re-derived:** `sed -n 259p` on the v9 subject reproduced the exact sentence carrying "and also fails routinely," and lines 45 and 824 were re-read to confirm the internal conflict.
- **RC-2 site re-derived:** `sed -n 27p` reproduced the "values v8 declared" sentence; the closing chain sentence was read in full.
- **RC-2 attribution proven by exact search of the v8 audit on disk:** `dcce5130dc6621133d21bb3c8868fb676af745605ba05ab5f6f9dd9d15b1152a` → **0 occurrences**; `e2476f12ef1333ea2acb85d5b8044ef200db3cc2a74be5394881fc65c65b7d88` → **0 occurrences**; the six v7/v6/v5 audit and review digests → **exactly 1 occurrence each**. The v8 audit's own on-disk whole-file digest was independently computed as `dcce5130…152a`.
- **Whole-file frequency sweep** over 21 qualitative-rate terms: three hits, one operative (line 259), two in labelled quotation/pattern-list positions.
- **Whole-file provenance-source sweep**: every "declared"/"inherited" attribution sentence re-read.

**Preserved from v9, validated by the v9 Sol review at this exact tree:**

- All 78 owned canonical files (inherited full read; inventory and per-skill counts re-verified in an earlier pass of this audit line at this identical HEAD and carried forward unchanged, not re-executed at v9; independently re-read in full by Sol at v5).
- Both input artifacts SHA-verified and read in full.
- Active target matrix, T7a/T7b acceptance, ADR-0016 + amendment, ports/orchestration contracts, frontmatter and hooks schemas, generated-projection owner map; Sol confirmed all 13 Codex overrides are `parity_only`, all 13 manifests name `codex-sync`, and parity audits return `[]` 13/13.
- **Six** live Go/shell owners classified with line counts; two live test suites executed (25 tests).
- **`CV-8`**: interpreter identity, exit status, stderr, output-file count, contrast run, sentinel-ordering run, static call-order, and the absent version precondition — Sol-reproduced.
- **`CASS-2` and the `D2b` correction — both observed classes recorded:**
  - **Byte-identical trials.** v6 authoring pass: 3 of 40 cross-form pairs, digests `773e9c09…`, `f7c79a19…`, `ac9a8f1d…`. Sol: same-form calls 35/36/37 sharing `29b2d058…`, and cross-form pair 15 at `36e70f12…`.
  - **Differing trials.** v6 authoring pass: 40 unique digests across 40 same-form invocations; 37 of 40 cross-form pairs differed. Sol: a three-sample run crossing a one-second boundary differed in all four fields. v5's original three samples were all raw-distinct.
  - **Field mechanics.** `budget.elapsed_ms` had 30 unique values of 40 with 13 agreeing unordered pairs; 18 of 39 consecutive same-form pairs differed *only* in that field. **Across the recorded comparison populations of both parties, no non-volatile field difference was observed** — a statement about those sampled populations, not a proof that none can occur.
  - **What this audit relies on — sampled matched-live-state structural equivalence.** Every sampled matched cross-form pair normalized equal under the tested four-field normalization, with no form-specific difference observed in that sample. **That sampled equivalence is what this audit relies on.** The particular digest `65ae94e3…` and 494 set-equal leaf paths reproduced across three independent sessions and are **recorded sample evidence, not a structural constant** — the v7 review observed a 565-leaf-path shape at the same cass version.
- Preserved counts re-confirmed: 520 / 718 lines; 78 paths / 14,015 lines / 466,080 bytes; 24 grandfather pins with exactly four in scope; 12 raw / 11 live AGY paths with per-file occurrence counts.
- Sol's independently executed witnesses for `AN-2`, `LIVE-1`, cc-hooks install and `_beads` block, converter line-deletion and source=output, `RCH` doctor/check inversion, and both swarm alias classes; the dcg 0.5.6 matrix; and the deterministic suites (`reviewer-adapters` 16/16, `codex-exec-lib` 9/9, ratchet PASS 24, swarm 6/6, CC lint PASS, `policy-dispatch` 28/28, MS validator PASS, MS bats 6/6, converter validator 2/0, `validate-dcg` PASS, `codex-sync --check` PASS, parity `[]` 13/13).
- Opening and closing HEAD/tree/status, re-verified at v9 and matching v8's recorded snapshot exactly.
- **What v9 itself performed, and nothing more:** re-read the v8 audit and the v8 Sol review in full; recomputed all eight predecessor digests against the files on disk (8/8 match); confirmed against the **v6 source** that Trials A/B/C and the three byte-identical digests originate at v6 §2.2 lines 96–121; applied RC-1 and RC-2 as asserted literal-count replacements; ran the §1.1 whole-file sweep; and re-verified the repository identity at close. **No cass command, no witness, no suite, and no repository read beyond `git rev-parse`/`git status` was executed at v9.**

### Not checked

**Added at v10:**

- **No CASS command was executed at v10**, and no new CASS population was created. Re-running CASS would produce a new sample rather than change what the inherited v6 population showed.
- **No skill, gate, validator, suite, generator, or projection command was executed at v10.** Every executed result in this document is inherited from the pass that ran it and is attributed there; the v9 Sol review independently re-ran the safe suites at this exact tree.
- **No repository file was read line-by-line at v10.** The three defect sites, the v8 audit digest search, and the two sweeps were the only reads; all substantive per-skill evidence is inherited and was validated by the v9 Sol review at this identical HEAD and tree.
- **No frequency, rate, or likelihood claim is made anywhere in this document**, and none can be supported by the finite inherited samples.

**Preserved from v9:**

- **All external mutation.** No account rotation, mail register/reserve/send/reset, runtime session, cass index/fix/sync/model-install/encrypt, ms feedback/outcome/index/reindex, rch exec/daemon/workers/fleet/ssh, hook installation, converter destructive witness against a live tree, `dcg allow-once` (human-only), Git mutation, deployment, merge, or push.
- Network install routes or release-artifact integrity; upstream DCG documentation was read, not installed.
- `tests/integration/test_multi_model_dispatch.bats`.
- The Cursor >100 KiB non-ASCII overflow (reasoned from code, not triggered).
- Full drift across every runtime projection; Codex ownership/parity for all 13 was checked.
- Long-horizon guardrail telemetry (`GUARDRAIL-VALUE-PROOF.md` needs N≥30 sessions; its own status note says the criterion is not cleared).
- Whether `CV-8`'s failure mode differs on Linux, or on macOS where `/bin/bash` has been replaced. Only `/bin/bash` 3.2.57 and `PATH` bash 5.3.15 were exercised.
- **The rate at which byte-identical `cass status` outputs occur.** The v6 pass's 123 command executions plus Sol's own trials establish that both outcomes happened; they do not establish a frequency, and the frequency would in any case depend on host load and index size. No claim about likelihood is made.
- **Whether cass's volatile-field set is exhaustive beyond the four observed.** No sampled comparison by either party surfaced a fifth changing field, but that is evidence of stability across the sampled populations, not proof of completeness. **The former "123 comparisons across both parties" total is withdrawn (v7 RC-1):** 123 is the **v6 pass's** *execution* count, and comparison populations from different trials and different parties are not additive. The exact populations are stated per trial in §2.2. Finite sampling cannot establish that no fifth field or alternate live status shape exists.
- **v6 did not re-read the 78 owned files line by line.** It re-measured the one disputed claim by execution plus every preserved headline count. Substantive per-skill findings are inherited from the full reads of v1+v2 and the targeted reads of v3–v5, all at this identical HEAD with a matching manifest, and were independently corroborated across all 13 skills by the v4 and v5 Sol reviews.
- **Nothing was re-executed at v9 to confirm inherited evidence.** RC-2 explicitly forbids re-running CASS merely to relabel provenance, so every CASS, witness, suite, and inventory fact in this file remains inherited at the strength its originating pass gave it. A reader who needs those facts re-proved must re-run them; v9 did not.
- **No `verdict.v2` is produced. No PASS is claimed.** This is advisory evidence; the context that authored it is not author-distinct from a Validate over the same subject.

### Residual risk

The underlying source risks are unchanged by an evidence correction and remain high: uncontained converter deletion (`CV-1`), line-deleting conversion (`N1`), contradictory default-on host enforcement (`CH-1`), RCH authority inversion (`RCH-A`), the untyped agent-mail admin surface (`AM-1`), false effect declarations across ten skills (`S1`), lexical-only swarm isolation (`SW-1`/`SW-3`), unbindable fixture freshness (`AN-2`), unbounded timeout fallback (`LIVE-1`), grandfathered Python on execution paths (`GOV-1`), and mechanism still shipped in skills and shell.

Report-level residuals:

1. **`CV-8`'s corrected impact narrows what the finding proves, and that cuts both ways.** The converter fails closed on Bash 3.2, so it is not a data-loss vector there. But the earlier overstatement sat in the *counted* ledger for two revisions. A runtime-impact claim belongs in a counted row only after execution on the platform it names.
2. **The `CASS-2` byte-identity error and its overcorrection were both methodology errors, in opposite directions.** v4 cited raw byte identity as the refutation basis. v5 replaced it with a universal denial — "never achievable," "no two invocations can ever be byte-identical" — generalized from three samples; trials recorded by both parties refute it, since byte identity did occur. The durable rule is narrower than either: **an equivalence claim over live command output must state its normalization, and a sample-window observation must not be promoted to a universal.** Both directions of that error appeared in consecutive revisions of this report, which is the strongest available argument for the rule.
3. **`MS-1a`** (§9.5) is disclosed rather than renumbered; a future pass must not treat it as a missing row.
4. **`S3`'s narrowing to four members** reduces a claimed rollup to what the evidence supports. If three further members exist, they were never enumerated in any predecessor and must be found, not assumed.

### Closing state

**Read fresh for v12.** Immediately before the single write of this file, the v12 pass performed
one read-only check of the repository — `git rev-parse HEAD`, `git rev-parse HEAD^{tree}`, and
`git status --porcelain` — and observed:

```text
worktree: /Users/bo/dev/agentops-worktrees/skill-overhaul
branch:   codex/skill-overhaul-20260724
HEAD:     0088c6e3824da201eabb1e751ac8e976599e0b5c
tree:     c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
status:   clean (0 porcelain rows)
```

**That is the whole of what the v12 pass measured**: three read-only Git queries, at the moment
stated, returning the values above. It is not a claim that any earlier pass re-verified anything,
and it is not a comparison against a predecessor's snapshot. Earlier snapshots belong to the passes
that measured them: **v8 recorded this HEAD/tree/status, and v9, v10, and v11 each recorded it
again at their own opening and close** — those are their measurements, not v12's, and each is
attributed in that pass's own checked list.

**v12 ran no witness, issued no cass command, and executed nothing against the repository beyond
the three read-only Git queries above.** No source, projection, verdict, Git index, commit, merge,
generated artifact, predecessor artifact, review, or remote state was changed at v12. Every witness
in §2 ran in a scratch directory outside the repository **at the pass credited with it**, and the
**v6 pass's** 123 cass command executions were read-only status queries.

---

*Written once, atomically, to `/tmp/agentops-opus5-verified-skill-audit-adapters-13-v14.md`. Its whole-file SHA-256, line count, and byte count are computed after the single write and reported by the caller — a file cannot truthfully contain its own whole-file digest.*

*Provenance chain, newest first — each link states the artifact it supersedes and the review that authorised the correction.*

*1. **v14** (this document) supersedes the **v13 audit** (`1d1709a3b5be5557e51776e94a0c11612028cf406cc16b74d8116ce8b335d3f5`, **1,146 lines / 138,200 bytes**, mode `0400`) under the **v13 Sol review** (`86e8b566998972f9e9c0277036ab9463e119d341cc9ba469b8216727f2290571`, **171 lines / 13,619 bytes**, mode `0444`), whose disposition is `REQUEST_CHANGES` **solely on RC-3** with every technical criterion reproduced. v14 is a **provenance-order reseal**: it rewrites this paragraph and changes no technical content, count, finding, severity, per-skill analysis, evidence item, limitation, or correction-history row.*

*2. **v13** superseded the **v12 audit** (`74224c9f64b4050632395a91cdd23e6a032a5aaccdb1e690efcdac17e06c83d5`, **1,133 lines / 136,158 bytes**) under the **v12 Sol review** (`dc5cb34fa623b44f67ea7bcd688c0d0679626c86869b8bd94e83d17d12a2ac53`, **180 lines / 13,175 bytes**), which raised exactly two report-integrity items and reproduced every technical criterion.*

*3. **v12** superseded the **v11 audit** (`c4ee8551cebfc321f91fe2a84b9e65d4d2385722e58773fe127e863eca38cec2`, **1,032 lines / 127,877 bytes**) as a **report-integrity reseal**, author-lane initiated: there is no Sol review of v11. v12 corrected point-of-use provenance only and changed no technical content, count, finding, severity, per-skill analysis, evidence item, limitation, or correction-history row.*

*4. **v11** superseded the **v10 audit** (`5724141e…63b0`, 907 lines / 115,140 bytes) under the **v10 Sol review** (`f13581e4…eaf0`, 223 lines / 15,574 bytes), adopting its two required corrections. **5.** v10 superseded the **v9 audit** (`8ea3ebda…8183`, 859 lines / 107,112 bytes) under the **v9 Sol review** (`47a29074…0a82`, 301 lines / 20,108 bytes), adopting its two required corrections. **6.** v9 superseded the **v8 audit** (`dcce5130…152a`, 797 lines) under the **v8 Sol review** (`e2476f12…5d88`, 528 lines), adopting its two required corrections. **7.** v8 superseded the **v7 audit** (`eb3fedd8…5ac1`, 775 lines) under the **v7 Sol review** (`e69e9d29…6920`, 464 lines). **8.** v7 superseded the **v6 audit** (`307bea30…1cbcb`, 730 lines) — **the pass that executed the 123 cass command executions this file cites** — under the **v6 Sol review** (`adb147b7…66b0`, 419 lines). **9.** v6 superseded the **v5 audit** (`a02c3876…40bb`, 724 lines) under the **v5 Sol review** (`71b1f36a…3998`, 384 lines). **The chain above is complete and strictly newest-first: v14 → v13 → v12 → v11 → v10 → v9 → v8 → v7 → v6 → v5.** Predecessors `…-v13.md` (`1d1709a3…`), `…-v13-review-sol.md` (`86e8b566…`), `…-v12.md` (`74224c9f…`), `…-v12-review-sol.md` (`dc5cb34f…`), `…-v11.md` (`c4ee8551…`), `…-v10.md` (`5724141e…`), `…-v10-review-sol.md` (`f13581e4…`), `…-v9.md` (`8ea3ebda…`), `…-v9-review-sol.md` (`47a29074…`), `…-v8.md` (`dcce5130…`), `…-v8-review-sol.md` (`e2476f12…`), `…-v7.md` (`eb3fedd8…`), `…-v7-review-sol.md` (`e69e9d29…`), `…-v6.md` (`307bea30…`), `…-v6-review-sol.md` (`adb147b7…`), `…-v5.md` (`a02c3876…`), `…-v5-review-sol.md` (`71b1f36a…`), `…-v4.md` (`3d31f5bb…`), `…-v4-review-sol.md` (`0992bcb5…`), `…-v3.md` (`90f88fae…`), `…-v3-review-sol.md` (`6bf86b57…`), `…-v2.md` (`64ef95bf…`), `…-v2-review-sol.md` (`5679d387…`), `…-13-review-sol.md` (`d88c0ef4…`), and `…-adapters-13.md` (`2a8f02ca…`) are all preserved unchanged. **This v14 summary attributes inherited work only to its originating pass; predecessor misattributions are retained only as marked correction history, each labelled with the version that originated it.** The preserved v7 and v8 artifacts literally contain the historical "this pass" / "fresh" misattributions that this chain quotes as withdrawn; those quotations remain solely as labelled correction history and assert nothing about the v14 pass.*

***v14 has not been independently reviewed.*** A fresh Sol review of **v13** exists and is v14's correction authority, but **no review of v14 itself has been performed**. ***No `verdict.v2`, no PASS, and no runtime verdict, validation, or implementation authority of any kind is claimed for v14.*** v14 **executed no technical witness, no CASS evidence, and no repository evidence at all** — it inherits, but does not re-establish, the technical criteria reproduced by the v12 and v13 Sol reviews. It is advisory evidence produced by an author lane. *v13 declared this lineage frozen; the v13 Sol review then raised RC-3, so v14 was authored as a report-only reseal. No further audit revision is proposed, and none is warranted absent a further binding review finding.*
