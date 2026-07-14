# CI/CD Architecture

Operating contract: [`AGENTS.md`](../AGENTS.md).

AgentOps produces candidate-bound deterministic evidence and one immutable
fresh-context Validate verdict. It does not choose or control Git delivery.
Repositories own delivery policy for local and cloud agents. A repository may
use direct push, a PR, hosted CI, or another adapter, and may select its own
deterministic checks at that boundary without running another semantic review.

Release tag pushes run a full, non-path-filtered deterministic repository CI
suite for the exact tagged SHA. That suite does not create or replace
Validate's semantic verdict. Releases are automated through GoReleaser with
SBOM generation and SLSA provenance attestation.

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

Blocking policy for optional Actions runs (must match the validate summary
failset): every job except those marked non-blocking / advisory below,
including the seven `validate-codex-*` jobs and `validate-headless-runtime-skills`
(split from the former aggregated `codex-runtime-sections` job, soc-ltp2).

## Product and Delivery Boundary

```text
Discovery -> Crank -> frozen candidate
  -> deterministic evidence for the exact candidate
  -> one fresh-context Validate verdict
  -> one Learn receipt
  -> repository-selected delivery adapter
  -> exact remote identity record
```

Hooks and CI jobs are repository adapters, not AgentOps lifecycle authority. A
fast deterministic hook is allowed when a repository wants one. Model review,
tracker closure, delivery queues, and full validation do not belong in that
hook. Exact-input receipts may be reused only when candidate, command, registry,
toolchain, and environment identities match.

## Workflow Map

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| Validate | `validate.yml` | Push to `main`, `v*` tag push, PRs to `main` | Repository CI policy and telemetry; tag pushes force every path-filtered release lane on and allowlist PR-only evidence jobs |
| Release Publisher | `release.yml` | Tag push (`v*`), manual dispatch | Build, publish, attest releases |
| Nightly | `nightly.yml` | Daily 6am UTC, manual | Public proof harness: full test suite + retrieval + security + compile cycle + Dream report-shape validation over repo-visible artifacts |
| Stale Issues | `stale.yml` | Weekly Monday 9am UTC | Auto-mark/close inactive issues and PRs |
| Label PRs | `labeler.yml` | PR opened/synced/reopened | Auto-label PRs by changed paths |

## Nightly vs Dream

AgentOps has two different overnight surfaces:

- **GitHub nightly** validates AgentOps the product. It runs in GitHub Actions against the checked-out repository and proves the CI, flywheel, and Dream report contracts still work.
- **The Dream loop** is the private local compounding engine. AgentOps runs it **in session** via the `/dream` skill against the real repo-local `.agents` corpus, writing the morning report defined in [Dream Report Contract](contracts/dream-report.md). To run it *unattended*, hand it to an orchestration substrate (the reference is NTM + MCP + managed-agents) as a scheduled dispatch — AgentOps ships no daemon or scheduler of its own.

They share primitive steps and report shapes, but they are not the same pipeline.

Important constraint: GitHub Actions cannot see the private `.agents/` corpus when that directory is gitignored. The nightly workflow is therefore a proof harness, not the user's primary Dream runtime.

The Nightly RPI Brief workflow is a prompt packet lane, not a CI-side agent
runner. It reads recent Nightly PR bodies, scheduled Nightly workflow results,
latest Validate runs, open PR check rollups, open "Nightly build failed" issues,
and the current "Nightly RPI auto prompt" issue. It emits structured
`summary.json` fields for `current_ci`, `open_prs`, `open_incidents`,
`prompt_issue`, and ranked `stabilization_targets`, then updates the prompt
issue with a ready `$agentops:rpi --auto` command. This keeps autonomous RPI
selection grounded in observed Nightly drift and current CI blockers while
avoiding hidden source-code mutation from GitHub Actions.

