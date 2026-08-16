# Skill quality audit — 2026-08-16

## Outcome

All 52 canonical source skills were inventoried and graded. The baseline at
`8694b07ebf8db602c3dcf5ef79091090976d1deb` had 34 deep-audit `PASS`, 15
`WARN`, and 3 `FAIL`. This change repairs the three blocking output-contract
defects, leaving 35 `PASS`, 17 `WARN`, and 0 `FAIL` without changing any static
readiness score.

The more important result is epistemic: **0/52 skills currently qualify for an
overall quality `PASS` under the updated rubric.** Twenty-five have E1
behavioral artifacts or directional probes, 27 are E0, and none has the
current, exact-version baseline/control matrix required for E2. A full-bundle
static safety review also found 5 `FAIL`, 8 `WARN`, 10 `NOT_PROVEN`, and only 29
uneventful static screens. The old A/B score is a static package-readiness
heuristic; it is not evidence that a skill is safe or improves outcomes.

This is one consolidated control artifact. Its consumer is this remediation PR
and later skill release decisions; it should be superseded, not appended to,
when the rubric or canonical inventory changes. Per-skill narrative reports
were deliberately not created.

## Scope and reproduction

Included: every direct `skills/*/SKILL.md` whose frontmatter declares
`metadata.canonical_status: canonical`. Excluded: nested fixtures,
`skills-codex/**`, `images/**`, catalogs, routers, and registries because those
are test data or generated projections rather than additional source skills.

- Denominator: 52 source skills; inventory-name SHA-256
  `50823f4004c0074d87c647fccbfcaa9484dcb5ac17ef319f71b18f8fe89e1456`.
- Baseline: clean `origin/main` commit
  `8694b07ebf8db602c3dcf5ef79091090976d1deb`.
- Structural conformance: 52/52 passed `heal.sh --check --strict`, with zero
  Pass-1 findings; source/projection count was 52/52.
- Baseline deep audit: 34 PASS, 15 WARN, 3 FAIL.
- Remediated deep audit: 35 PASS, 17 WARN, 0 FAIL.
- Static readiness: 40 A, 12 B, 0 C, 0 S; mean 21.81, median 22, range 15–26.
- Bundle safety screen: 29 PASS, 8 WARN, 5 FAIL, 10 NOT_PROVEN. PASS here means
  only that complete static inspection found no defect; no mutating or networked
  workflow was executed.
- Behavioral evidence: 27 E0, 25 E1, 0 E2, 0 E3.

The inventory hash can be reproduced with:

```bash
find skills -mindepth 2 -maxdepth 2 -name SKILL.md -print \
  | LC_ALL=C sort | sed 's#skills/##; s#/SKILL.md##' | shasum -a 256
```

The corpus sweep used the owning audit command for each source package:

```bash
bash skills/skill-builder/scripts/heal.sh --check --strict
for skill in skills/*; do
  test -f "$skill/SKILL.md" || continue
  bash skills/skill-builder/scripts/audit.sh "$skill" --json "/tmp/$(basename "$skill").json"
done
```

## Grade legend

- **Deep audit:** repository structural and content-discipline checks. `WARN`
  remains advisory; `FAIL` is blocking.
- **Static:** deterministic 0–30 package-readiness score and C/B/A/S band. It
  evaluates neither the safety gate nor behavioral effectiveness.
- **Safety:** full-package static review of scripts, tools, effects, filesystem
  and network reach, secrets, approval, bounds, and cleanup. `PASS` is an
  uneventful static screen, not runtime proof. `NOT_PROVEN` means consequential
  controls live outside the package and were not attested here.
- **E0:** no skill-specific behavioral scenario or usable directional receipt
  was found.
- **E1:** a feature, self-test, targeted test, or historical directional probe
  exists, but no current exact-version baseline comparison establishes outcome
  delta. The hand-maintained probe ledger is counted only as E1 because its
  rows do not bind the current package digest or cover the full activation and
  output matrix.
- **E2/E3:** defined in
  [the current rubric](../reference/skill-quality-rubric.md); none was awarded.

The arrow in three rows shows baseline to remediated deep-audit status. Findings
are the remaining Pass-2 warnings after remediation.

