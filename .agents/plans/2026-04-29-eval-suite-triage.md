---
id: plan-2026-04-29-eval-suite-triage
type: plan
date: 2026-04-29
source: "[[.agents/research/2026-04-29-eval-suite-triage]]"
epic: ag-v29
---

# Plan: Eval Suite Triage

## Context

AgentOps now has a real eval subsystem: schemas, `ao eval` commands, public canary suites, baseline records, scorecards, runtime adapter scaffolding, CI advisory wiring, and pre-push integration. The problem is signal ambiguity. A passing suite can mean "a contract string is still present," "a lower-level test suite passed," "a baseline did not regress," or "a behavior fixture succeeded." Those are all useful, but they should not be reported as the same kind of evidence.

This plan does not replace the suite. It classifies evidence, tightens baseline governance, refactors brittle static checks, splits fast/full lanes, adds behavioral fixtures where scorecards currently only test labels, and keeps live runtime/holdout checks opt-in until calibrated.

Applied findings:

- `f-2026-04-27-004` — Duplicated validators drift. The plan requires paired fixtures and deduplication before deleting mirrored checks.
- `f-2026-04-14-002` — Closure and baseline proof must cite durable artifacts. The plan keeps run/baseline artifacts explicit and audit-ready.
- `f-2026-04-14-001` — `ao eval` and pre-push command changes need paired command tests.
- `f-2026-04-25-001` — Green product gates do not replace final disposition and closure proof.

## Files to Modify

| File | Change |
|------|--------|
| `docs/contracts/eval-environment.md` | Clarify evidence kinds, fast/full/live lanes, baseline policy, and kill/refactor criteria. |
| `schemas/eval-suite.v1.schema.json` | Add evidence-kind metadata or equivalent typed tags for suites/cases. |
| `cli/internal/eval/types.go` | Add Go types/constants for evidence kinds if schema adds first-class fields. |
| `cli/internal/eval/coverage.go` | Report evidence-kind coverage and low-signal/uncategorized cases. |
| `cli/internal/eval/coverage_test.go` | Test evidence-kind coverage, missing categories, and uncategorized reporting. |
| `cli/internal/eval/baseline.go` | Add or reuse baseline audit helpers if policy checks live in Go. |
| `cli/cmd/ao/eval.go` | Add baseline audit or suite-lane command flags if needed. |
| `cli/cmd/ao/eval_test.go` | Pair command surface changes with JSON tests. |
| `scripts/eval-agentops.sh` | Split fast/full lanes and make baseline warnings policy-aware. |
| `tests/scripts/` | Add shell/BATS tests for lane selection and baseline audit behavior. |
| `evals/agentops-core/*.json` | Classify suites/cases, refactor or delete low-signal checks, and update baseline policies. |
| `.agents/evals/baselines/*.baseline.json` | Add, refresh, remove, or relocate baselines according to policy. |
| `docs/TESTING.md` | Explain the new eval tiers and when to run each lane. |
| `.github/workflows/validate.yml` | Keep normal PR live runtime checks non-blocking; adjust deterministic lane only after script changes. |

## Boundaries

**Always:** Preserve deterministic public canaries; keep normal PR evals offline; make evidence kind mechanically auditable; require replacement proof before deleting checks; preserve JSON output contracts; keep live runtime and private holdout checks advisory until variance is known.

**Ask First:** Before moving public baselines out of `.agents/evals/baselines/`; before making any live/headless runtime suite blocking; before deleting a suite with no replacement proof.

