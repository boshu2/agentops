# Fresh independent review — AgentOps verified skill audit outcomes 12 v13

**Date:** 2026-07-28  
**Reviewer:** Sol, fresh context  
**Disposition:** **REQUEST_CHANGES**

## 1. Reviewed subject

The reviewed bytes were:

```text
path    /tmp/agentops-opus5-verified-skill-audit-outcomes-12-v13.md
sha256  3eacc27504c36c23346f37372be59ad6a40ddeafd37376568d4cc1476c5bfe24
lines   1349
bytes   205016
mode    0444
```

The repository subject was the clean landing worktree:

```text
path        /Users/bo/dev/agentops-worktrees/skill-overhaul
HEAD        0088c6e3824da201eabb1e751ac8e976599e0b5c
HEAD tree   c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
HEAD:skills a264078d2795f3f7238e0c9ff5a8c15149a77c8d
status      clean
```

The subject identity and repository identities matched at open and close.

## 2. Executive judgment

The v13 report remains a strong and unusually well-evidenced audit. Its exact
106-row manifest, package counts, finding ledgers, target/seam assignments,
effect buckets, Python-ratchet result, closed-layout result, and the substantive
behavioral analysis of all twelve skills reproduce against the pinned landing.
The four intended v12-to-v13 edit topics materially improve the report:

- `goals measure --json --help` is now attributed to the exact v8 Sol review;
- the broad v8-only paragraph was replaced by a heterogeneous origin table;
- §11 no longer says v10 completed attribution;
- the close attempts to enumerate the complete historical origin set.

That is not enough for PASS. The new provenance repair still contains a direct
contradiction about `security/scripts/validate.sh` and two review-count errors.
Two stale current-version labels also survive, and three per-skill sections make
source-checkable factual claims that are false. These are report-integrity
defects; they do not overturn the core repository findings, but this task asks
for an exact, independently proved audit rather than an approximately correct
one.

## 3. Blocking findings

### B1 — the repaired provenance map is still internally and historically inconsistent

The §9.1 origin table says:

> `security/scripts/validate.sh` — the v5 Sol advisory only

That `only` is false.

- The sealed v8 Sol review
  `c57b71d410b6249e4b94481bec78ee61e84c1c40b6256ab338111aba47aa2a66`
  records `security validator: PASS` in its fresh-probes list.
- The sealed v9 Sol review
  `c22ce97509b9642610e2449dc7ef1914ed05c124e015c8ca2b2bb4db3efad046`
  records `security validator PASS with bytecode redirected outside the
  repository`.
- v13's own §8.9 `Checked` block correctly attributes the most recent
  scratch-confined execution to that v9 review.

The same repaired site then says the executable evidence originates in **three**
reviews. Its own table spans the v5, v7, v8, and v9 reviews: **four** review
contexts. The close footer says the complete evidence was produced by “four
author passes and four reviews,” then enumerates the v6 Sol advisory, v7 Sol
review, v8 Sol review, v9 Sol review, and v5 Sol advisory: **five** review or
advisory contexts.

This directly reopens the v12 review's provenance blocker. The repair must use
one stated rule consistently:

- if the table records the most recent execution, attribute the security
  validator to v9 and retain v5/v8 as prior executions;
- if it records every execution, enumerate v5, v8, and v9 for that validator;
- if “origin” means first known execution, remove `only`, distinguish origin
  from later reconfirmation, and keep point-of-use attribution consistent.

Then correct the executable-review count to four and the complete
review/advisory-context count to five.

### B2 — two operative stale deictic labels survive the version sweep

Two current-body sites still describe old generations as the current one:

- §8.9 `not_checked`: “Neither the v8 pass nor **this v10 reseal** executed …”
- §12 manifest provenance: “**in this v8 pass** …”

In v13 these must be “the v10 reseal” and “the v8 pass,” respectively, with the
later no-rerun boundary stated independently where needed. This is especially
important because §11 and §10 claim the stale/current-pass attribution class is
now completely measured.

### B3 — `automation-shape-routing` has nine declared spec sections, not eight

Section 8.1 says `skill.spec.json` contradicts `SKILL.md` on “sections
(8 declared, 0 present).” Direct parsing of
`skills/automation-shape-routing/skill.spec.json` returns **9** section objects:

```text
title
three-shapes
decision-rule
spike-nuances
traps
worked-examples
workflow-template
handoff
contract-note
```

The stale-second-source finding remains valid. The declared count must be
changed from eight to nine.

### B4 — the `craft-goal` strength and validator line count are false

Section 8.2 calls `craft-goal` the “Highest craft score of the twelve.” A fresh
twelve-skill `audit.sh` sweep gives:

```text
security       25/30
doc            24/30
pattern-mining 24/30
skill-builder  24/30
standards      23/30
craft-goal     22/30
```

