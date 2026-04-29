---
id: research-2026-04-29-ci-tests-hooks-noise-audit
type: research
date: 2026-04-29
topic: ci-tests-hooks-noise-audit
backend: codex-sub-agents
---
# Research: CI Tests and Hooks Noise Audit

## Executive Summary

AgentOps has broad validation coverage, but the policy surface has drifted in ways
that create noise and hide intent. The highest-priority finding is that
`retrieval-quality` exists as a workflow job but is absent from both
`summary.needs` and the AGENTS CI policy table, so it is outside the documented
blocking/non-blocking contract.

The hook runtime has good basics: timeouts, kill switches on curated critical
hooks, JSON-output schema tests, and several blocking hooks that exit with clear
reasons. The main weakness is that hook preflight is still curated by a static
list, while the manifest has 38 Claude handlers across 12 events. Local gates
also duplicate large portions of the remote pipeline without a single inventory
of check purpose, advisory/blocking status, and fast/full behavior.

## Applied Prior Knowledge

- `.agents/learnings/agentops-mac-nami-2026-03-05-testing-anti-patterns.md`:
  tests must prove correctness, not just field presence. This shaped the
  recommendation to replace grep/count-only gate tests with stubbed execution
  matrices and behavioral assertions.
- `.agents/findings/f-2026-04-27-002.md`: `scripts/pre-push-gate.sh` and
  `tests/scripts/pre-push-gate.bats` drift together when stubs are asymmetric.
  This shaped the recommendation for a generated or manifest-backed gate list.
- `.agents/findings/f-2026-04-27-004.md`: duplicated scanners need shared
  fixtures for every accepted syntax. This shaped the CI parity recommendation
  to add orphan-workflow-job fixtures.
- `.agents/findings/f-2026-04-25-001.md`: closeout must include worktree and
  validation disposition. This shaped the plan's final verification issue.

## Findings

### CI Policy Drift

- `.github/workflows/validate.yml` defines `retrieval-quality` at line 525, but
  the `summary.needs` list at line 893 omits it. That makes the job an orphan:
  it runs, but the summary aggregator neither reports nor classifies it.
- AGENTS documents four non-blocking jobs at lines 69-70 and lists CI jobs at
  lines 193-227, but it has no `retrieval-quality` row.
- `docs/CI-CD.md` is stale: it still says the validate workflow is a 30-job
  pipeline at lines 3, 9, 33, and says summary needs all 30 jobs at line 71.
  Current workflow evidence shows 32 non-summary jobs, 31 in `summary.needs`,
  plus `summary`.
- `scripts/validate-ci-policy-parity.sh` compares AGENTS jobs to
  `summary.needs` only: it extracts `summary.needs` at lines 58-78 and compares
  AGENTS rows to that set at lines 138-160. It does not parse all top-level jobs,
  so it cannot catch orphan jobs.
- `doctor-check` is marked `continue-on-error` at
  `.github/workflows/validate.yml:793`, but the step also runs
  `/tmp/ao-test doctor 2>/dev/null || true` at line 809. That prevents GitHub
  from recording the advisory job as failed when `ao doctor` actually fails.

### Hook Runtime Surface

- `hooks/hooks.json` has 12 events and 38 handlers. The hottest paths are
  `UserPromptSubmit` with 6 handlers at lines 52-86 and `PreToolUse` with
  14 handlers at lines 88-229.
- `hooks/codex-hooks.json` intentionally has a narrower runtime surface:
  5 events and 22 handlers.
- `PostToolUse` has an unscoped handler group at `hooks/hooks.json:251` with
  `go-complexity-precommit`, `go-vet-post-edit`, `research-loop-detector`, and
  `context-monitor`. Every post-tool event invokes those scripts and relies on
  script-level early exits.
- `scripts/validate-hook-preflight.sh` enforces kill switches and safety checks
  only for a curated `HOOK_FILES` array at lines 33-47. It separately checks
  manifest script existence at lines 253-260, but it does not apply the full
  kill-switch and unsafe-shell policy to every registered manifest script.
- `tests/hooks/test-orphan-hooks.sh` finds unregistered JSON-emitting hooks but
  exits 0 even with warnings at lines 89-101. That is currently advisory, not a
  hard contract.

### Local Gate Taxonomy

- `scripts/pre-push-gate.sh --fast` is changed-file selective. It classifies
  files into categories at lines 263-333 and routes checks through
  `needs_check()` at lines 335-356.
- Full `scripts/pre-push-gate.sh` runs without category pruning and includes
  headless runtime smoke and worktree disposition by default at lines 551 and
  813.
- `scripts/ci-local-release.sh --fast` is not changed-file selective. It skips
  Phase 4 heavy checks at lines 697-718, while still running broad Phase 2,
  Phase 3, and Phase 3b checks at lines 631-695.