If you want scheduled private Dream runs, delegate them to an orchestration
substrate (the reference is NTM + MCP + managed-agents) and wire a scheduled
dispatch (a managed-agent driver or cron) that runs the Dream loop on a schedule;
the substrate owns the wake, scheduling, and supervision semantics. For the
cross-vendor private local chain that combines Dream, Claude/Codex runners,
RPI/evolve, and PR digest output, see
[`docs/runbooks/nightly-evolution.md`](runbooks/nightly-evolution.md).

## validate.yml Architecture

The validate workflow runs many focused jobs across 4 tiers of parallelism. Most jobs run independently with no `needs` dependencies, maximizing throughput.

### Job Dependency Graph

```text
                    ┌───────────────────────────────────────────────┐
                    │   independent validate jobs, path-filtered    │
                    │                                               │
                    │  doc-release-gate    smoke-test               │
                    │  hook-preflight                               │
                    │  validate-ci-policy-parity                    │
                    │  validate-codex-* runtime/parity checks       │
                    │  validate-goals/registry/flywheel gates       │
                    │  embedded-sync       cli-docs-parity          │
                    │  agentops-contract-canaries                  │
                    │  eval-workbench-verify                       │
                    │  factory/practice advisory observations       │
                    │  shellcheck          markdownlint             │
                    │  security-scan       security-toolchain-gate  │
                    │  skill-integrity     skill-schema             │
                    │  skill-dependency-check                       │
                    │  contract-compatibility-gate                  │
                    │  memrl-health        plugin-load-test         │
                    │  go-build            windows-smoke            │
                    │  cli-integration                              │
                    │  file-manifest-overlap                        │
                    │  skill-lint          learning-coherence       │
                    │  bats-tests          check-test-staleness     │
                    └──────────────┬────────────────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │  go-build (must complete)   │
                    └──┬─────────────┬─────────┬──┘
                       │             │         │
                 ┌─────┴───┐  ┌──────┴───┐ ┌───┴─────────┐
                 │ doctor- │  │coverage- │ │json-flag-   │
                 │  check  │  │ ratchet  │ │consistency  │
                 └────┬────┘  └────┬─────┘ └──────┬──────┘
                      │            │              │
                    ┌─┴────────────┴──────────────┴─┐
                    │           summary             │
                    │  (needs: all validate jobs)   │
                    │  if: always()                 │
                    └───────────────────────────────┘
```

### The `summary` Aggregator Pattern

The final `summary` job lists every other job in its `needs` array and runs with `if: always()`. It fails when any `needs.*.result` is `failure`. Advisory and warn-only jobs avoid blocking through `continue-on-error: true` at the job or step level, so their findings remain visible without producing a failing `needs` result. This single aggregator is the rollup target for any required check on tags/PRs/manual dispatch — only `summary` needs to be required, not every individual job. Whether `summary` is required is repository policy. It never substitutes for or changes the author-distinct semantic verdict.

Current non-blocking validate jobs are `doctor-check`, `factory-claim-ledger-strict`, `practice-citations`, `check-test-staleness`, and `swarm-evidence`. `executable-spec-link-integrity` is blocking on the `ao goals scenarios --lint` link check (soc-x7y9f); only its inner `ao goals trace --orphans` step stays warn-only. `security-toolchain-gate` is blocking. The old `agentops-eval-advisory` job is no longer part of `validate.yml`; `agentops-contract-canaries` remains the blocking deterministic test gate for the stable public canary subset.

For normal `main` pushes and PRs, the `changes` job path-filters expensive lanes.
For `refs/tags/v*` pushes, `changes` forces every category output to `true` and
skips the path-filter step. The release-tag `summary` also fails if any job is
unexpectedly skipped. PR-only evidence jobs are allowlisted because tag push
events do not have a pull request body to inspect. A green release Validate run
therefore means every blocking release lane ran for the exact tagged SHA;
skipped is not treated as passed for releases.

## Blocking vs Soft Gates

### Soft Gates / Advisory Job Triage SLAs (continue-on-error: true)

