# Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan/wave-sequence is a **hypothesis**. Each wave is an experiment
that produces evidence; that evidence re-plans the remaining waves. This is what
makes `--auto` *autonomous* rather than *blind*.

## At every wave boundary

Crank returns targeted deterministic evidence directly to the orchestrator.
When acceptance, dependencies, write scope, and risk remain unchanged, the
accepted Premortem verdict remains valid and the orchestrator may pull the next
sequential wave. A material change routes through Discovery and one new Premortem.
Validate and Learn do not run between unchanged waves.

## At the frozen tranche boundary

The mandatory route is `Validate -> Learn -> orchestrator` once. Validate
produces proof and structured observations. Learn binds those observations to
the immutable verdict and emits exactly one plan-impact disposition:

- `material_change` when cited evidence invalidates or changes a remaining-plan
  assumption;
- `no_change` when work remains but no plan mutation is warranted;
- `terminal` when no work remains.

The orchestrator then applies the matching transition:

1. **Material change** — invoke Discovery with the cited Learn packet to
   re-plan the remaining waves. The changed plan may autonomously:
   - **refactor** a downstream wave's scope (split, merge, narrow, widen),
   - **insert** a new wave the evidence revealed is needed,
   - **drop** a wave the evidence made unnecessary,
   - **reorder** waves as the critical path shifts,
   - **re-scope / re-prioritize / re-sequence** beads,
   - **escalate** (circuit-breaker) when the evidence invalidates the objective itself.
   Persist the mutated plan so the next wave reads the current plan.
2. **No change** — explicitly retry, continue, stop, or escalate. Do not
   fabricate a learning or a material mutation.
3. **Terminal** — close the tick. Do not re-plan or invoke Premortem.

Before tranche freeze, leaf completion updates the remaining-plan snapshot but
does not by itself invalidate Premortem. Material plan-input change does. After
freeze, a first introduced acceptance defect may receive one consolidated
repair and affected-claim closure. A second distinct repair need forces
`REPLAN` and re-slicing through Discovery instead of another review loop.

## Bounds (so agility ≠ thrash)

Re-planning is an orchestrator decision backed by one immutable
[run-disposition record](pull-flow-governor.md). Max-attempts, oscillation, and
no-progress are evidence for `HOLD`, not proof that a human is required. No
phase or session-local checkpoint owns an allowance or retry counter. The
operator is touched only at the terminal objective or an evidence-backed
`ANDON` — never just to approve a pivot.

## Anti-patterns this rule kills

- **Waterfall**: executing the initial wave list to the letter because "that was the plan."
- **Retry-not-replan**: re-cranking a failed wave on the same objective forever instead of asking whether the *remaining plan* should change.
- **Permission-seeking**: pausing to ask the operator to approve a pivot that `--auto` already authorizes.

## How the phase skills feed this loop

`/crank` emits wave evidence to the orchestrator. At the bounded tranche
boundary, `/validate` hands one immutable verdict to `/learn`; `/learn` returns
plan impact to the orchestrator. Only the orchestrator selects `/discovery` as
the re-plan engine and sends a materially changed plan through `/premortem`. No
phase swallows a finding into a silent retry.
