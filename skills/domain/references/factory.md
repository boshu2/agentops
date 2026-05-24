---
name: Factory
kind: concept
status: draft
see-also: [loop, evolve, rpi, context-compiler]
---
# Factory

The **out-of-session driver** of the [Loop](loop.md): the loop run unattended over a bead queue, with operator-only stop. Factory is one of the two drivers of the same loop body; it runs the identical [RPI](rpi.md) tick that an in-session [Evolve](evolve.md) run does. The difference is the driver (a substrate, not a person) and the stop policy (only an operator marker halts it).

## The substrate owns it, not AgentOps

AgentOps 3.0 ships **no** always-on daemon, scheduler, or overnight runner — those surfaces were **deleted** in the 3.0 rearchitecture (see [`docs/adr/ADR-0009-daemon-deletion-in-session-only.md`](../../../docs/adr/ADR-0009-daemon-deletion-in-session-only.md)). The Factory driver is the orchestration substrate's job. **Gas City** is the reference substrate: its Orders drive the loop, its queue holds the work, its agents inherit the AgentOps skills via overlay. AgentOps stays zero-dependency in a plain session through the Evolve driver.

## Mayor-driven dispatch (honest current state)

On the reference Gas City City, dispatch is **mayor-driven** today: a long-lived mayor agent runs `bd ready`, then `gc sling`s the next bead to a refinery worker that runs `ao rpi <bead>`; cron `exec` Orders handle scheduled maintenance. Order-level autonomous dispatch (a cooldown Order binding the next ready bead to a formula on its own) is a known **Gas City maturity gap** — GC Orders have no per-fire variable binding — tracked as an upstream contribution (`soc-5jwah`), not a turnkey AgentOps feature.

## When to use

- Use **Factory** for the unattended, queue-driven driver. Do not use it for an AgentOps-shipped daemon; that surface was deleted when out-of-session orchestration moved to the substrate.
- The substrate dispatches a whole loop as one invocable unit. The Factory driver never drives the loop's insides; rpi is never re-expressed as substrate workflow steps.

## Bounded context

The Factory *driver* is **substrate-owned (orchestration)**. The *loop it runs* is **BC3 Loop (AgentOps)**. The seam between them is the load-bearing DDD boundary: orchestration (when/where/who-supervises) versus the loop and its context (what the agent does, how context compounds).

## See also

- `loop.md` — the umbrella; Factory is one of its two drivers
- `evolve.md` — the in-session driver running the same tick
- `rpi.md` — the tick the Factory driver runs unattended