**Never:** Claim public canaries prove live agent quality; make exact prose matching the primary behavioral score; require Claude/Codex auth in normal PR validation; delete a low-signal check without naming the stronger gate that replaces it.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| Public canary suites | `find evals/agentops-core -maxdepth 1 -name '*.json' -type f | wc -l` | 56 |
| Promoted baselines | `find .agents/evals/baselines -maxdepth 1 -name '*.baseline.json' -type f | wc -l` | 54 |
| Coverage inventory | `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval coverage --root ../evals/agentops-core --json` | 56 suites, 281 cases, 219 critical cases |
| Expectation mix | `jq -r '.cases[]?.expectations[]?.type' evals/agentops-core/*.json | sort | uniq -c | sort -nr` | 1083 `artifact_contains`; 211 `stdout_contains`; 137 `exit_code`; 1 `schema_valid` |
| Case mix | `jq -r '.cases[]? | [.kind, (.runtime // "")] | @tsv' evals/agentops-core/*.json | sort | uniq -c | sort -nr` | 143 artifact-only cases; 137 shell command cases; 2 mock artifact cases |
| Baseline policy modes | `jq -r '[.id,.baseline_policy.mode] | @tsv' evals/agentops-core/*.json | sort` | All 56 suites declare `none` despite 54 baseline files |
| Existing CI eval status | `rg -n "agentops-eval-advisory|eval-agentops" .github/workflows/validate.yml scripts/pre-push-gate.sh` | CI advisory job and pre-push conditional eval lane exist |

## Implementation

### 1. Classify Evidence Kinds And Coverage

Add a first-class evidence-kind taxonomy to the eval contract and schema. Recommended values:

- `contract_canary`: static or schema-backed tripwire preserving product contracts.
- `gate_wrapper`: command wrapper around an existing lower-level test, linter, or validator.
- `behavior_fixture`: deterministic fixture that exercises product behavior directly.
- `baseline_regression`: candidate-vs-baseline comparison with promoted baseline policy.
- `scorecard_fixture`: category-level scorecard pass/fail/regression fixture.
- `live_runtime`: Claude/Codex execution or headless runtime proof.
- `holdout`: private or public scenario/holdout proof.

Coverage should report these kinds alongside domain, dimension, and runtime. Missing kind metadata should fail the eval meta-suite after a short migration window.

### 2. Align Baseline Policy With Artifacts

Define baseline dispositions:

- `none`: no baseline expected and no missing-baseline warning.
- `compare`: a baseline file is expected and missing is a warning or failure according to `blocking_gate`.
- `promotable`: baseline can be created but is not required yet.

Add a mechanical baseline audit that finds:

- suite declares `none` but has a baseline file;
- suite declares `compare` but baseline is missing;
- baseline exists for no current suite;
- baseline run points at a stale suite hash;
- baseline promotion lacks rationale.

### 3. Refactor Or Kill Brittle Artifact-Only Canaries

For artifact-only cases with many exact string expectations:

- keep as `contract_canary` only when the strings are public contract surface;
- convert to shell or Go invariant checks when the test really wants parser behavior, command behavior, or policy behavior;
- merge with generated parity gates when the check only repeats `scripts/generate-cli-reference.sh --check`, hook/doc parity, or docs release gates;
- delete only after the replacement command is named in the suite or plan proof.

Initial high-priority targets:

- scorecard category suites that only check labels in source;
- generated CLI docs string sweeps already covered by CLI docs parity;
- repeated source string checks where unit tests already exercise the same behavior.

### 4. Split Fast And Full Deterministic Lanes

Make `scripts/eval-agentops.sh --fast` mean "small, critical, changed-surface smoke." Add a full deterministic lane for the current broad corpus.

Candidate command shape:

```bash
scripts/eval-agentops.sh --fast
scripts/eval-agentops.sh --full-deterministic
scripts/eval-agentops.sh --suite evals/agentops-core/<suite>.json
```

Avoid repeated `ao` builds where multiple command cases can share one built binary. Keep CI `agentops-eval-advisory` on the full deterministic lane only if wall time stays acceptable; otherwise keep PR CI on fast and run full deterministic nightly/release.

### 5. Add Behavioral Scorecard Fixtures

Replace label-only scorecard canaries with fixture run records:

- known-good RPI run with complete artifacts, correct phase order, preserved objective, validation separation, scenario satisfaction, and runtime safety;
- negative RPI run missing one category at a time;
- known-good skill-change run with structural, trigger, runtime, scenario, and stocktake categories;
- negative skill-change run that regresses one category at a time.

These fixtures should drive `ao eval scorecard` and assert both pass and regression/failure outcomes.

### 6. Add Opt-In Live Runtime And Holdout Lane

Expose a safe lane for live/headless runtime and private holdout suites:

