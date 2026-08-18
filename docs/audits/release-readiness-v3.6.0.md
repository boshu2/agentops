# Release readiness — v3.6.0

Date: 2026-08-17. Subject: `main` at `621dbb575` (release prep #1071 atop the
post-3.5.0 delta: the operations-layer alignment #1051 and its residuals #1054,
anti-ceremony enforcement #1050 and the RPI guard #1064, skill-contract
alignment #1062/#1066/#1069, the WIP convergence and evidence-boundary
hardening #1065, the eval program #1033 and the routing/tier-2/estate-ablation
corpora, Gas City Codex trust pre-seed #1031, and the between-releases
release-path smoke #1030). Tag `v3.6.0` is **not yet cut**; per the binding
process rule, this record exists BEFORE the tag.

## Verdict: PASS — official mode, score 9.0 / threshold 8

Local-ci run `20260818T012452Z` (`scripts/ci-local-release.sh
--release-version 3.6.0 --readiness-mode official --security-mode full`, real
HIL target, at the merged release SHA). First lap at this SHA, no reruns.

| Dimension | Status | Points |
|---|---|---|
| SIL (full race suite, 75.8% statement coverage) | pass | 2 |
| VIL (gates, regen, digital twin) | pass | 2 |
| HIL (real target) | pass | 2 |
| Artifacts (SBOM, manifest) | pass | 1.5 |
| Security (full mode) | pass | 1.5 |
| Evals | not_applicable this run | 0 |

72 checks, 0 failures. The working tree stayed clean across the run.

## HIL evidence (no waiver used)

Real target, strong workflow: `ao` built from the release SHA on Bo-Mac
(Darwin arm64) reported `ao version 3.6.0` (version_verified=true), then ran a
full `ao init` scaffold and `ao status` in a scratch repository.
Published-asset re-verification happens post-tag via Release Publisher, as with
3.3.0, 3.4.0, and 3.5.0.

- `release-readiness.json` sha256: `445c5617394da1cbc2e4aacc418c3fbc932c94e34f7d0cfd57f18863ae482367`
- `hil-evidence.json` sha256: `ffac307d9b939fc0edffb800791b5d7c13e20a762f4d7bc7c7cff8473d8ae29c`

## Security

Full mode, gate PASS: 0 critical, 0 high, 0 security-high, 3 medium, 0 low.
Nine tools ran (golangci-lint, gitleaks, semgrep, trivy, gosec, govulncheck,
hadolint, go-test, plus the toolchain driver). Four were skipped and one was
absent on this host (`pytest` not installed; `ruff`, `shellcheck`, and `radon`
skipped). The absent and skipped tools cover lanes that hosted CI runs
separately on the same SHA — the PR's `security` job passed there.

## Disclosure

- This is a minor release that removes public surfaces. `ao flywheel`, `plan`
  manifest mode (added in 3.5.0), and the seven-move `operating-loop` workflow
  are retired; `ao session handoff` moves its write path to `.agents/ao/handoff/`;
  `ao init` stops minting four consumer-free directories; and `codebase-recon`
  writes new packs under `.agents/scratch/`. Strict semver would call that a
  major. The operator chose minor, consistent with 3.4.0 removing the
  orchestration pack in a minor. All six removals and the write-path move are
  named in a `## Breaking Changes` section of the curated notes rather than
  being carried silently.
- Measured behavioral skill-probe coverage is an honest **0/12** under the v3
  evidence contract introduced this release. The wave-1 classifications
  produced earlier in this cycle are retained as `LEGACY-UNVERIFIED` rather
  than counted, because the probe harness did not isolate the skill corpus
  between control and treatment arms. Skill-efficacy claims in this release are
  directional, not proven.
- The estate-ablation aggregate counts are labeled legacy-unverified and
  non-promotable: hypotheses, not proof of causal effects, generalized token
  cost, or executor behavior.
- **A commitment carried since 3.3.0 is closed by this release.** The
  between-releases `goreleaser release --snapshot` CI smoke, deferred three
  times, ships as `.github/workflows/release-path-smoke.yml` with a shared
  negative witness.
- The 3.5.0 -> 3.6.0 bump initially missed the seventh version surface
  (`images/claude/verify.sh`), whose guard then rejected the correct version.
  `check_manifest_version_consistency` compares only the two Claude manifests
  to each other and structurally could not catch it. A regression test
  (`cli/cmd/ao/version_manifest_parity_test.go`) now binds the `version`
  fallback to every version-bearing surface, with its negative witness proven
  in both directions.
- `tests/docs/validate-goal-count.sh` exits 1 with no diagnostic because its
  pipe-table parser predates the current prose-plus-list `GOALS.md`. It is
  wired into `tests/run-all.sh`, which is not in CI or the release path, so it
  gates nothing and did not affect this verdict. Filed for separate repair.
