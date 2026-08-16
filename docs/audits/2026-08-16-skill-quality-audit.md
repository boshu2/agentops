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
core but use AgentOps host extensions, and two also violate `allowed-tools`
format. A full-bundle static safety review found 22 `FAIL`, 6 `WARN`, 11
`NOT_PROVEN`, and only 13 uneventful static screens. The static readiness score
is a package-shape heuristic; it is not evidence that a skill is portable,
safe, or improves outcomes.

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
- Strict portable Agent Skills frontmatter: 0 PASS, 50 `FAIL-HX`
  (`HOST_EXTENDED`), and 2 `FAIL-HX+AT`. All 52 pass the portable
  name/directory/description core; all 52 also have seven universal non-spec
  top-level fields and list-valued AgentOps metadata where the portable
  extension map requires string values. `research` and `status` additionally
  use comma-delimited `allowed-tools` where the specification requires a
  space-separated string.
- Baseline deep audit: 34 PASS, 15 WARN, 3 FAIL.
- Remediated deep audit: 35 PASS, 17 WARN, 0 FAIL.
- Static readiness: 16 A, 36 B, 0 C, 0 S; mean 18.65, median 19, range 12–24
  on the 10-category 0–30 static package-readiness score.
- Bundle safety screen: 13 PASS, 6 WARN, 22 FAIL, 11 NOT_PROVEN. PASS here means
  only that complete static inspection found no defect; no audited skill's
  mutating or networked workflow was executed.
- Behavioral evidence: 14 E0, 38 E1, 0 E2, 0 E3.

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
frontmatter keys and string-to-string metadata extension point. The official
`skills-ref` implementation at `https://github.com/agentskills/agentskills`,
commit `69ef37e9424c0a7ea9dd2293b559e43ec8176379`, also rejects all 52; its first
diagnostic is flow-style YAML compatibility, so the table uses the independently
checked normative extension defects instead of conflating valid YAML style with
the specification. The compatibility replay used that detached commit and,
from its `skills-ref/` directory, `uv run skills-ref validate
<absolute-skill-dir>` for each canonical package:

```python
ALLOWED = {"name", "description", "license", "compatibility", "metadata", "allowed-tools"}
extras = set(frontmatter) - ALLOWED
bad_metadata = {key for key, value in frontmatter.get("metadata", {}).items()
                if not isinstance(key, str) or not isinstance(value, str)}
allowed_tools = frontmatter.get("allowed-tools")
bad_allowed_tools = (allowed_tools is not None and
                     (not isinstance(allowed_tools, str) or "," in allowed_tools))
```

```bash
REPO_ROOT="$(pwd -P)"
SKILLS_REF_TMP="$(mktemp -d)"
SKILLS_REF_CHECKOUT="$SKILLS_REF_TMP/agentskills"
git clone https://github.com/agentskills/agentskills.git "$SKILLS_REF_CHECKOUT"
git -C "$SKILLS_REF_CHECKOUT" checkout --detach \
  69ef37e9424c0a7ea9dd2293b559e43ec8176379
(cd "$SKILLS_REF_CHECKOUT/skills-ref" && uv sync)
for skill in "$REPO_ROOT"/skills/*; do
  test -f "$skill/SKILL.md" || continue
  (cd "$SKILLS_REF_CHECKOUT/skills-ref" && uv run skills-ref validate "$skill")
done
```

## Grade legend

- **Deep audit:** repository structural and content-discipline checks. `WARN`
  remains advisory; `FAIL` is blocking.
- **Portable:** strict published Agent Skills frontmatter. `FAIL-HX` means the
  portable identity core passes but host extension fields make the source
  non-portable as written; `+AT` adds an `allowed-tools` format violation. The
  AgentOps repository profile is graded separately.
- **Static:** deterministic 0–30 package-readiness score and C/B/A/S band. It
  evaluates neither the safety gate nor behavioral effectiveness.
- **Safety:** full-package static review of scripts, tools, effects, filesystem
  and network reach, secrets, approval, bounds, and cleanup. `PASS` is an
  uneventful static screen, not runtime proof. `NOT_PROVEN` means consequential
  controls live outside the package and were not attested here.
