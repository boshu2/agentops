---
id: plan-2026-04-29-ci-tests-hooks-noise-audit
type: plan
date: 2026-04-29
epic: agentops-dtg
research: .agents/research/2026-04-29-ci-tests-hooks-noise-audit.md
complexity: standard
---
# Plan: CI Tests and Hooks Noise Audit

## Context

Goal: audit AgentOps CI tests and hooks so noisy signals are reduced and every
test/hook has a stronger, explicit purpose.

Refined scope:

- Classify each CI/local/hook validation surface by purpose and blocking policy.
- Fix immediate CI policy blind spots.
- Strengthen hook and gate tests that currently rely on static grep/count checks.
- Preserve coverage while making advisory noise explicit.

Applied findings:
`agentops-mac-nami-2026-03-05-testing-anti-patterns.md`,
`f-2026-04-27-002`, `f-2026-04-27-004`, `f-2026-04-25-001`.

## Boundaries

- Do not remove validation gates in the first pass unless a replacement contract
  exists and passes targeted tests.
- Do not promote advisory checks to blocking without updating AGENTS,
  `docs/CI-CD.md`, workflow summary policy, and parity tests.
- Do not change hook warning thresholds without a fixture showing the old signal
  was noise for a legitimate workflow.
- Do not add symlinks.

## Baseline Audit

Commands already run during discovery:

```bash
ao search "ci tests hooks reduce noise validation goals"
ao lookup --query "ci tests hooks reduce noise validation goals" --limit 5
jq -r '.hooks | to_entries[] | [.key, ([.value[]?.hooks[]?] | length)] | @tsv' hooks/hooks.json
rg -n "^(  [a-zA-Z0-9_-]+:|    continue-on-error:|    timeout-minutes:|    needs:)" .github/workflows/validate.yml
find scripts -maxdepth 2 -type f -name '*.sh' | wc -l
find hooks -maxdepth 1 -type f -name '*.sh' | wc -l
find tests -type f \( -name '*.sh' -o -name '*.bats' -o -name '*.ps1' -o -name '*.py' \) | wc -l
bash scripts/validate-ci-policy-parity.sh
bash scripts/validate-hooks-doc-parity.sh
bash scripts/validate-hook-preflight.sh
bash tests/hooks/test-orphan-hooks.sh
```

Key results:

- CI policy parity currently passes: `31 jobs; 4 non-blocking`.
- Hook docs parity currently passes: `8 files checked, active hooks: 12`.
- Hook preflight currently passes, but uses a curated hook list.
- Orphan hook audit reports 34 registered hook scripts out of 48 and exits 0.
- Repository surface is broad: 144 shell scripts under `scripts/`, 48 hook
  scripts, and 151 shell/BATS/PowerShell/Python test files.
- Workflow has an orphan job: `retrieval-quality` is defined but not present in
  `summary.needs` or the AGENTS CI table.

Baseline audit status: WARN. Existing gates pass, but at least one CI job is
outside the policy contract they validate.

## Files To Modify

Likely write set:

- `.github/workflows/validate.yml`
- `AGENTS.md`
- `docs/CI-CD.md`
- `docs/TESTING.md`
- `docs/documentation-index.md` if a new contract doc is added
- `docs/contracts/validation-surface-inventory.md` or an equivalent contract
- `scripts/validate-ci-policy-parity.sh`
- `tests/scripts/test-ci-policy-parity.sh`
- `scripts/validate-hook-preflight.sh`
- `tests/hooks/test-orphan-hooks.sh`
- `tests/hooks/*.bats` as needed for fixtures
- `scripts/pre-push-gate.sh`
- `tests/scripts/pre-push-gate.bats`
- `tests/scripts/ci-local-release.bats`
- Optional inventory file such as `scripts/validation-surface-inventory.json`

## Implementation Issues

### agentops-dtg.1: Close CI Policy Blind Spots

Files:

- `.github/workflows/validate.yml`
- `AGENTS.md`
- `docs/CI-CD.md`
- `scripts/validate-ci-policy-parity.sh`
- `tests/scripts/test-ci-policy-parity.sh`

Work:

- Decide and encode `retrieval-quality` policy. The recommended default is
  explicit soft gate: present in `summary.needs`, absent from the fail condition,
  marked in docs/AGENTS as non-blocking, and preferably `continue-on-error: true`.
- Extend `validate-ci-policy-parity.sh` to parse all top-level workflow jobs and
  fail when a non-`summary` job is missing from `summary.needs` or AGENTS.
- Add a fixture in `tests/scripts/test-ci-policy-parity.sh` for an orphan
  workflow job.
- Let `doctor-check` report advisory failure through GitHub by removing the
  inner `|| true` if the job should mean "`ao doctor` runs without error."

Acceptance:

- `validate-ci-policy-parity` catches orphan jobs.
- AGENTS, docs, summary needs, and workflow fail condition agree.
- `doctor-check` still does not block summary but can visibly fail its own job.

### agentops-dtg.2: Add Validation Surface Inventory