`craft-goal` is therefore not highest. The same section calls
`scripts/validate.sh` a “three-line shim”; the tracked file is **6 physical
lines**, exactly as v13's own §8.0 taxonomy already says. Its behavioral
classification is still correct: the script only delegates to
`heal.sh --check --strict` and never reads a compiled goal.

### B5 — the `doc` validator has 14 checks, but they are not “all 14 greps”

Section 8.3 repeatedly says all 14 checks are greps of doc's own prose. The
validator contains:

- **14 checks**;
- **14 grep invocations distributed across 13 checks**;
- one separate `[ -f SKILL.md ]` existence check;
- 12 grep invocations involving `SKILL.md`;
- two grep invocations against package reference files.

Section 8.0 already gives the more defensible physical description: 24 lines
and 13 lines containing `grep`. The substantive conclusion still holds—the
validator is static and does not exercise doc generation—but the numeric
description in §8.3, §9.1, §10, and the witness should be made exact.

## 4. Twelve-skill review, one by one

| Skill | Intent, actual behavior, and RPI placement | Evidence / strengths / defects / improvements | Result | Confidence |
|---|---|---|---|---|
| `automation-shape-routing` | Pure pre-loop router; support target, `cross_cutting + standalone`, T3; selects and hands off without starting a runtime. | The `awk` boundary and smallest-shape rule are real strengths. The stale second metadata source and invocability spelling defect reproduce. Five improvements are present and appropriately scoped. Correct 8 declared sections to 9. | **REQUEST_CHANGES (B3)** | High |
| `craft-goal` | Campaign-level goal compiler/linter; `goal_design`, T3, above bounded RPIs; it authors but does not exercise continuation authority. | Structural-only validator, absent output witness, and unstated continuation boundary reproduce. The proposed hard-envelope/token witness is discriminating. Five improvements are useful. Correct the score ranking and 3-line claim. | **REQUEST_CHANGES (B4)** | High |
| `doc` | Caller-authorized implementation operation; `implement_method`, T5; scaffold/refresh policy is bounded, while references leak work-creation and next-action authority. | False-empty effects, forbidden `.agents/doc`, stale surfaces, weak validator, work-authority references, and markdown/coverage defects reproduce. The no-overwrite fixture is a strong future witness. Five improvements cover the repair. Correct the validator arithmetic. | **REQUEST_CHANGES (B5)** | High |
| `domain` | Exact read-only terminology lookup; evidence target, `plan_input + cross_cutting`, T4; returns definition plus source and stops. | Boundary and provenance are clean; no validator is a low-severity ceremony gap. Five improvements are present without inflating the role. | **PASS** | High |
| `goals` | Caller-selected measurement; product target, `product_input + goal_observe`, T3; observes goals but live commands may write snapshots and `render --out`. | Eight-command tree, qualified `RunMeasure` path, false-empty effects, and documentation drift reproduce. Global `--json` refutation is correctly repaired and attributed. Three future witnesses and five improvements are sound. | **PASS** | High |
| `operationalize` | Post-verdict advisory proposal; evolution target, `post_verdict + plan_input`, T6; neither creates work nor validates/promotes its own output. | Three-instance floor, reapply proof, and quote bank are strong. Unschemaed `.v1` and write-vs-return ambiguity reproduce. Five improvements are coherent. | **PASS** | High |
| `pattern-mining` | Post-verdict promotion decision; evolution target, `post_verdict + plan_input`, T6; promotes only after exemplar, holdout, and back-application checks. | The live `jq` validator and its 2/2 Bats fixtures reproduce and are the best current output witness in scope. False effects, forbidden layout, and undeclared `jq` reproduce. Five improvements are appropriately declaration/location focused. | **PASS** | High |
| `product` | Creates/refines caller-authorized `PRODUCT.md`; product target, `product_input`, T3; shapes future intent without selecting work. | Evidence/aspiration split is strong. Byte-preservation has no validator and the context/detector gaps reproduce. The proposed preservation witness and five improvements are adequate. | **PASS** | High |
| `security` | Bounded evidence collection; evidence target, `validate_evidence + standalone`, T4; it has no remediation, risk-acceptance, promotion, or release authority. | Broken duplicate gate, 4/6 redteam failures, nonexistent declared report, release verbs, false effects, stale flags, and weak validator all reproduce. The future root-gate/redteam witness is compatible with deleting the duplicate. The row's core analysis passes, but its stale label and cross-report execution attribution do not. | **REQUEST_CHANGES (B1, B2)** | High |
| `skill-builder` | Structural skill-package lifecycle; implementation target, `implement_method`, T2; contract-v3 compiler/readiness remain shadow and non-authoritative. | Typed effects, fail-closed probe confinement, mutation hash proof, and self-witnessing invariants are real strengths. Five ratchet failures, uncovered build-report write, heal/spec drift, red containment harness, fail-open `rg`, template output omission, and layout defect reproduce. Five improvements map to the repair owner. | **PASS** | High |
| `standards` | Advisory standards selection; evidence target, `plan_input + implement_method + validate_evidence`, T4; it neither edits nor judges continuation. | The 9 owner-only / 9 template-only / 7 shared path split reproduces. Three conflicting frontmatter surfaces and missing propagation checks justify the template P0. The semantic three-surface witness and five improvements are sound. | **PASS** | High |
| `toil-mining` | Explicit-history observation; evolution target, `goal_observe + post_verdict`, T6; caller supplies JSONL paths and the helper performs retrieval/reporting only. | Retrieval helper and three tests pass. False effects, three-way output mismatch, forbidden layout, invocability mismatch, and prose-only clustering/ranking reproduce. The floor, six-permutation, and missing-factor witnesses are discriminating; five improvements are present. | **PASS** | High |

