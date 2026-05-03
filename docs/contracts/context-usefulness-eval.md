# Context Usefulness Eval Contract

> **Status:** Draft
> **Surface:** Wave 0 deterministic context-packet A/B for `context_off` versus `context_on`
> **Consumers:** `ao eval` context-variant orchestration, deterministic SIL canaries, future context lifecycle work
> **Source:** `.agents/plans/2026-05-03-context-packet-ab-wave0.md` (`soc-t3y1`)

This contract defines the first narrow AgentOps proof for context usefulness:
run the same deterministic evaluation case with repository context absent and
present, preserve normal hooks in both legs, and record whether context changed
the outcome, the evidence used, and the cost profile.

## Wave 0 Scope

Wave 0 is deterministic SIL only. It varies the context packet input while
holding the runtime, task, hooks, skill loading, and evaluator expectations
constant.

Wave 0 includes:

- `context_off`: run against an isolated `.agents` root that contains no useful
  context for the case.
- `context_on`: run against an isolated `.agents` root that contains the
  committed useful context fixture for the case.
- per-case scorecard rows that attribute outcome changes to the context variant
  when the evidence supports that attribution.
- negative-control sentinels proving irrelevant, stale, and toxic context can be
  present in a fixture root without becoming active decision evidence.

Wave 0 explicitly defers:

- lifecycle integration and startup context assembly policy.
- Dream, corpus fitness, MemRL, maturity, quarantine, and pruning loops beyond
  deterministic sentinel fixtures.
- blocking CI, release gates, promotion gates, or corpus-wide ratchets.
- live HIL/VIL authority and variance methodology.

`curated_only`, `stale_sentinel`, and `toxic_sentinel` are evidence labels for
fixture assertions only. Wave 0 implementations MUST NOT treat them as accepted
context variants, scorecard verdicts, gate modes, or corpus lifecycle policy.

## Context Variants

`context_off` means the case runs with context intentionally absent. The runner
SHOULD provide absence through an isolated `AO_AGENTS_DIR` fixture root rather
than by mutating the live repository `.agents` directory.

`context_on` means the case runs with context intentionally present. The runner
SHOULD provide presence through a separate isolated `AO_AGENTS_DIR` fixture root
that contains only the durable artifacts needed by the case.

Both variants MUST preserve the normal AgentOps hook environment. Context mode
MUST NOT set `AGENTOPS_HOOKS_DISABLED=1`, directly or indirectly. If a
context-mode run records `AGENTOPS_HOOKS_DISABLED=1` because context mode set or
inherited it, the run is degraded and cannot be used as context-usefulness
evidence.

Context variants are a separate eval axis from skill/hook baseline modes:

- `context_off` and `context_on` answer "did the provided repository context
  help this task?"
- `skill-off`, `skill-on`, and `both` in the Eval Baseline-A/B Contract answer
  "did AgentOps skill/hook loading help this task?"

Wave 0 runners SHOULD reject or explicitly mark inconclusive any composition
that would combine context variants with hook suppression before a separate
composition contract exists.

## Minimum Scorecard Fields

A Wave 0 context-usefulness scorecard MUST include enough information to compare
the two context legs without relying on transcripts alone.

Minimum top-level fields:

```json
{
  "schema_version": 1,
  "suite_id": "<suite ID>",
  "suite_path": "<path passed to ao eval run>",
  "generated_at": "<UTC ISO-8601>",
  "context_off": {},
  "context_on": {},
  "aggregate_delta": 0.0,
  "per_case": []
}
```

Minimum top-level leg fields:

```json
{
  "variant": "context_off | context_on",
  "context_root_label": "<stable fixture/root label>",
  "run_id": "<run ID for this leg>",
  "aggregate_score": 0.0,
  "status": "pass | fail | error | skipped | inconclusive"
}
```

Minimum paired per-case fields:

```json
{
  "case_id": "<case ID>",
  "context_off": {},
  "context_on": {},
  "score_delta": 0.0,
  "status_delta": 0,
  "decision_evidence": [],
  "ignored_context_evidence": [],
  "token_delta": null,
  "tool_delta": null,
  "degraded_reason": null,
  "artifact_attribution": []
}
```

The scorecard MAY group per-case rows into paired `context_off`/`context_on`
comparisons, but each leg still needs a stable `case_id`, `variant`,
`context_root_label`, `run_id`, `status`, and `score`.

## Evidence Semantics

`decision_evidence` records why the scorer believes the task outcome changed or
held steady. Acceptable entries include checked files, command outputs,
structured evaluator facts, or explicit transcript snippets when no mechanical
artifact exists.

`ignored_context_evidence` records why present context was not used or was
properly ignored. This field is load-bearing for negative controls: a
`context_on` pass is not useful context evidence when the task succeeded for
unrelated reasons and there is no evidence that the supplied context affected
the decision.

`token_delta` and `tool_delta` record cost differences when available. They MAY
be `null` when the runtime cannot report them deterministically. A missing token
or tool count does not invalidate the scorecard; it only prevents efficiency
claims.

`artifact_attribution` is a vocabulary for naming the artifacts that affected
the score. Wave 0 values SHOULD be plain artifact paths or stable labels such as
`none`, `context_fixture`, `decision_output`, and `transcript_excerpt`. Future
MemRL, maturity, and lifecycle systems may extend this vocabulary, but Wave 0
MUST NOT require those systems.

## Degraded Runs

`degraded_reason` is `null` only when the leg is valid evidence for the declared
variant. It MUST be a short stable string when execution completed but the leg
cannot support a clean context-usefulness claim.

Required degraded reasons:

| Reason | Meaning |
|---|---|
| `hooks_disabled` | Context mode set or inherited `AGENTOPS_HOOKS_DISABLED=1`. |
| `context_root_unavailable` | The isolated fixture root could not be read. |
| `context_root_mutated_live_state` | The run wrote fixture or sentinel data into the live repo `.agents` path. |
| `context_variant_unsupported` | The requested variant is not part of Wave 0. |
| `insufficient_decision_evidence` | The scorer cannot justify the status or delta from durable evidence. |
| `runtime_unavailable` | The declared deterministic runtime was unavailable or skipped. |

When `degraded_reason` is non-null, `task_status` MAY still report what happened,
but aggregate context delta calculations MUST either exclude the leg or mark the
case inconclusive.

## Non-Goals

This contract does not define corpus health, finding promotion, session-start
context selection, Dream scheduling, pruning thresholds, stale-context detection,
toxic-context detection, or release-blocking policy. Those systems can consume
Wave 0 evidence later, but they are not prerequisites for Wave 0.
