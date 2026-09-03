# Behavioral probe ledger

> **This file is hand-maintained, never generated.** It used to live inside
> `skills/SKILL-TIERS.md`, which is regenerated from SKILL.md frontmatter — a
> regeneration wiped the ledger and the (non-blocking) coverage gate silently
> reported 0/11 measured for weeks. A measured result cannot be derived from
> frontmatter, so it does not belong in a generated file. Parsed by
> `scripts/check-skill-probe-coverage.sh` (heading + table format are load-bearing).
>
> **HONESTY.** A current probe verdict records response-shape
> BEHAVIOR-CHANGE, never quality uplift. `BEHAVIORAL` and `INERT` count toward
> coverage only after capture-time fixture hashes and producer configuration
> are bound in a manifest accepted by the fail-closed harness.
> `LEGACY-UNVERIFIED` preserves an earlier reported classification whose
> fixture bytes predate that manifest contract. Those bytes may still explain
> history, but replay cannot establish who produced them, under which config,
> or that generation is reproducible. `PRELUDE-ONLY` may have a valid bound
> capture, but its treatment was distilled text rather than the canonical
> `SKILL.md`, so it never counts as skill coverage. One row per (skill, probe,
> run) — append rather than erase history.
>
> **CURRENT-ROW EVIDENCE SYNTAX.** Every `BEHAVIORAL`, `REGRESSIVE`, or `INERT` row must carry
> exactly one ``scorecard: `docs/evals/scorecards/...json` `` pointer in Notes.
> That repo-relative pointer is machine-readable; the coverage gate derives the
> fixture manifest from the v3 scorecard and recomputes its bound classification.
> Other prose and links in Notes are explanatory only and never make a row count.

## Behavioral Probe Ledger (MEASUREMENT STATUS)

