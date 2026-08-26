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

On current main, the coverage gate counts **0/11** product/judgment skills as
measured. The denominator is DECLARED: 12 skills carry a product/judgment tier
badge and one — `goals`, a pure `alias-of fitness` whose body delegates
verbatim — is excluded in `scripts/.skill-probe-denominator-exclusions`,
because a probe of it would measure `fitness` and report the verdict under the
wrong name. The legacy rows remain historical evidence and do not count until a
v3 capture-manifest-backed run records a current directional verdict. The
newly added `anti-ceremony` skill has no current result, and the
`operationalize` runs are meta-tier, so their historical v1/v2 results do not
change that 0/11 denominator.

Seven of the eleven historical probe GROUPS behind those rows are SATURATED
under `skill.probe-headroom` (`bash scripts/check-skill-probe-headroom.sh`):
the control arm already aced the scenario at two effort levels, so their INERT
classifications were void rows rather than honest nulls. A new row citing a
SATURATED group is not evidence — pre-screen the scorecards with
`cli/cmd/probe-headroom` before appending.
