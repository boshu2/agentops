# Eval Environment Contract

> **Status:** Draft
> **Suite Schema:** `eval-suite.v1.schema.json`
> **Run Schema:** `eval-run.v1.schema.json`
> **Consumers:** future `ao eval` commands, release validation, RPI validation, and runtime/model comparison workflows

This contract defines the durable shape for AgentOps evaluation suites, run
records, scorecards, baselines, and gate tiers. It exists so skills, hooks, CLI
commands, RPI flows, and Claude/Codex runtime behavior can be compared against
stable evidence instead of self-reported success.

## Scope

V1 covers:

- checked-in public canary suites
- private holdout suites that use the same schema
- deterministic offline evaluation tiers
- optional live runtime tiers for Claude and Codex
- baseline capture and promotion rules
- candidate-vs-baseline score comparison
- run records and scorecards for local, CI, nightly, release, and model-upgrade use

V1 does not require:

- live model access for normal PR validation
- committing private holdout cases
- exact prose matching when a mechanical artifact, command result, or structured
  output can be scored
- blocking gates for live runtime suites before variance is known and ratcheted

## Storage

Use this storage split:

| Path | Purpose | Commit policy |
|------|---------|---------------|
| `evals/` | Public canary suites and committed fixtures | Checked in |
| `.agents/evals/runs/<run-id>/` | Generated run records, transcripts, scorecards, and diagnostics | Local/generated |
| `.agents/evals/baselines/` | Local baseline candidates and promoted baseline snapshots | Commit only when explicitly promoted |
| `.agents/holdout/` or private suite roots | Private holdouts | Do not commit unless intentionally publishing a canary |

Public and private suites both validate against
`eval-suite.v1.schema.json`. Run outputs validate against
`eval-run.v1.schema.json`.

## Public Canaries

Public canaries are reviewable, checked-in suites that every implementer may
read and run. They are meant to catch known regressions and preserve product
contracts.

Required properties:

- stored under `evals/`
- use `visibility: public_canary`
- deterministic suites must run without network, model auth, or host-specific
  local state
- fixture paths must be repo-relative and committed
- expected outputs must prefer mechanical checks over subjective prose
- critical public canaries must require a perfect pass rate before becoming
  blocking

Public canaries are allowed to become PR or pre-push gates after they are stable.
They are not sufficient by themselves because public cases can be overfit.

## Private Holdouts

Private holdouts are unseen suites used to detect overfitting and validate real
behavior beyond the public corpus.

Rules:

- use `visibility: private_holdout`
- validate against the same suite schema as public canaries
- may live in `.agents/holdout/`, `.agents/evals/holdouts/`, or an operator-owned
  private suite root
- must not be required for normal open source PR validation
- must not be read by the implementing agent unless the run is explicitly in an
  evaluator role
- may be used in nightly, release, model-upgrade, or post-merge validation

Private holdouts should report aggregate scores and regression categories without
revealing scenario text when secrecy matters.

## Eval Tiers

Suites declare one tier:

| Tier | Deterministic | Runtime access | Intended gate |
|------|---------------|----------------|---------------|
| `deterministic` | Yes | None | PR, pre-push, CI |
| `headless` | Mostly | Local Claude/Codex CLI inventory or mocked runtime I/O | CI advisory or release |
| `live` | No | Authenticated Claude/Codex model execution | Nightly, release, model upgrade |
| `release` | Mixed | Deterministic plus approved live/advisory suites | Release readiness |

The first blocking tier must be deterministic. Live runtime suites are opt-in and
advisory until repeated runs establish variance, timeout behavior, and cost.

## Runtime Isolation

Live or headless runtime evaluations must record runtime hygiene:

- isolated `HOME` for tool state where practical
- isolated `CODEX_HOME` for Codex runs
- scrub every `AGENTOPS_RPI_RUNTIME*` environment variable
- capture runtime name, runtime version, model, timeout, attempts, and skipped
  reason when unavailable
- write transcripts or stream logs as run artifacts

A live suite that cannot run because auth, runtime, model, budget, or network is
unavailable should produce `status: skipped` or `verdict: advisory`, not a false
regression.

## Score Dimensions

Canonical score dimensions are normalized numbers from `0.0` to `1.0`:

| Dimension | Meaning |
|-----------|---------|
| `correctness` | The task or command produced the required result. |
| `process_adherence` | The run followed required workflow, phase, or skill contract steps. |
| `artifact_quality` | Required artifacts exist, validate, and contain useful evidence. |
| `runtime_compatibility` | Behavior remains compatible across declared runtimes and models. |
| `efficiency` | The run stayed within expected time, budget, retry, and output limits. |
| `safety` | The run respected isolation, scope, permission, and no-leak requirements. |
| `learning_closure` | Findings, scorecards, baselines, and follow-up work were recorded when required. |

A suite declares dimension weights and thresholds. A run records case-level
dimension scores and aggregate dimension scores. A critical dimension regression
can fail a candidate even when the aggregate score improves.

## Baseline Lifecycle

Baselines are explicit snapshots, not whatever happened to pass most recently.

Lifecycle:

1. **Capture:** run a suite against a known reference and write an eval run record.
2. **Compare:** run the candidate and compare aggregate, dimension, and critical
   case deltas against the baseline.
3. **Review:** inspect scorecard evidence, skipped cases, environment hygiene, and
   any known variance.
4. **Promote:** record the promoted run id, candidate ref, baseline path, promoter,
   timestamp, and rationale.
5. **Ratchet:** make the suite blocking only after the baseline is stable enough
   for the target gate.

Promotion requires:

- no failed critical public canary cases
- no safety regression
- no unreviewed private-holdout regression when holdouts were run
- durable suite and run artifacts
- explicit human or release process rationale

Live runtime baselines must include repeat count, variance notes, skipped cases,
and advisory/blocking status. A single live pass is not enough to make a live
suite blocking.

## Verdict Rules

`eval-run.v1.schema.json` records both status and verdict.

Status describes execution:

- `pass`
- `fail`
- `error`
- `skipped`
- `inconclusive`

Verdict describes comparison:

- `pass`
- `fail`
- `improvement`
- `regression`
- `advisory`
- `inconclusive`

A candidate is better only when it preserves critical cases and improves or holds
the relevant baseline dimensions. A candidate that improves efficiency while
regressing safety or runtime compatibility is not better.

## Relationship To Existing Contracts

This contract builds on:

- [Headless Invocation Standards](headless-invocation-standards.md) for live
  Claude/Codex process execution
- [Hook Runtime Contract](hook-runtime-contract.md) for hook I/O expectations
- [Codex Skill API](codex-skill-api.md) for Codex runtime skill behavior
- [Session Intelligence Trust Model](session-intelligence-trust-model.md) for
  trusted runtime context inputs
- `scenario.v1.schema.json` for existing holdout scenario files

The eval suite schema may reference scenario fixtures, but scenario files remain
their own artifact type.