| Skill | Probe | Date | Verdict | Notes |
|---|---|---|---|---|
| `crank` | `crank` | 2026-07-08 | LEGACY-UNVERIFIED | Historical classification: INERT, with stored rates 1.0 vs 1.0; evidence: docs/evals/2026-07-08-skill-probe-crank.md |
| `graphify` | `graphify-tool-preference` | 2026-06-30 | LEGACY-UNVERIFIED | Historical classification: INERT, 0/2 treatment responses followed the instruction; fixtures were reconstructed rather than verbatim captures; evidence: docs/evals/2026-07-08-skill-probe-graphify-calibration.md |
| `premortem` | `premortem-self-validation` | 2026-08-04 | LEGACY-UNVERIFIED | Historical quiz classification: BEHAVIORAL; the response named the planted self-validation flaw at stored rates xhigh 1/2→2/2 and low 0/2→2/2; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `standards` | `standards-go-conventions` | 2026-08-04 | LEGACY-UNVERIFIED | Historical quiz classification: BEHAVIORAL; the response contained both Go shapes at stored rates xhigh 1/2→2/2 and low 0/2→2/2; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `validate` | `validate-not-proven` | 2026-08-04 | LEGACY-UNVERIFIED | Historical quiz classification: INERT; stored response rates were 2/2 in both arms under both config labels; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `security` | `security-coverage-gap` | 2026-08-04 | LEGACY-UNVERIFIED | Historical quiz classification: INERT; stored response rates were 2/2 in both arms under both config labels; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `reality-check` | `reality-check-gap` | 2026-08-04 | LEGACY-UNVERIFIED | Historical quiz classification: INERT; stored response rates were 2/2 in both arms under both config labels; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `crank` | `crank-luna` | 2026-08-04 | LEGACY-UNVERIFIED | Historical classification: INERT; stored response rates were 2/2 in both arms under both config labels; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `validate` | `validate-not-proven-v2` | 2026-08-05 | LEGACY-UNVERIFIED | Historical quiz classification: INERT; the hardened stored responses remained 2/2 in both arms under both config labels |
| `security` | `security-coverage-gap-v2` | 2026-08-05 | LEGACY-UNVERIFIED | Historical quiz classification: INERT; stored responses caught the buried scanner warning in both arms under both config labels |
| `reality-check` | `reality-check-gap-v2` | 2026-08-05 | LEGACY-UNVERIFIED | Historical report: INERT, initially 1/2 vs 1/2 and extended to 6/6 in both arms; the committed initial-batch bytes now rescore 2/2 vs 2/2, an unresolved legacy-evidence mismatch |
| `operationalize` | `anti-ceremony-creation-gate` | 2026-08-16 | PRELUDE-ONLY | Hash-bound injected-prelude run, historically classified INERT at 2/2 versus 2/2; honest replay scorecard: `docs/evals/scorecards/2026-08-16/anti-ceremony-low-v1-replay.json`; fixture manifest: `evals/skill-probes/anti-ceremony-creation-gate/fixtures-low-2026-08-16/fixture-set.json`. The immutable `anti-ceremony-low.json` scorecard is superseded because its loaded-skill wording outruns the v1 binding; interpretation: `docs/evals/2026-08-16-anti-ceremony-creation-gate.md` |
| `operationalize` | `anti-ceremony-creation-gate-v2` | 2026-08-16 | UNMEASURED | First canonical-skill attempt: control 2/2 usable, treatment 0/2 usable after a leading-hyphen CLI dispatch defect; no fixture set was published. Attempt scorecard: `docs/evals/scorecards/2026-08-16/anti-ceremony-low-v2.json`; interpretation: `docs/evals/2026-08-16-anti-ceremony-creation-gate.md` |
| `operationalize` | `anti-ceremony-creation-gate-v2` | 2026-08-16 | LEGACY-UNVERIFIED | Compatibility-only v2 canonical-SKILL run, historically classified INERT at control 2/2 versus treatment 2/2. Its retained scorecard and fixture predate the v3 response-only, counterbalanced, self-contained capture contract, so they do not count as current evidence: `docs/evals/scorecards/2026-08-16/anti-ceremony-low-v2b.json`; `evals/skill-probes/anti-ceremony-creation-gate-v2/fixtures-low-2026-08-16-v2b/fixture-set.json`; interpretation: `docs/evals/2026-08-16-anti-ceremony-creation-gate.md` |
| `premortem` | `premortem-plan-shape-t2` | 2026-08-26 | WITHDRAWN | Filed as BEHAVIORAL, withdrawn same day: the fresh validator dumped the committed transcripts and every rep — both arms — had fetched `skills/premortem/SKILL.md` off disk (control-1 `sed`, control-2 `cat`, exit 0), so the arms were not differentiated by the bound bytes and the separation was band-spray variance between two arms holding the same skill. The harness now degrades any rep whose transcript shows a successful SKILL.md read (`skill-read-contamination`); the regenerated scorecard reads UNMEASURED 0/0 usable. `premortem` remains UNMEASURED. Evidence: `docs/evals/scorecards/2026-08-26/premortem-plan-shape-t2-low.json` (regenerated), fixtures unchanged. |
| `premortem` | `premortem-plan-shape-t2` | 2026-09-03 | BEHAVIORAL | scorecard: `docs/evals/scorecards/2026-09-03/premortem-plan-shape-t2-low.json`; network-sealed dispatch (one removed-on-exit run directory reset per rep; reads denied under the real HOME, the real temp root and cache root, the checkout, the git common directory, the real CODEX_HOME, and every skill root; network denied except to a harness-owned CONNECT proxy with a pinned host allowlist; web search disabled; one generated config bound by text and digest; auth copied; capture-contract v3 seal block whose profile the verifier rebuilds and digest-checks); producer gpt-5.6-luna effort low; control 0/2, treatment 1/2 (treatment-2 PRESENT, both seeded defects named), no rep ran a command, no refused egress; headroom SEPARATED; N=2, directional; response-shape behavior change, never quality uplift. Fourth capture of this probe on 2026-09-03; the three superseded ones (each under a seal a judge then broke, deleted with their scorecards) read low BEHAVIORAL 1/1, low INERT 0/2, low INERT 0/1 usable. |
| `premortem` | `premortem-plan-shape-t2` | 2026-09-03 | INERT | scorecard: `docs/evals/scorecards/2026-09-03/premortem-plan-shape-t2-xhigh.json`; same seal and producer at effort xhigh; control 0/2, treatment 0/2, no rep ran a command, no refused egress; headroom SEPARATED; N=2, directional. Superseded xhigh captures read INERT 0/2, BEHAVIORAL 2/2, INERT 0/2. Across all four captures the treatment arm showed the marks in band in 4 of 15 usable reps and the control arm in 0 of 16; that aggregate spans different harness bytes and is disclosure, not a ledger row. |

As of the 2026-08-16 provenance migration the coverage gate counted **0/12**
product/judgment skills as measured (see the dated update below for the current
number). The
denominator is now DECLARED rather than inferred: 13 skills carry a
product/judgment tier badge and one — `goals`, a pure `alias-of fitness` whose
body delegates verbatim — is excluded in
`scripts/.skill-probe-denominator-exclusions`, because a probe of it would
measure `fitness` and report the verdict under the wrong name. The headline
reads 0/12 before and after that change for different reasons: the alias left
the denominator and the newly landed judgment skill `one-way-door` entered it,
honestly unmeasured. On 2026-09-03 ADR-0018 retired `goals` outright, so the
denominator is the 12 badge-carrying skills with no exclusion entry. The legacy rows remain historical evidence and do not
count until a v3 capture-manifest-backed run records a current directional
verdict. The `anti-ceremony` skill has no current result, and the
`operationalize` runs are meta-tier, so their historical v1/v2 results do not
change that denominator.

