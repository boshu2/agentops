# Phase Budget Migration

Historical RPI implementations assigned independent time boxes, attempt
allowances, and wave caps to individual phases. Those controls are retired.
They could reset in a fresh process, disagree across references, and spend work
without a durable admission.

The sole live contract is the persistent
[pull-flow governor](pull-flow-governor.md):

- initialization declares run-wide wave, reviewer-token, elapsed-time,
  review-context, and deterministic-execution ceilings;
- every Crank or Validate dispatch reports all projected charges, including
  explicit zeroes;
- the governor validates and persists an admission before dispatch;
- fresh processes resume the same counters;
- a meter refuses only an action with a positive projected charge that would
  exceed the declared ceiling;
- phase results return evidence to the orchestrator and never increment a
  private attempt, retry, helper, or wave allowance.

Complexity may still scale the depth of Discovery, Premortem, and Validate. It
does not create a second controller. A timeout or repeated result is evidence
for `REPAIR`, `REPLAN`, or a breaker request; the persistent governor alone
decides whether the run continues, enters `HOLD`, or reaches `ANDON`.

There is no phase-specific budget flag or disable-budget escape. Operators who
need different ceilings initialize a new run with explicit run-wide values;
an existing run state is never silently replaced.
