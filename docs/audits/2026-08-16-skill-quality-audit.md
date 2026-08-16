# Skill quality audit — 2026-08-16

## Outcome

All 52 canonical source skills were inventoried and graded. The baseline at
`8694b07ebf8db602c3dcf5ef79091090976d1deb` had 34 deep-audit `PASS`, 15
`WARN`, and 3 `FAIL`. This change repairs the three blocking output-contract
defects, leaving 35 `PASS`, 17 `WARN`, and 0 `FAIL` without changing any static
readiness score when the revised scorer is applied to both snapshots.

The more important result is epistemic: **0/52 skills currently qualify for an
overall quality `PASS` under the updated rubric.** None has the current,
exact-version baseline/control matrix required for E2. Strict portable Agent
Skills frontmatter is 0/52 PASS: all 52 sources have a valid portable identity
core but use AgentOps host extensions. A full-bundle static safety review found
5 `FAIL`, 9 `WARN`, 10 `NOT_PROVEN`, and only 28 uneventful static screens. The
static readiness score is a package-shape heuristic; it is not evidence that a
skill is portable, safe, or improves outcomes.

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
- Package-file denominator: 284 Git-tracked files under those 52 direct source
  packages. The two nested fixture `SKILL.md` files are excluded.
- Baseline: clean `origin/main` commit
  `8694b07ebf8db602c3dcf5ef79091090976d1deb`.
- AgentOps repository profile: 52/52 passed the local v2 frontmatter validator
  and `heal.sh --check --strict`, with zero Pass-1 findings; source/projection
  count was 52/52.
- Strict portable Agent Skills frontmatter: 0 PASS, 52 `FAIL-HX`
  (`HOST_EXTENDED`). All 52 pass the portable name/directory/description core;
  all 52 also have seven universal non-spec top-level fields and list-valued
  AgentOps metadata where the portable extension map requires string values.
- Baseline deep audit: 34 PASS, 15 WARN, 3 FAIL.
- Remediated deep audit: 35 PASS, 17 WARN, 0 FAIL.
- Static readiness: 16 A, 36 B, 0 C, 0 S; mean 18.65, median 19, range 12–24
  on the 10-category 0–30 static package-readiness score.
- Bundle safety screen: 28 PASS, 9 WARN, 5 FAIL, 10 NOT_PROVEN. PASS here means
  only that complete static inspection found no defect; no mutating or networked
  workflow was executed.
- Behavioral evidence: 17 E0, 35 E1, 0 E2, 0 E3.

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

Portable classification used the current specification's six allowed
frontmatter keys and string-to-string metadata extension point. The pinned
official `skills-ref` implementation also rejects all 52; its first diagnostic
is flow-style YAML compatibility, so the table uses the independently checked
normative extension defects instead of conflating valid YAML style with the
specification:

```python
ALLOWED = {"name", "description", "license", "compatibility", "metadata", "allowed-tools"}
extras = set(frontmatter) - ALLOWED
bad_metadata = {key for key, value in frontmatter.get("metadata", {}).items()
                if not isinstance(key, str) or not isinstance(value, str)}
```

## Grade legend

- **Deep audit:** repository structural and content-discipline checks. `WARN`
  remains advisory; `FAIL` is blocking.
- **Portable:** strict published Agent Skills frontmatter. `FAIL-HX` means the
  portable identity core passes but host extension fields make the source
  non-portable as written. The AgentOps repository profile is graded separately.
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
are the remaining Pass-2 warnings after remediation. `FAIL-HX` has the same
universal cause in every row; per-skill extension variants are summarized below.

