# CI/CD Architecture

AgentOps uses a local push-as-CI gate for routine direct-main work. The release authority for a normal `main` push is the installed local cockpit gate: `.git/hooks/pre-push` chains to `scripts/hooks/pre-push.local`, builds a fresh `ao`, runs the full `go test ./... -race -shuffle=on -count=1` suite for pushes to `main`, then serializes the mutable `ao gate check --fast` and head-bound pawl checks. GitHub Actions remain useful telemetry and a tag/PR/manual backstop; they are not the routine release gate for direct-main pushes.

Release tag pushes run a full, non-path-filtered Validate verdict for the exact tagged SHA. Releases are automated through GoReleaser with SBOM generation and SLSA provenance attestation.

The CI job source of truth is `docs/contracts/ci-jobs.yaml`. The explanatory
table below is rendered by `scripts/generate-ci-jobs-table.sh`; edit the
manifest, not generated rows.

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
| **codex-parity** | Codex runtime sections, generated `parity_only` twins sourced from `skills/`, hand-maintained cataloged `bespoke` artifacts, backbone prompts, override coverage, RPI contract, lifecycle guards, and parity drift (GOALS.md directive D7) | A `parity_only` twin differs from its source or transform, a cataloged `bespoke` artifact is missing or stale under deliberate review, an artifact is unclassified, or a Codex runtime/contract/override mismatch exists |
| **doctrine-proof** | Flywheel/goals/wiring/corpus/finding-registry/memrl/sovereignty/three-gap proofs PLUS spec-linkage — executable-spec link integrity (`ao goals scenarios --lint`), compact AGENTS route contract, docs↔learning references (scenario↔test linkage moved to `skill-gates`) | A failing GOALS/doctrine proof gate, a broken directive↔scenario link, an AGENTS route-contract violation, or a dangling docs↔learning reference |
| **eval** | Eval baseline-audit drift-only gate (`stale_suite_hashes>0`), eval-skill-delta dry-run, workbench golden state (D10 delta), and offline retrieval-quality bench + comparison smoke | A promoted baseline's suite SHA drift, broken delta/harness infrastructure, a workbench golden-state regression, or a retrieval-quality regression |
| **skill-eval** | T1 changed-files-scoped: gates each CHANGED skill's SKILL.md through Jeff Emanuel's `ms` (meta_skill v0.1.2) lint + validate via `scripts/skill-eval.sh`. Pinned-`ms` install gated on `ms --version` before the gate runs — a failed install HARD-FAILS the job (never green-skips). Runs `ms` only for skills whose `skills/<id>/**` changed in the PR. Also carries an I0-INFORMATIONAL step (ag-iyu4) — `scripts/skill-probe-i0.sh` runs the deterministic lexical trigger ranker (`scan_descriptions.py --probe`, ag-7led) over each `trigger_probes:` phrase, writes a per-skill JSON receipt to `.agents/ao/skill-eval/<id>.json` (uploaded as the `skill-retrieval-probe-receipts` artifact), and asserts byte-stable determinism across two runs. The I0 step is `continue-on-error` INSIDE this job, so it produces no separate PR check and cannot block; a non-deterministic probe is surfaced as a `::warning::` only. GATE-PROMOTION GUARD: the probe stays I0 (no blocking assertion) until this receipt lane runs green + byte-stable across the corpus for a 2-WEEK STABILITY BASELINE of merges. | A blocking `ms` finding (no-secrets/no-injection/safe-paths/required-metadata/no-cycle/valid-version) on a changed skill's SKILL.md, or a pinned-`ms`-install failure. The I0 retrieval-probe step never contributes to this job's pass/fail (informational; not a PR check). |
| **process-hygiene** | Doc-release stabilization (skill counts + links), `tests/_quarantine/` empty (D3), test-count non-regression ratchet, file-manifest overlap self-test, plus advisory test-staleness, swarm-evidence, and Evidence-line lint | Skill-count drift, a non-empty quarantine, a net per-package test-count decrease without a `Test-Removal-Reason:` trailer, or a manifest-overlap regression |