- **E0:** no skill-specific behavioral scenario or usable directional receipt
  was found.
- **E1:** a behavior-shaped scenario, self-test, targeted test, or historical
  directional probe exists, but no current exact-version baseline comparison
  establishes outcome delta. The hand-maintained probe ledger is counted only as E1 because its
  rows do not bind the current package digest or cover the full activation and
  output matrix.
- **E2/E3:** defined in
  [the current rubric](../reference/skill-quality-rubric.md); none was awarded.

The arrow in three rows shows baseline to remediated deep-audit status. Findings
are the remaining Pass-2 warnings after remediation. `FAIL-HX` has the same
universal cause in every row; the two `+AT` variants are called out directly.

| Skill | Deep audit | Portable | Static | Safety | Behavior | Remaining findings |
|---|---:|---:|---:|---:|---:|---|
| `account-rotation` | PASS | FAIL-HX | 16/30 B | NOT_PROVEN | E0 | — |
| `agent-mail` | WARN | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | constraints-frontloaded, quality-rubric |
| `agent-native` | PASS | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | — |
| `agy-native` | PASS | FAIL-HX | 18/30 B | NOT_PROVEN | E0 | — |
| `anti-ceremony` | PASS | FAIL-HX | 17/30 B | PASS | E1 | — |
| `automation-shape-routing` | PASS | FAIL-HX | 16/30 B | PASS | E0 | — |
| `bootstrap` | PASS | FAIL-HX | 18/30 B | WARN | E1 | — |
| `cass` | WARN | FAIL-HX | 24/30 A | FAIL | E1 | references-modularization |
| `cc-hooks` | WARN | FAIL-HX | 17/30 B | FAIL | E1 | references-modularization |
| `codebase-recon` | PASS | FAIL-HX | 22/30 A | PASS | E1 | — |
| `codex-exec` | WARN | FAIL-HX | 16/30 B | NOT_PROVEN | E1 | constraints-frontloaded, quality-rubric |
| `converter` | PASS | FAIL-HX | 20/30 B | FAIL | E1 | — |
| `council` | WARN | FAIL-HX | 19/30 B | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `craft-goal` | WARN | FAIL-HX | 19/30 B | PASS | E0 | references-modularization |
| `dcg` | PASS | FAIL-HX | 21/30 A | NOT_PROVEN | E1 | — |
| `doc` | PASS | FAIL-HX | 22/30 A | FAIL | E1 | — |
| `domain` | PASS | FAIL-HX | 18/30 B | PASS | E0 | — |
| `fitness` | PASS | FAIL-HX | 18/30 B | WARN | E0 | — |
| `goals` | PASS | FAIL-HX | 13/30 B | FAIL | E1 | — |
| `handoff` | PASS | FAIL-HX | 14/30 B | PASS | E1 | — |
| `idea-genie` | WARN | FAIL-HX | 23/30 A | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `implement` | FAIL→PASS | FAIL-HX | 20/30 B | FAIL | E1 | — |
| `learn` | PASS | FAIL-HX | 16/30 B | WARN | E1 | — |
| `ms` | PASS | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | — |
| `ntm` | WARN | FAIL-HX | 16/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric |
| `operationalize` | PASS | FAIL-HX | 17/30 B | WARN | E1 | — |
| `pattern-mining` | PASS | FAIL-HX | 22/30 A | PASS | E1 | — |
| `plan` | FAIL→WARN | FAIL-HX | 20/30 B | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `postmortem` | PASS | FAIL-HX | 21/30 A | FAIL | E1 | — |
| `premortem` | WARN | FAIL-HX | 21/30 A | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `product` | PASS | FAIL-HX | 16/30 B | PASS | E0 | — |
| `rch` | PASS | FAIL-HX | 19/30 B | NOT_PROVEN | E1 | — |
| `reality-check` | PASS | FAIL-HX | 20/30 B | PASS | E1 | — |
| `refactor` | PASS | FAIL-HX | 19/30 B | FAIL | E1 | — |
| `research` | WARN | FAIL-HX+AT | 20/30 B | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `reverse-engineer` | PASS | FAIL-HX | 23/30 A | FAIL | E1 | — |
| `rpi` | WARN | FAIL-HX | 21/30 A | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `sbh` | PASS | FAIL-HX | 17/30 B | NOT_PROVEN | E0 | — |
| `scaffold` | PASS | FAIL-HX | 21/30 A | FAIL | E1 | — |
| `scope` | WARN | FAIL-HX | 18/30 B | PASS | E0 | constraints-frontloaded |
| `security` | PASS | FAIL-HX | 23/30 A | FAIL | E1 | — |
| `shared` | PASS | FAIL-HX | 12/30 B | PASS | E0 | — |
| `skill-builder` | PASS | FAIL-HX | 23/30 A | WARN | E1 | — |
| `standards` | WARN | FAIL-HX | 21/30 A | PASS | E1 | constraints-frontloaded, quality-rubric |
| `status` | PASS | FAIL-HX+AT | 14/30 B | PASS | E1 | — |
| `swarm` | PASS | FAIL-HX | 13/30 B | FAIL | E1 | — |
| `test` | PASS | FAIL-HX | 21/30 A | FAIL | E1 | — |
| `toil-mining` | PASS | FAIL-HX | 18/30 B | WARN | E1 | — |
| `using-flywheel` | FAIL→WARN | FAIL-HX | 18/30 B | FAIL | E0 | constraints-frontloaded |
| `using-gc` | WARN | FAIL-HX | 15/30 B | NOT_PROVEN | E0 | constraints-frontloaded, quality-rubric, references-modularization |
| `validate` | WARN | FAIL-HX | 21/30 A | FAIL | E1 | constraints-frontloaded, quality-rubric |
| `workflow-builder` | PASS | FAIL-HX | 16/30 B | FAIL | E0 | — |