| Skill | Deep audit | Portable | Static | Safety | Behavior | Remaining findings |
|---|---:|---:|---:|---:|---:|---|
| `account-rotation` | PASS | FAIL-HX | 16/30 B | NOT_PROVEN | E0 | — |
| `agent-mail` | WARN | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | constraints-frontloaded, quality-rubric |
| `agent-native` | PASS | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | — |
| `agy-native` | PASS | FAIL-HX | 18/30 B | NOT_PROVEN | E0 | — |
| `anti-ceremony` | PASS | FAIL-HX | 17/30 B | PASS | E1 | — |
| `automation-shape-routing` | PASS | FAIL-HX | 16/30 B | PASS | E0 | — |
| `bootstrap` | PASS | FAIL-HX | 18/30 B | PASS | E0 | — |
| `cass` | WARN | FAIL-HX | 24/30 A | FAIL | E1 | references-modularization |
| `cc-hooks` | WARN | FAIL-HX | 17/30 B | WARN | E1 | references-modularization |
| `codebase-recon` | PASS | FAIL-HX | 22/30 A | WARN | E1 | — |
| `codex-exec` | WARN | FAIL-HX | 16/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `converter` | PASS | FAIL-HX | 20/30 B | FAIL | E1 | — |
| `council` | WARN | FAIL-HX | 19/30 B | PASS | E1 | constraints-frontloaded, quality-rubric |
| `craft-goal` | WARN | FAIL-HX | 19/30 B | PASS | E0 | references-modularization |
| `dcg` | PASS | FAIL-HX | 21/30 A | NOT_PROVEN | E1 | — |
| `doc` | PASS | FAIL-HX | 22/30 A | PASS | E1 | — |
| `domain` | PASS | FAIL-HX | 18/30 B | PASS | E0 | — |
| `fitness` | PASS | FAIL-HX | 18/30 B | WARN | E0 | — |
| `goals` | PASS | FAIL-HX | 13/30 B | PASS | E1 | — |
| `handoff` | PASS | FAIL-HX | 14/30 B | PASS | E1 | — |
| `idea-genie` | WARN | FAIL-HX | 23/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `implement` | FAIL→PASS | FAIL-HX | 20/30 B | PASS | E1 | — |
| `learn` | PASS | FAIL-HX | 16/30 B | PASS | E1 | — |
| `ms` | PASS | FAIL-HX | 19/30 B | WARN | E1 | — |
| `ntm` | WARN | FAIL-HX | 16/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `operationalize` | PASS | FAIL-HX | 17/30 B | PASS | E1 | — |
| `pattern-mining` | PASS | FAIL-HX | 22/30 A | PASS | E1 | — |
| `plan` | FAIL→WARN | FAIL-HX | 20/30 B | PASS | E1 | constraints-frontloaded, quality-rubric |
| `postmortem` | PASS | FAIL-HX | 21/30 A | PASS | E1 | — |
| `premortem` | WARN | FAIL-HX | 21/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `product` | PASS | FAIL-HX | 16/30 B | PASS | E0 | — |
| `rch` | PASS | FAIL-HX | 19/30 B | NOT_PROVEN | E0 | — |
| `reality-check` | PASS | FAIL-HX | 20/30 B | PASS | E1 | — |
| `refactor` | PASS | FAIL-HX | 19/30 B | PASS | E1 | — |
| `research` | WARN | FAIL-HX | 20/30 B | WARN | E1 | constraints-frontloaded, quality-rubric |
| `reverse-engineer` | PASS | FAIL-HX | 23/30 A | FAIL | E1 | — |
| `rpi` | WARN | FAIL-HX | 21/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `sbh` | PASS | FAIL-HX | 17/30 B | NOT_PROVEN | E0 | — |
| `scaffold` | PASS | FAIL-HX | 21/30 A | PASS | E1 | — |
| `scope` | WARN | FAIL-HX | 18/30 B | PASS | E0 | constraints-frontloaded |
| `security` | PASS | FAIL-HX | 23/30 A | FAIL | E1 | — |
| `shared` | PASS | FAIL-HX | 12/30 B | PASS | E0 | — |
| `skill-builder` | PASS | FAIL-HX | 23/30 A | WARN | E1 | — |
| `standards` | WARN | FAIL-HX | 21/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `status` | PASS | FAIL-HX | 14/30 B | PASS | E1 | — |
| `swarm` | PASS | FAIL-HX | 13/30 B | WARN | E1 | — |
| `test` | PASS | FAIL-HX | 21/30 A | PASS | E1 | — |
| `toil-mining` | PASS | FAIL-HX | 18/30 B | WARN | E1 | — |
| `using-flywheel` | FAIL→WARN | FAIL-HX | 18/30 B | FAIL | E0 | constraints-frontloaded |
| `using-gc` | WARN | FAIL-HX | 15/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric, references-modularization |
| `validate` | WARN | FAIL-HX | 21/30 A | WARN | E1 | constraints-frontloaded, quality-rubric |
| `workflow-builder` | PASS | FAIL-HX | 16/30 B | PASS | E0 | — |

