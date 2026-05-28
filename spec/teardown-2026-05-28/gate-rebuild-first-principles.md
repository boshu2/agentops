# Gate Rebuild — First Principles (no strangler)

> Teardown epic ag-097. Directive (Bo, 2026-05-28): rebuild the gates from first principles, no strangler. Supersedes the "gates are healthy, leave them" read — the *aggregate* (67 jobs grown by accretion) is the slop, even though each is individually load-bearing.

## First principle
A CI gate layer answers exactly one question per push: **"is this change safe to merge?"** Everything else is structure. The current `validate.yml` answers it with **67 fine-grained jobs / 1,978 lines**, grown one-accretion-at-a-time. The *checks* (59 invoked `scripts/{check,validate}-*.sh|py`) are legitimate — they're the actual safety logic. The **orchestration** is the slop: 67 separate GitHub jobs, each with its own runner spin-up, `needs:` wiring, and a name registered in branch protection.

**Rebuild = re-orchestrate, not re-check.** Keep all 59 scripts. Collapse 67 jobs → ~10 purpose-grouped jobs, each running its family of scripts internally. Same coverage, ~85% fewer jobs, far less YAML.

## Target taxonomy (67 → 10)
Derived by clustering every job by what it *protects*:

| New job | Absorbs (old job count) | Tier |
|---|---|---|
| `correctness` | go-build, windows-smoke, cli-integration, bats-tests, smoke-test, json-flag-consistency (8) | T0/T1 |
| `lint` | shellcheck, markdownlint, skill-lint (3) | T0 |
| `security` | security-scan, security-toolchain-gate (2) | T1 |
| `skills-integrity` | skill-schema, skill-frontmatter, skill-dependency-check, skill-integrity, validate-skill-body-refs, validate-headless-runtime-skills, plugin-load-test (9) | T1 |
| `contracts-sync` | registry-check, validate-registry-drift, cli-docs-parity, validate-context-map-drift, validate-skill-domain-map-golden, validate-sku-catalog-drift, check-skill-catalog-drift, validate-bounded-contexts-drift, embedded-sync, validate-ci-policy-parity, contract-compatibility-gate, validate-contracts-structural-floor (14) | T1 |
| `codex-parity` | all 7 validate-codex-* (7) | T1 |
| `doctrine-proof` | validate-flywheel-proof, validate-flywheel-compounding-snapshot, validate-goals-validate, validate-three-gap-supergate, validate-wiring-closure, validate-corpus-freshness, validate-finding-registry, memrl-health, validate-sovereignty-proof-citations (9) | T2 |
| `spec-linkage` | executable-spec-link-integrity, validate-scenario-test-linkage, validate-agents-split, validate-docs-learning-references (4) | T1 |
| `eval` | agentops-eval-baseline-audit, eval-skill-delta, eval-workbench-verify, retrieval-quality (4) | T2 |
| `process-hygiene` | validate-pr-evidence-claims, validate-quarantine-empty, validate-test-count-noregress, doc-release-gate, file-manifest-overlap, check-test-staleness, doctor-check, swarm-evidence, lint-evidence-lines (9) | T1/I0 |
| `changes` / `summary` | plumbing — keep (3, push retired into merge) | — |

Each new job is a thin matrix/step list calling the existing scripts. Advisory checks (`*-advisory`, swarm-evidence, doctor-check) run inside their group but are `continue-on-error` so they report without blocking.

## Coverage-proof (the no-strangler safety net)
"No strangler" = no parallel parity period, so the design must *prove* no check is dropped, mechanically:

```
# Every script invoked by the OLD validate.yml must be invoked by the NEW one.
comm -23 \
  <(git show main:.github/workflows/validate.yml | grep -oE 'scripts/[a-z0-9./_-]+\.(sh|py)' | sort -u) \
  <(grep -oE 'scripts/[a-z0-9./_-]+\.(sh|py)' .github/workflows/validate.yml | sort -u)
# MUST be empty. Ship as a one-shot check in the rebuild PR body (Evidence:).
```
Plus: the rebuild PR runs the NEW `validate.yml` against itself — if a script is mis-wired, that PR's own CI goes red. Main stays protected until the PR is green. Blast radius = the PR, not main.

## The one genuinely-dangerous step: branch-protection required checks
**This is why "no strangler" on the sole gate is high-stakes and must be operator-coordinated.** GitHub branch protection lists *required status checks by job name*. Collapsing 67 → 10 renames the required checks. If the protection list isn't updated in lockstep with the merge:
- old required names never report on new PRs → **every PR blocks forever**, OR
- if removed too early → PRs merge without the new checks running.

**Cutover (atomic, admin-gated):**
1. Merge the rebuilt `validate.yml` PR (its own CI proves the 10 jobs pass).
2. *Immediately* update branch protection required-checks: remove the 64 old names, add the 10 new ones (`gh api repos/boshu2/agentops/branches/main/protection/required_status_checks`).
3. Verify a follow-up no-op PR is gated by exactly the 10 new checks.

Steps 2–3 are repo-admin actions with repo-wide blast radius → **operator-confirmed**, not autonomous.

## Execution (no strangler = one swap, not 67 migrations)
- **Cycle 1 (this):** design + epic + coverage-proof mechanism. ✓
- **Cycle 2:** write the new `validate.yml` (10 jobs) reusing all 59 scripts; coverage-proof empty; open PR; its own CI (running the new file) must go green.
- **Cycle 3 (operator-coordinated):** merge + branch-protection required-checks cutover + verify.

## What this does NOT touch
The 59 check scripts (the safety logic), the skills, the contracts. Only `.github/workflows/validate.yml` orchestration + branch-protection settings. That is the whole rebuild.
