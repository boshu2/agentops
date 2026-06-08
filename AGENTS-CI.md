# AGENTS-CI.md — CI gates, triage SLAs, deferred hardening, and what each job checks

> Sibling of [`AGENTS.md`](AGENTS.md), [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CODEX.md`](AGENTS-CODEX.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md). Split out of the monolithic AGENTS.md per soc-vuu6.3.

## CI Validation — Passing the Pipeline

All pushes to `main`, `v*` release tag pushes, and PRs run `.github/workflows/validate.yml`. Release tag pushes force every path-filtered release lane on, and the summary fails if any release lane is skipped unexpectedly. PR-only evidence jobs are allowlisted on tag pushes because tag events have no PR body; other skipped jobs are not treated as release verdicts. **Run checks locally before pushing.** The summary job gates on all jobs that are not marked non-blocking or advisory in the table below. `executable-spec-link-integrity` is now blocking (T1, soc-x7y9f): its directive↔scenario link lint fails the summary on any broken edge.
Blocking policy list (must match the validate summary failset): every job in the CI table below except jobs marked `(non-blocking)`, including the seven `validate-codex-*` and `validate-headless-runtime-skills` jobs (split from the previous aggregated `codex-runtime-sections` job, soc-ltp2).

#### Advisory Job Triage SLAs (post-merge advisory policy, soc-z7qq)

Advisory and warn-only jobs run on every PR but their failure does NOT block merge. Most surface a `(advisory)` suffix on the GitHub check name. (`executable-spec-link-integrity` was promoted to blocking in soc-x7y9f; only its inner `ao goals trace --orphans` step remains warn-only inside the now-required job.) Each listed job has a triage SLA or explicit info-only handling — when the job has been red for longer than its SLA, follow the escalation rule.

| Job | Triage SLA | Escalation rule |
|---|---|---|
| **doctor-check** | 30d | Open a `bd` issue tracking the stale CLI reference; prioritize when the next `cli/cmd/ao/**` PR lands. Runs as an advisory step inside the consolidated `correctness` job, not a standalone GitHub job. |
| **check-test-staleness** | none (info-only) | Read the report; no merge or release impact. Item 33 — drift signal, not a gate. |
| **swarm-evidence** | none (info-only) | Read the report; no merge or release impact. Item 34 — informational artifact validation. |
| **executable-spec-link-integrity** (inner trace --orphans step) | none (warn-only) | The job is now blocking (soc-x7y9f) on the `ao goals scenarios --lint` link check; only the inner `ao goals trace --orphans` whole-chain audit stays warn-only. Read the trace output for orphan-chain defects (tracked under soc-gqhrz); no merge impact from that step. |

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is currently warn-only with a deferred promotion gate. Promotion criterion: `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**, where `baseline_p_at_5 = 0.30` is pinned in `docs/CI-CD.md` §"Retrieval-bench ratchet" until a durable non-`.agents` baseline artifact is introduced. The 14-consecutive-nightly observation window is intentionally observational — not yet wired into a counter — so flips to blocking remain a manual decision after the window is documented green.

#### DEFERRED CI Hardening (soc-mi17)

These CI 1-40 items are intentionally not being hardened in this wave. Revisit only when the named promotion trigger fires.

| Item | Current handling | Rationale | Promotion trigger |
|---|---|---|---|
| **1 — go-build error** | DEFER | Compilation breakage is developer hygiene; `cd cli && make build && make test` already exists in the local checklist. | Promote to FIX if a merged `main` commit reaches CI with the same build-class failure twice in 30 days despite local pre-push guidance. |
| **7 — cli-integration cascade** | DEDUPE/DEFER | Failures cascade from build/test root causes, primarily items 1 and 4. | Promote to FIX if `cli-integration` fails independently after items 1 and 4 are green for two consecutive affected runs. |
| **13 — contract-compatibility** | DEFER | The gate is doing its job; failures indicate real schema or catalog drift. | Promote to FIX if the same false-positive contract failure repeats twice in a quarter. |
| **14 — smoke-test Python 3.14** | DEFER | Rare flake; workflow pinning already narrows the surface. | Promote to FIX if the Python 3.14 smoke failure appears in two separate PRs or nightlies within 30 days. |
| **21 — GoReleaser publish failure** | DEFER | Release publish failures are covered by the `pre-tag-ci-validation` pattern and release discipline. | Promote to FIX if a publish failure recurs on two consecutive release attempts with the same root cause. |
| **22 — doc-release blocks publish** | DEDUPE/DEFER | This is a cascade from item 12 doc-release drift, now covered by pre-push gating. | Promote to FIX if publish is blocked by doc-release after item 12's local gate has passed on the release branch. |
| **23 — markdownlint** | DEFER | Rare and cheap to repair locally. | Promote to FIX if markdownlint failures occur more than twice in a quarter or block a release branch. |
| **24 — shellcheck** | DEFER | Rare and cheap to repair locally. | Promote to FIX if shellcheck failures occur more than twice in a quarter or block a release branch. |
| **27 — plugin-load-test manifest** | DEFER | Low failure rate and the gate catches real manifest/plugin-structure drift. | Promote to FIX if plugin-load-test reports a false positive twice in a quarter. |
| **30 — memrl-health degraded** | DEFER | Rare health signal; investigate when it actually fires. | Promote to FIX if `memrl-health` fires more than once per quarter. |
| **39 — nightly Static Validation** | DEFER | Nightly-only signal should be bundled with future nightly stabilization if the pattern persists. | Promote to FIX if static validation fails in 3 of 10 consecutive nightlies outside a known knowledge-cycle quarantine. |


### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **go-gate-shadow** | Required Go-gate authority lane for the migration to `ao gate check --full`; runs the single Go registry with GitHub annotations, JSON evidence, workflow coverage, and `--require-workflow-parity` so blocking validate.yml scripts cannot drift outside the Go gate contract | A blocking Go-gate check failure, a workflow coverage parity gap for a non-deferred blocking script, or an inability to produce/upload the `ao-gate-shadow-report` JSON artifact |
| **correctness** | `ao` builds (Linux + native Windows smoke via matrix); Go tests pass with `-race`/coverage floor; embedded lib/skills in sync; Go complexity budget; CLI + v2.18 integration; release smoke; JSON-flag consistency; bats; Python smoke; advisory `ao doctor` dead-reference check | Build/test failure, race, coverage-floor regression, embedded drift, a function exceeding cyclomatic complexity 25, integration/smoke/bats/JSON-flag breakage, or Windows-smoke regression |
| **lint** | ShellCheck (error severity) on all `.sh`, markdownlint on docs, and the skill lint suite (`tests/skills/run-all.sh`) | Unquoted shell variables, markdown formatting regressions, or a skill-lint rule violation |
| **security** | No hardcoded secrets or dangerous patterns (`curl\|sh`, `rm -rf /`); unified security toolchain gate (`scripts/security-gate.sh --mode quick`) — gosec, golangci-lint, gitleaks, trivy, semgrep — blocking on any CRITICAL/HIGH finding | Hardcoded API keys/passwords in non-test files, a dangerous pattern, or a CRITICAL/HIGH security/quality finding |
| **skill-gates** | Consolidated skill-authoring gate surface (ag-87sv) — structural heal (`--strict`), SKILL.md schema + v2 frontmatter, skill-body command/flag refs resolve against the live CLI, skill-flow connectivity + closed `consumes` vocabulary, scenario↔test linkage (`@covered-by`), and the six-surface derived-artifact drift sweep (`scripts/regen-all.sh --check`) | A SKILL.md schema/frontmatter violation, a stranded skill-body command/flag ref, a skill-flow connectivity break, an unlinked Gherkin scenario, or six-surface derived-artifact drift (`regen-all.sh --check`) |
| **skills-integrity** | Skill dependencies resolve, headless-runtime skills, manifests valid against versioned schemas, no symlinks, local-only `.agents/`, plugin directory structure | An unresolved skill dependency, a symlink, or invalid plugin/manifest structure |
| **contracts-sync** | Every derived artifact is in sync — `registry.json`, CLI docs, context map, skill-domain map, SKU catalog, skill catalog (advisory), bounded-contexts, embedded lib/skills, CI policy parity, contract compatibility + next-work parity, contracts structural floor — plus the official AgentOps contract canaries | Editing a source (skill/CLI/contract) without regenerating its derived artifact, a contract-compat break, or a contract canary regression |
| **codex-parity** | Codex runtime sections, generated artifacts, backbone prompts, override coverage, RPI contract, lifecycle guards, and parity drift (GOALS.md directive D7). `skills-codex/` is manually maintained — audit drift with `scripts/audit-codex-parity.sh` | A Codex runtime/artifact/contract/parity drift between `skills/` and the manually-maintained `skills-codex/` |
| **doctrine-proof** | Flywheel/goals/wiring/corpus/finding-registry/memrl/sovereignty/three-gap proofs PLUS spec-linkage — executable-spec link integrity (`ao goals scenarios --lint`), AGENTS.md tiered split, docs↔learning references (scenario↔test linkage moved to `skill-gates`) | A failing GOALS/doctrine proof gate, a broken directive↔scenario link, an AGENTS.md split-contract violation, or a dangling docs↔learning reference |
| **eval** | Eval baseline-audit drift-only gate (`stale_suite_hashes>0`), eval-skill-delta dry-run, workbench golden state (D10 delta), and offline retrieval-quality bench + comparison smoke | A promoted baseline's suite SHA drift, broken delta/harness infrastructure, a workbench golden-state regression, or a retrieval-quality regression |
| **skill-eval** | T1 changed-files-scoped: gates each CHANGED skill's SKILL.md through Jeff Emanuel's `ms` (meta_skill v0.1.2) lint + validate via `scripts/skill-eval.sh`. Pinned-`ms` install gated on `ms --version` before the gate runs — a failed install HARD-FAILS the job (never green-skips). Runs `ms` only for skills whose `skills/<id>/**` changed in the PR. Also carries an I0-INFORMATIONAL step (ag-iyu4) — `scripts/skill-probe-i0.sh` runs the deterministic lexical trigger ranker (`scan_descriptions.py --probe`, ag-7led) over each `trigger_probes:` phrase, writes a per-skill JSON receipt to `.agents/ao/skill-eval/<id>.json` (uploaded as the `skill-retrieval-probe-receipts` artifact), and asserts byte-stable determinism across two runs. The I0 step is `continue-on-error` INSIDE this job, so it produces no separate PR check and cannot block; a non-deterministic probe is surfaced as a `::warning::` only. GATE-PROMOTION GUARD: the probe stays I0 (no blocking assertion) until this receipt lane runs green + byte-stable across the corpus for a 2-WEEK STABILITY BASELINE of merges. | A blocking `ms` finding (no-secrets/no-injection/safe-paths/required-metadata/no-cycle/valid-version) on a changed skill's SKILL.md, or a pinned-`ms`-install failure. The I0 retrieval-probe step never contributes to this job's pass/fail (informational; not a PR check). |
| **process-hygiene** | Doc-release stabilization (skill counts + links), `tests/_quarantine/` empty (D3), test-count non-regression ratchet, file-manifest overlap self-test, plus advisory test-staleness, swarm-evidence, and Evidence-line lint | Skill-count drift, a non-empty quarantine, a net per-package test-count decrease without a `Test-Removal-Reason:` trailer, or a manifest-overlap regression |

### Nightly Workflow Jobs

`.github/workflows/nightly.yml` runs at 06:00 UTC daily and on `workflow_dispatch`.

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **cli-tests** | Go CLI tests with `-race` and coverage | Test regression in `cli/internal/**` |
| **static-validation** | Smoke, doc-release, and hooks/docs parity gates | Skill/doc drift slipping past pre-push |
| **retrieval-bench** | Synthetic + live corpus retrieval precision/coverage gates | P@3 < 0.67 or live coverage < 0.80 |
| **security-toolchain** | Full `security-gate.sh` (semgrep, gosec, gitleaks, trivy, hadolint) | Scanner findings or toolchain install flake |
| **knowledge-cycle** | Deduped compile + dream-cycle + Athena follow-up sharing one substrate (`scripts/nightly-knowledge-cycle.sh`); corpus-empty precondition skip per `f-2026-04-30-002`; single `nightly-knowledge-cycle` triage artifact replaces three (compile-report, dream-cycle-report, Athena) — `soc-2xmg` | Compile health gate fails, dream-cycle proof regresses, or substrate inputs missing |

**Knowledge-cycle precondition:** the `knowledge-cycle` job calls `scripts/nightly-knowledge-cycle.sh precondition` before any compile/dream/Athena stage. When `total_artifacts == 0`, the cycle SKIPs every downstream stage with reason `corpus-empty`; when `total_artifacts > 0 && total_citations_in_window == 0`, it SKIPs with reason `corpus-dormant` rather than failing three separate jobs on the same unavailable-corpus condition. Override with `NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1` for diagnostic runs. Static Validation (`#39` in the CI failure ranking) remains in its own `static-validation` job by design — see plan `2026-05-03-ci-failures-1-40-handling.md` §nightly-knowledge-cycle-dedupe.