Files:

- New inventory file, recommended:
  `scripts/validation-surface-inventory.json`
- New contract doc, recommended:
  `docs/contracts/validation-surface-inventory.md`
- `docs/documentation-index.md` if adding the contract doc
- One validator or parity script consuming the inventory

Work:

- Add a small machine-readable inventory for CI jobs, local gates, hook gates,
  and major test runners.
- Required fields: `id`, `surface`, `command`, `category`, `blocking_policy`,
  `fast_behavior`, `full_behavior`, `ci_job`, `docs_owner`.
- Wire at least one existing checker to the inventory so it prevents drift
  rather than serving as another stale table.
- Implement the inventory and its consuming check in the same issue. Do not land
  a standalone inventory file without a failing fixture proving it catches drift.

Acceptance:

- Inventory is checked by at least one automated test or validator.
- Inventory distinguishes pre-push `--fast` changed-file selection from
  `ci-local-release.sh --fast` heavy-check skipping.

### agentops-dtg.3: Expand Hook Preflight and Orphan-Hook Contracts

Files:

- `scripts/validate-hook-preflight.sh`
- `tests/hooks/test-orphan-hooks.sh`
- `tests/hooks/hook-output-schema.bats`
- `tests/hooks/hook-stdin-contracts.bats`
- Optional allowlist for unregistered non-JSON utility scripts

Work:

- Derive registered hook scripts from `hooks/hooks.json` and
  `hooks/codex-hooks.json`.
- Apply kill-switch, timeout, unsafe shell, and output-shape checks to every
  manifest-registered script or require an explicit allowlist.
- Make orphan JSON-emitting hook scripts blocking unless allowlisted.
- Add tests for manifest-derived coverage and one orphan JSON fixture.
- Add the expansion in report-first mode or with a narrow allowlist so existing
  unregistered non-JSON utility scripts do not become accidental blockers.

Acceptance:

- Hook preflight cannot miss newly registered hook scripts.
- Orphan JSON-emitting hooks are either blocked or explicitly allowed.
- Hot advisory hooks are classified instead of silently adding context noise.

### agentops-dtg.4: Replace Brittle Local-Gate Assertions

Files:

- `scripts/pre-push-gate.sh`
- `tests/scripts/pre-push-gate.bats`
- `tests/scripts/ci-local-release.bats`
- Optional inventory consumer from `agentops-dtg.2`

Work:

- Remove stale pre-push coverage-floor comments/stubs unless restoring the check.
- Add stubbed execution tests for `ci-local-release.sh --fast` that prove heavy
  checks are skipped and required broad checks still run.
- Replace gate-count assertions with inventory-backed assertions when possible.
- Keep pre-push BATS stubs synchronized with actual checks.

Acceptance:

- Local-gate tests fail when a required check is removed or a fast/full behavior
  changes without updating the inventory.
- No misspelled historical `check-cmdao-coverage-floor.sh` stub remains unless
  it is intentionally used.

### agentops-dtg.5: Sync Docs and Run Targeted Gates

Files:

- `AGENTS.md`
- `docs/CI-CD.md`
- `docs/TESTING.md`
- `docs/documentation-index.md` if contract docs changed

Work:

- Update CI job counts, dependency graph text, soft-gate list, local-only and
  skipped-remote-parity sections.
- Clarify `tests/run-all.sh` as a tier runner, not the full CI source of truth.
- Document distinct `--fast` semantics for pre-push and local release gates.
- Run targeted validation and record any remaining advisory-only noise.

Acceptance:

- Docs match implementation.
- Targeted gates pass.
- Any remaining warning-only surface has an explicit rationale.

## File Dependency Matrix

| Task | File | Access | Notes |
|------|------|--------|-------|
| agentops-dtg.1 | `.github/workflows/validate.yml` | write | Classify retrieval-quality and doctor-check semantics. |
| agentops-dtg.1 | `AGENTS.md` | write | CI table and non-blocking policy. |
| agentops-dtg.1 | `docs/CI-CD.md` | write | Immediate job-count and soft-gate docs. |
| agentops-dtg.1 | `scripts/validate-ci-policy-parity.sh` | write | Parse all top-level jobs. |
| agentops-dtg.1 | `tests/scripts/test-ci-policy-parity.sh` | write | Add orphan-job fixture. |
| agentops-dtg.2 | `scripts/validation-surface-inventory.json` | write | New inventory, if this path is selected. |
| agentops-dtg.2 | `docs/contracts/validation-surface-inventory.md` | write | Contract doc if inventory is contractual. |
| agentops-dtg.2 | `docs/documentation-index.md` | write | Required if adding a contract. |
| agentops-dtg.2 | `scripts/validate-ci-policy-parity.sh` | read/write | Optional inventory consumer. |
| agentops-dtg.3 | `scripts/validate-hook-preflight.sh` | write | Manifest-derived registered script checks. |
| agentops-dtg.3 | `tests/hooks/test-orphan-hooks.sh` | write | Fail/allowlist orphan JSON output. |
| agentops-dtg.3 | `tests/hooks/*.bats` | write | Add hook contract fixtures as needed. |
| agentops-dtg.4 | `scripts/pre-push-gate.sh` | write | Remove stale comments or wire inventory. |
| agentops-dtg.4 | `tests/scripts/pre-push-gate.bats` | write | Sync stubs with actual checks. |
| agentops-dtg.4 | `tests/scripts/ci-local-release.bats` | write | Add stubbed fast-path behavior tests. |
| agentops-dtg.5 | `AGENTS.md` | write | Final docs sync after implementation. |
| agentops-dtg.5 | `docs/CI-CD.md` | write | Final docs sync after implementation. |
| agentops-dtg.5 | `docs/TESTING.md` | write | Tier runner and gate guidance. |
| agentops-dtg.5 | `docs/documentation-index.md` | write | Final contract index sync if needed. |