### Safety evidence for every non-PASS grade

The rubric's severity thresholds were applied to every tracked file. The 28
remaining `PASS` rows are negative static screens: no concrete gap was found,
but runtime safety was not exercised. Non-PASS decisions retain their evidence
here so the classification can be replayed without inferring it from a label.

| Skill | Grade | Evidence and decision basis |
|---|---:|---|
| `cass` | FAIL | `scripts/{recover.sh,quick_analysis.sh,multi_machine_search.sh}` contain uncapped search/index fallbacks, persistent raw-history logs, and host-derived path/SSH fanout without complete bounds or cleanup. |
| `converter` | FAIL | `scripts/convert.sh` can remove a caller-selected existing output and uses link-following copy behavior without a separate destructive confirmation or source-tree containment. |
| `reverse-engineer` | FAIL | `scripts/{reverse_engineer.py,fetch_url.py,binary/*,security/scan_secrets.sh}` expose unbounded process, fetch, archive, weak-isolation, and matched-secret-output paths. |
| `security` | FAIL | `scripts/security_suite.py` executes targets without effective filesystem/network containment and can retain secret-bearing process/stdout/stderr evidence; descendants/effects are incomplete. |
| `using-flywheel` | FAIL | `SKILL.md` declares `effects: []` while directing `curl` provisioning, installs, remote/runtime writes, and factory/swarm dispatch. |
| `cc-hooks` | WARN | Bundled hook dispatch is fail-open, while telemetry, sentinels, and backups persist without a complete retention contract. |
| `codebase-recon` | WARN | `scripts/validate-output.sh` and the executable-probe workflow lack an enforced sandbox and deadline for target execution. |
| `fitness` | WARN | The declared read-only output conflicts with persisted and overwritable goal snapshots/renders in `SKILL.md`. |
| `ms` | WARN | Reindex/cleanup safety depends on an external helper not contained or attested by this package. |
| `research` | WARN | Browser/network behavior is absent from declared effects and lacks package-enforced bounds. |
| `skill-builder` | WARN | Multi-surface generation in `scripts/{build.sh,init.sh,heal.sh}` is nontransactional and unbounded; partial writes can survive failure. |
| `swarm` | WARN | `scripts/dispatch_once.py` accepts an arbitrary executor callback without timeout, cancellation, cleanup, or effect-authorization enforcement. |
| `toil-mining` | WARN | `scripts/recent_human.py` emits raw human-session text and absolute source paths and supports persistence without secret/PII/path redaction or sensitive-output approval. This is local disclosure risk, not a direct external or destructive path. |
| `validate` | WARN | Declared effects omit intent-snapshot and subject-manifest writes performed by the documented workflow. |
| `account-rotation` | NOT_PROVEN | Rotation and credential protections reside in the external account/runtime implementation; this package contains no enforceable runtime control. |
| `agent-mail` | NOT_PROVEN | Consequential message, hook-install, and destructive-message behavior is owned by the external `am` runtime rather than bundled enforcement. |
| `agent-native` | NOT_PROVEN | The bundled fake runner is test support; real model dispatch/session containment belongs to selected external runtimes. |
| `agy-native` | NOT_PROVEN | Session creation and cleanup are delegated to the external `agy` runtime. |
| `codex-exec` | NOT_PROVEN | The package specifies bounds, but actual process-group containment and cleanup live in the external Codex/wrapper implementation. |
| `dcg` | NOT_PROVEN | Configuration writes and enforcement are performed by the external `dcg` CLI. |
| `ntm` | NOT_PROVEN | Pane lifecycle, command dispatch, cancellation, and cleanup are owned by the external NTM runtime. |
| `rch` | NOT_PROVEN | Remote compilation, daemon, worker, and transport controls live in the external RCH implementation. |
| `sbh` | NOT_PROVEN | Destructive storage reclamation and host mutation are implemented by the external `sbh` command, not this one-file package. |
| `using-gc` | NOT_PROVEN | Gas City provisioning, dispatch, and runtime controls live outside the package and were not executed or attested. |

### Behavioral evidence map

