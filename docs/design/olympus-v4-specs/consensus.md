> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Multi-Model Consensus

> Multiple models agree before big ratchets lock.

**Date:** 2026-02-17
**Status:** Draft (v4)
**Depends On:** `validation.md`, `execution.md`

---

## Overview

Multi-model consensus replaces human approval gates at ratchet points. Instead of a human reviewing a plan or implementation, multiple LLM models independently evaluate the work and vote. Consensus pass advances the phase automatically. Consensus fail escalates to human (exit code 2).

This is an **optional upgrade**. When consensus is disabled (default), Zeus gate phases behave exactly as they do today: exit code 2, human reviews, `ol zeus advance` to proceed. When enabled, consensus runs automatically at gate phases and either passes (exit 0, auto-advance) or fails (exit 2, human escalation).

---

## Why Multi-Model

Single-model validation has a known failure mode: the model that wrote the code rationalizes its own output. Olympus already solves this architecturally (separate `ol validate` invocation, no shared context). Multi-model consensus adds a second layer: **independent models with different training biases are less likely to share the same blind spots.**

This is the two-person integrity principle applied to LLMs. Not redundancy for reliability -- diversity for coverage.

---

## Model Registry

The model registry manages available models and their adapters.

### ModelAdapter Interface

```go
// ModelAdapter evaluates work and returns a verdict.
type ModelAdapter interface {
    // Name returns the adapter's identifier (e.g., "claude", "gpt4", "stub").
    Name() string

    // Evaluate takes a prompt describing the work to review and returns a verdict.
    // The prompt includes the plan/code, acceptance criteria, and review instructions.
    // The adapter is responsible for calling the underlying model.
    Evaluate(ctx context.Context, req EvalRequest) (*Verdict, error)
}
```

### EvalRequest

```go
type EvalRequest struct {
    // QuestID is the quest being evaluated.
    QuestID string

    // BeadID is the specific bead (empty for quest-level gates).
    BeadID string

    // Phase is the gate phase (PLAN_GATE, VIBE_GATE).
    Phase string

    // Prompt is the rendered review prompt with full context.
    Prompt string

    // Artifacts are paths to review materials (plans, code, council reports).
    Artifacts []string
}
```

### Verdict

```go
type Verdict struct {
    // Model is the adapter name that produced this verdict.
    Model string

    // Approve is the binary decision: pass or fail.
    Approve bool

    // Confidence is 0.0-1.0, optional (0 means not provided).
    Confidence float64

    // Reasoning is the model's explanation for its decision.
    Reasoning string

    // Timestamp records when the verdict was produced.
    Timestamp time.Time
}
```

### Built-in Adapters

| Adapter | Implementation | Use Case |
|---------|---------------|----------|
| `claude` | Spawns `claude -p` subprocess | Primary evaluator |
| `stub` | Returns configurable fixed verdict | Testing, CI |

The `claude` adapter follows the same subprocess pattern as `dispatchPhase()` in `internal/zeus/dispatch.go`: spawn `claude -p <prompt>`, capture stdout, parse verdict from output. No direct API calls in Go code.

Additional adapters (GPT-4, Gemini, local models) can be added by implementing `ModelAdapter`. The registry discovers adapters at startup from config.

---

## Consensus Algorithm

### Majority Vote with Configurable Quorum

The algorithm is simple: collect verdicts from N models, count approvals, compare against quorum threshold.

```go
type ConsensusConfig struct {
    // Enabled activates multi-model consensus at gate phases.
    // When false, gates behave as human approval (exit 2).
    Enabled bool

    // Models lists the adapters to use (e.g., ["claude", "gpt4"]).
    // Minimum 2 for meaningful consensus. Stub counts for testing.
    Models []string

    // Quorum is the fraction of models that must approve (0.0-1.0).
    // Default: 0.67 (2/3 majority).
    Quorum float64

    // Timeout is the maximum time to wait for all model responses.
    // Default: 5m.
    Timeout time.Duration
}
```

### Evaluation Flow

1. Build review prompt from quest/bead state and phase artifacts.
2. Fan out to all configured model adapters concurrently.
3. Collect verdicts (respect timeout; missing verdicts count as abstain).
4. Compute approval ratio: `approvals / total_responses`.
5. Compare against quorum threshold.
6. Record full voting log.

### ConsensusResult

```go
type ConsensusResult struct {
    // Passed is true if the quorum threshold was met.
    Passed bool

    // Verdicts contains every model's individual verdict.
    Verdicts []Verdict

    // ApprovalRatio is approvals/total (0.0-1.0).
    ApprovalRatio float64

    // Quorum is the threshold that was required.
    Quorum float64

    // Summary is a human-readable explanation of the outcome.
    Summary string

    // Timestamp records when consensus was evaluated.
    Timestamp time.Time
}
```

### Decision Table

| Scenario | Approvals | Total | Ratio | Quorum | Result |
|----------|-----------|-------|-------|--------|--------|
| Unanimous pass (3 models) | 3 | 3 | 1.00 | 0.67 | PASS |
| Unanimous fail (3 models) | 0 | 3 | 0.00 | 0.67 | FAIL |
| Split (2/3) | 2 | 3 | 0.67 | 0.67 | PASS |
| Split (1/3) | 1 | 3 | 0.33 | 0.67 | FAIL |
| One timeout (2 respond, both approve) | 2 | 2 | 1.00 | 0.67 | PASS |
| All timeout | 0 | 0 | 0.00 | 0.67 | FAIL |

