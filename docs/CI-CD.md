# CI/CD Architecture

CI ensures code quality, security, and release integrity for the AgentOps repository. Every push and PR runs the validation pipeline. Releases are automated through GoReleaser with SBOM generation and SLSA provenance attestation.

## Workflow Map

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| Validate | `validate.yml` | Push to `main`, PRs to `main` | Primary quality gate |
| Release Publisher | `release.yml` | Tag push (`v*`), manual dispatch | Build, publish, attest releases |
| Nightly | `nightly.yml` | Daily 6am UTC, manual | Public proof harness: full test suite + retrieval + security + compile cycle + Dream report-shape validation over repo-visible artifacts |
| Nightly RPI Brief | `nightly-rpi-brief.yml` | Daily 11:30am UTC, manual | Builds a two-week Nightly evidence digest and updates the `$agentops:rpi --auto` prompt packet issue |
| Stale Issues | `stale.yml` | Weekly Monday 9am UTC | Auto-mark/close inactive issues and PRs |
| Label PRs | `labeler.yml` | PR opened/synced/reopened | Auto-label PRs by changed paths |

## Nightly vs Dream

AgentOps has two different overnight surfaces:

- **GitHub nightly** validates AgentOps the product. It runs in GitHub Actions against the checked-out repository and proves the CI, flywheel, and Dream report contracts still work.
- **`ao overnight`** is the private local compounding engine. It runs on the operator's machine against the real repo-local `.agents` corpus and writes the morning report defined in [Dream Report Contract](contracts/dream-report.md).

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

If you want scheduled private Dream runs, use `ao overnight setup` to inspect the
host, persist `dream.*` config, and generate host-specific `launchd`, `cron`, or
`systemd` assistance artifacts. The host scheduler still owns the actual wake
and scheduling semantics. For the cross-vendor private local chain that combines
Dream, Claude/Codex runners, RPI/evolve, and PR digest output, see
[`docs/runbooks/nightly-evolution.md`](runbooks/nightly-evolution.md).

## validate.yml Architecture

The validate workflow runs many focused jobs across 4 tiers of parallelism. Most jobs run independently with no `needs` dependencies, maximizing throughput.

### Job Dependency Graph