Advisory and warn-only jobs can run in optional Actions contexts, but their
failure does not rewrite an already recorded semantic verdict. Most surface a
`(advisory)` suffix on the GitHub check name. Keep this table in sync with the
validate summary failset via `scripts/validate-ci-policy-parity.sh`. When a job
has been red longer than its SLA, follow the escalation rule.

| Job | Triage SLA | Escalation rule |
|-----|------------|-----------------|
| `doctor-check` | 30d | Open a `br` issue tracking the stale CLI reference; prioritize when the next `cli/cmd/ao/**` change lands. Runs as an advisory step inside the consolidated `correctness` job, not a standalone GitHub job. |
| `factory-claim-ledger-strict` | 14d | Advisory claim-ledger drift observation for Wave 1E promotion evidence. |
| `practice-citations` | 14d | Advisory strict walk for missing or invalid `practices: [slug,...]` citations. |
| `check-test-staleness` | none (info-only) | Read the report; no merge or release impact. Item 33 — drift signal, not a gate. |
| `swarm-evidence` | none (info-only) | Read the report; no merge or release impact. Item 34 — informational artifact validation. |
| `executable-spec-link-integrity` (inner `trace --orphans` step) | none (warn-only) | Job is blocking (soc-x7y9f) on `ao goals scenarios --lint`; only the inner `ao goals trace --orphans` whole-chain audit stays warn-only. Read the trace for orphan-chain defects (tracked under soc-gqhrz); no merge impact from that step. |

### Retrieval-bench ratchet (nightly)

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is a **warn-then-fail ratchet** with a deferred promotion. The job currently runs warn-only on every nightly. Promotion to blocking is a manual decision after the following observational window is documented green:

- **Promotion criterion:** `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**.
- **Baseline source:** pinned fallback `baseline_p_at_5 = 0.30` in this section. Do not store the baseline under repo-root `.agents/`; that tree is local runtime state and is blocked by `scripts/check-no-tracked-agents.sh`.
- **Future durable source:** if the ratchet needs a machine-readable baseline, add it outside `.agents/` and update this section in the same change.
- **Observation window:** intentionally observational. The 14-consecutive-nightly counter is not yet wired into automation; track manually until a separate bead promotes the gate. This avoids accidental promotion during corpus quarantine windows (`f-2026-04-30-002`).

When the window closes green and the gate is promoted, update this section. Until then, retrieval-bench red is informational; do not block release on it.

### DEFERRED CI Hardening

These CI 1-40 items are intentionally not being hardened in this wave. Revisit only when the named promotion trigger fires.

| Item | Current handling | Rationale | Promotion trigger |
|---|---|---|---|
| **1 — go-build error** | DEFER | Compilation breakage is developer hygiene; `cd cli && make build && make test` already exists in the local checklist. | Promote to FIX if a merged `main` commit reaches CI with the same build-class failure twice in 30 days despite focused local evidence. |
| **7 — cli-integration cascade** | DEDUPE/DEFER | Failures cascade from build/test root causes, primarily items 1 and 4. | Promote to FIX if `cli-integration` fails independently after items 1 and 4 are green for two consecutive affected runs. |
| **13 — contract-compatibility** | DEFER | The gate is doing its job; failures indicate real schema or catalog drift. | Promote to FIX if the same false-positive contract failure repeats twice in a quarter. |
| **14 — smoke-test Python 3.14** | DEFER | Rare flake; workflow pinning already narrows the surface. | Promote to FIX if the Python 3.14 smoke failure appears in two separate PRs or nightlies within 30 days. |
| **21 — GoReleaser publish failure** | DEFER | Release publish failures are covered by the `pre-tag-ci-validation` pattern and release discipline. | Promote to FIX if a publish failure recurs on two consecutive release attempts with the same root cause. |
| **22 — doc-release blocks publish** | DEDUPE/DEFER | Cascade from item 12 doc-release drift, covered by the repository's deterministic gate. | Promote to FIX if publish is blocked by doc-release after item 12's exact-input gate has passed on the release branch. |
| **23 — markdownlint** | DEFER | Rare and cheap to repair locally. | Promote to FIX if markdownlint failures occur more than twice in a quarter or block a release branch. |
| **24 — shellcheck** | DEFER | Rare and cheap to repair locally. | Promote to FIX if shellcheck failures occur more than twice in a quarter or block a release branch. |
| **27 — plugin-load-test manifest** | DEFER | Low failure rate; gate catches real manifest/plugin-structure drift. | Promote to FIX if plugin-load-test reports a false positive twice in a quarter. |
| **30 — memrl-health degraded** | DEFER | Rare health signal; investigate when it actually fires. | Promote to FIX if `memrl-health` fires more than once per quarter. |
| **39 — nightly Static Validation** | DEFER | Nightly-only signal; bundle with future nightly stabilization if the pattern persists. | Promote to FIX if static validation fails in 3 of 10 consecutive nightlies outside a known knowledge-cycle quarantine. |

### Blocking Gates (all others)

Every other job is blocking for workflows that the repository configures to
require `summary`. If any fails, `summary` exits non-zero.

## CI Jobs and What They Check

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

## What Breaks CI

Consolidated checklist of rules enforced by the pipeline:

1. **No symlinks.** `plugin-load-test` rejects all symlinks in the repo. If you need the same file in multiple places, copy it.
2. **Skill counts must be synced.** Adding or removing a skill directory requires `scripts/sync-skill-counts.sh`. Forgetting this fails `doc-release-gate`.
3. **Every `references/*.md` must be linked in SKILL.md.** If a file exists in `skills/<name>/references/`, the skill's SKILL.md must contain a markdown link to it. Check with `heal.sh --strict`.
4. **Embedded hooks must stay in sync (2.x / opt-in only).** AgentOps 3.0 ships zero hooks by default. If you edit the legacy `hooks/` tree or opt-in `skills/cc-hooks` references for a custom install, run `cd cli && make sync-hooks`. Checked by `embedded-sync` and `go-build`.
5. **CLI docs must stay in sync.** After adding/changing CLI commands or flags: run `scripts/generate-cli-reference.sh`. Checked by `cli-docs-parity`.
6. **Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/documentation-index.md`. Checked by `contract-compatibility-gate`.
7. **Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15). Checked by `go-build` via `check-go-complexity.sh`.
8. **Windows installer smoke must pass.** PowerShell installers need to parse, temp installs need to work, and focused Windows-sensitive Go tests must pass on `windows-latest`. Checked by `windows-smoke`.
9. **No TODOs in SKILL.md.** Use **br** issue tracking instead (`BEADS_DIR="$(ao beads dir)" br create …`). Checked by `skill-lint`.
10. **No secrets in code.** `security-scan` greps for hardcoded passwords, API keys, and tokens in non-test files.
11. **No dangerous shell patterns.** `security-scan` rejects `rm -rf /`, `curl | sh`, etc. in scripts (with explicit exceptions for installer scripts).

## Local CI Guide

### scripts/ci-local-release.sh

The local CI bundle can reproduce the remote deterministic pipeline in 7 phases:

| Phase | Description | Parallelism |
|-------|-------------|-------------|
| 1 | Required tool check (bash, git, jq, go, shellcheck, markdownlint) | Sequential |
| 2 | Quick independent checks: doc-release gate, manifest validation, hook preflight, parity checks, secret scans, MemRL health, etc. | Parallel (capped at half CPU cores, min 4) |
| 3 | Medium-weight checks: CLI docs parity, ShellCheck, markdownlint, smoke tests, integration tests, coverage floor | Parallel |
| 3b | Remote-parity checks also covered by `validate.yml` | Parallel |
| 4 | Heavy checks: Go build + race tests, hook integration tests, SBOM generation, security toolchain gate | Parallel |
| 5 | CLI smoke tests: `ao` init/bootstrap smoke, release smoke test | Parallel |
| 6 | Post-hoc `$HOME/.agents` content-hash gate | Sequential |
| 7 | Release readiness evidence: HIL capture plus 8/10 readiness score | Sequential |

### Flags

```bash
scripts/ci-local-release.sh              # Full gate (~100s)
scripts/ci-local-release.sh --fast       # Skip race tests, security gate, SBOM, hook integration (~20s)
scripts/ci-local-release.sh --jobs 8     # Override parallel job cap
scripts/ci-local-release.sh --security-mode quick  # Quick security scan
scripts/ci-local-release.sh --release-version 2.X.Y --hil-target 'local:bushido:ao version'
scripts/ci-local-release.sh --release-version 2.X.Y --hil-waiver 'target unavailable'
```

In `--fast` mode, Phase 4 skips race tests, hook integration tests, SBOM generation, and the security gate. It still builds the binary and runs release validation.
When `--release-version` is set, Phase 7 runs in official mode and fails unless the readiness score is at least 8/10 with SIL/VIL pass and HIL pass or waiver.

### Minimum Focused Checks

Select the commands that cover the changed surface and record them against the
exact candidate. Repository policy may run broader checks before delivery:

```bash
bash skills/heal-skill/scripts/heal.sh --strict   # Skill integrity
./tests/docs/validate-doc-release.sh               # Skill counts + links
./scripts/check-contract-compatibility.sh           # Contract refs + JSON validity

# If you changed Go code:
cd cli && make build && make test

# If you changed Windows installers, Codex install surfaces, or OS-specific file locking:
powershell -ExecutionPolicy Bypass -File .\tests\windows\test-windows-smoke.ps1

# If you changed legacy hooks or lib/hook-helpers.sh (opt-in installs only):
cd cli && make sync-hooks
```

### Local-Only Checks

Four checks run only in the local CI gate and are intentionally excluded from `validate.yml`:

| Script | Reason |
|--------|--------|
| `check-doctor-health.sh` | Already present in `validate.yml` as the `doctor-check` job; duplicating it adds no value |
| `check-go-command-test-pair.sh` | Go-specific pairing check; CI has a dedicated `go-build` job that covers this surface |
| `validate-skill-cli-snippets.sh` | Verifies `ao ...` snippets in `skills/` and `skills-codex/` against the built CLI help surface so stale commands and flags fail locally |
| `release-cadence-check.sh` | Only relevant at release time; not meaningful in a per-push pipeline |

### Skipped Remote-Parity Checks

One CI check is intentionally **not** wired into the local gate:

| Script | Reason |
|--------|--------|
| `validate-learning-coherence.sh` | Fails on pre-existing frontmatter-only learning files; needs repo cleanup before local enforcement |

## Hookless by default

AgentOps 3.0 ships **zero runtime hooks by default**. Explicit commands produce
the candidate's evidence before Validate. A consumer repository may install a
small deterministic hook or rely on external CI, but that adapter owns only its
repository policy. It must not invoke a model, mutate the semantic verdict,
close tracker work, or replay an unchanged full suite. The workflow is guided by
skills plus the `ao` CLI; context flows through explicit channels, not hook side
effects.

If you want a bounded deterministic hook of your own, author it with the
`hooks-authoring` skill and keep its authority in the repository that installs
it. Do not infer live behavior from tracked examples; inspect the repository's
actual Git and CI configuration.

## Security Gate

### scripts/security-gate.sh

Orchestrates the unified security scanning pipeline. Delegates to `scripts/toolchain-validate.sh` for actual scanner invocation.

```bash
scripts/security-gate.sh --mode quick          # Fast scan (CI default)
scripts/security-gate.sh --mode full           # Full suite (nightly, release)
scripts/security-gate.sh --mode full --json    # Machine-readable output
scripts/security-gate.sh --require-tools       # Fail if scanners missing
```

### scripts/toolchain-validate.sh

Runs the scanner invocation contract used by `scripts/security-gate.sh`, including JSON output, quick-mode skips, and gate exit codes.

```bash
scripts/toolchain-validate.sh --quick --gate --json
scripts/toolchain-validate.sh --gate --json
```

### Scanners

| Scanner | Target | Purpose |
|---------|--------|---------|
| semgrep | Go, Python, Shell | Static analysis for security anti-patterns |
| gosec | Go | Go-specific security linter |
| gitleaks | Git history | Detect leaked secrets in commits |
| golangci-lint | Go | Comprehensive Go linter suite |
| trivy | Filesystem | Vulnerability scanning, SBOM generation |
| hadolint | Dockerfiles | Dockerfile best practices |
| ruff | Python | Python linter |
| radon | Python | Cyclomatic complexity for Python |
| ShellCheck | Shell | Shell script analysis (also runs standalone in validate.yml) |

## Release Workflow

### Pipeline

The release workflow (`release.yml`) triggers on version tags (`v*`) or manual dispatch:

1. **Pre-flight gates:** `doc-release-gate` and `pre-publish-evidence` are both blocking
2. **Version resolution:** Extracts version from tag or manual input
3. **Validation:** Verifies tag exists, Homebrew token is valid
4. **Release notes:** Extracts from CHANGELOG.md via `scripts/extract-release-notes.sh`
5. **Pre-publish evidence:** Generates CycloneDX SBOM, runs the full security gate, and writes release readiness before GoReleaser can start
6. **Publish:** GoReleaser builds cross-platform binaries (darwin/linux/windows, amd64/arm64)
7. **Post-publish:** Applies curated release notes and uploads the already-passed SBOM, security report, and readiness evidence as release assets
8. **Attestation:** SLSA provenance via `actions/attest-build-provenance@v4` covering all tarballs, checksums, SBOM, security report, and readiness
9. **Homebrew:** GoReleaser auto-updates `boshu2/homebrew-agentops` tap

Manual dispatch is a rerun path, not the primary publish path for a new version. For a fresh release, push the tag. For post-tag fixes, use `scripts/retag-release.sh vX.Y.Z`. Do not start a manual dispatch in parallel with the tag-push workflow for the same tag.

### Release Timing

- AgentOps does not enforce a minimum gap between releases.
- Draft releases do not notify watchers and can be used freely for CI testing.
- Curated release notes are written to `docs/releases/YYYY-MM-DD-v<version>-notes.md` before tagging.

### Release Commands

```bash
# Normal release
git tag v2.X.0 && git push origin v2.X.0

# Retag (roll post-tag commits into existing release)
scripts/retag-release.sh v2.X.0

# Local validation before tagging
scripts/ci-local-release.sh --release-version 2.X.0 --hil-target 'local:bushido:ao version'
```

## Script Categories

| Category | Pattern | Examples | Purpose |
|----------|---------|----------|---------|
| Validation | `validate-*.sh` | `validate-embedded-sync.sh`, `validate-hook-preflight.sh`, `validate-skill-schema.sh` | CI checks that verify invariants |
| CI | `ci-*.sh`, `check-*.sh` | `ci-local-release.sh`, `check-go-complexity.sh`, `check-contract-compatibility.sh` | CI orchestration and specific checks |
| Release | `release-*.sh`, `extract-*.sh`, `retag-*.sh` | `release-smoke-test.sh`, `extract-release-notes.sh`, `retag-release.sh` | Release pipeline support |
| Security | `security-*.sh`, `toolchain-*.sh` | `security-gate.sh`, `toolchain-validate.sh` | Security scanning orchestration |
| Generation | `generate-*.sh` | `generate-cli-reference.sh` | Regenerate derived artifacts |
| Sync | `sync-*.sh` | `sync-skill-counts.sh` | Keep cross-referenced files in sync |
| Maintenance | `prune-*.sh` | `prune-agents.sh` | Clean up bloated directories |