## Live Main-Push Path

```text
git push origin HEAD:main
  -> .git/hooks/pre-push
  -> scripts/hooks/pre-push.local
  -> go build ./...
  -> go test ./... -race -shuffle=on -count=1
  -> acquire host-local lock for mutable gate/provenance/pawl surfaces
  -> ao gate check --fast
  -> scripts/check-pawl-pre-push.sh
  -> remote main fast-forward
```

The tracked `.githooks/` directory is legacy tracker plumbing. Git uses
`.git/hooks` unless configured otherwise; `scripts/install-pre-push-gate.sh`
installs or chains the live gate in the shared Git directory.

## Workflow Map

| Workflow | File | Trigger | Purpose |
|---|---|---|---|
| Validate | `validate.yml` | `main`, `v*`, PR, manual | Routine telemetry; blocking backstop for tags, PRs, and manual validation |
| Release Publisher | `release.yml` | `v*`, manual | Build, publish, and attest releases |
| Nightly | `nightly.yml` | Daily, manual | Public test, retrieval, security, and report-contract proof |
| Nightly RPI Brief | `nightly-rpi-brief.yml` | Daily, manual | Builds an evidence digest and prompt packet; does not mutate source |
| Stale Issues | `stale.yml` | Weekly | Repository issue/PR hygiene |
| Label PRs | `labeler.yml` | PR events | Labels changed paths |

## Nightly and private compounding

GitHub Nightly validates the public product against the checked-out repository.
It cannot see gitignored `.agents/` state. Private compounding runs in-session or
through an explicitly selected external scheduling substrate; AgentOps ships no
daemon or scheduler. Corpus-empty or corpus-dormant conditions skip the private
knowledge cycle with an explicit reason instead of converting unavailable local
state into three CI failures.

## validate.yml architecture

Validate jobs are path-filtered and mostly independent. The final `summary`
aggregator uses `if: always()` and fails when any blocking dependency fails.
Tag pushes force release categories on and reject unexpected skips; PR-only
evidence jobs are explicitly allowlisted on tag events.

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

The final `summary` job lists every other job in its `needs` array and runs with `if: always()`. It fails when any `needs.*.result` is `failure`. Advisory and warn-only jobs avoid blocking through `continue-on-error: true` at the job or step level, so their findings remain visible without producing a failing `needs` result. This single aggregator is the rollup target for any required check on tags/PRs/manual dispatch — only `summary` needs to be required, not every individual job. Note: under the push-to-main model (ag-qidx) branch protection is **off**, so on routine `main` pushes the authoritative release gate is the local pre-push Go gate (`ao gate check`); `validate.yml`/`summary` is a CI backstop on tags, PRs, and manual dispatch, not a required main-push gate.

There are no job-level `continue-on-error` entries in current `validate.yml`.
Advisory behavior is scoped to named steps inside blocking consolidated jobs;
the generated CI table remains the blocking-policy source.

For normal `main` pushes and PRs, the `changes` job path-filters expensive lanes.
For `refs/tags/v*` pushes, `changes` forces every category output to `true` and
skips the path-filter step. The release-tag `summary` also fails if any job is
unexpectedly skipped. PR-only evidence jobs are allowlisted because tag push
events do not have a pull request body to inspect. A green release Validate run
therefore means every blocking release lane ran for the exact tagged SHA;
skipped is not treated as passed for releases.

## Blocking vs Soft Gates

### Advisory steps inside blocking jobs

These observations do not change their containing job's required checks:

| Job | Triage SLA | Reason |
|-----|------------|--------|
| `doctor-check` | 30d | Reports stale CLI references; CI environment lacks some expected tools |
| `check-test-staleness` | none (info-only) | Advisory -- flags tests that may need updating (item 33) |
| `swarm-evidence` | none (info-only) | Advisory -- validates swarm evidence artifact shape; missing/malformed swarm artifacts are informational, not blocking (item 34) |
| `executable-spec-link-integrity` inner trace | none (warn-only) | Scenario lint is blocking; only `ao goals trace --orphans` remains observational |

### Retrieval-bench ratchet (nightly)

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is a **warn-then-fail ratchet** with a deferred promotion. The job currently runs warn-only on every nightly. Promotion to blocking is a manual decision after the following observational window is documented green:

- **Promotion criterion:** `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**.
- **Baseline source:** pinned fallback `baseline_p_at_5 = 0.30` in this section. Do not store the baseline under repo-root `.agents/`; that tree is local runtime state and is blocked by `scripts/check-no-tracked-agents.sh`.
- **Future durable source:** if the ratchet needs a machine-readable baseline, add it outside `.agents/`, wire the nightly workflow to that source, and update this section in the same change.
- **Observation window:** intentionally observational. The 14-consecutive-nightly counter is not yet wired into automation; track manually until a separate bead promotes the gate. This avoids accidental promotion during corpus quarantine windows (`f-2026-04-30-002`).

When the window closes green and the gate is promoted, update `.github/workflows/nightly.yml` and this section. If promotion changes a `Validate` job policy, update `docs/contracts/ci-jobs.yaml` and regenerate the CI job table. Until then, retrieval-bench red is informational; do not block release on it.

### Deferred hardening triggers

| Item | Revisit when |
|---|---|
| Go build | The same build-class failure reaches merged `main` twice in 30 days |
| CLI integration cascade | It fails independently after build and core tests are green twice |
| Contract compatibility | The same false positive repeats twice in a quarter |
| Python 3.14 smoke | It recurs in two independent runs within 30 days |
| GoReleaser publish | The same root cause breaks two consecutive release attempts |
| Doc-release publish block | It blocks publish after the local doc gate passed |
| Markdownlint or ShellCheck | Either fails more than twice in a quarter or blocks release |
| Plugin manifest | The same false positive repeats twice in a quarter |
| MemRL health | It fires more than once per quarter |
| Nightly static validation | It fails in 3 of 10 runs outside a known quarantine |

These are promotion triggers, not current waivers for required checks.

### Blocking Gates (all others)

Every other job is blocking. If any of these fail, `summary` exits non-zero and the PR/push is rejected.

## What Breaks CI

Consolidated checklist of rules enforced by the pipeline:

1. **No symlinks in distributable skill/plugin trees.** Copy shared skill references. The tracked root `CLAUDE.md -> AGENTS.md` compatibility alias is intentional.
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

The local CI gate mirrors the remote pipeline and runs in 7 phases:

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

### Minimum Checks Before Any Push

From CLAUDE.md -- the bare minimum before pushing:

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

## Hookless by default - local gate is the release authority

AgentOps 3.0 ships **zero runtime hooks by default**. Repository contributors can
opt into the local cockpit gate by running `scripts/install-pre-push-gate.sh`,
which installs the current gate into the shared `.git/hooks` directory for the
main checkout and linked worktrees. What a session hook used to enforce
implicitly is now enforced by explicit local commands and this installed
pre-push chain, primarily `ao gate check --fast`, the push-to-main full race
suite, and the pawl pre-push check. GitHub Actions remain available as manual,
PR, and release-tag backstops, but they are not the routine release authority.
The workflow is guided by skills plus the `ao` CLI; context flows through explicit
channels (`ao inject` / context packets through ports), not hook side effects.

If you want a bounded gate of your own (block a dangerous operation, bootstrap a
session, run a parity check), author it with the `hooks-authoring` skill. Do not
infer live gate behavior from tracked `.githooks/`; use `git config --get
core.hooksPath` and `scripts/install-pre-push-gate.sh` to inspect or refresh the
actual hook chain.

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
