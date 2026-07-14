# Repair and Retry Logic

Retries are orchestrator decisions. Phase skills emit evidence and stop at
their boundary; none converts its own result into a cross-phase retry.

## Premortem repair

Premortem judges a plan. WARN or FAIL returns that plan to its author for a
bounded repair and another Premortem. Between waves, the input must be an exact
changed plan from an explicit orchestrator request. Validate and Learn cannot
invoke Premortem.

## Crank recovery

Crank preserves transient worker failure evidence and returns DONE, PARTIAL,
or BLOCKED at the wave boundary. It owns no retry allowance or task budget; a
later action requires an explicit orchestrator decision and durable admission
from the persistent governor. Crank does not invoke Discovery, Learn, or
Premortem.

## Post-verdict decision

The required sequence is `Validate -> Learn -> orchestrator`:

1. Validate emits an immutable PASS, WARN, or FAIL verdict with structured
   observations.
2. Learn binds the verdict digest and emits `remaining_work` plus exactly one
   plan-impact disposition:
   - `material_change`;
   - `no_change`;
   - `terminal`.
3. The orchestrator chooses the next action:
   - material change with remaining work: Discovery changes the remaining plan,
     then Premortem judges that exact changed plan;
   - no change with remaining work: retry, continue, stop, or escalate
     explicitly;
   - terminal: close without re-plan or Premortem.

A direct `validate -> crank`, `validate -> premortem`, or
`learn -> premortem` transition is invalid. Retry history stays in evidence;
the ordered completion packet still carries one receipt per umbrella.

## Stuckness and escalation

Max-attempts, oscillation, and no-progress evidence are stuckness signals, not
proof that human authority is required. The orchestrator submits them to the
persistent governor. `HOLD` carries the matching blocker class and is the only
state that authorizes a helper; `UNSTUCK` returns a new approach as `REPAIR`,
and `ESCALATE` reaches `ANDON`. Explicit judgment or a positive projected
charge that exceeds a hard ceiling reaches `ANDON` without a helper.

The rung is a three-state machine (auto -> helper -> human), never a jump
straight to the operator:

- `HOLD -> ONE-HELPER` — a tripped breaker holds the bead and dispatches exactly
  one bounded fresh-context (or cross-family) helper pass with the blocker,
  evidence, and attempt history.
- `HELPER-UNSTUCK -> AUTO-REDO` — if the helper clears the blocker, control
  returns to an explicit orchestrator decision and the proof is re-run
  automatically; no human is paged.
- `HELPER-ESCALATE -> HUMAN` — only when that single helper pass also fails (or
  the class is a refusal/judgment/spent-ceiling skip) does the bead reach the
  operator.
