# Idempotency And Resume

Resume consumes exact-input artifacts; it does not replay phases by habit.

## Discovery

For the same objective, inspect the canonical execution packet and its ordered
receipt index:

- reuse brainstorm/research/plan artifacts whose objective, acceptance, base,
  dependencies, write scope, and risk still match;
- rerun only the earliest step invalidated by new evidence;
- reuse the bound Premortem verdict while its exact inputs match; and
- create one fresh Premortem only for a materially changed plan.

Do not run history search, research, plan, Premortem, or artifact generation merely
because `/discovery` was invoked again. Do not create a duplicate epic when an
open tracker item already represents the same accepted behavior.

## Validate

A final Validate verdict is immutable and exact-candidate-bound. Resume may
reuse exact-input factual receipts, but a changed candidate needs affected-claim
closure and a new final binding. It does not automatically rerun Postmortem,
Retro, Forge, Vibe, or a council. Learn consumes the one final verdict once.

## RPI

RPI persists its run disposition and execution packet across invocations.
`--from=<phase>` is legal only when all earlier canonical receipts resolve and
still match their inputs. Legacy phase summaries are optional link-only
projections and are never resume authority.

At an incomplete three-wave boundary, persist the current leaf, next failing
proof, plan/Premortem identities, and wave receipts. Resume the same leaf; do
not freeze, Validate, Learn, deliver, or
pull another leaf merely because the boundary was reached.
