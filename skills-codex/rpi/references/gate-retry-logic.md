# Repair and Retry Logic

Retries are orchestrator decisions. Phase skills emit evidence and stop at
their boundary; none converts its own result into a cross-phase retry.

## Premortem repair

Premortem judges a plan. WARN or FAIL returns that plan to its author for a
bounded repair and another Premortem. Between waves, the input must be an exact
changed plan from an explicit orchestrator request. Validate and Learn cannot
invoke Premortem.

## Crank recovery

Crank may retry a transient worker operation inside the same wave within its
declared task budget. At the wave boundary it emits DONE, PARTIAL, or BLOCKED
plus evidence to the orchestrator. It does not invoke Discovery, Learn, or
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

An attempt cap or oscillation is a stuckness signal, not proof that human
authority is required. The orchestrator may request one bounded fresh-context
helper with the blocker, evidence, and attempts. UNSTUCK returns to an explicit
orchestrator decision. ESCALATE reaches the operator. Refusal, explicit
judgment, or a genuinely spent hard time/cost/quota ceiling may skip the helper.
