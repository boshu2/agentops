# Eval Baseline-A/B Contract

> **Status:** Draft
> **Surface:** `ao eval run --baseline-mode={skill-on,skill-off,both}` plus the `DeltaScorecard` JSON artifact
> **Consumers:** SIL canaries that want to measure per-skill value-add; future HIL/VIL extensions
> **Source:** `.agents/plans/2026-05-02-lid-primitives-eval-integration.md` (W1.1–W1.4)

This contract defines what `--baseline-mode` does, what the `DeltaScorecard`
means, and what the `SuiteEnvironment.DisableHooks` toggle is and is not.

## Boundary: skill/hook A/B is not context A/B

`--baseline-mode` is reserved for the skill/hook loading axis:
`skill-on`, `skill-off`, and `both`. It MUST NOT be overloaded with context
packet variants such as `context_off` or `context_on`.

Context usefulness evaluation is a separate axis defined by the
[Context Usefulness Eval Contract](context-usefulness-eval.md). Context modes
compare isolated context roots while preserving hooks. They MUST NOT set
`AGENTOPS_HOOKS_DISABLED=1` as a way to create a baseline.

Future runners that need to compose skill/hook A/B with context A/B need a
separate composition contract. Until then, scorecards should keep
`DeltaScorecard` skill/hook deltas separate from context-usefulness scorecards.

## What it is

A primitive borrowed from Linked Intent (LID) iteration-1: run the same
evaluation suite twice — once with skills/hooks loaded as authored
(`skill-on`), once with skill loading suppressed (`skill-off`) — and report
the per-case pass/fail delta plus aggregate score delta.

LID's iteration-1 measured +36–56% pass-rate deltas across 3 skills; this
primitive lets AgentOps run the same kind of measurement against its own
skill catalog so each skill earns (or fails to earn) its prompt budget.

## What "skill suppression" actually disables

`--baseline-mode=skill-off` (or `OverrideDisableHooks=true` in
`LiveRuntimeOptions`/`RunOptions`) sets the case env var
`AGENTOPS_HOOKS_DISABLED=1` and, on the live-runtime path, appends the
following sentence to the runtime prompt:

> Constraint: Do NOT load additional skills or plugins. Work only with base
> agent capabilities.

The env var is honored by `hooks/session-start.sh:7`. Any AgentOps hook
that auto-loads skills MUST honor `AGENTOPS_HOOKS_DISABLED=1` for
skill-suppression to be a faithful baseline. The hook audit (W1.1
implementation) confirmed `session-start.sh` is the single skill-loading
hook today; new skill-loading hooks that do not honor this env var will
silently bias future baseline-A/B measurements.

The `EnvironmentRecord.HooksDisabled` field on the resulting `RunRecord`
reflects the effective state — i.e., `suite.Environment.DisableHooks ||
opts.OverrideDisableHooks`.

## What it does NOT disable

- Hooks that perform sanitization or kill-switch logic (e.g., the worker
  environment sanitization in `session-start.sh`).
- Hooks unrelated to skill loading (lint gates, audit logs, etc.).
- Skills that are loaded by the calling runtime *outside* AgentOps' hook
  surface (e.g., a Claude Code plugin marketplace install).

The toggle is **structural at the AgentOps boundary**. If your runtime
loads skills through another path, this primitive will under-report the
delta. Document that limitation in any suite that uses `--baseline-mode`.

## DeltaScorecard semantics

```json
{
  "schema_version": 1,
  "suite_id": "<suite ID>",
  "suite_path": "<path passed to ao eval run>",
  "generated_at": "<UTC ISO-8601>",
  "skill_on_run_id":  "<RunID of the skill-on leg>",
  "skill_off_run_id": "<RunID of the skill-off leg>",
  "skill_on_aggregate":  <0..1>,
  "skill_off_aggregate": <0..1>,
  "aggregate_delta":     <skill_on_aggregate - skill_off_aggregate>,
  "per_case": [
    {
      "case_id": "<case ID>",
      "skill_on_status":  "pass | fail | error | skipped | inconclusive",
      "skill_off_status": "pass | fail | error | skipped | inconclusive",
      "skill_on_score":   <0..1>,
      "skill_off_score":  <0..1>,
      "delta": -1 | 0 | +1
    }
  ]
}
```

`per_case[].delta` semantics:

- `+1` — skill-on passes AND skill-off does not pass (skill helped)
- `-1` — skill-off passes AND skill-on does not pass (skill hurt)
- `0`  — both legs agree on pass/non-pass

Per-leg `RunRecord` artifacts are persisted alongside the scorecard when
`--out` is supplied; their paths get a `-skill-on` / `-skill-off` suffix
inserted before the file extension. RunIDs get the same suffix when
`--run-id` is supplied.

## When to use which mode

| Mode | Use case |
|---|---|
| `skill-on` (default) | Routine eval runs; back-compat with pre-LID behavior. |
| `skill-off` | Diagnostic: re-run a failing eval with hooks suppressed to determine whether the failure was caused by a hook side effect. |
| `both` | Periodic measurement of per-skill value-add; SIL canaries that gate on a minimum aggregate delta; future release-readiness evidence. |

## SIL vs HIL/VIL — current scope

This contract covers the **SIL (deterministic + mock/shell runtime)** tier.
HIL/VIL (live Claude/Codex runtime) extension is **deferred** to a later
epic because it requires N-run variance bands to distinguish a real delta
from runtime nondeterminism. Do not ship a single-shot live `--baseline-mode=both`
score as authoritative evidence until the variance methodology is in place.

## Backward compatibility

- Existing suites that omit `environment.disable_hooks` continue to run
  with hooks enabled (default `false`).
- Existing single-leg `ao eval run` invocations behave identically — the
  default mode is `skill-on`.
- The `EnvironmentRecord.HooksDisabled` field uses `omitempty`; existing
  golden snapshots that did not include it will still match the new run
  records when `disable_hooks` is unset.

## Round-trip / regression guards

- `SuiteEnvironment` and `EnvironmentRecord` round-trip JSON snapshot
  tests live in `cli/internal/eval/types_test.go`.
- `effectiveDisableHooks` table-driven tests in `cli/internal/eval/runtime_test.go`.
- `computeDelta` table-driven tests + `appendBaselineSuffix` path-mangling
  tests in `cli/internal/eval/baseline_ab_test.go`.
- Live smoke: `evals/agentops-core/lid-primitives-demo.json` produces a
  +0.5 aggregate delta and a per-case delta of `[+1, 0]`.