Seven of the eleven historical probe GROUPS behind those rows are SATURATED
under `skill.probe-headroom` (`bash scripts/check-skill-probe-headroom.sh`):
the control arm already aced the scenario at two effort levels, so their INERT
classifications were void rows rather than honest nulls. A new row citing a
SATURATED group is not evidence — pre-screen the scorecards with
`cli/cmd/probe-headroom` before appending.

**Update 2026-08-26 — the seeded-defect (tier-2) wave.** Four tier-2
seeded-defect probes were authored for `validate`, `premortem`, `council`, and
`one-way-door`, and 28 live canonical-skill dispatches were captured against
gpt-5.6-luna with an isolated producer home. **Zero rows survived** and the
coverage gate counted **12/12 unmeasured** at that date: the one row filed (`premortem`,
BEHAVIORAL) was WITHDRAWN the same day when the fresh validator proved every
rep — both arms — had read the skill off disk mid-run (see the WITHDRAWN row
above and the skill-read-contamination rule the harness now enforces). The
scenarios' outcomes live in [`RUNBOOK.md`](RUNBOOK.md):

- `one-way-door-batch-t2` and `validate-seeded-closeout-t2` are **SATURATED**
  (control ≥ 0.75 at `low` and `xhigh`) — retired, no row. The control arm
  produced the disciplined answer with no skill bytes at all, so tier 2 did not
  escape the ceiling for those two disciplines. That is a result about the
  producer's native behavior, not about the skills.
- `council-caller-challenge-t2` was measured at one effort level with a control
  rate of `1.00`. Since the single-level rule, `skill.probe-headroom` classifies
  the group **UNMEASURED** ("capture a second level before any verdict row") —
  **no row is appended over an aced control arm.**

The honest headline: zero of four judgment disciplines measured at this
producer altitude — two ceilings, one insufficient capture, one contaminated
withdrawal. The instrument caught all four states correctly; the next wave
runs only after dispatch is filesystem-sealed.

**2026-09-03, first pass (history).** Filesystem-sealed dispatch landed: an
outer `sandbox-exec` profile denying the checkout and every skill root, a
scratch HOME/CODEX_HOME, one empty workspace per rep, the prompt on stdin only,
and the seal bound into capture-contract v3 as a condition of coverage. Two
contamination paths were caught on the way and are still trapped: reading a
sibling rep's prompt from a shared workspace, and reading any `SKILL.md`
(including codex's bundled system skill). The rows that capture produced are
NOT in the table above; every one of them was superseded by a later recapture,
and the numbers below belong to the pass that produced them.

**2026-09-03, second pass.** Two fresh judges read that seal and found nine
defects in it, from a readable temp hierarchy holding 425 stale probe
directories to a scratch CODEX_HOME that symlinked the operator's real config
and so started the operator's MCP servers inside every rep. The seal was
rebuilt (one removed-on-exit run directory, the real HOME and temp root
read-denied, link and clone denied, a sanitized per-rep config) and the bound
seal block gained the keys that make those claims checkable. The rows in the
table were recaptured under it; the first-pass sets were deleted with their
scorecards rather than kept, since a seal nothing can check is evidence about
the instrument, not the skill.

**2026-09-03, third pass.** Two more judges read THAT seal and both failed it.
The headline gap was the network: the outer profile was `(allow default)` and
codex's own sandbox is bypassed inside it, so a rep could fetch the canonical
`SKILL.md` over HTTPS while the filesystem seal proved nothing about what it
read. A judge demonstrated it. The seal now denies `network*` except to a
harness-owned local CONNECT proxy with a pinned host allowlist, and the bound
block can rebuild its own profile text so its roots are the bytes the kernel
enforced rather than assertions beside them. Because the harness bytes moved
again, the rows above are the recapture under this seal (fixture sets
`fixtures-{low,xhigh}-2026-09-03-network-sealed`); the headline is **1/12
measured**, and any further harness change orphans the rows again until the
next recapture, which the gate reports per set. What
the verifier proves is narrow and worth stating plainly: the profile digest
reconstructs from the block, the required roots are denied, the wrap names the
system seatbelt, the network mode is the proxy allowlist, and the rep
environment and config are the recorded ones. It does not prove the absence of
every other ambient capability; the profile is still `(allow default)` outside
the denies it names, so process capabilities other than the network remain
open. Details: `RUNBOOK.md`, "Third pass".

The gate prints each set's effective eligibility on every run, for every row
that names a scorecard, so a reader never has to take a scorecard's own
`coverage_eligible` field for the answer.