- `docs/CI-CD.md` has stale local/remote parity claims: it says
  `validate-learning-coherence.sh` is not wired into the local gate at lines
  170-176, but `scripts/ci-local-release.sh` runs learning coherence at line 691
  and CI blocks on it at `.github/workflows/validate.yml:829` and 957.
- `tests/scripts/ci-local-release.bats` claims to exercise fast-path behavior at
  lines 4-6, but the visible tests mostly validate flags and grep for helpers at
  lines 92-121. That is structural coverage, not behavioral gate coverage.
- `scripts/pre-push-gate.sh` still comments about a `cmd/ao` coverage floor at
  line 13, but the current script does not run that check. CI enforces the floor
  at `.github/workflows/validate.yml:605`, and
  `tests/scripts/pre-push-gate.bats:304` still stubs a misspelled historical
  `check-cmdao-coverage-floor.sh`.

### Likely Noise Sources

- Advisory eval canaries: documented as non-blocking in `docs/TESTING.md:94-98`
  while baselines and variance policy stabilize.
- Security toolchain gate: installs latest external scanners and is soft in
  `docs/CI-CD.md:90-92`.
- Retrieval quality: explicitly warning-only in `.github/workflows/validate.yml`
  lines 546-552, but not classified in summary or docs.
- Go build warnings: agent hub hash drift and slow package warnings are emitted
  at `.github/workflows/validate.yml:618-624` and 627-655.
- Interactive hooks: `intent-echo`, `research-loop-detector`,
  `write-time-quality`, and `go-vet-post-edit` use heuristics likely to warn
  during broad audits or generated/shell-heavy edits.

## Coverage Validation

Explored:

- CI workflow: `.github/workflows/validate.yml`
- CI policy docs: `AGENTS.md`, `docs/CI-CD.md`, `docs/TESTING.md`
- CI policy parity: `scripts/validate-ci-policy-parity.sh`,
  `tests/scripts/test-ci-policy-parity.sh`
- Hook manifests and validators: `hooks/hooks.json`, `hooks/codex-hooks.json`,
  `scripts/validate-hook-preflight.sh`,
  `scripts/validate-hooks-doc-parity.sh`
- Hook tests: `tests/hooks/*.bats`, `tests/hooks/test-orphan-hooks.sh`
- Local gates: `scripts/pre-push-gate.sh`, `scripts/ci-local-release.sh`,
  `scripts/validate-go-fast.sh`, `tests/scripts/pre-push-gate.bats`,
  `tests/scripts/ci-local-release.bats`, `tests/run-all.sh`

Gaps:

- Did not run the full BATS suite or local release gate; this was discovery, not
  implementation.
- Did not inspect every individual hook body line-by-line. The plan should
  include a manifest-driven audit so coverage is systematic instead of sampled.

## Depth Validation

| Area | Depth | Notes |
|------|-------|-------|
| CI summary policy | 4 | Verified workflow jobs, summary failset, docs, parity script, and tests. |
| Hook runtime manifest | 3 | Verified handler counts, hot events, scoped vs unscoped handlers, and preflight scope. |
| Local gates | 3 | Verified fast/full semantics, duplication, stale docs, and brittle tests. |
| Individual hook heuristics | 2 | Sampled likely noisy hooks; needs systematic issue-level audit before changing thresholds. |
| Test assertion quality | 2 | Identified structural tests; implementation should broaden with fixtures. |

## Assumptions

- `retrieval-quality` should probably be an explicit soft gate, not a blocking
  merge gate, because the workflow already treats its threshold as warn-only.
- Hook noise should be reduced by better classification and trigger tests before
  removing hooks.
- A small machine-readable validation inventory is preferable to another
  narrative-only table, because drift is already coming from duplicated shell,
  YAML, docs, and BATS surfaces.

## Recommended Plan Shape

Use a standard complexity plan with five issues:

1. Close the immediate CI policy blind spots.
2. Add a validation-surface inventory that gives each CI/local/hook gate a
   purpose, owner surface, blocking policy, and fast/full behavior.
3. Expand hook preflight and hook parity coverage from curated lists to manifest
   registered handlers.
4. Reduce local gate drift and brittle tests by making pre-push/local-release
   tests exercise behavior.
5. Update docs and run targeted gates so the new policy is visible and enforced.

## Test Levels

- L0: shell parser/unit fixtures for parity scripts and hook validators.
- L1: BATS tests for pre-push/local-release fast/full behavior and hook stdin/
  output contracts.
- L2: integration-level local gates: `validate-ci-policy-parity.sh`,
  `validate-hooks-doc-parity.sh`, `validate-hook-preflight.sh`,
  `bats tests/hooks/*.bats tests/scripts/*.bats`.
- L3: optional full `scripts/ci-local-release.sh --fast` or full gate after
  implementation, depending on touched files and runtime/tool availability.