### Safety evidence for every non-PASS grade

The rubric's severity thresholds were applied to every tracked file. The 13
remaining `PASS` rows are negative static screens: no concrete gap was found,
but runtime safety was not exercised. Non-PASS decisions retain their evidence
here so the classification can be replayed without inferring it from a label.

| Skill | Grade | Evidence and decision basis |
|---|---:|---|
| `cass` | FAIL | `scripts/{recover.sh,quick_analysis.sh,multi_machine_search.sh}` contain uncapped search/index fallbacks, persistent raw-history logs, and host-derived path/SSH fanout without complete bounds or cleanup. |
| `cc-hooks` | FAIL | `SKILL.md` and `references/PATTERNS.md` author and persist arbitrary hook commands, automatic formatters, command rewrites, and permission decisions without general containment, deadlines, cancellation, or descendant cleanup. |
| `converter` | FAIL | `scripts/convert.sh` can remove a caller-selected existing output and uses link-following copy behavior without a separate destructive confirmation or source-tree containment. |
| `council` | FAIL | `SKILL.md` requires independent/model judge contexts while declaring only report writes; it enforces no judge-count, deadline, packet-size, or data boundary. |
| `doc` | FAIL | Mandatory `references/validation-rules.md` workflows write and execute a throwaway scanner and can issue credentialed live-cluster `oc` queries without approval, allowlists, deadlines, or declared network/credential effects. |
| `goals` | FAIL | The alias declares `effects: []` but applies `fitness` exactly, inheriting snapshot and caller-selected render writes; the empty declaration materially contradicts behavior. |
| `idea-genie` | FAIL | Duel mode requires at least two fresh/model contexts while declaring only portfolio writes and no upper bound, deadline, or data boundary. |
| `implement` | FAIL | The workflow executes caller/project acceptance commands without a declared process effect or package-enforced containment, deadline, cancellation, or cleanup. |
| `plan` | FAIL | The workflow persists intent bytes through Validate and requires vendor vanilla quickstarts without declaring those write/process/network effects or bounding their execution. |
| `postmortem` | FAIL | The workflow permits independently dispatched judges over verdict evidence while declaring only report writes and no dispatch, data, deadline, or cleanup controls. |
| `premortem` | FAIL | The workflow constructs defeating inputs, command sequences, or repository state and runs checks without disposable isolation, restoration, declared execution effects, or process bounds. |
| `refactor` | FAIL | Normal regression commands remain arbitrary and unbounded; the separate at-most-two disposable-probe rule does not contain or time-limit those checks. |
| `research` | FAIL | The workflow requires current external primary sources while declaring only report writes and no network, credential, data, or execution bounds. |
| `reverse-engineer` | FAIL | `scripts/{reverse_engineer.py,fetch_url.py,binary/*,security/scan_secrets.sh}` expose unbounded process, fetch, archive, weak-isolation, and matched-secret-output paths. |
| `rpi` | FAIL | `scripts/run_once.py` accepts and directly invokes four arbitrary callables without timeout, cancellation, containment, or cleanup. |
| `scaffold` | FAIL | The workflow runs target-selected build, test, and lint commands; once-only dispatch does not bound wall time, descendants, filesystem/network reach, or cleanup. |
| `security` | FAIL | `scripts/security_suite.py` executes targets without effective filesystem/network containment and can retain secret-bearing process/stdout/stderr evidence; descendants/effects are incomplete. |
| `swarm` | FAIL | `scripts/dispatch_once.py` invokes an arbitrary supplied executor while `SKILL.md` expressly disclaims bounding its transitive writes, runs, or reach; no timeout, cancellation, containment, or cleanup exists. |
| `test` | FAIL | The workflow mutates production logic with nontransactional restoration and runs arbitrary project and `npx` commands without process/network containment, deadlines, or cleanup. |
| `using-flywheel` | FAIL | `SKILL.md` declares `effects: []` while directing `curl` provisioning, installs, remote/runtime writes, and factory/swarm dispatch. |
| `validate` | FAIL | Declared effects omit exact-intent snapshots and caller-selected manifest writes; the helper persists possibly sensitive caller intent, and the workflow re-executes acceptance commands without complete process bounds. |
| `workflow-builder` | FAIL | The skill authors runnable caller-supplied executor dispatch code; at-most-once execution and lexical scopes do not provide approval, containment, timeout, cancellation, or cleanup. |
| `bootstrap` | WARN | The skill can create persistent `.agents/ao/verdicts/sha256/` storage while declaring only project-document writes; explicit request and never-overwrite keep this a local contract gap. |
| `fitness` | WARN | The declared read-only output conflicts with persisted and overwritable goal snapshots/renders in `SKILL.md`. |
| `learn` | WARN | Persistent scratch output is described as TTL'd, but the package defines no duration, expiration, or cleanup mechanism. |
| `operationalize` | WARN | Quote-bank requirements can persist verbatim session, diff, verdict, and command-output excerpts without secret/PII redaction or sensitive-output approval. |
| `skill-builder` | WARN | Multi-surface generation in `scripts/{build.sh,init.sh,heal.sh}` is nontransactional and unbounded; partial writes can survive failure. |
| `toil-mining` | WARN | `scripts/recent_human.py` emits raw human-session text and absolute source paths and supports persistence without secret/PII/path redaction or sensitive-output approval. This is local disclosure risk, not a direct external or destructive path. |
| `account-rotation` | NOT_PROVEN | Rotation and credential protections reside in the external account/runtime implementation; this package contains no enforceable runtime control. |
| `agent-mail` | NOT_PROVEN | Consequential message, hook-install, and destructive-message behavior is owned by the external `am` runtime rather than bundled enforcement. |
| `agent-native` | NOT_PROVEN | The bundled fake runner is test support; real model dispatch/session containment belongs to selected external runtimes. |
| `agy-native` | NOT_PROVEN | Session creation and cleanup are delegated to the external `agy` runtime. |
| `codex-exec` | NOT_PROVEN | The package specifies bounds, but actual process-group containment and cleanup live in the external Codex/wrapper implementation. |
| `dcg` | NOT_PROVEN | Configuration writes and enforcement are performed by the external `dcg` CLI. |
| `ms` | NOT_PROVEN | Consequential reindex and stale-server cleanup enforcement lives in repository-global `scripts/ms-reindex.sh` and the external `ms` runtime, outside this package. |
| `ntm` | NOT_PROVEN | Pane lifecycle, command dispatch, cancellation, and cleanup are owned by the external NTM runtime. |
| `rch` | NOT_PROVEN | Remote compilation, daemon, worker, and transport controls live in the external RCH implementation. |
| `sbh` | NOT_PROVEN | Destructive storage reclamation and host mutation are implemented by the external `sbh` command, not this one-file package. |
| `using-gc` | NOT_PROVEN | Gas City provisioning, dispatch, and runtime controls live outside the package and were not executed or attested. |

