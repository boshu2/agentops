# CI/CD Architecture

CI ensures code quality, security, and release integrity for the AgentOps repository. Every push and PR runs a 29-job validation pipeline. Releases are automated through GoReleaser with SBOM generation and SLSA provenance attestation.

## Workflow Map

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| Validate | `validate.yml` | Push to `main`, PRs to `main` | Primary quality gate (29 jobs) |
| Release Publisher | `release.yml` | Tag push (`v*`), manual dispatch | Build, publish, attest releases |
| Nightly | `nightly.yml` | Daily 6am UTC, manual | Full test suite + security + Athena knowledge cycle |
| Stale Issues | `stale.yml` | Weekly Monday 9am UTC | Auto-mark/close inactive issues and PRs |
| Label PRs | `labeler.yml` | PR opened/synced/reopened | Auto-label PRs by changed paths |

## validate.yml Architecture

The validate workflow runs **29 jobs** across 4 tiers of parallelism. Most jobs run independently with no `needs` dependencies, maximizing throughput.

### Job Dependency Graph

```text
                    ┌───────────────────────────────────────────────┐
                    │         25 independent parallel jobs          │
                    │                                               │
                    │  doc-release-gate    smoke-test               │
                    │  hook-preflight      validate-hooks-doc-parity│
                    │  validate-ci-policy-parity                    │
                    │  codex-runtime-sections                       │
                    │  embedded-sync       cli-docs-parity          │
                    │  shellcheck          markdownlint             │
                    │  security-scan       security-toolchain-gate  │
                    │  skill-integrity     skill-schema             │
                    │  skill-dependency-check                       │
                    │  contract-compatibility-gate                  │
                    │  memrl-health        plugin-load-test         │
                    │  go-build            cli-integration          │
                    │  file-manifest-overlap                        │
                    │  skill-lint          learning-coherence       │
                    │  bats-tests          check-test-staleness     │
                    └────────────────┬──────────────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │  go-build (must complete)   │
                    └──┬─────────────┬─────────┬──┘
                       │             │         │
                 ┌─────┴──┐  ┌──────┴───┐ ┌───┴──────────┐
                 │ doctor- │  │coverage- │ │json-flag-   │
                 │  check  │  │ ratchet  │ │consistency  │
                 └────┬────┘  └────┬─────┘ └──────┬──────┘
                      │            │              │
                    ┌─┴────────────┴──────────────┴─┐
                    │           summary             │
                    │  (needs: ALL 28 jobs)         │
                    │  if: always()                 │
                    └───────────────────────────────┘
```

### The `summary` Aggregator Pattern

The final `summary` job lists every other job in its `needs` array and runs with `if: always()`. It checks each job's result and fails if any **blocking** job did not succeed. This single aggregator is the branch protection target -- repository settings only need to require `summary` to pass, not every individual job.

Notably, `summary` excludes `security-toolchain-gate`, `doctor-check`, and `check-test-staleness` from its failure condition (these are soft gates), while still listing them in `needs` so they appear in the summary output.

## Blocking vs Soft Gates

### Soft Gates (continue-on-error: true)

These jobs run but their failure does **not** block merges:

| Job | Reason |
|-----|--------|
| `security-toolchain-gate` | External scanner tools may be unavailable; pattern scan (`security-scan`) is the blocking check |
| `doctor-check` | Reports stale CLI references; CI environment lacks some expected tools |
| `check-test-staleness` | Advisory -- flags tests that may need updating |

### Blocking Gates (all others)

Every other job is blocking. If any of these fail, `summary` exits non-zero and the PR/push is rejected.

## What Breaks CI

Consolidated checklist of rules enforced by the pipeline:

1. **No symlinks.** `plugin-load-test` rejects all symlinks in the repo. If you need the same file in multiple places, copy it.
2. **Skill counts must be synced.** Adding or removing a skill directory requires `scripts/sync-skill-counts.sh`. Forgetting this fails `doc-release-gate`.
3. **Every `references/*.md` must be linked in SKILL.md.** If a file exists in `skills/<name>/references/`, the skill's SKILL.md must contain a markdown link to it. Check with `heal.sh --strict`.
4. **Embedded hooks must stay in sync.** After editing `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`: run `cd cli && make sync-hooks`. Checked by `embedded-sync` and `go-build`.
5. **CLI docs must stay in sync.** After adding/changing CLI commands or flags: run `scripts/generate-cli-reference.sh`. Checked by `cli-docs-parity`.
6. **Contracts must be catalogued.** Files added to `docs/contracts/` need a link in `docs/INDEX.md`. Checked by `contract-compatibility-gate`.
7. **Go complexity budget.** New/modified functions must stay under cyclomatic complexity 25 (warn at 15). Checked by `go-build` via `check-go-complexity.sh`.
8. **No TODOs in SKILL.md.** Use `bd` issue tracking instead. Checked by `skill-lint`.
9. **No secrets in code.** `security-scan` greps for hardcoded passwords, API keys, and tokens in non-test files.
10. **No dangerous shell patterns.** `security-scan` rejects `rm -rf /`, `curl | sh`, etc. in scripts (with explicit exceptions for installer scripts).

