---
id: research-2026-04-29-eval-suite-triage
type: research
date: 2026-04-29
---

# Research: Eval Suite Triage

**Backend:** inline. The research skill normally dispatches an exploration agent, but this Codex thread's tool policy only permits subagents when the user explicitly asks for subagents or parallel delegation. Discovery continued inline with scoped source reads and mechanical audits.

**Scope:** Current `ao eval` engine, suite schema, public canary corpus, promoted baselines, runner script, CI/pre-push integration, runtime adapter surface, scorecards, and prior eval-environment artifacts.

## Summary

The eval suite is valuable, but it is not one kind of evaluation. It is a layered deterministic proof harness: mostly static contract canaries and executable wrappers around lower-level gates, plus baseline comparison infrastructure and scorecard scaffolding. The main improvement is not "add more evals"; it is to classify evidence kinds, align baseline policy with reality, convert brittle artifact-only checks into real invariant tests, split fast/full lanes, and reserve live runtime/holdout checks for advisory or release workflows until calibrated.

## Key Files

| File | Purpose |
|------|---------|
| `docs/contracts/eval-environment.md` | Defines public canaries, private holdouts, eval tiers, score dimensions, and baseline lifecycle. |
| `schemas/eval-suite.v1.schema.json` | Defines suite metadata, domains, tiers, runtimes, baseline policy, cases, and expectations. |
| `cli/internal/eval/engine.go` | Runs deterministic suites, command cases, expectations, scoring, and run records. |
| `cli/internal/eval/coverage.go` | Counts domain, dimension, and runtime coverage across suites. |
| `cli/internal/eval/scorecard.go` | Builds RPI and skill-change category scorecards from run records. |
| `cli/internal/eval/runtime.go` | Provides optional Claude/Codex live runtime adapter machinery, currently not exposed through normal deterministic `ao eval run`. |
| `cli/cmd/ao/eval.go` | CLI surface for `ao eval run`, `compare`, `baseline`, `scorecard`, and `coverage`. |
| `scripts/eval-agentops.sh` | Local/CI wrapper that runs suites, compares promoted baselines, writes artifacts, and reports warnings/failures. |
| `evals/agentops-core/*.json` | Public deterministic canary corpus. |
| `.agents/evals/baselines/*.baseline.json` | Promoted baseline run snapshots for most public canaries. |

## Findings

### 1. The contract already says public canaries are not enough

The eval contract defines public canaries as checked-in deterministic suites and explicitly says they preserve product contracts, but are not sufficient because public cases can be overfit (`docs/contracts/eval-environment.md:48`, `docs/contracts/eval-environment.md:65`). It also defines private holdouts and live runtime tiers separately (`docs/contracts/eval-environment.md:68`, `docs/contracts/eval-environment.md:87`). This means the suite should not claim to prove live agent quality until holdouts and live tiers are actually running.

### 2. The runner is deterministic-first by design

`RunSuite` loads a suite, infers a deterministic runtime, rejects non-deterministic runtimes, runs each case, scores the run, and writes a run record (`cli/internal/eval/engine.go:20`, `cli/internal/eval/engine.go:48`, `cli/internal/eval/engine.go:52`, `cli/internal/eval/engine.go:57`, `cli/internal/eval/engine.go:97`). The CLI describes this release as deterministic and says live Claude/Codex adapters are a later runtime tier (`cli/cmd/ao/eval.go:27`). That is a correct boundary, but it should be reflected in suite labels and docs so operators do not overread deterministic canaries as live behavioral evals.

### 3. Live runtime code exists, but it is an opt-in adapter layer

`RunLiveRuntime` returns skipped unless `Enabled` is set, probes runtime executables only after that gate, isolates environment, and records skipped reasons (`cli/internal/eval/runtime.go:102`, `cli/internal/eval/runtime.go:143`, `cli/internal/eval/runtime.go:161`, `cli/internal/eval/runtime.go:168`, `cli/internal/eval/runtime.go:471`). The adapter also scrubs runtime-specific environment prefixes and supports isolated `HOME`/`CODEX_HOME` (`cli/internal/eval/runtime.go:310`, `cli/internal/eval/runtime.go:326`, `cli/internal/eval/runtime.go:337`, `cli/internal/eval/runtime.go:350`). The right next step is not to make this PR-blocking; it is to add an explicit headless/live lane with skip and variance semantics.

### 4. The current suite is broad, but heavily static

Mechanical inventory from `ao eval coverage` reported 56 suites, 281 cases, and 219 critical cases. The expectation inventory is dominated by static text checks: 1083 `artifact_contains`, 211 `stdout_contains`, 137 `exit_code`, and 1 `schema_valid`. The case inventory is 143 artifact-only checks and 137 shell command checks.

This is not bad, but it means the suite is mostly a contract and gate harness. Coverage currently counts declared domains, dimensions, and runtimes (`cli/internal/eval/coverage.go:12`, `cli/internal/eval/coverage.go:24`, `cli/internal/eval/coverage.go:34`) and then reports missing buckets by presence (`cli/internal/eval/coverage.go:81`, `cli/internal/eval/coverage.go:98`, `cli/internal/eval/coverage.go:171`, `cli/internal/eval/coverage.go:228`). It does not distinguish a real behavioral fixture from a text tripwire.

