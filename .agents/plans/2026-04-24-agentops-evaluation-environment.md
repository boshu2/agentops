---
id: plan-2026-04-24-agentops-evaluation-environment
type: plan
date: 2026-04-24
source: "[[.agents/research/2026-04-24-agentops-evaluation-environment]]"
---

# Plan: AgentOps Evaluation Environment

## Context

AgentOps has structural validation for skills, hooks, Codex artifacts, RPI contracts, and retrieval quality, but no unified way to score whether a skill, hook, CLI, runtime, or model change made outcomes better or worse. The plan creates a repo-native eval subsystem that treats AgentOps itself as the product under test.

Applied findings:
- `f-2026-04-14-001` — Every new `cli/cmd/ao` eval command must ship with direct command tests in the same slice.
- `f-2026-04-14-002` — Baselines and scorecards must cite durable checked-in eval packs or committed run packets, not transient local-only seed files.
- `learning-2026-04-14-scrub-rpi-runtime-from-raw-validation` — Runtime eval runners must scrub host RPI/runtime variables.
- `2026-04-07-v2.35.0-release-postmortem` — Split validation into local, CI, and live/nightly layers because no single environment catches every failure surface.

## Files to Modify

| File | Change |
|------|--------|
| `docs/contracts/eval-environment.md` | **NEW** — eval architecture, suite tiers, baseline lifecycle, score dimensions. |
| `docs/INDEX.md` | Add contract entry. |
| `schemas/eval-suite.v1.schema.json` | **NEW** — checked-in suite and case manifest schema. |
| `schemas/eval-run.v1.schema.json` | **NEW** — run result and scorecard schema. |
| `evals/agentops-core/` | **NEW** — public canary eval packs for skills, hooks, CLI, and RPI. |
| `cli/cmd/ao/eval*.go` | **NEW** — `ao eval run`, `ao eval compare`, `ao eval baseline`, and JSON output. |
| `cli/internal/eval/` | **NEW** — suite loader, runner, scorer, baseline comparator, runtime adapter interfaces. |
| `cli/cmd/ao/eval*_test.go` | **NEW** — command tests paired with production command files. |
| `cli/internal/eval/*_test.go` | **NEW** — loader, scorer, baseline, env-isolation, and fixture tests. |
| `cli/docs/COMMANDS.md` | Regenerate after adding commands. |
| `skills/scenario/SKILL.md` | Align documented scenario authoring with actual CLI behavior. |
| `cli/cmd/ao/scenario_add.go` | **NEW** — implement `ao scenario add` or remove documented command from skill if deferred. |
| `scripts/eval-agentops.sh` | **NEW** — stable local entrypoint for deterministic eval suites. |
| `.github/workflows/validate.yml` | Add non-blocking/advisory eval job first, then ratchet later. |
| `docs/TESTING.md` | Document eval tiers, when to run them, and baseline promotion rules. |

## Boundaries

**Always:** Keep deterministic evals runnable offline; isolate HOME/CODEX_HOME for live runtime runs; scrub `AGENTOPS_RPI_RUNTIME*`; emit JSON scorecards; keep private holdouts separate from public canaries; preserve existing validation scripts and wrap/reuse them instead of deleting them.

**Ask First:** Before making live model evals blocking in PR CI; before committing private holdout scenarios; before promoting a new baseline after a known behavioral regression.

**Never:** Gate normal PRs on nondeterministic live model availability; score exact prose when a mechanical artifact or command result can be scored; let implementing agents read private holdouts; treat 95% live pass rate as equivalent to 100% canary pass rate.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| Shared skill count | `find skills -mindepth 1 -maxdepth 1 -type d | wc -l` | 69 |
| Codex skill count | `find skills-codex -mindepth 1 -maxdepth 1 -type d | wc -l` | 69 |
| Claude hook handlers | `jq '[.hooks | to_entries[] | .value[] | .hooks[]] | length' hooks/hooks.json` | 28 |
| Codex hook handlers | `jq '[.hooks | to_entries[] | .value[] | .hooks[]] | length' hooks/codex-hooks.json` | 7 |
| Eval-adjacent files | `rg --files tests scripts cli/cmd/ao cli/internal/rpi skills | rg '(rpi|scenario|headless|codex|retrieval|runtime|skill).*(_test\\.go|\\.sh|\\.bats|\\.md|\\.json)$' | wc -l` | 653 |
| Scenario subcommands | `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao scenario --help` | `init`, `list`, `validate`; no `add` |
| Retrieval baseline smoke | `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao retrieval-bench --json` | 6 queries, `avg_precision_at_k=1.0`, `avg_mrr=1.0` |
| Code map availability | `test -d docs/code-map && echo present || echo absent` | absent |
| Tracker health | `bd ready --json`; `bd list --type epic --status open --json` | both crashed inside Dolt storage; discovery continued in tasklist mode |

