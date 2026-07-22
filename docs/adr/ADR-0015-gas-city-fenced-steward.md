# ADR-0015: Gas City Fenced Steward Factory

- **Status:** Superseded on 2026-07-22 by the thin native pack boundary
- **Scope:** Historical record of the rejected GC33 control-plane expansion
- **Current authority:** [Gas City reliability boundary](../operations/gas-city-reliability.md)
- **Migration evidence:** [GC33 migration provenance](../contracts/gc33-migration-provenance.md)

## Historical decision

The original 3.3 design proposed a Fenced Steward factory around Gas City. It
added an AgentOps-owned program graph, admission and delivery schema families,
a graph reducer, packets between roles, a feeder, retry state, fenced delivery
epochs, and a custom Go delivery subsystem.

That design produced real fixes, but it duplicated state already owned by Gas
City, Beads, Git, and GitHub. Its feedback loop also depended on the factory
under development, making small failures expensive to isolate. The operator
stopped that rollout before release.

## Superseding decision

AgentOps 3.3 ships a thin optional pack over official Gas City:

- Beads owns work and dependencies.
- Gas City owns sessions, routing, formulas, propulsion, and OTEL.
- Git owns candidate commits and worktree isolation.
- Existing AgentOps manifests and verdicts own semantic validation.
- GitHub owns PRs, hosted checks, and merge state.

AgentOps supplies role prompts, one native formula, exact official toolchain
materialization, and bounded helpers for bootstrap, invocation, worktrees,
Refiner delivery, and teardown. There is no AgentOps GC state machine, protocol
schema family, daemon, reducer, or custom delivery command.

The retained role policy is Fable Mayor and Refiner, Sol-high planning and
fresh validation, Terra-high default implementation, and Opus-medium overflow.
Luna is support-only. Main remains moving; the Refiner rebases at most once,
proves candidate content is preserved, reruns gates, and otherwise creates
rework rather than locking main.

## Consequences

The former implementation remains available in Git history, not in the active
runtime. Development uses a fast offline contract and one opt-in native
qualification at a candidate boundary. A live mixed-provider canary is allowed
only after fresh validation and protected delivery.