| Skill | Deep audit | Static | Safety | Behavior | Remaining findings |
|---|---:|---:|---:|---:|---|
| `account-rotation` | PASS | 21/30 A | NOT_PROVEN | E0 | — |
| `agent-mail` | WARN | 22/30 A | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `agent-native` | PASS | 21/30 A | NOT_PROVEN | E0 | — |
| `agy-native` | PASS | 23/30 A | NOT_PROVEN | E0 | — |
| `anti-ceremony` | PASS | 20/30 B | PASS | E0 | — |
| `automation-shape-routing` | PASS | 19/30 B | PASS | E0 | — |
| `bootstrap` | PASS | 23/30 A | PASS | E0 | — |
| `cass` | WARN | 26/30 A | FAIL | E1 | references-modularization |
| `cc-hooks` | WARN | 19/30 B | WARN | E0 | references-modularization |
| `codebase-recon` | PASS | 24/30 A | WARN | E1 | — |
| `codex-exec` | WARN | 19/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `converter` | PASS | 23/30 A | FAIL | E0 | — |
| `council` | WARN | 22/30 A | PASS | E0 | constraints-frontloaded, quality-rubric |
| `craft-goal` | WARN | 22/30 A | PASS | E0 | references-modularization |
| `dcg` | PASS | 23/30 A | NOT_PROVEN | E1 | — |
| `doc` | PASS | 24/30 A | PASS | E1 | — |
| `domain` | PASS | 21/30 A | PASS | E0 | — |
| `fitness` | PASS | 23/30 A | WARN | E0 | — |
| `goals` | PASS | 19/30 B | PASS | E0 | — |
| `handoff` | PASS | 19/30 B | PASS | E0 | — |
| `idea-genie` | WARN | 25/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `implement` | FAIL→PASS | 22/30 A | PASS | E1 | — |
| `learn` | PASS | 18/30 B | PASS | E1 | — |
| `ms` | PASS | 22/30 A | WARN | E1 | — |
| `ntm` | WARN | 19/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `operationalize` | PASS | 22/30 A | PASS | E0 | — |
| `pattern-mining` | PASS | 24/30 A | PASS | E1 | — |
| `plan` | FAIL→WARN | 22/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `postmortem` | PASS | 23/30 A | PASS | E1 | — |
| `premortem` | WARN | 23/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `product` | PASS | 21/30 A | PASS | E0 | — |
| `rch` | PASS | 24/30 A | NOT_PROVEN | E0 | — |
| `reality-check` | PASS | 23/30 A | PASS | E1 | — |
| `refactor` | PASS | 23/30 A | PASS | E1 | — |
| `research` | WARN | 22/30 A | WARN | E1 | constraints-frontloaded, quality-rubric |
| `reverse-engineer` | PASS | 25/30 A | FAIL | E1 | — |
| `rpi` | WARN | 23/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `sbh` | PASS | 22/30 A | NOT_PROVEN | E0 | — |
| `scaffold` | PASS | 23/30 A | PASS | E1 | — |
| `scope` | WARN | 21/30 A | PASS | E0 | constraints-frontloaded |
| `security` | PASS | 25/30 A | FAIL | E1 | — |
| `shared` | PASS | 18/30 B | PASS | E0 | — |
| `skill-builder` | PASS | 25/30 A | WARN | E1 | — |
| `standards` | WARN | 24/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `status` | PASS | 19/30 B | PASS | E0 | — |
| `swarm` | PASS | 15/30 B | WARN | E1 | — |
| `test` | PASS | 23/30 A | PASS | E1 | — |
| `toil-mining` | PASS | 21/30 A | PASS | E1 | — |
| `using-flywheel` | FAIL→WARN | 21/30 A | FAIL | E0 | constraints-frontloaded |
| `using-gc` | WARN | 18/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric, references-modularization |
| `validate` | WARN | 23/30 A | WARN | E1 | constraints-frontloaded, quality-rubric |
| `workflow-builder` | PASS | 22/30 A | PASS | E0 | — |

## Findings that matter

1. **The previous score overclaimed.** Safety was one compensable 0–3 category
   and behavioral tests were optional. A polished bundle could receive an A or
   S without full safety review or evidence that it helped. The scorer now
   emits `scope: static-package-readiness`, `safety_gate_evaluated: false`, and
   `effectiveness_evaluated: false`; the schema and regression test bind that
   claim.
2. **Three source skills failed one real contract check.** `plan`, `implement`,
   and `using-flywheel` described their outputs in their bodies but omitted the
   canonical `output_contract` metadata. The source fields and generated
   projections are repaired, and a focused test prevents recurrence.
3. **Five packages have concrete safety failures despite strong static
   scores.** `cass` has unbounded search/index fallbacks, unsafe host-to-path
   fanout, and raw history/log exposure. `converter` accepts destructive output
   paths and follows links while copying. `reverse-engineer` has unbounded
   subprocess/fetch/archive paths, weak execution isolation, and secret-output
   exposure. `security` executes targets without effective filesystem/network
   containment and can persist secret-bearing output. `using-flywheel` declares
   `effects: []` despite provisioning, remote/runtime writes, and dispatch.
   The relevant owners are `skills/cass/scripts/{recover.sh,quick_analysis.sh,
   multi_machine_search.sh}`, `skills/converter/scripts/convert.sh`,
   `skills/reverse-engineer/scripts/{reverse_engineer.py,fetch_url.py}`,
   `skills/security/scripts/security_suite.py`, and
   `skills/using-flywheel/SKILL.md`.