### Behavioral evidence map

E1 discovery covered package-local scenarios/tests and repository-global
artifacts that explicitly bind a skill slug and exercise its workflow or
implementation. Behavior-shaped examples and recovery playbooks count as
scenarios regardless of filename; illustrative prose without an explicit input
and observable outcome, and static prose-presence assertions, do not. `S` is a
scenario or self-test definition, `T` an executable targeted test, and `R` a
stored directional receipt. These artifacts establish only E1; none binds the current
package digest to the complete baseline/control matrix required for E2.

| E1 skill | Kind | Qualifying evidence |
|---|---:|---|
| `agent-mail` | R | `evals/routing-probes/{templates.json,results/2026-08-05-batch-2.md}`; joint routing evidence explicitly lists the skill but selected `swarm`, so it is confounded. |
| `agent-native` | T | `tests/integration/test_multi_model_dispatch.bats` executes the bundled fake runner and checks identity, context separation, and degradation. |
| `anti-ceremony` | S/T | `skills/rpi/references/rpi.feature`, `skills/rpi/tests/test_run_once.py`, and `scripts/check-cathedral-cut-conformance.py` exercise guard order and STOP. |
| `bootstrap` | S | `skills/bootstrap/references/examples.md` defines three caller-input scenarios with observable create, preserve, and inspection-only filesystem outcomes. |
| `cass` | S | `skills/cass/SELF-TEST.md`. |
| `cc-hooks` | T | `tests/scripts/{policy-dispatch,installed-skill-edit-guard,installed-skill-edit-telemetry,cross-runtime-hook-baseline}.bats` execute bundled hooks. |
| `codebase-recon` | S/T | `skills/codebase-recon/references/codebase-recon.feature`; `tests/scripts/agentops-native-skills.bats`. |
| `codex-exec` | S | `skills/codex-exec/SKILL.md` defines a runnable bounded example and observable absent, timeout, and nonzero terminal branches. |
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
| `rch` | S | `skills/rch/references/RECOVERY_PLAYBOOKS.md` defines twelve signal-to-diagnostic-to-fix-to-verification scenarios linked by the skill. |
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
`automation-shape-routing`, `craft-goal`, `domain`, `fitness`, `ntm`, `product`,
`sbh`, `scope`, `shared`, `using-flywheel`, `using-gc`, and `workflow-builder`.
Their nearest artifacts were illustrative or procedural prose without labeled
observable scenarios, structural grep/validators, deferred witnesses, or tests
of an external implementation that did not bind the current skill. In particular,
the `fitness` candidates exercise the separately declared `ao goals` surface,
so they do not establish skill-bound E1 evidence.