E1 discovery covered package-local scenarios/tests and repository-global
artifacts that explicitly bind a skill slug and exercise its workflow or
implementation. Static prose assertions did not count. `S` is a scenario or
self-test definition, `T` an executable targeted test, and `R` a stored
directional receipt. These artifacts establish only E1; none binds the current
package digest to the complete baseline/control matrix required for E2.

| E1 skill | Kind | Qualifying evidence |
|---|---:|---|
| `agent-mail` | R | `evals/routing-probes/{templates.json,results/2026-08-05-batch-2.md}`; joint routing evidence explicitly lists the skill but selected `swarm`, so it is confounded. |
| `agent-native` | T | `tests/integration/test_multi_model_dispatch.bats` executes the bundled fake runner and checks identity, context separation, and degradation. |
| `anti-ceremony` | S/T | `skills/rpi/references/rpi.feature`, `skills/rpi/tests/test_run_once.py`, and `scripts/check-cathedral-cut-conformance.py` exercise guard order and STOP. |
| `cass` | S | `skills/cass/SELF-TEST.md`. |
| `cc-hooks` | T | `tests/scripts/{policy-dispatch,installed-skill-edit-guard,installed-skill-edit-telemetry,cross-runtime-hook-baseline}.bats` execute bundled hooks. |
| `codebase-recon` | S/T | `skills/codebase-recon/references/codebase-recon.feature`; `tests/scripts/agentops-native-skills.bats`. |
| `converter` | T | `skills/converter/scripts/validate.sh`, `tests/skills/test-runtime-cursor-smoke.sh`, and `evals/agentops-core/converter-update-runtime-parity.json`. |
| `council` | S/T | `tests/explicit-skill-requests/prompts/council.txt` and `tests/integration/test_multi_model_dispatch.bats`. |
| `dcg` | S/R | `skills/dcg/SELF-TEST.md`; `docs/audits/2026-07-28-skill-overhaul-reboot/wave-reports/w7.md`. |
| `doc` | S/T | `skills/doc/references/{doc,oss-docs,readme}.feature`; `evals/agentops-core/docs-release-governance.json`. |
| `goals` | S | `tests/explicit-skill-requests/prompts/goals.txt` plus its archived assertion harness. The harness now skips, so this is scenario-only evidence. |
| `handoff` | T/R | `cli/cmd/ao/handoff_test.go`; the repaired dry-run/schema receipt in `docs/audits/2026-07-28-skill-overhaul-reboot/wave-reports/w7.md`. |
| `idea-genie` | S/T | `skills/idea-genie/references/{idea-challenge,idea-genie}.feature`; `tests/scripts/agentops-native-skills.bats`. |
| `implement` | S | `skills/implement/references/implement.feature`. |
| `learn` | S | `skills/learn/references/learn.feature`. |
| `ms` | T | `skills/ms/tests/mcp-search.bats`; `tests/python/test_ms_adoption_probe.py`. |
| `operationalize` | S | `evals/skill-probes/anti-ceremony-creation-gate/probe.json` explicitly declares this skill; no run receipt is committed. |
| `pattern-mining` | S/T | `skills/pattern-mining/references/pattern-mining.feature`; `tests/scripts/agentops-native-skills.bats`. |
| `plan` | S | `skills/plan/references/plan.feature`. |
| `postmortem` | S | `skills/postmortem/references/postmortem.feature`. |
| `premortem` | S/R | `skills/premortem/references/premortem.feature`; `evals/skill-probes/premortem-self-validation/`; `docs/evals/2026-08-04-probe-wave-1.md`. |
| `reality-check` | R | `evals/skill-probes/reality-check-gap{,-v2}/`; `docs/evals/2026-08-04-probe-wave-1.md`. |
| `refactor` | S | `skills/refactor/references/refactor.feature`. |
| `research` | S | `skills/research/references/research.feature`. |
| `reverse-engineer` | S/T | `skills/reverse-engineer/references/reverse-engineer.feature`; `scripts/{self_test.sh,repo_fixture_test.sh,validate.sh}`. |
| `rpi` | S/T | `skills/rpi/references/rpi.feature`; `skills/rpi/tests/test_run_once.py`; `evals/agentops-core/rpi-behavior.json`. |
| `scaffold` | S | `skills/scaffold/references/scaffold.feature`. |
| `security` | S/T/R | `skills/security/references/{security,security-suite}.feature`; `skills/security/scripts/validate.sh`; `tests/scripts/test-security-suite-redteam.sh`; `evals/agentops-core/security-suite-behavioral-gates.json`. |
| `skill-builder` | S/T | `skills/skill-builder/references/{heal,skill-auditor,skill-builder}.feature`; its three mutation scripts. |
| `standards` | R | `evals/skill-probes/standards-go-conventions/`; `docs/evals/2026-08-04-probe-wave-1.md`. |
| `status` | T | `cli/internal/statusapp/statusapp_test.go`, `cli/cmd/ao/flag_matrix_test.go`, and behavioral cases in `evals/agentops-core/status-quickstart-dashboard.json`. |
| `swarm` | T/R | `skills/swarm/tests/test_dispatch_once.py`; `evals/routing-probes/results/2026-08-05-batch-2.md`. |
| `test` | S | `skills/test/references/test.feature`. |
| `toil-mining` | T | `skills/toil-mining/tests/test_recent_human.py`. |
| `validate` | S/T/R | `skills/validate/references/validate.feature`; `skills/validate/scripts/test_validate.py`; `evals/skill-probes/validate-not-proven{,-v2}/`. |