```text
                    ┌───────────────────────────────────────────────┐
                    │         27 independent parallel jobs          │
                    │                                               │
                    │  doc-release-gate    smoke-test               │
                    │  hook-preflight      validate-hooks-doc-parity│
                    │  validate-ci-policy-parity                    │
                    │  codex-runtime-sections                       │
                    │  embedded-sync       cli-docs-parity          │
                    │  agentops-contract-canaries                  │
                    │  agentops-eval-advisory                      │
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

The final `summary` job lists every other job in its `needs` array and runs with `if: always()`. It checks each job's result and fails if any **blocking** job did not succeed. This single aggregator is the branch protection target -- repository settings only need to require `summary` to pass, not every individual job.

Notably, `summary` excludes `agentops-eval-advisory`, `security-toolchain-gate`, `doctor-check`, `check-test-staleness`, and `swarm-evidence` from its failure condition (these are soft gates), while still listing them in `needs` so they appear in the summary output. `agentops-contract-canaries` is the blocking deterministic test gate for the stable public canary subset.

## Blocking vs Soft Gates

### Soft Gates (continue-on-error: true)

These jobs run but their failure does **not** block merges. Each carries an `(advisory)` suffix in its GitHub check name. Triage SLAs and escalation rules are codified in root `AGENTS.md` §Advisory Job Triage SLAs — keep that table and this one in sync (`scripts/validate-ci-policy-parity.sh`).

| Job | Triage SLA | Reason |
|-----|------------|--------|
| `agentops-eval-advisory` | 7d (release-blocking when stale) | The broad eval/canary corpus still runs on every PR, but brittle exact-string checks and baseline ratchets stay advisory until promoted |
| `security-toolchain-gate` | 14d | External scanner tools may be unavailable; pattern scan (`security-scan`) is the blocking check. Install steps use 3-attempt exponential-backoff retry to absorb transient trivy/hadolint network timeouts (item 40, soc-z7qq) |
| `doctor-check` | 30d | Reports stale CLI references; CI environment lacks some expected tools |
| `check-test-staleness` | none (info-only) | Advisory -- flags tests that may need updating (item 33) |
| `swarm-evidence` | none (info-only) | Advisory -- validates swarm evidence artifact shape; missing/malformed swarm artifacts are informational, not blocking (item 34) |

### Retrieval-bench ratchet (nightly)

The `retrieval-bench` job (nightly, see `.github/workflows/nightly.yml`) is a **warn-then-fail ratchet** with a deferred promotion. The job currently runs warn-only on every nightly. Promotion to blocking is a manual decision after the following observational window is documented green:

- **Promotion criterion:** `nightly_p_at_5 ≥ baseline_p_at_5` for **14 consecutive nightlies**.
- **Baseline source:** pinned fallback `baseline_p_at_5 = 0.30` in this section. Do not store the baseline under repo-root `.agents/`; that tree is local runtime state and is blocked by `scripts/check-no-tracked-agents.sh`.
- **Future durable source:** if the ratchet needs a machine-readable baseline, add it outside `.agents/` and update this section plus AGENTS.md in the same PR.
- **Observation window:** intentionally observational. The 14-consecutive-nightly counter is not yet wired into automation; track manually until a separate bead promotes the gate. This avoids accidental promotion during corpus quarantine windows (`f-2026-04-30-002`).

When the window closes green and the gate is promoted, update both this section and the AGENTS.md advisory table. Until then, retrieval-bench red is informational; do not block release on it.

Deferred CI hardening decisions for items 1, 7, 13, 14, 21, 22, 23, 24, 27, 30, and 39 are tracked in root `AGENTS.md` §DEFERRED CI Hardening, including the promotion triggers that would move each item back to FIX scope.

### Blocking Gates (all others)

Every other job is blocking. If any of these fail, `summary` exits non-zero and the PR/push is rejected.

## What Breaks CI

Consolidated checklist of rules enforced by the pipeline:

1. **No symlinks.** `plugin-load-test` rejects all symlinks in the repo. If you need the same file in multiple places, copy it.
2. **Skill counts must be synced.** Adding or removing a skill directory requires `scripts/sync-skill-counts.sh`. Forgetting this fails `doc-release-gate`.
3. **Every `references/*.md` must be linked in SKILL.md.** If a file exists in `skills/<name>/references/`, the skill's SKILL.md must contain a markdown link to it. Check with `heal.sh --strict`.
4. **Embedded hooks must stay in sync.** After editing `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`: run `cd cli && make sync-hooks`. Checked by `embedded-sync` and `go-build`.
5. **CLI docs must stay in sync.** After adding/changing CLI commands or flags: run `scripts/generate-cli-reference.sh`. Checked by `cli-docs-parity`.
6. **Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/documentation-index.md`. Checked by `contract-compatibility-gate`.
7. **Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15). Checked by `go-build` via `check-go-complexity.sh`.
8. **Windows installer smoke must pass.** PowerShell installers need to parse, temp installs need to work, and focused Windows-sensitive Go tests must pass on `windows-latest`. Checked by `windows-smoke`.
9. **No TODOs in SKILL.md.** Use `bd` issue tracking instead. Checked by `skill-lint`.
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
| 5 | CLI smoke tests: hook install smoke, `ao init --hooks` + RPI smoke, release smoke test | Parallel |
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

# If you changed hooks or lib/hook-helpers.sh:
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

## Git Hooks

Hooks are installed via `ao init --hooks` or `ao hooks install`. They live in `hooks/` (source of truth) and are embedded into the CLI binary via `cli/embedded/hooks/`.

### Pre-commit Hooks

| Hook | Purpose |
|------|---------|
| `go-complexity-precommit.sh` | Enforces cyclomatic complexity budget on staged Go files (warn 15, fail 25) |
| `pre-mortem-gate.sh` | Validates pre-mortem checklist completion before commit |
| `task-validation-gate.sh` | Validates task metadata and constraints |

### Pre-push Hooks

| Hook | Purpose |
|------|---------|
| `ratchet-advance.sh` | Checks that quality ratchet metrics have not regressed |

### Session Hooks

The `ao` CLI also installs Claude Code session hooks (`SessionStart`, `PreToolUse`, `PostToolUse`, `UserPromptSubmit`) that drive AgentOps workflow nudges, validation gates, and JIT context. These are managed separately from git hooks.

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

1. **Pre-flight gates:** `doc-release-gate` (blocking) + `security-gate` (soft -- release proceeds if security-gate fails)
2. **Version resolution:** Extracts version from tag or manual input
3. **Validation:** Verifies tag exists, Homebrew token is valid
4. **Release notes:** Extracts from CHANGELOG.md via `scripts/extract-release-notes.sh`
5. **Publish:** GoReleaser builds cross-platform binaries (darwin/linux/windows, amd64/arm64)
6. **Post-publish:** Applies curated release notes, generates CycloneDX SBOM, runs full security gate, writes advisory VIL readiness, uploads SBOM + security report + readiness as release assets
7. **Attestation:** SLSA provenance via `actions/attest-build-provenance@v4` covering all tarballs, checksums, SBOM, security report, and readiness
8. **Homebrew:** GoReleaser auto-updates `boshu2/homebrew-agentops` tap

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