## Findings that matter

1. **The canonical sources are host-extended, not portable as written.** All 52
   pass the portable identity/description core and the AgentOps v2 profile, but
   all 52 fail strict portable frontmatter. Seven universal top-level AgentOps
   fields sit outside the six published fields, and `capabilities`, `effects`,
   and `dependencies` are lists where portable `metadata` is string-to-string.
   `research` and `status` also separate `allowed-tools` with commas instead of
   spaces.
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
4. **Twenty-two packages have concrete safety failures.** Bundled or generated
   execution/storage defects affect `cass`, `cc-hooks`, `converter`,
   `reverse-engineer`, `rpi`, `security`, `swarm`, and `workflow-builder`.
   Undeclared or unbounded process, model, network, persistence, or transitive
   effects affect `council`, `doc`, `goals`, `idea-genie`, `implement`, `plan`,
   `postmortem`, `premortem`, `refactor`, `research`, `scaffold`, `test`,
   `using-flywheel`, and `validate`. The evidence table names the concrete path
   and decision basis for each; strong static package shape does not offset one.
5. **Six packages need lower-impact safety hardening and eleven cannot be
   attested from the bundle alone.** The WARN set is `bootstrap`, `fitness`,
   `learn`, `operationalize`, `skill-builder`, and `toil-mining`. The
   NOT_PROVEN set delegates consequential controls to external CLIs or
   repo-global code: `account-rotation`, `agent-mail`, `agent-native`,
   `agy-native`, `codex-exec`, `dcg`, `ms`, `ntm`, `rch`, `sbh`, and `using-gc`.