The E0 set is `account-rotation`, `agy-native`,
`automation-shape-routing`, `bootstrap`, `codex-exec`, `craft-goal`, `domain`,
`fitness`, `ntm`, `product`, `rch`, `sbh`, `scope`, `shared`,
`using-flywheel`, `using-gc`, and `workflow-builder`. Their nearest artifacts
were prose, structural grep/validators, deferred witnesses, or tests of an
external implementation that did not bind the current skill. In particular,
the repo-global `codex-exec` wrapper tests contradict the package's mandatory
process-group-reaping behavior, and the `fitness` candidates exercise the
separately declared `ao goals` surface; neither received E1 credit.

## Findings that matter

1. **The canonical sources are host-extended, not portable as written.** All 52
   pass the portable identity/description core and the AgentOps v2 profile, but
   all 52 fail strict portable frontmatter. Seven universal top-level AgentOps
   fields sit outside the six published fields, and `capabilities`, `effects`,
   and `dependencies` are lists where portable `metadata` is string-to-string.
   Additional extensions are `user-invocable` (35), `context` (24), `model`
   (1), rich `graph_root` (11), `internal` (2), and `external_dependencies` (2).
2. **The previous score overclaimed.** Safety was one compensable 0–3 category
   and behavioral tests were optional. It also awarded solid score 2 whenever
   helper, asset, or subagent directories were absent, without establishing
   that absence was justified. The scorer now labels its limited scope and
   assigns absent optional surfaces uncertainty score 1; focused tests bind
   both facts while preserving the existing 10-category/30-point wire shape.
   Presence branches remain mechanical and ceremony-gameable, so these values
   stay advisory and cannot satisfy any conformance, safety, or effectiveness
   gate.
3. **Three source skills failed one real contract check.** `plan`, `implement`,
   and `using-flywheel` described their outputs in their bodies but omitted the
   canonical `output_contract` metadata. The source fields and generated
   projections are repaired, and a focused test prevents recurrence.
4. **Five packages have concrete safety failures despite strong static
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
5. **Nine packages need safety hardening and ten cannot be attested from the
   bundle alone.** The WARN set is `cc-hooks`, `codebase-recon`, `fitness`, `ms`,
   `research`, `skill-builder`, `swarm`, `toil-mining`, and `validate`.
   `toil-mining` was moved out of PASS because it exposes raw human-session
   text and absolute paths without redaction or sensitive-output approval. The
   NOT_PROVEN set
   delegates consequential controls to external CLIs or repo-global code:
   `account-rotation`, `agent-mail`, `agent-native`, `agy-native`, `codex-exec`,
   `dcg`, `ntm`, `rch`, `sbh`, and `using-gc`.
6. **Behavioral proof is the corpus-wide gap.** Thirty-five skills have only E1
   evidence and 17 have E0. The prior count missed 10 explicitly skill-bound
   scenarios, tests, or receipts. The existing probe ledger includes useful
   directional experiments, but its current rows neither identify the exact
   package version nor cover direct, indirect, incomplete-input,
   should-not-trigger, edge, coexistence, and output behavior together.