- explicit selection only;
- clean skip when auth/runtime/model/network is unavailable;
- isolated `HOME` and `CODEX_HOME`;
- scrubbed runtime environment;
- transcript/scorecard artifacts;
- advisory/nightly/release default, not PR blocking.

## Tests

**`cli/internal/eval/coverage_test.go`**
- `TestBuildCoverageReportEvidenceKinds`
- `TestBuildCoverageReportMissingEvidenceKind`
- `TestBuildCoverageReportUncategorizedSuites`

**`cli/internal/eval/baseline_test.go`**
- `TestAuditBaselinePolicy_MissingCompareBaseline`
- `TestAuditBaselinePolicy_UnexpectedBaselineForNone`
- `TestAuditBaselinePolicy_OrphanBaseline`
- `TestAuditBaselinePolicy_StaleSuiteHash`

**`cli/cmd/ao/eval_test.go`**
- JSON tests for any new `eval coverage` or baseline audit flags.
- Command registration tests if new subcommands are added.

**`tests/scripts/`**
- `eval-agentops` fast/full lane selection.
- Missing baseline warnings obey suite policy.
- Advisory/live lane skips unavailable runtime without failing PR-style mode.

**`evals/agentops-core/fixtures/`**
- Good and regressed scorecard run records.
- Fixture suites that prove scorecard failure paths, not only category labels.

## Conformance Checks

| Issue | Check Type | Check |
|-------|------------|-------|
| ag-v29.1 | command | `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./internal/eval -run 'TestBuildCoverageReport'` |
| ag-v29.1 | command | `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval coverage --root ../evals/agentops-core --json | jq -e '.evidence_kinds'` |
| ag-v29.2 | command | `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval -run 'Baseline|Eval.*Baseline'` |
| ag-v29.2 | command | `scripts/eval-agentops.sh --fast` emits no unintended missing-baseline warnings |
| ag-v29.3 | command | `jq` audit reports zero unclassified artifact-only cases outside an allowlist |
| ag-v29.3 | tests | Converted invariant tests pass for each refactored suite |
| ag-v29.4 | command | `bash tests/scripts/test-eval-agentops-lanes.sh` |
| ag-v29.5 | command | `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./internal/eval ./cmd/ao -run 'Scorecard'` |
| ag-v29.6 | command | live/headless lane returns skipped/advisory when runtime unavailable |

## Verification

1. `python3 -m json.tool schemas/eval-suite.v1.schema.json >/dev/null`
2. `find evals/agentops-core -maxdepth 1 -name '*.json' -print0 | xargs -0 -n1 python3 -m json.tool >/dev/null`
3. `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval`
4. `bash tests/scripts/test-eval-agentops-lanes.sh`
5. `scripts/eval-agentops.sh --fast`
6. `scripts/eval-agentops.sh --full-deterministic`
7. `scripts/pre-push-gate.sh --fast`

## Issues

### Issue ag-v29.1: Classify eval evidence kinds and coverage taxonomy

**Dependencies:** None

**Acceptance:** Coverage output reports evidence kinds; suites can be mechanically audited for kind coverage; docs explain that contract canaries are not behavioral evals.

**Description:** Add the taxonomy before rewriting suites so keep/refactor/kill decisions are mechanical.

### Issue ag-v29.2: Align eval baseline policy with promoted artifacts

**Dependencies:** ag-v29.1

**Acceptance:** A mechanical command reports zero baseline policy mismatches; new suites have explicit baseline disposition; `eval-agentops` warnings are intentional and actionable.

**Description:** Make suite metadata match the 54 promoted baseline files and future baseline policy.

### Issue ag-v29.3: Refactor brittle artifact-only canaries into invariant checks

**Dependencies:** ag-v29.1

**Acceptance:** Artifact-only case count is reduced or justified by evidence kind; converted checks have focused fixtures/tests; removed checks are covered by named existing gates.

**Description:** Convert high-value static sweeps into executable invariants and kill/merge low-signal duplicates.

### Issue ag-v29.4: Split fast/full eval lanes and deduplicate gate wrappers

**Dependencies:** ag-v29.1

