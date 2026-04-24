---
id: pre-mortem-2026-04-24-agentops-evaluation-environment
type: pre-mortem
date: 2026-04-24
source: "[[.agents/plans/2026-04-24-agentops-evaluation-environment]]"
prediction_ids:
  - pm-20260424-001
  - pm-20260424-002
  - pm-20260424-003
---

# Pre-Mortem: AgentOps Evaluation Environment

## Council Verdict: WARN

| ID | Judge | Finding | Severity | Prediction |
|----|-------|---------|----------|------------|
| pm-20260424-001 | Missing-Requirements | Public canaries alone will overfit quickly unless private holdouts and baseline promotion rules are part of the contract. | significant | The first eval suite reports green while live RPI still regresses on unseen goals. |
| pm-20260424-002 | Feasibility | Live runtime/model evals will be flaky if they become PR-blocking before auth, cost, timeouts, and variance are calibrated. | significant | A good PR gets blocked by model availability or nondeterministic scoring instead of a product regression. |
| pm-20260424-003 | Spec-Completeness | Scenario authoring is currently documented but not implemented in `ao scenario`; eval corpus work depends on closing that mismatch. | moderate | Eval authors hand-write inconsistent scenarios and the suite drifts from the skill contract. |

## Pseudocode Fixes

Finding: pm-20260424-002 — live runtime env contamination and nondeterminism
Severity: significant
Fix (pseudocode):

```go
func sanitizedEvalEnv(base []string) []string {
    denyPrefixes := []string{"AGENTOPS_RPI_RUNTIME"}
    out := make([]string, 0, len(base))
    for _, kv := range base {
        keep := true
        for _, prefix := range denyPrefixes {
            if strings.HasPrefix(kv, prefix+"=") {
                keep = false
                break
            }
        }
        if keep {
            out = append(out, kv)
        }
    }
    out = append(out, "AGENTOPS_HOOKS_DISABLED=1")
    return out
}
```

Affected files: `cli/internal/eval/runner.go`, `cli/internal/eval/runtime_*.go`

## Shared Findings

- The plan is strategically correct but should stay advisory for live model runs until repeated nightly/release runs establish variance.
- The storage split between checked-in `evals/` and generated `.agents/evals/` is necessary because `.agents/holdout` is ignored and unsuitable as the only baseline source.
- The first hard gate should be deterministic: schema validation, fixture commands, hook IO, CLI JSON validity, and RPI artifact/phase checks.

## Known Risks Applied

- `f-2026-04-14-001` — Applied to require direct `cli/cmd/ao` tests for every new `ao eval` command.
- `f-2026-04-14-002` — Applied to require durable eval pack and baseline paths.
- `learning-2026-04-14-scrub-rpi-runtime-from-raw-validation` — Applied to require `AGENTOPS_RPI_RUNTIME*` scrubbing.
- `2026-04-07-v2.35.0-release-postmortem` — Applied to require local, CI, and live/nightly layers.

## Concerns Raised

- "100%" must be defined as 100% pass on a named critical canary suite over a configured repeat count; anything broader is not measurable.
- LLM judge scorecards need calibration data before they can block releases.
- RPI canaries must score file artifacts and commands first; exact prose matching will be brittle.

## Recommendation

Proceed with the plan as a staged implementation. Keep Wave 1 and Wave 2 focused on deterministic contracts and `ao eval` core. Do not make live runtime/model evals blocking until at least one stable baseline exists and variance is reported.

## Decision Gate

[ ] PROCEED - Council passed, ready to implement
[x] ADDRESS - Fix concerns before implementing
[ ] RETHINK - Fundamental issues, needs redesign
