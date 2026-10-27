# Wave 3 loading experiment: implement, triage, security, review

Execution status (2026-10-26): [results and limitations](../../../docs/evals/2026-10-26-wave3-opus-5-5.md).
The registration below is preserved as written; its "not run" statements refer to registration time.

Pre-registered 2026-10-25 for epic age-pv2j (bead age-pv2j.39), before any subject was built
or any run started. Nothing here has been run. The bars below are fixed; a run lane may not
change them after its first counted run.

## Question

Implement and Triage have real content gains when forced to load (+0.58 and +0.53 in the
[2026-10-22 report](../../../docs/evals/2026-10-22-plugin-eval-opus-5-5.md)) but almost never
load on their own; Security loads on its own behavior request but on none of its seven blind
requests; Review loads on few of either. Rounds 3 to 5 found that description edits and listing
cuts did not move loading on their own, and three description trials missed their bars. For
each of the four skills this experiment asks one question: does a single description variant,
written in advance, raise natural loading to the bar below, and does a shorter listing do so
without any wording change?

## Subjects

All subjects are copies of the repository's `plugin/` tree at the commit the run starts from
(record the SHA when building). They live in the run directory outside Git, as in October.

| Subject | What changes |
|---|---|
| C | Nothing. The control, run in the same time window as every other arm. |
| D-implement, D-triage, D-security, D-review | C with that one skill's `description:` replaced by the variant below, in the copy only. One subject per skill, so a variant can neither help nor steal from another variant. |
| L | C minus `using-gc`, `agy-native` and `reverse-engineer`: the three unrelated skills round 4 removed for its size-matched control. No wording changes. |

The variants are not applied to `skills/`. A variant reaches the repository only through the
adoption rule below and a separate change.

## Descriptions

| Skill | Current description (as shipped) | Variant for D |
|---|---|---|
| implement | Change or repair code, config or services without weakening tests; report what ran and what did not. Use when: implementing a change or fixing a defect. | Make the code change a ticket or bug report asks for, then prove it with the project's own checks. Use when: told to fix, change or build something, such as "go ahead and fix it". |
| triage | Turn incoming reports, external PRs and alerts into evidence-based decisions and agent-ready briefs. Use when: intake arrives. For accepted work, use Plan. | Decide what happens to something that just arrived: take it, park it, close it as a duplicate, ask for more, or hand it to another owner. Use when: a new issue, alert or outside PR needs that call. |
| security | Review code for security problems; scan for vulnerabilities, secrets, dependency and prompt risks. Use when: asked whether code is safe to ship, even one small handler. | Find what an attacker could exploit or what leaks in code, config, workflows or dependencies, marking each finding proven or suspected. Use when: asked if anything is exploitable, leaking or "nasty" before a push or release. |
| review | Give advisory feedback on a plan, design or code change. Use when: asked for an opinion or a look-over, even informally. Not for acceptance; use Validate. | Give an honest opinion on a PR, diff, design or idea: located findings, one suggested change each. Use when: asked to look something over, what you would change, or "dumb idea or fine?". Not for sign-off; use Validate. |