7. **The remaining content warnings are not automatically defects.** Four
   kernels are over the local 250-line advisory threshold (`cass`, `cc-hooks`,
   `craft-goal`, `using-gc`), and the other warnings mainly ask for front-loaded
   constraints or a visible quality rubric. Bulk prose edits to satisfy those
   heuristics would be score-chasing, not demonstrated product improvement.
8. **The auditor has known coverage defects.** Its external-fixture Pass 1 can
   fail without binding the aggregate verdict; `heal.sh` documents checks it
   does not implement and rejects a documented direct `skills-codex` target;
   Pass-2 evidence strings are generic; optional scorer subprocesses fail open
   to `null`; and `test-mutation-boundaries.sh` has a pre-existing expected-exit
   mismatch. These limit what this sweep proves and belong to the audit-tool
   owner, not to 52 individual skill rewrites.

## Remediation plan

| Priority | Action | Concrete consumer and done condition |
|---|---|---|
| Landed here | Separate portable conformance, AgentOps profile checks, static readiness, safety, and effectiveness; remove automatic solid credit for absent optional components. | Reviewers cannot mistake a local/profile or A/B result for portable, behavioral, or safety proof. |
| Landed here | Add honest output contracts to `plan`, `implement`, and `using-flywheel`; regenerate owned projections. | The 52-skill non-strict deep audit has zero FAIL and the focused regression stays green. |
| P0 | Repair the five safety FAIL packages as bounded, separately reviewable behavior changes. Start by blocking unsafe output/fetch/exec paths and secret emission, then correct declared effects and add planted-negative tests. | Each package passes fresh static review plus isolated runtime tests for its exact dangerous paths; no score increase substitutes for that evidence. |
| P1 | Choose and implement one portable boundary: migrate canonical frontmatter into the six-field contract with string metadata, or generate and validate a spec-conformant portable projection while naming canonical sources as AgentOps-only. | A pinned portable validator passes every artifact advertised as portable; host extensions remain explicit and losslessly owned elsewhere. |
| P1 | Repair the audit engine's binding/target/evidence defects before treating it as a conformance oracle outside canonical `skills/*`. | Known-good external fixtures cannot PASS when Pass 1 fails; documented target modes work or are removed; evidence names the actual missing condition. |
| P1 | Build E2 evaluations only for the release-critical spine first: `plan`, `implement`, `validate`, `rpi`, `security`, and `skill-builder`. Use no-skill controls and the five activation/output cases in the rubric. | A release decision consumes exact-version receipts; a failing or inert skill is simplified, reshaped, or retired rather than granted points for files. |
| P1 | Resolve the nine safety WARN and ten NOT_PROVEN packages, prioritizing process execution, sensitive local content, network/credentials, destructive storage, and external factories. | Every high-impact path has least-privilege bounds, approval points, time/resource limits, cleanup evidence, and an attested owner even when enforcement lives outside the bundle. |
| P2 | Address a remaining warning only when a real probe, operator incident, or maintenance defect shows harm. | The observed defect disappears; no package gains files solely to raise a heuristic score. |
| Park | Do not mass-add per-skill rubrics, self-tests, reference folders, assets, or delegation packets. | Reconsider only when a named behavior or consumer requires one. |

## Checked and not checked

Checked: canonical inventory completeness and 284-file denominator;
source/projection count; strict AgentOps v2 frontmatter and repository
structural checks; pinned portable-spec and `skills-ref` compatibility
classification; all 52 non-strict deep audits before and after remediation;
deterministic score distribution; all tracked files in the 52 packages through
a complete static safety screen, with high-risk implementations read line by
line; package-local and explicitly skill-bound repository-global behavioral
evidence; schema/tool honesty fields; generated Codex and Gemini projection
drift; focused regression tests.

Not checked: a complete baseline/treatment behavioral matrix for any skill;
every intended model/host/catalog combination; live execution of mutating,
networked, credentialed, destructive, or external-factory workflows; runtime
controls owned by external tools for the ten safety-NOT_PROVEN packages; and
empirical false-positive/false-negative activation rates. These omissions are
why the corpus result is `NOT_PROVEN`, not a quality `PASS`.
