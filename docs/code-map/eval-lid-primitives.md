---
id: code-map-eval-lid-primitives-2026-05-02
type: code-map
date: 2026-05-02
status: initial — Wave 1 (skill-on/off A/B) shipped; Wave 2 (finding-coverage) deferred
---

# Eval LID-Primitives Codebase Map

> **Scope:** Files touched by the LID-primitives integration epic
> (`soc-x5yz`). Wave 1 (skill-on/off A/B baseline) is shipped here; Wave 2
> (finding-coverage tracking) is filed but not yet implemented. Cross-link
> from the eval surface for newcomers who want to add or modify the
> baseline-A/B / coverage primitives.
>
> **Primary contracts:**
> - `docs/contracts/eval-baseline-ab.md` — `--baseline-mode` semantics + `DeltaScorecard` shape
> - `docs/contracts/eval-environment.md` — base eval environment contract
> - Operator-local plan archive: see `.agents/plans/` on the implementing operator's box (not tracked; bead `soc-x5yz` covers the plan history)

## At A Glance

| Surface | Path | Role |
|---|---|---|
| `--baseline-mode` flag | `cli/cmd/ao/eval.go` | cobra wiring; routes to `RunSuite` (single leg) or `RunBaselineAB` (both legs) |
| Suite-level toggle | `cli/internal/eval/types.go::SuiteEnvironment.DisableHooks` | suite-declared hook suppression |
| Run-level toggle | `cli/internal/eval/runtime.go::LiveRuntimeOptions.OverrideDisableHooks`, `cli/internal/eval/types.go::RunOptions.OverrideDisableHooks` | caller-toggled override (used by baseline-A/B runner to A/B the same suite without mutation) |
| Effective-flag helper | `cli/internal/eval/runtime.go::effectiveDisableHooks` | OR semantics for the live-runtime path |
| Baseline-A/B runner | `cli/internal/eval/baseline_ab.go::RunBaselineAB` | drives `RunSuite` twice with toggled overrides; synthesizes `DeltaScorecard` |
| Delta scorecard type | `cli/internal/eval/baseline_ab.go::DeltaScorecard, AssertionDelta` | per-case + aggregate delta JSON shape |
| Delta computation | `cli/internal/eval/baseline_ab.go::computeDelta, deltaSign` | pure function; table-driven tests |
| Output writer | `cli/internal/eval/baseline_ab.go::WriteDeltaScorecard` | persists scorecard to `--delta-out` path |
| Demo fixture | `evals/agentops-core/lid-primitives-demo.json` | smallest fixture proving the wiring works end-to-end |
| SessionStart kill-switch | `hooks/session-start.sh:7` | honors `AGENTOPS_HOOKS_DISABLED=1`; pre-existed this epic |
| Hook audit | (this code-map) | only `session-start.sh` auto-loads skills today; future skill-loading hooks MUST honor the env var |

## Wave 1 Test Surfaces

| File | Test |
|---|---|
| `cli/internal/eval/types_test.go` | Round-trip JSON snapshots for `SuiteEnvironment.DisableHooks` and `EnvironmentRecord.HooksDisabled` (per `f-2026-04-26-001`) |
| `cli/internal/eval/runtime_test.go` | `effectiveDisableHooks` triad, `liveRuntimeEnv` env-var presence/absence, `liveRuntimePrompt` constraint sentence, `liveEnvironmentRecord` reflects effective state |
| `cli/internal/eval/baseline_ab_test.go` | `IsValidBaselineMode` reject table, `deltaSign` triad, `computeDelta` pass/fail/missing-case matrix, `appendBaselineSuffix` path mangling |

## Live Smoke

```bash
ao eval run evals/agentops-core/lid-primitives-demo.json --baseline-mode=both --delta-out /tmp/delta.json
# → Eval baseline-AB agentops-core.lid-primitives-demo: skill-on=1.0000 (pass) skill-off=0.5000 (fail) delta=+0.5000 cases=2
```

The fixture has two cases: one delta-bearing (asserts `AGENTOPS_HOOKS_DISABLED` is unset, so it passes only on the skill-on leg) and one tied invariant (always passes). The resulting per-case deltas are `[+1, 0]`, demonstrating both delta and tied semantics in a single run.

## Wave 2 (deferred — not in this code map yet)

- `cli/internal/eval/types.go::Expectation.FindingRefs` — pending; will add `[]string` field
- `cli/cmd/ao/eval_finding_coverage.go` (proposed) — name TBD; `eval coverage` is already taken by domain coverage in `cli/internal/eval/coverage.go`. Open question: rename one of them or pick a distinct subcommand name (`eval finding-coverage`?). See plan §"Wave 2".

## Adding a new baseline-A/B-eligible suite

1. Set `environment.disable_hooks: false` in the suite (default — leave unset).
2. Add at least one assertion that depends on hook state (e.g., a shell case checking `AGENTOPS_HOOKS_DISABLED`, or a stdout pattern that only appears when a skill is loaded).
3. Run `ao eval run <suite>.json --baseline-mode=both --delta-out <out>.json` and inspect `aggregate_delta`.
4. If `aggregate_delta == 0`, no assertion in your suite actually depends on hook state — the A/B is uninformative. Add a delta-bearing case before relying on the score.

## Open questions / follow-ups

- HIL/VIL extension (live Claude/Codex runtime A/B with N-run variance bands) is **out of scope** for Wave 1. Filed as follow-up; needs a separate epic to design the variance methodology.
- Promotion of skill-baseline-delta to a `release-readiness.json` blocking gate is gated on ≥3 measured skills with delta ≥ +20% (per cross-vendor council verdict, 2026-05-02; verdict artifact lives operator-local in personal-site `.agents/council/`).
- The Wave 2 anchor (`finding_refs`) is intentionally NOT introduced in Wave 1 — pick the namespace decision deliberately based on Wave 1 evidence.