## Implementation

### 1. Define Eval Contracts And Storage

Create `docs/contracts/eval-environment.md` and schemas for:

- `eval_suite`: suite id, domain (`skill|hook|cli|rpi|runtime|retrieval`), tier (`deterministic|headless|live|release`), cases, fixtures, allowed runtimes, scoring dimensions, baseline policy.
- `eval_run`: run id, git SHA, candidate ref, baseline ref, runtime, model, suite id, case results, aggregate score, verdict, artifacts, transcript paths, environment scrub record.
- `eval_scorecard`: dimension scores for correctness, process adherence, artifact quality, runtime compatibility, efficiency, safety, and learning closure.

Use `evals/` for committed canary packs and `.agents/evals/` for generated local outputs. Public canaries must be stable and reviewable. Private holdouts can stay outside git but must use the same schema.

### 2. Build `ao eval` CLI Core

Add `ao eval` with:

- `ao eval run --suite <id|path> --runtime <static|mock|claude|codex> --model <name> --out <dir> --json`
- `ao eval compare --baseline <run.json> --candidate <run.json> --json`
- `ao eval baseline capture --suite <id> --out <dir>`
- `ao eval baseline promote --run <run.json> --baseline-dir <dir>`

The first implementation should run deterministic suites and mocked runtime adapters. Live Claude/Codex adapters can share the same interface but start as opt-in.

Key helpers:

```go
type Suite struct {
    ID string
    Domain string
    Tier string
    Cases []Case
}

type Runner interface {
    Run(ctx context.Context, suite Suite, opts RunOptions) (RunResult, error)
}

type Score struct {
    Correctness float64
    ProcessAdherence float64
    ArtifactQuality float64
    RuntimeCompatibility float64
    Efficiency float64
    Safety float64
    LearningClosure float64
}
```

### 3. Seed Public Canary Suites

Create `evals/agentops-core/` with initial suites:

- `cli-smoke`: command help, JSON output, `ao scenario`, `ao retrieval-bench`, `ao rpi verify`.
- `hook-io`: fixture stdin events for key hooks, expected exit codes and JSON decisions.
- `skill-contracts`: selected skill invocation contracts for `rpi`, `discovery`, `validation`, `scenario`, `heal-skill`, and their Codex counterparts.
- `runtime-inventory`: wrap existing headless inventory checks as an eval case with structured results.
- `rpi-canary`: fixture repo plus goal, expected artifacts, phase order, pre-mortem/vibe verdict extraction, scenario satisfaction score, and no-host-env-leak checks.

Fix the scenario command mismatch in the same wave: either implement `ao scenario add` according to `skills/scenario/SKILL.md`, or patch the skill to stop documenting a nonexistent command. Prefer implementing the command because eval authors need a durable scenario-authoring path.

### 4. Add Live Runtime Adapters

Add Claude and Codex adapters using `docs/contracts/headless-invocation-standards.md` and `scripts/validate-headless-runtime-skills.sh` as precedent. Requirements:

- Isolated HOME and CODEX_HOME.
- `env -u AGENTOPS_RPI_RUNTIME` and scrub all `AGENTOPS_RPI_RUNTIME*`.
- Timeout, budget, and retry fields in the run result.
- Transcript/stream capture in `.agents/evals/runs/<run-id>/`.
- A strict JSON result extraction path with fallback diagnostics.
- Runtime matrix metadata: runtime, version, model, profile, auth availability, skipped reason.

Live suites should be advisory by default and suitable for nightly/release/model-upgrade validation, not required for every PR.

### 5. Score RPI And Skill Changes Against Baselines

Define scoring for RPI canaries:

- Required artifacts exist: execution packet, plan, pre-mortem report, phase summaries, evaluator artifacts, scenario results when scenarios exist.
- Required phase order observed.
- No objective narrowing from lifecycle objective to one child slice.
- No Codex wrapper-command orchestration when `$skill` chaining is required.
- Tests/commands in the fixture repo pass.
- Validator phase is separate from implementation phase when live runtime data exists.
- Scorecard does not rely on self-declared success alone.

For skill changes, compare:

- Structural pass/fail from existing validators.
- Trigger clarity via explicit skill request tests.
- Runtime inventory and live smoke when available.
- Scenario satisfaction and RPI canary score deltas.
- Stocktake dimensions from `skills/heal-skill/references/skill-stocktake.md`: actionability, scope fit, uniqueness, currency, trigger clarity.

Use candidate-vs-baseline comparison rather than a single absolute grade. A change is "better" only when it improves or preserves critical canary scores without regressing runtime compatibility, safety, or artifact quality.

### 6. Wire Gates, Reports, And Ratchets

Add `scripts/eval-agentops.sh --fast` for deterministic local use and a non-blocking CI job first. Once baselines stabilize:

- PR gate: deterministic suites only.
- Nightly: live Claude/Codex runtime suites and repeated RPI canaries.
- Release/model-upgrade gate: compare candidate model/runtime against promoted baselines; require 100% pass on critical canaries and report variance for noncritical suites.
- Pre-push: run fast suites when `skills/`, `skills-codex/`, `hooks/`, `cli/cmd/ao/`, or eval packs change.

Outputs:

- `.agents/evals/runs/<run-id>/run.json`
- `.agents/evals/runs/<run-id>/scorecard.md`
- `.agents/evals/baselines/<suite>/<runtime>/<model>/baseline.json`
- `docs/releases/<date>-eval-summary.md` for release-grade baseline updates.

## Tests

**`cli/internal/eval/*_test.go`**:
- suite schema validation
- missing fixture failure
- deterministic scorer behavior
- baseline compare: pass, regression, improvement, inconclusive
- env scrub removes `AGENTOPS_RPI_RUNTIME*`

**`cli/cmd/ao/eval*_test.go`**:
- command registration
- `--json` emits valid JSON
- `run`, `compare`, and `baseline` commands handle bad paths and missing suites
- production command tests paired with every new command file

**`cli/cmd/ao/scenario_add_test.go`**:
- creates schema-valid scenario file
- rejects invalid thresholds/statuses
- stable ID generation with injectable clock

**Shell/BATS**:
- `scripts/eval-agentops.sh --fast` succeeds on fixtures
- hook-IO eval cases enforce expected decisions
- CI job remains advisory until baseline ratchet flips

## Conformance Checks

| Issue | Check Type | Check |
|-------|------------|-------|
| 1 | files_exist | `docs/contracts/eval-environment.md`, `schemas/eval-suite.v1.schema.json`, `schemas/eval-run.v1.schema.json` |
| 1 | command | `./scripts/check-contract-compatibility.sh` |
| 2 | tests | `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval` |
| 2 | content_check | `cli/docs/COMMANDS.md` includes `ao eval` after regeneration |
| 3 | files_exist | `evals/agentops-core/suite.json` |
| 3 | command | `cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao scenario --help` includes `add` |
| 4 | tests | `bash tests/scripts/test-headless-runtime-skills.sh` remains green |
| 5 | command | `scripts/eval-agentops.sh --fast --suite rpi-canary` |
| 6 | command | `scripts/pre-push-gate.sh --fast` |

## Verification

1. `cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval`
2. `scripts/eval-agentops.sh --fast`
3. `bash tests/scripts/test-headless-runtime-skills.sh`
4. `bash scripts/validate-hooks-doc-parity.sh`
5. `bash scripts/validate-codex-rpi-contract.sh`
6. `bash scripts/validate-codex-lifecycle-guards.sh`
7. `bash scripts/audit-codex-parity.sh`
8. `scripts/pre-push-gate.sh --fast`

## Issues

### Issue 1: Define eval contracts, schemas, and docs

**Dependencies:** None
**Acceptance:** Contract docs are indexed, schemas validate, and the docs explain public canaries, private holdouts, baseline promotion, score dimensions, and live runtime tiers.
**Description:** Add the contract and schemas before implementing commands so eval artifacts have a stable format.

### Issue 2: Implement `ao eval` deterministic core

**Dependencies:** Issue 1
**Acceptance:** `ao eval run/compare/baseline --json` works on fixture suites; command tests cover success and failure paths; CLI docs are regenerated.
**Description:** Add `cli/internal/eval` and `cli/cmd/ao/eval*.go`.

### Issue 3: Seed public canary suites and fix scenario authoring

**Dependencies:** Issue 1, Issue 2
**Acceptance:** Checked-in suites cover CLI, hooks, skills, runtime inventory, and RPI; `ao scenario add` exists or the skill docs are corrected; scenario files validate.
**Description:** Create `evals/agentops-core/` and close the scenario doc/CLI mismatch.