### 5. Baseline policy and baseline artifacts are misaligned

There are 54 promoted baseline files under `.agents/evals/baselines/`, but all 56 public suites currently declare `baseline_policy.mode = none`. The schema already supports `none`, `compare`, and `promotable` (`schemas/eval-suite.v1.schema.json:139`, `schemas/eval-suite.v1.schema.json:146`). The wrapper still compares any baseline file it finds and warns when one is missing (`scripts/eval-agentops.sh:222`, `scripts/eval-agentops.sh:262`, `scripts/eval-agentops.sh:274`). This works operationally, but the suite metadata does not tell the truth about intended baseline behavior.

### 6. Scorecard canaries currently prove category presence more than category behavior

The scorecard implementation can build category reports and compare category deltas (`cli/internal/eval/scorecard.go:60`, `cli/internal/eval/scorecard.go:90`, `cli/internal/eval/scorecard.go:155`, `cli/internal/eval/scorecard.go:186`). But the public RPI scorecard suite only checks that category strings such as `artifact-completeness`, `phase-order`, and `runtime-safety` appear in `scorecard.go` (`evals/agentops-core/rpi-scorecard.json:30`, `evals/agentops-core/rpi-scorecard.json:35`, `evals/agentops-core/rpi-scorecard.json:85`). The skill-change scorecard suite does the same for `structural`, `trigger`, `runtime`, `scenario`, and `stocktake` (`evals/agentops-core/skill-change-scorecard.json:31`, `evals/agentops-core/skill-change-scorecard.json:52`, `evals/agentops-core/skill-change-scorecard.json:73`). These are contract canaries, not behavior checks; they should be backed by fixture run records and negative cases.

### 7. Some eval suites duplicate CI and pre-push gate responsibilities

The eval wrapper is wired into CI as a continue-on-error advisory job (`.github/workflows/validate.yml:143`, `.github/workflows/validate.yml:145`, `.github/workflows/validate.yml:172`). The pre-push gate also runs evals when eval-related files change and makes local fast evals advisory unless `PRE_PUSH_STRICT_EVAL=1` is set (`scripts/pre-push-gate.sh:308`, `scripts/pre-push-gate.sh:854`, `scripts/pre-push-gate.sh:868`). This is a useful integration, but several suites wrap lower-level gates already run elsewhere. Those cases should be tagged as gate wrappers and deduplicated where a single lower-level proof can feed multiple suite dimensions.

### 8. Prior post-mortem already warned against over-trusting green evals

The eval-environment post-mortem says the system delivered deterministic contracts, runner, baselines, public canaries, runtime adapters, and gate integration (`.agents/council/2026-04-25-post-mortem-agentops-eval-environment-longhaul.md:20`). It also says proof remained WARN because closure/disposition gaps existed despite green product gates (`.agents/council/2026-04-25-post-mortem-agentops-eval-environment-longhaul.md:47`, `.agents/council/2026-04-25-post-mortem-agentops-eval-environment-longhaul.md:56`). That is the key lesson: evals should report product signal, but they need external closure/disposition proof before a session is truly done.

## Recommendations

1. Keep the eval subsystem and public canary corpus.
2. Add an explicit evidence-kind taxonomy to suites/cases and coverage output.
3. Align baseline policy metadata with promoted baselines and missing-baseline warnings.
4. Refactor artifact-only suites into focused invariant tests where possible.
5. Merge or remove checks already covered by stronger generated-doc, CLI parity, or unit-test gates.
6. Replace label-only scorecard canaries with fixture-based pass/fail/regression cases.
7. Split `--fast` from heavier full deterministic suites.
8. Add optional live/headless and private holdout lanes, but keep them advisory/nightly/release-scoped until variance is known.

## Quality Validation

Coverage checked: prior eval-environment research, plan, pre-mortem, post-mortem, eval contract, suite schema, run engine, expectation engine, coverage command, scorecard engine, runtime adapter, CLI command, wrapper script, CI integration, pre-push integration, public suite inventory, baseline inventory, and current remote-compute canary addition.

Depth ratings:

| Area | Depth | Notes |
|------|-------|-------|
| Deterministic runner semantics | 3/4 | Source and CLI behavior are clear. |
| Suite corpus signal quality | 3/4 | Inventory and examples show the main pattern; a future implementation should produce per-suite cull reports. |
| Baseline lifecycle | 3/4 | Contract, script behavior, and artifact count are clear; promotion ownership needs product decision. |
| Scorecard behavior | 2/4 | Implementation is clear, but current public canaries are thin. |
| Live runtime and holdouts | 2/4 | Adapter and contract exist; no normal live suite lane is active. |

Gaps: The research did not execute the full eval suite because this discovery is planning-only and prior CI had just validated it. It did run coverage and static corpus audits.
