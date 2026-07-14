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
   Persist the mutated plan so the next wave reads the current plan.
2. **No change** — explicitly retry, continue, stop, or escalate. Do not
   fabricate a learning or a material mutation.
3. **Terminal** — close the tick. Do not re-plan or invoke Premortem.

For either nonterminal branch, completion of the prior leaf updates the
remaining-plan snapshot. Before requesting another Crank wave, the orchestrator
runs exactly one bounded Premortem over the next leaf, changed ordering/scope,
and new evidence; completed candidate proof is not replayed. A first introduced
acceptance defect may receive one consolidated repair. A second distinct repair
need forces `REPLAN` and re-slicing through Discovery instead of another review
loop.

## Bounds (so agility ≠ thrash)

Re-planning uses the same persistent run governor as every other action.
Projected token/time charges require admission, while max-attempts,
oscillation, and no-progress evidence use the governor's protected breaker
path. No phase or session-local checkpoint creates a second allowance. The
operator is touched only at the terminal objective or an `ANDON` backed by the
governor — never just to approve a pivot.

## Anti-patterns this rule kills

- **Waterfall**: executing the initial wave list to the letter because "that was the plan."
- **Retry-not-replan**: re-cranking a failed wave on the same objective forever instead of asking whether the *remaining plan* should change.
- **Permission-seeking**: pausing to ask the operator to approve a pivot that `--auto` already authorizes.

## How the phase skills feed this loop

`/crank` emits wave evidence to `/validate`; `/validate` hands its immutable
verdict to `/learn`; `/learn` returns plan impact to the orchestrator. Only the
orchestrator selects `/discovery` as the re-plan engine and sends a changed plan
through `/premortem`. No phase swallows a finding into a silent retry.