## Local CI Guide

### scripts/ci-local-release.sh

The local CI gate mirrors the remote pipeline and runs in 5 phases:

| Phase | Description | Parallelism |
|-------|-------------|-------------|
| 1 | Required tool check (bash, git, jq, go, shellcheck, markdownlint) | Sequential |
| 2 | Quick independent checks: doc-release gate, manifest validation, hook preflight, parity checks, secret scans, MemRL health, etc. | Parallel (capped at half CPU cores, min 4) |
| 3 | Medium-weight checks: CLI docs parity, ShellCheck, markdownlint, smoke tests, integration tests, coverage floor | Parallel |
| 4 | Heavy checks: Go build + race tests, hook integration tests, SBOM generation, security toolchain gate | Parallel |
| 5 | CLI smoke tests: hook install smoke, `ao init --hooks` + RPI smoke, release smoke test | Parallel |

### Flags

```bash
scripts/ci-local-release.sh              # Full gate (~100s)
scripts/ci-local-release.sh --fast       # Skip race tests, security gate, SBOM, hook integration (~20s)
scripts/ci-local-release.sh --jobs 8     # Override parallel job cap
scripts/ci-local-release.sh --security-mode quick  # Quick security scan
```

In `--fast` mode, Phase 4 skips race tests, hook integration tests, SBOM generation, and the security gate. It still builds the binary and runs release validation.

### Minimum Checks Before Any Push

From CLAUDE.md -- the bare minimum before pushing:

```bash
bash skills/heal-skill/scripts/heal.sh --strict   # Skill integrity
./tests/docs/validate-doc-release.sh               # Skill counts + links
./scripts/check-contract-compatibility.sh           # Contract refs + JSON validity

# If you changed Go code:
cd cli && make build && make test

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

The `ao` CLI also installs Claude Code session hooks (`SessionStart`, `PreToolUse`, `PostToolUse`, `UserPromptSubmit`) that drive the AgentOps workflow nudges and knowledge injection. These are managed separately from git hooks.

## Security Gate

### scripts/security-gate.sh

Orchestrates the unified security scanning pipeline. Delegates to `scripts/toolchain-validate.sh` for actual scanner invocation.

```bash
scripts/security-gate.sh --mode quick          # Fast scan (CI default)
scripts/security-gate.sh --mode full           # Full suite (nightly, release)
scripts/security-gate.sh --mode full --json    # Machine-readable output
scripts/security-gate.sh --require-tools       # Fail if scanners missing
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

### scripts/security-toolchain-validate.sh

Validates that the security toolchain itself is correctly installed and functional. Used by `security-toolchain-gate` in CI.

## Release Workflow

### Pipeline

The release workflow (`release.yml`) triggers on version tags (`v*`) or manual dispatch:

1. **Pre-flight gates:** `doc-release-gate` (blocking) + `security-gate` (soft -- release proceeds if security-gate fails)
2. **Version resolution:** Extracts version from tag or manual input
3. **Validation:** Verifies tag exists, Homebrew token is valid
4. **Release notes:** Extracts from CHANGELOG.md via `scripts/extract-release-notes.sh`
5. **Publish:** GoReleaser builds cross-platform binaries (darwin/linux/windows, amd64/arm64)
6. **Post-publish:** Applies curated release notes, generates CycloneDX SBOM, runs full security gate, uploads SBOM + security report as release assets
7. **Attestation:** SLSA provenance via `actions/attest-build-provenance@v3` covering all tarballs, checksums, SBOM, and security report
8. **Homebrew:** GoReleaser auto-updates `boshu2/homebrew-agentops` tap

Manual dispatch is a rerun path, not the primary publish path for a new version. For a fresh release, push the tag. For post-tag fixes, use `scripts/retag-release.sh vX.Y.Z`. Do not start a manual dispatch in parallel with the tag-push workflow for the same tag.

### Release Cadence

- **Weekly release train (Fridays).** One published release per week max.
- **Security hotfixes** are the only exception -- ship same day as patch version.
- **No single-commit releases** for non-security fixes. Batch into the weekly train.
- Draft releases do not notify watchers and can be used freely for CI testing.
- Curated release notes are written to `.agents/releases/YYYY-MM-DD-v<version>-notes.md` before tagging.
- The release skill includes a 7-day cadence check in its pre-flight.

### Release Commands

```bash
# Normal release
git tag v2.X.0 && git push origin v2.X.0

# Retag (roll post-tag commits into existing release)
scripts/retag-release.sh v2.X.0

# Local validation before tagging
scripts/ci-local-release.sh
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