6. **Behavioral proof is the corpus-wide gap.** Thirty-eight skills have only E1
   evidence and 14 have E0. The original automated count missed 13 skill-bound
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
| P0 | Repair the 22 safety FAIL packages as bounded, separately reviewable behavior changes. Start with arbitrary process/model dispatch, credentialed network access, destructive paths, secret emission, and false effect declarations; add planted-negative tests around each repaired boundary. | Each package passes fresh static review plus isolated runtime tests for its exact dangerous paths; no score increase substitutes for that evidence. |
| P1 | Choose and implement one portable boundary: migrate canonical frontmatter into the six-field contract with string metadata, or generate and validate a spec-conformant portable projection while naming canonical sources as AgentOps-only. | A pinned portable validator passes every artifact advertised as portable; host extensions remain explicit and losslessly owned elsewhere. |
| P1 | Repair the audit engine's binding/target/evidence defects before treating it as a conformance oracle outside canonical `skills/*`. | Known-good external fixtures cannot PASS when Pass 1 fails; documented target modes work or are removed; evidence names the actual missing condition. |
| P1 | Build E2 evaluations only for the release-critical spine first: `plan`, `implement`, `validate`, `rpi`, `security`, and `skill-builder`. Use no-skill controls and the five activation/output cases in the rubric. | A release decision consumes exact-version receipts; a failing or inert skill is simplified, reshaped, or retired rather than granted points for files. |
| P1 | Resolve the six safety WARN and eleven NOT_PROVEN packages, prioritizing sensitive local content, retention, external enforcement, credentials, destructive storage, and factories. | Every high-impact path has least-privilege bounds, approval points, time/resource limits, cleanup evidence, and an attested owner even when enforcement lives outside the bundle. |
| P2 | Address a remaining warning only when a real probe, operator incident, or maintenance defect shows harm. | The observed defect disappears; no package gains files solely to raise a heuristic score. |
| Park | Do not mass-add per-skill rubrics, self-tests, reference folders, assets, or delegation packets. | Reconsider only when a named behavior or consumer requires one. |

## Checked and not checked

Checked: canonical inventory completeness and 284-file denominator;
source/projection count; strict AgentOps v2 frontmatter and repository
structural checks; current portable specification and commit-pinned `skills-ref`
compatibility classification; all 52 non-strict deep audits before and after remediation;
deterministic score distribution; all tracked files in the 52 packages through
a complete static safety screen, with high-risk implementations read line by
line; package-local and explicitly skill-bound repository-global behavioral
evidence; schema/tool honesty fields; generated Codex and Gemini projection
drift; focused regression tests.

Not checked: a complete baseline/treatment behavioral matrix for any skill;
every intended model/host/catalog combination; live execution of mutating,
networked, credentialed, destructive, or external-factory workflows; runtime
controls owned by external tools for the eleven safety-NOT_PROVEN packages; and
empirical false-positive/false-negative activation rates. These omissions are
why the corpus result is `NOT_PROVEN`, not a quality `PASS`.