Tie-breaking: there is no tie. The quorum is a strict threshold (`>=`). If the ratio exactly meets the quorum, it passes. If not, it fails. No weighted voting in Phase 1 -- confidence scores are recorded but not used in the decision.

---

## Conflict Resolution

When consensus fails (quorum not met), the result includes:

1. **Each model's reasoning** -- Why it approved or rejected.
2. **Disagreement summary** -- What the models disagree about.
3. **Escalation path** -- Exit code 2, human reviews the voting log.

The human sees the full voting record and can either:
- Fix the issues and re-run consensus.
- Override with `ol zeus advance` (manual gate pass).

Consensus failure is information, not a dead end. The voting log tells the human exactly what to focus on.

---

## Integration with Zeus

### Gate Phase Dispatch

In `dispatchPhase()`, gate phases (PLAN_GATE, VIBE_GATE) check the consensus config:

```
if consensus.enabled AND phase is gate:
    result = consensus.Evaluate(models, review_prompt)
    if result.passed:
        return nil  // auto-advance
    else:
        return exit 2  // escalate to human
else:
    // existing behavior: exit 2, human reviews
```

### Which Phases Use Consensus

| Phase | Consensus? | Reason |
|-------|-----------|--------|
| PLAN_GATE | Yes | Plan quality affects everything downstream |
| PRE_MORTEM_GATE | No | Pre-mortem is advisory, not blocking |
| CRANK_VALIDATE | No | Mechanical (go build/vet/test), no LLM needed |
| VIBE_GATE | Yes | Implementation quality gate before post-mortem |
| POST_MORTEM | No | Knowledge extraction, not approval |

Only PLAN_GATE and VIBE_GATE use consensus. These are the two phases where "does this work meet the bar?" is the question. Other gates are either mechanical (CRANK_VALIDATE) or advisory (PRE_MORTEM_GATE, POST_MORTEM).

---

## Configuration

### .ol/config.yaml

```yaml
consensus:
  enabled: false          # Default: off (human gates)
  models:
    - claude
    - stub               # For testing; replace with real model in production
  quorum: 0.67           # 2/3 majority
  timeout: 5m
```

### Environment Variable Override

```
OL_CONSENSUS_ENABLED=true    # Enable consensus
OL_CONSENSUS_QUORUM=0.5      # Lower threshold (not recommended)
```

---

## CLI

### `ol validate consensus`

Manual consensus invocation for debugging and gate override.

```
ol validate consensus --quest <id> --bead <id>

Output (table):
  MODEL     VERDICT    CONFIDENCE  REASONING
  claude    APPROVE    0.85        Plan covers all acceptance criteria...
  gpt4      REJECT     0.72        Missing error handling for timeout...

  CONSENSUS: FAIL (1/2 = 0.50, quorum 0.67)

Output (json):
  { "passed": false, "approval_ratio": 0.5, ... }

Exit codes: 0=consensus pass, 1=error, 2=consensus fail
```

---

## Audit Trail

Every consensus evaluation is recorded in the run ledger:

```
.ol/runs/<bead-id>/<attempt>-consensus.json
```

Record fields:

| Field | Purpose |
|-------|---------|
| `quest_id` | Parent quest |
| `bead_id` | What was evaluated (empty for quest-level) |
| `phase` | Which gate phase |
| `passed` | Aggregate outcome |
| `approval_ratio` | Numeric result |
| `quorum` | Threshold used |
| `verdicts` | Full list of model verdicts |
| `timestamp` | When evaluated |

This integrates with the existing run ledger pattern from `context.md`. Same directory, same append-only semantics, same provenance guarantees.

---

## Relationship to Stage 1 / Stage 2

Consensus does **not** replace Stage 1 or Stage 2. It complements them:

| Gate | Type | Override? |
|------|------|----------|
| Stage 1 (go build/vet/test) | Mechanical | No. Never. |
| Stage 2 (merge) | Mechanical | No. Requires Stage 1 pass. |
| Consensus (plan/vibe) | LLM-based | Yes, via `ol zeus advance`. |

Stage 1 is a hard gate. Consensus is a soft gate backed by multi-model agreement. A failing test always blocks. A consensus failure escalates to human judgment.

---

## Phase 1 Scope

This spec covers Phase 1 (the current implementation target):

| In Scope | Out of Scope (Future) |
|----------|----------------------|
| Majority vote algorithm | Weighted voting by model confidence |
| Claude + stub adapters | GPT-4, Gemini, local model adapters |
| PLAN_GATE + VIBE_GATE | Custom gate phases |
| Fixed quorum threshold | Adaptive quorum based on phase risk |
| Sequential evaluation | Learning from past consensus patterns |

Phase 1 is intentionally minimal. The interface (`ModelAdapter`, `ConsensusConfig`) is designed for extension; the implementation is designed for correctness.

---

*v4 consensus spec -- 2026-02-17*
