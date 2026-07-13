# Agile Re-Plan Loop (the anti-waterfall rule)

The initial plan/wave-sequence is a **hypothesis**. Each wave is an experiment
that produces evidence; that evidence re-plans the remaining waves. This is what
makes `--auto` *autonomous* rather than *blind*.

## At every wave boundary (and after the validation phase)

The mandatory route is `Validate -> Learn -> orchestrator`. Validate produces
proof and structured observations. Learn binds those observations to the
immutable verdict and emits exactly one plan-impact disposition:

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
   Persist the mutated plan so the next wave reads the current plan, then run
   Premortem on that exact changed plan before proceeding.
2. **No change** — explicitly retry, continue, stop, or escalate. Do not
   fabricate a learning or invoke Premortem.
3. **Terminal** — close the tick. Do not re-plan or invoke Premortem.

## Bounds (so agility ≠ thrash)

Re-planning shares the run's circuit breakers — token/time budget, the attempt
cap, and **oscillation detection** (if the plan flips the same decision back and
forth across waves, stop and surface it). Honor the autonomous-session scope
(CLAUDE.md): at ≥5 ships in one session, the postmortem checkpoint is mandatory
and may itself end the session. The operator is touched only at the terminal
objective or a breaker trip that survives its bounded helper pass — never just
to approve a pivot.

## Anti-patterns this rule kills

- **Waterfall**: executing the initial wave list to the letter because "that was the plan."
- **Retry-not-replan**: re-cranking a failed wave on the same objective forever instead of asking whether the *remaining plan* should change.
- **Permission-seeking**: pausing to ask the operator to approve a pivot that `--auto` already authorizes.

## How the phase skills feed this loop

`/crank` emits wave evidence to `/validate`; `/validate` hands its immutable
verdict to `/learn`; `/learn` returns plan impact to the orchestrator. Only the
orchestrator selects `/discovery` as the re-plan engine and sends a changed plan
through `/premortem`. No phase swallows a finding into a silent retry.