**Acceptance:** `scripts/eval-agentops.sh` exposes documented fast/full suite selection; pre-push and CI use the intended lane; duplicate command wrappers are merged or explicitly justified.

**Description:** Keep a cheap local signal and move broad deterministic proof to the right lane.

### Issue ag-v29.5: Add behavioral fixture scorecards for RPI and skill changes

**Dependencies:** ag-v29.1, ag-v29.2

**Acceptance:** Scorecard suites fail on fixture regressions and pass on known-good fixtures; tests assert category-level failures and baseline deltas instead of only checking labels exist in source.

**Description:** Turn scorecards from category-existence canaries into behavior/regression fixtures.

### Issue ag-v29.6: Add live-runtime and holdout lanes without blocking PRs

**Dependencies:** ag-v29.1, ag-v29.4, ag-v29.5

**Acceptance:** Live/headless suites can be selected explicitly, skip cleanly when unavailable, record runtime metadata, and are excluded from normal PR blocking gates.

**Description:** Wire the optional live/holdout proof path without destabilizing normal PR validation.

## Execution Order

**Wave 1:** ag-v29.1

**Wave 2:** ag-v29.2, ag-v29.3, ag-v29.4

**Wave 3:** ag-v29.5

**Wave 4:** ag-v29.6

## File Dependency Matrix

| File/Area | Owner Issue | Downstream |
|-----------|-------------|------------|
| `docs/contracts/eval-environment.md` | ag-v29.1 | all issues |
| `schemas/eval-suite.v1.schema.json` | ag-v29.1 | ag-v29.2, ag-v29.3, ag-v29.4 |
| `cli/internal/eval/types.go` | ag-v29.1 | ag-v29.2, ag-v29.3, ag-v29.4 |
| `cli/internal/eval/coverage.go` | ag-v29.1 | ag-v29.3, ag-v29.4 |
| `cli/internal/eval/baseline.go` | ag-v29.2 | ag-v29.5 |
| `scripts/eval-agentops.sh` | ag-v29.2, ag-v29.4 | ag-v29.6 |
| `evals/agentops-core/*.json` | ag-v29.3 | ag-v29.5 |
| `.agents/evals/baselines/*.baseline.json` | ag-v29.2 | ag-v29.5 |
| `cli/internal/eval/scorecard.go` | ag-v29.5 | ag-v29.6 |
| `.github/workflows/validate.yml` | ag-v29.4, ag-v29.6 | release validation |

## File-Conflict Matrix

| File/Area | Conflict Risk | Resolution |
|-----------|---------------|------------|
| `schemas/eval-suite.v1.schema.json` | High | ag-v29.1 owns schema changes; other issues wait. |
| `cli/internal/eval/types.go` | High | ag-v29.1 owns enum/type additions; later issues only consume them. |
| `scripts/eval-agentops.sh` | Medium | Serialize ag-v29.2 baseline warning work before ag-v29.4 lane split. |
| `evals/agentops-core/*.json` | High | Each worker owns a named suite subset and updates a manifest of touched suites. |
| `.agents/evals/baselines/*.baseline.json` | Medium | Only ag-v29.2 and ag-v29.5 update baselines; require explicit promotion rationale. |

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| Mechanical Enforcement | PASS | Every issue has a command or machine-readable audit. |
| External Validation | PASS | Eval changes run through `ao eval`, Go tests, script tests, and pre-push. |
| Feedback Loops | PASS | Baseline policy and scorecards feed future comparison. |
| Separation Over Layering | PASS | Taxonomy precedes suite rewrite; live runtime remains separate from deterministic PR gates. |
| Process Gates First | PASS | Schema/coverage/baseline audit land before deleting or moving suites. |
| Cross-Layer Consistency | PASS | Docs, schema, Go types, CLI, scripts, suites, and CI are all listed. |
| Phased Rollout | PASS | The plan keeps live/holdout checks advisory until calibrated. |

Unchecked rules: 0

## Next Steps

Run `$pre-mortem .agents/plans/2026-04-29-eval-suite-triage.md --quick`, then hand `ag-v29.1` to `$implement` or run `$crank` on the epic after approval.
