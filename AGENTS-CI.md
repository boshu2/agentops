# AGENTS-CI.md — CI gates, triage SLAs, deferred hardening, and what each job checks

> Sibling of [`AGENTS.md`](AGENTS.md), [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CODEX.md`](AGENTS-CODEX.md), [`AGENTS-RUNTIME.md`](AGENTS-RUNTIME.md). Split out of the monolithic AGENTS.md per soc-vuu6.3.

## Actions Backstop

AgentOps validation and repository delivery are separate transitions. A local or
cloud agent freezes one candidate, runs deterministic evidence, obtains one
fresh-context Validate verdict, and records Learn before the repository's own
delivery policy begins. Direct push, a PR, external CI, and release automation
are all valid repository adapters; none creates or upgrades the semantic
verdict. `.github/workflows/validate.yml` remains useful for explicit workflow
dispatch, external collaboration, and `v*` release tags. Release tag pushes
force every path-filtered release lane on, and the summary fails if any release
lane is skipped unexpectedly. PR-only evidence jobs are allowlisted on tag
pushes because tag events have no PR body; other skipped jobs are not treated as
release verdicts.
Blocking policy list for optional Actions runs (must match the validate summary failset): every job in the CI table below except jobs marked `(non-blocking)`, including the seven `validate-codex-*` and `validate-headless-runtime-skills` jobs (split from the previous aggregated `codex-runtime-sections` job, soc-ltp2).

#### Advisory Job Triage SLAs (post-merge advisory policy, soc-z7qq)

Advisory and warn-only jobs can run in optional Actions contexts, but their
failure does not rewrite an already recorded semantic verdict. Most surface a
`(advisory)` suffix on the GitHub check name. (`executable-spec-link-integrity`
was promoted to blocking in soc-x7y9f; only its inner `ao goals trace --orphans`
step remains warn-only inside the now-required job.) Each listed job has a
triage SLA or explicit info-only handling — when the job has been red for longer
than its SLA, follow the escalation rule.

| Job | Triage SLA | Escalation rule |
|---|---|---|
| **doctor-check** | 30d | Open a `br` issue tracking the stale CLI reference; prioritize when the next `cli/cmd/ao/**` PR lands. Runs as an advisory step inside the consolidated `correctness` job, not a standalone GitHub job. |
| **check-test-staleness** | none (info-only) | Read the report; no merge or release impact. Item 33 — drift signal, not a gate. |
| **swarm-evidence** | none (info-only) | Read the report; no merge or release impact. Item 34 — informational artifact validation. |
| **executable-spec-link-integrity** (inner trace --orphans step) | none (warn-only) | The job is now blocking (soc-x7y9f) on the `ao goals scenarios --lint` link check; only the inner `ao goals trace --orphans` whole-chain audit stays warn-only. Read the trace output for orphan-chain defects (tracked under soc-gqhrz); no merge impact from that step. |

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is currently warn-only with a deferred promotion gate. Promotion criterion: `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**, where `baseline_p_at_5 = 0.30` is pinned in `docs/CI-CD.md` §"Retrieval-bench ratchet" until a durable non-`.agents` baseline artifact is introduced. The 14-consecutive-nightly observation window is intentionally observational — not yet wired into a counter — so flips to blocking remain a manual decision after the window is documented green.

#### DEFERRED CI Hardening (soc-mi17)

These CI 1-40 items are intentionally not being hardened in this wave. Revisit only when the named promotion trigger fires.

| Item | Current handling | Rationale | Promotion trigger |
|---|---|---|---|
| **1 — go-build error** | DEFER | Compilation breakage is developer hygiene; `cd cli && make build && make test` already exists in the local checklist. | Promote to FIX if a merged `main` commit reaches CI with the same build-class failure twice in 30 days despite focused local evidence. |
| **7 — cli-integration cascade** | DEDUPE/DEFER | Failures cascade from build/test root causes, primarily items 1 and 4. | Promote to FIX if `cli-integration` fails independently after items 1 and 4 are green for two consecutive affected runs. |
| **13 — contract-compatibility** | DEFER | The gate is doing its job; failures indicate real schema or catalog drift. | Promote to FIX if the same false-positive contract failure repeats twice in a quarter. |
| **14 — smoke-test Python 3.14** | DEFER | Rare flake; workflow pinning already narrows the surface. | Promote to FIX if the Python 3.14 smoke failure appears in two separate PRs or nightlies within 30 days. |
| **21 — GoReleaser publish failure** | DEFER | Release publish failures are covered by the `pre-tag-ci-validation` pattern and release discipline. | Promote to FIX if a publish failure recurs on two consecutive release attempts with the same root cause. |
| **22 — doc-release blocks publish** | DEDUPE/DEFER | This is a cascade from item 12 doc-release drift, covered by the repository's deterministic gate. | Promote to FIX if publish is blocked by doc-release after item 12's exact-input gate has passed on the release branch. |
| **23 — markdownlint** | DEFER | Rare and cheap to repair locally. | Promote to FIX if markdownlint failures occur more than twice in a quarter or block a release branch. |
| **24 — shellcheck** | DEFER | Rare and cheap to repair locally. | Promote to FIX if shellcheck failures occur more than twice in a quarter or block a release branch. |
| **27 — plugin-load-test manifest** | DEFER | Low failure rate and the gate catches real manifest/plugin-structure drift. | Promote to FIX if plugin-load-test reports a false positive twice in a quarter. |
| **30 — memrl-health degraded** | DEFER | Rare health signal; investigate when it actually fires. | Promote to FIX if `memrl-health` fires more than once per quarter. |
| **39 — nightly Static Validation** | DEFER | Nightly-only signal should be bundled with future nightly stabilization if the pattern persists. | Promote to FIX if static validation fails in 3 of 10 consecutive nightlies outside a known knowledge-cycle quarantine. |


### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **go-gate-shadow** | Required Go-gate authority lane (`ao gate check --full`); runs the single Go registry with GitHub annotations, JSON evidence, workflow coverage, and `--require-workflow-parity` so blocking validate.yml scripts cannot drift outside the Go gate contract | A blocking Go-gate check failure, a workflow coverage parity gap for a non-deferred blocking script, or an inability to produce/upload the `ao-gate-shadow-report` JSON artifact |
| **correctness** | `ao` builds (Linux + native Windows smoke via matrix); Go tests pass with `-race`/coverage floor; embedded lib/skills in sync; Go complexity budget; CLI + v2.18 integration; release smoke; JSON-flag consistency; bats; Python smoke; advisory `ao doctor` dead-reference check | Build/test failure, race, coverage-floor regression, embedded drift, a function exceeding cyclomatic complexity 25, integration/smoke/bats/JSON-flag breakage, or Windows-smoke regression |
| **security** | No hardcoded secrets or dangerous patterns (`curl\|sh`, `rm -rf /`); unified security toolchain gate (`scripts/security-gate.sh --mode quick`) — gosec, golangci-lint, gitleaks, trivy, semgrep — blocking on any CRITICAL/HIGH finding | Hardcoded API keys/passwords in non-test files, a dangerous pattern, or a CRITICAL/HIGH security/quality finding |

### Nightly Workflow Jobs

`.github/workflows/nightly.yml` runs at 06:00 UTC daily and on `workflow_dispatch`.

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **cli-tests** | Go CLI tests with `-race` and coverage | Test regression in `cli/internal/**` |
| **static-validation** | Smoke, doc-release, and hooks/docs parity gates | Skill/doc drift absent from the candidate receipt |
| **retrieval-bench** | Synthetic + live corpus retrieval precision/coverage gates | P@3 < 0.67 or live coverage < 0.80 |
| **security-toolchain** | Full `security-gate.sh` (semgrep, gosec, gitleaks, trivy, hadolint) | Scanner findings or toolchain install flake |