Same-wave write conflicts:

- `AGENTS.md` and `docs/CI-CD.md` are written by agentops-dtg.1 and
  agentops-dtg.5, so agentops-dtg.5 must run after agentops-dtg.1.
- `scripts/validate-ci-policy-parity.sh` may be touched by agentops-dtg.1 and
  agentops-dtg.2. If the inventory is wired there, serialize agentops-dtg.2
  after the immediate orphan-job fix or merge those edits in one worker.

## Execution Order

Wave 1:

- agentops-dtg.1
- agentops-dtg.3

Wave 2:

- agentops-dtg.2 after agentops-dtg.1 if it consumes CI parity.
- agentops-dtg.4 after agentops-dtg.2 if inventory-backed assertions are used.

Wave 3:

- agentops-dtg.5 after all code/test behavior settles.

## Test Plan

L0:

```bash
bash -n scripts/validate-ci-policy-parity.sh
bash -n scripts/validate-hook-preflight.sh
bash -n scripts/pre-push-gate.sh
bash -n scripts/ci-local-release.sh
```

L1:

```bash
bash tests/scripts/test-ci-policy-parity.sh
bats tests/scripts/pre-push-gate.bats tests/scripts/ci-local-release.bats
bats tests/hooks/*.bats
```

L2:

```bash
bash scripts/validate-ci-policy-parity.sh
bash scripts/validate-hooks-doc-parity.sh
bash scripts/validate-hook-preflight.sh
bash tests/hooks/test-orphan-hooks.sh
bash tests/docs/validate-doc-release.sh
```

L3:

```bash
bash scripts/pre-push-gate.sh --fast --scope worktree
bash scripts/ci-local-release.sh --fast
```

L3 note: `ci-local-release.sh --fast` is final-wave validation. Earlier waves
should run issue-targeted gates first to avoid noisy unrelated failures while the
policy surface is still being normalized.

## Pre-Mortem Hardening

Quick pre-mortem verdict: WARN.

Required plan hardening applied:

- `agentops-dtg.2` must ship the inventory together with a consuming validator
  and a failing drift fixture. This prevents creating another stale table.
- `agentops-dtg.3` must use report-first behavior or an explicit allowlist while
  expanding hook preflight from curated scripts to manifest-derived scripts.
- Final full local release validation is deferred to `agentops-dtg.5`; earlier
  issues use targeted gates to keep unrelated noise from blocking the audit.

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001 Mechanical enforcement | PASS | Plan adds parity checks and behavior fixtures rather than docs-only cleanup. |
| PR-002 External validation | PASS | Uses workflow, manifest, and BATS fixtures to validate shell behavior. |
| PR-003 Feedback loops | PASS | Advisory/soft gates remain visible through summary/docs classification. |
| PR-004 Separation of concerns | PASS | Immediate CI policy, hook contracts, local gate tests, and docs sync are separate tasks. |
| PR-005 Process gates | PASS | Pre-mortem plus targeted L0-L3 gates are specified. |
| PR-006 Cross-layer consistency | PASS | AGENTS, docs, workflow, shell validators, and tests are all in scope. |
| PR-007 Phased rollout | PASS | Wave 1 fixes blind spots; Wave 2 adds inventory/tests; Wave 3 syncs docs and gates. |

## Symbol Verification

Verified symbols/paths during research:

- `summary.needs` and fail condition in `.github/workflows/validate.yml`
- `extract_summary_needs` and `extract_summary_failset` in
  `scripts/validate-ci-policy-parity.sh`
- `HOOK_FILES` in `scripts/validate-hook-preflight.sh`
- `needs_check` in `scripts/pre-push-gate.sh`
- `run_step_bg` and Phase 2-5 calls in `scripts/ci-local-release.sh`
- `collect_target_files`, `collect_test_names_from_file`, and Go test run
  logic in `scripts/validate-go-fast.sh`

Stale symbol warnings: none.

## Next Steps

1. Run quick pre-mortem on this plan.
2. If pre-mortem is PASS/WARN, use `/crank agentops-dtg` or implement
   `agentops-dtg.1` first as the smallest high-signal slice.