Every section contains intent, RPI placement and exclusions, live behavior,
strengths, exact defects, a witness, ledger, checked/not-checked bounds,
residual risk, and exactly five numbered improvements.

## 5. Aggregate and manifest checks

The following reproduced exactly:

- 12 in-scope skills and **106** unique tracked files;
- package counts `2, 3, 17, 1, 1, 1, 3, 1, 11, 46, 16, 4 = 106`;
- every embedded path and SHA-256 against the corresponding `HEAD` blob;
- canonical path-order manifest SHA-256
  `ab994cd63b2027901f4447f93816e3cc40da9119fa9e2a6511d7d758cf10ab73`;
- **8 P0**, **21 unique P1**, and **6 P2** ledger entries;
- 25 P1 section occurrences: `P1-LAYOUT` four times,
  `P1-INIT-OC` twice, and the other 19 IDs once;
- effects buckets `4 + 5 + 1 + 1 + 1 = 12`, with raw frontmatter
  **9 empty / 3 nonempty**;
- 49 live skills, 43 explicit `output_contract`, and the exact six absent:
  `cass`, `cc-hooks`, `dcg`, `implement`, `ms`, `plan`;
- four forbidden audited `.agents/` destinations:
  `audits`, `doc`, `patterns`, `toil-mining`;
- Python ratchet: head scope exit 0; upstream scope exit 1 with 12 branch-wide
  failures, exactly five in audited scope, all in `skill-builder`; 24
  grandfathered paths, zero stale entries, zero snapshot growth.

The 106 manifest rows are byte-identical between v12 and v13. The v12 B3
repair—five recomputations, not three—is complete.

## 6. Fresh checks performed

Checked independently:

- all supplied and predecessor artifact identities and extents;
- full v12-to-v13 diff;
- full v13 text, all twelve `SKILL.md` files, all six package validators, and
  the cited high-risk source files;
- active overhaul plan, operating-loop contract, Codex projection contract,
  contract-v3 authority rail, and ADR-0016;
- exact manifest membership and every blob digest;
- aggregate ledgers, multiplicities, effects, outputs, dependencies, authority,
  destinations, and Python-file classes;
- `audit.sh` for all twelve: 10 PASS, `skill-builder` WARN, `toil-mining` WARN;
- craft validator exit 0;
- doc validator 14 passed / 0 failed;
- toil validator three tests PASS and contract PASS;
- security validator PASS in a scratch archive;
- pattern-mining Bats 2/2 PASS;
- migration readiness PASS: 49 rows, 1 ready, 48 explicit blockers;
- mutation harness exit 1 at its first assertion, with `fix_rc=2`;
- package-local security gate exit 1 on missing package toolchain;
- security redteam exit 3 with four FAIL and two PASS;
- focused goals Go tests PASS and `goals measure --json --help` exit 0 with
  `--json` under Global Flags;
- open/close repository identity and cleanliness.

Mutating or bytecode-prone probes were confined to a scratch archive with
Python cache output redirected outside the repository. No repository file or
Git state was changed.

Not checked:

- productive execution of any mutating skill;
- a live goals measurement or documentation/product generation;
- the binary-executing security suite and release mode;
- future witnesses that do not yet exist;
- semantic quality of the other 37 live skills beyond the corpus-wide
  declaration sweeps;
- external repositories or the historical unrecoverable intermediate v10
  bytes.

## 7. Required repair and disposition

Before PASS:

1. reconcile the security-validator history and the three/four/five provenance
   counts in §9.1, §10, §13, and the close;
2. replace the two stale `this v10` / `this v8` labels;
3. change the automation spec section count from 8 to 9;
4. remove the false `craft-goal` highest-score claim and use the six-line
   validator description already present in §8.0;
5. describe the doc validator exactly as 14 static checks with 14 grep
   invocations across 13 of them, not 14 checks that are all greps.

The underlying twelve-skill audit remains substantively credible, and the exact
manifest passes. The reviewed v13 bytes do not pass as a self-consistent,
fully attributed final report.

**Final disposition: REQUEST_CHANGES.**