The round-4 triage variant ("Weigh a new bug report, feature request, outside PR or alert on
evidence; brief accepted work...") loaded 3 of 6 against a 4 of 6 bar and is not reused. The
variants above quote request shapes from the blind set's style, not its text; none copies a
blind request.

## Requests

Each arm runs, per skill:

- **Natural, 6 runs:** the skill's behavior case, `evals/plugin-eval/behavior/<skill>/case.yaml`,
  as written (no slash prefix).
- **Blind, 7 runs:** the round-1 routing request three times and the round-2 blind requests
  twice each, the same set and weights as round 2 of the October report.

| Skill | Round-1 routing request (3 runs) | Blind requests, 2 runs each, from the October run directory |
|---|---|---|
| implement | `evals/plugin-eval/routing/implement/case.yaml` (FE-212 pagination fix) | `cases-r2/routing2/implement-r2` (slugify accents), `implement-r3` (API-388 rate limiter burst) |
| triage | `evals/plugin-eval/routing/triage/case.yaml` (#2291 CSV export drops emoji contacts) | `triage-r2` (Android 15 crash report), `triage-r3` (api-gateway 5xx alert owned by another team) |
| security | `evals/plugin-eval/routing/security/case.yaml` (pull_request_target preview workflow) | `security-r2` (live Stripe key and path traversal), `security-r3` (invoices service before a public release) |
| review | `evals/plugin-eval/routing/review/case.yaml` (search-as-you-type hook) | `review-r2` (feature flags in one S3 file), `review-r3` (thumbnail worker pool PR) |

The blind requests live in `~/.local/share/agentops/evals/2026-10-22-plugin-ab/cases-r2/routing2/`
and were written by an agent that had not seen the descriptions. Copy them into the run
directory unchanged; do not edit them.

Steal check, D arms only, reported and not part of the bar: one run each of two neighbours'
round-1 routing requests (implement: plan, refactor; triage: plan, navigate; security: review,
reverse-engineer; review: validate, research). A neighbour request that loads the variant's
skill instead of its own is reported as a steal and goes to the owner before any shipping change.

## Harness

`claude plugin eval` as installed (record the version per run), model `claude-opus-5-5`,
`--ablation none` (with-plugin arm only: this experiment measures loading), `--no-publish`,
`--trust-plugin`, `--threshold 0`, `--keep-temp`, sandboxed home per run, JSON written outside
the repository. Per request, the arms C, D-skill and L run concurrently in the same time window.
Loading is read from the deterministic `skill-loaded` grader (the Skill tool call), never from
the response text. Before any counted run, the parse check
(`claude plugin eval . --eval-dir <dir> --case 'zz-none-*' --no-publish`) must load every copied
case with no load failure.

## Bars, fixed in advance

**Adoption.** Adopt a skill's variant only if, in its D arm, all three hold:

1. the skill loads on at least **4 of 6** natural runs of its behavior case; and
2. the skill loads on at least **3 of 7** blind runs; and
3. **no loss on its behavior case:** the D arm's score on those 6 natural runs (criteria met,
   pooled over runs, graded by `evals/plugin-eval/grade.py`, `skill-loaded` excluded) is not
   lower than the C arm's score on its 6 natural runs by more than 0.15, the suite's noise bar.

An adopted variant still passes the repository's shipping checks before it merges
(`scripts/regen-all.sh --check`, `tests/skills/test-token-budgets.sh`,
`scripts/check-routing-probe-goldens.sh`), as round 4 required.

**A null is listing-bound.** A variant that misses any of the three conditions is not adopted,
and the result is reported as listing-bound: consistent with rounds 3 to 5, where the loading
limit sat at the level of the listing and every miss loaded no skill at all. It is not reported
as a wording problem awaiting a better variant, and no further description trial for that skill
starts from this result without a new hypothesis the owner accepts.

**Listing arm, read beside D.** Reported per skill, not an adoption rule: L meeting the same
two loading conditions (4 of 6 natural and 3 of 7 blind) while C does not is directional
evidence that listing length limits that skill. For security, L also removes
`reverse-engineer`, which ships a secret scan; if any C run on a security request loads
`reverse-engineer`, the security L reading is marked confounded rather than read as length.

**Which leg binds, from October (3.11, for reference only).** Implement: own 0/6, blind 0/7.
Triage: own 0/6, blind 3/7. Security: own 6/6, blind 0/7. Review: own 0/6, blind 2/7. The bar
is absolute; C's same-day numbers, not these, are the comparison.

Report for every arm: loads per request, the Fisher exact two-sided p for D against C and for
L against C on pooled natural plus blind loads (13 per arm), and the D and C behavior scores.

## Stop

- Evaluator spend reaches **$27** (envelope $30).
- A subject fails the parse check, or any request errors on every run of an arm.
- A run that errors is reported as an error; a replacement run is added and labelled as one,
  never written over the failed attempt.

## Spend estimate

At the October per-run cost ($0.14 with the plugin; every run here has the plugin):

| Item | Runs | Cost |
|---|---:|---:|
| 4 skills x 3 arms (C, D, L) x 13 runs (6 natural + 7 blind) | 156 | $21.84 |
| Steal check, 4 D arms x 2 neighbour requests x 1 run | 8 | $1.12 |
| Grading the natural runs with `grade.py`, 4 skills x 3 arms x 6 responses at about $0.02 | 72 responses | $1.44 |
| **Total** | **164 runs** | **about $24.40** |

The evaluator's own default judge calls on the behavior cases' criteria are not in the
October per-run figure; they are Haiku calls and small beside generation.

## Limits known in advance

- One model and one runtime; Codex, Gemini and Cursor loading are not measured.
- Six natural and seven blind runs per arm show only large loading changes.
- L changes entry count and listing characters together, as round 4's arm did; it cannot
  separate the two.
- The variants were written by the author of this plan, who has read the skills and the blind
  requests; a variant tuned to the blind set's style is a risk the steal check and the behavior
  case only partly cover.