### Issue 4: Add live Claude/Codex runtime adapters

**Dependencies:** Issue 2
**Acceptance:** Live adapters can be skipped with an explicit reason, run in isolated homes, scrub host env, capture transcripts, and emit scorecard fields without being required in PR CI.
**Description:** Extend the runner interface to execute real runtime prompts.

### Issue 5: Add RPI and skill-change scorecards

**Dependencies:** Issue 3, Issue 4
**Acceptance:** RPI canary produces a scorecard with artifact, phase-order, objective-spine, validation separation, scenario satisfaction, and runtime-safety dimensions. Skill-change scorecard reports structural, trigger, runtime, scenario, and stocktake deltas.
**Description:** Turn "better or worse" into candidate-vs-baseline score comparisons.

### Issue 6: Integrate eval gates, baselines, and ratchets

**Dependencies:** Issue 2, Issue 3, Issue 5
**Acceptance:** `scripts/eval-agentops.sh --fast` runs locally; CI has an advisory eval job; baseline promotion is explicit; docs explain when a suite becomes blocking.
**Description:** Wire evals into local, CI, nightly, release, and model-upgrade workflows without making nondeterministic live model runs block normal PRs.

## Execution Order

**Wave 1:** Issue 1
**Wave 2:** Issue 2
**Wave 3:** Issue 3 and Issue 4
**Wave 4:** Issue 5
**Wave 5:** Issue 6

## File Dependency Matrix

| File/Area | Owner Issue | Downstream |
|-----------|-------------|------------|
| `docs/contracts/eval-environment.md` | 1 | 2, 3, 5, 6 |
| `schemas/eval-*.json` | 1 | 2, 3, 5 |
| `cli/internal/eval/` | 2 | 3, 4, 5, 6 |
| `cli/cmd/ao/eval*.go` | 2 | 6 |
| `evals/agentops-core/` | 3 | 5, 6 |
| `cli/cmd/ao/scenario_add.go` | 3 | 5 |
| Runtime adapters | 4 | 5, 6 |
| `scripts/eval-agentops.sh` | 6 | CI/pre-push |

## File-Conflict Matrix

| Files | Conflict Risk | Resolution |
|-------|---------------|------------|
| `cli/cmd/ao/*` | Medium | Issue 2 owns `eval*.go`; Issue 3 owns `scenario_add.go`; regenerate CLI docs after both. |
| `.github/workflows/validate.yml` | Low | Only Issue 6 edits CI after commands exist. |
| `docs/INDEX.md` | Low | Issue 1 owns contract index edit. |
| `evals/agentops-core/` | Low | Issue 3 owns initial corpus. |

## Cross-Wave Shared File Registry

| Shared File | Waves | Rule |
|-------------|-------|------|
| `cli/docs/COMMANDS.md` | 2, 3, 6 | Regenerate once after all CLI command changes land. |
| `docs/TESTING.md` | 1, 6 | Issue 1 can add contract references; Issue 6 adds operational commands. |
| `.github/workflows/validate.yml` | 6 | Keep advisory until at least one release cycle proves stability. |

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical Enforcement | PASS | Every issue has command/file checks and JSON schemas. |
| PR-002: External Validation | PASS | Candidate-vs-baseline comparison and runtime adapters are separate from implementer self-assessment. |
| PR-003: Feedback Loops | PASS | Scorecards and baseline ratchets feed future model/skill decisions. |
| PR-004: Separation Over Layering | PASS | Contract, CLI core, canaries, live adapters, scorecards, and gates are staged separately. |
| PR-005: Process Gates First | PASS | Schemas and baseline rules precede CLI and gate integration. |
| PR-006: Cross-Layer Consistency | PASS | Skills, hooks, CLI, CI, docs, schemas, and Codex artifacts are included. |
| PR-007: Phased Rollout | PASS | Advisory first; blocking only after baseline stability. |

Unchecked rules: 0

## Post-Merge Cleanup

After implementation, audit for scaffold-era names in `cli/internal/eval/`, ensure generated CLI docs are updated, and run `rg -n 'TODO|FIXME|HACK|XXX' evals cli/internal/eval cli/cmd/ao/eval*.go`.

## Next Steps

- Run `$pre-mortem .agents/plans/2026-04-24-agentops-evaluation-environment.md --quick`.
- If the pre-mortem is PASS/WARN, implement Wave 1 first.
