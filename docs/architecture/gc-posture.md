# GC Posture: Gas City Is an Adapter Behind AgentOps Ports

> **One sentence:** Gas City (GC) is the recommended *production* supervisor for
> the out-of-session Factory lane, but it sits **behind** AgentOps's daemon ports
> as **one opt-in, swappable adapter** — it is not the runtime AgentOps runs
> inside.

This is the canonical boundary doc. Where a one-liner elsewhere says "Gas City is
the reference substrate," this page is the definition it points at.

## Posture: which way the hexagon points

AgentOps owns the [ports](ports-and-adapters.md). The inner hexagon — the domain
plus the port interfaces it declares — is the only thing the rest of the system
depends on. Anything that drives the loop out of session (always-on, scheduled,
over a queue) is an **adapter** plugging into those ports.

Gas City satisfies the out-of-session **Factory driver** role. That makes it a
**driving adapter** for the unattended loop, in the same category as the
in-session `evolve`/`dream` adapters described in
[ports-and-adapters.md](ports-and-adapters.md). The direction is fixed and
non-negotiable:

- **AgentOps core owns the ports.** The loop body, the ratchet, `inject`, and
  `compile` are AgentOps; an adapter *calls* them.
- **GC is one adapter, not the substrate AgentOps is built on.** It is **opt-in**
  (a plain `evolve` session needs zero GC) and **swappable** (another substrate —
  or a bare cron, or a human running the loop by hand — can satisfy the same
  Factory-driver role without touching the domain).

The governing test, from [The Canonical Loop Model](canonical-loop-model.md): is
it about *when, where, who supervises, or coordination*? That is the substrate
(GC). Is it about *what the loop does or how context compounds*? That is AgentOps,
and it never moves into the adapter.

## The guardrail (hard limit)

**Never register AgentOps's own state — the corpus, the bead/Dolt store, ratchet
evidence, handoffs — as a GC-managed-city resource.**

GC's value is supervising *agents and jobs*. Its managed-city lifecycle (health
checks, restart-on-flap, teardown) is built for ephemeral workers, not for the
durable state that AgentOps is the source of truth for. Hand that state to the
city lifecycle and a health-check disagreement turns into a restart storm against
a resource that should simply be *read*.

This is not hypothetical. The Mount Olympus reference implementation hit exactly
this failure: wiring durable state into the supervised-resource lifecycle produced
roughly **1,478** supervisor flaps before the resource was demoted back out of the
city's control. (That figure is the stated rationale from the Olympus incident,
not an AgentOps-measured in-repo metric.)

The rule that prevents it: AgentOps state is owned by **AgentOps's own driven
adapters** — `storage_fs` for packets, the git adapter for history, the
beads/Dolt adapter for the tracker (see the driven-adapter forecast in
[ports-and-adapters.md](ports-and-adapters.md)). GC drives *the loop*; it does not
own *the loop's records*.

## Sovereignty: no cloud required

GC is **local and self-hostable**. Running the AgentOps Factory lane on GC
preserves AgentOps's core promise: **no cloud required**. There is no managed
service in the critical path — no hosted control plane you must authenticate to,
no external queue you cannot run on your own box. A laptop, a workstation, or a
home server runs the whole Factory loop offline.

Because GC is an adapter and not a dependency, the sovereignty guarantee is
structural rather than aspirational: swap GC out and the in-session `evolve`
driver still runs the identical loop with zero external services. The substrate is
a convenience for unattended throughput, never a gate on owning your own stack.

## Cross-references

- [Ports and Adapters](ports-and-adapters.md) — the hexagonal seam this posture
  rests on; where driving/driven adapters and the "how to add a new adapter"
  recipe live.
- [The Canonical Loop Model](canonical-loop-model.md) — "one loop body, two
  drivers": the in-session Evolve driver is AgentOps-shipped, the out-of-session
  Factory driver is substrate-owned (GC reference). The boundary table there is
  the companion to this page.
- [using-gc skill](https://github.com/boshu2/agentops/blob/main/skills/using-gc/SKILL.md)
  — the agent-facing workflow for driving the loop on a Gas City City.
- [dependencies.md](../dependencies.md) — where GC sits in the guided-dependency
  list (alongside `bd`), not wrapped by `ao`.

> External context: the Mount Olympus repo's
> `docs/decisions/2026-05-21-ports-and-adapters-pattern.md` records the
> demote-GC-to-adapter decision in a different codebase. It is referenced here as
> prior art, not as an in-repo source.