4. **Eight packages need safety hardening and ten cannot be attested from the
   bundle alone.** The WARN set is `cc-hooks`, `codebase-recon`, `fitness`, `ms`,
   `research`, `skill-builder`, `swarm`, and `validate`. The NOT_PROVEN set
   delegates consequential controls to external CLIs or repo-global code:
   `account-rotation`, `agent-mail`, `agent-native`, `agy-native`, `codex-exec`,
   `dcg`, `ntm`, `rch`, `sbh`, and `using-gc`.
5. **Behavioral proof is the corpus-wide gap.** Twenty-five skills have only E1
   evidence and 27 have E0. The existing probe ledger includes useful
   directional experiments, but its current rows neither identify the exact
   package version nor cover direct, indirect, incomplete-input,
   should-not-trigger, edge, coexistence, and output behavior together.
6. **The remaining content warnings are not automatically defects.** Four
   kernels are over the local 250-line advisory threshold (`cass`, `cc-hooks`,
   `craft-goal`, `using-gc`), and the other warnings mainly ask for front-loaded
   constraints or a visible quality rubric. Bulk prose edits to satisfy those
   heuristics would be score-chasing, not demonstrated product improvement.
7. **The auditor has known coverage defects.** Its external-fixture Pass 1 can
   fail without binding the aggregate verdict; `heal.sh` documents checks it
   does not implement and rejects a documented direct `skills-codex` target;
   Pass-2 evidence strings are generic; optional scorer subprocesses fail open
   to `null`; and `test-mutation-boundaries.sh` has a pre-existing expected-exit
   mismatch. These limit what this sweep proves and belong to the audit-tool
   owner, not to 52 individual skill rewrites.

## Remediation plan

| Priority | Action | Concrete consumer and done condition |
|---|---|---|
| Landed here | Separate static readiness from safety and effectiveness in the rubric, scorer output, schema, and tests. | Reviewers cannot mistake an A/B package score for behavioral or safety proof. |
| Landed here | Add honest output contracts to `plan`, `implement`, and `using-flywheel`; regenerate owned projections. | The 52-skill non-strict deep audit has zero FAIL and the focused regression stays green. |
| P0 | Repair the five safety FAIL packages as bounded, separately reviewable behavior changes. Start by blocking unsafe output/fetch/exec paths and secret emission, then correct declared effects and add planted-negative tests. | Each package passes fresh static review plus isolated runtime tests for its exact dangerous paths; no score increase substitutes for that evidence. |
| P1 | Repair the audit engine's binding/target/evidence defects before treating it as a conformance oracle outside canonical `skills/*`. | Known-good external fixtures cannot PASS when Pass 1 fails; documented target modes work or are removed; evidence names the actual missing condition. |
| P1 | Build E2 evaluations only for the release-critical spine first: `plan`, `implement`, `validate`, `rpi`, `security`, and `skill-builder`. Use no-skill controls and the five activation/output cases in the rubric. | A release decision consumes exact-version receipts; a failing or inert skill is simplified, reshaped, or retired rather than granted points for files. |
| P1 | Resolve the eight safety WARN and ten NOT_PROVEN packages, prioritizing process execution, network/credentials, destructive storage, and external factories. | Every high-impact path has least-privilege bounds, approval points, time/resource limits, cleanup evidence, and an attested owner even when enforcement lives outside the bundle. |
| P2 | Address a remaining warning only when a real probe, operator incident, or maintenance defect shows harm. | The observed defect disappears; no package gains files solely to raise a heuristic score. |
| Park | Do not mass-add per-skill rubrics, self-tests, reference folders, assets, or delegation packets. | Reconsider only when a named behavior or consumer requires one. |

## Checked and not checked

Checked: canonical inventory completeness; source/projection count; strict
frontmatter and structural checks; all 52 non-strict deep audits before and
after remediation; deterministic score distribution; all 286 tracked files in
the 52 packages through a complete static safety screen, with high-risk
implementations read line by line; local behavioral artifact and probe-ledger
presence; schema/tool honesty fields; generated Codex and Gemini projection
drift; focused regression tests.

Not checked: a complete baseline/treatment behavioral matrix for any skill;
every intended model/host/catalog combination; live execution of mutating,
networked, credentialed, destructive, or external-factory workflows; runtime
controls owned by external tools for the ten safety-NOT_PROVEN packages; and
empirical false-positive/false-negative activation rates. These omissions are
why the corpus result is `NOT_PROVEN`, not a quality `PASS`.
