# ADR-0009: Delete the Daemon — AgentOps Is In-Session Only, Out-of-Session Opts Into Gas City

- **Status:** Accepted (2026-05-24)
- **Author:** AgentOps maintainers
- **Tracking:** bead `soc-j7a5q`
- **Builds on:** [ADR-0002](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md) (hookless-first), [ADR-0007](ADR-0007-deterministic-loop-only-operator-stops.md) (deterministic evolve loop)
- **Supersedes:** the "Software factory daemon (`ao daemon`)" surface as an AgentOps-shipped capability.

## Context

AgentOps 3.0 converged on an honest identity: it is the **in-session agent operating loop plus the context compiler that feeds it**. The in-session loop — `rpi` (inner), `evolve` (outer), `crank`/`swarm` (in-session agent teams), the skills as the portable runtime, and the `.agents/` corpus as the compounding moat — is the product. It runs end-to-end in a plain session with zero AgentOps-managed always-on infrastructure.

Out-of-session orchestration (an always-on daemon, a scheduler, an overnight runner, a queue-driven dispatcher) is a *separate* concern. The 2.x codebase shipped that concern as `agentopsd` plus `ao daemon` / `ao schedule` / `ao overnight`. The `soc-2rtm0` rip removed the out-of-session orchestration CLI (daemon + rpi-supervisor/phased/parallel + factory + gc-bridge). This ADR records **why that removal was a delete, not a deprecate**, and names the rejected alternative.

Two facts forced the decision to be written down rather than left implicit:

1. **The delete decision rested on two contradictory same-day docs.** One design doc (`agentops-on-gascity` §7) argued to *keep `agentopsd` standalone*; that position was lost to a cascade-delete. The opposing position — delete it, AgentOps has no core to protect — won, but the rationale was never captured as an accepted record. Oracle flagged the contradiction.
2. **Olympus, cited as proof, actually KEPT its daemon.** "Mount Olympus proves you can run the loop out of session" is true, but Olympus did *not* delete its daemon — its typed Rust daemon **is** the product. Using Olympus as evidence for deleting AgentOps' daemon, without naming the difference in posture, was a category error.

The resolving distinction: **AgentOps has no sovereign core of its own.** It is a Gas City *reference configuration* (a `city.toml` + `packs/agentops`). It has nothing to protect by owning a daemon, so it deletes its daemon and opts into Gas City for always-on. **Olympus is a sovereign product** with its own Rust core; its daemon is the thing it protects, so it keeps it. Same fractal loop, opposite daemon posture, for a principled reason.

## Decision

1. **AgentOps ships no standalone daemon, scheduler, or overnight runner.** The `agentopsd` daemon and the `ao daemon` / `ao schedule` / `ao overnight` out-of-session surfaces are **deleted**, not deprecated, not retained behind a flag.
2. **The in-session loop is the zero-dependency sovereignty floor.** `rpi`, `evolve`, `crank`/`swarm`, the ratchet rules, the skills runtime, and the context compiler (`ao inject`/`ao compile`) all run in a plain session with no daemon, no scheduler, and no cloud. This is the floor every AgentOps user gets unconditionally.
3. **Always-on opts into an orchestration substrate.** When work must run unattended over a queue, a substrate drives it. AgentOps ships a **reference Gas City City** (`city.toml` + `packs/agentops`) for that. Gas City drives agents that *use* AgentOps: a mayor agent (coordinate, merge, notify the human) and refinery workers (run `ao rpi`) inherit the AgentOps skills via overlay.
4. **The seam is the DDD boundary.** Orchestration (when/where/who-supervises/coordination) is the substrate's; the loop and its context (what the agent does, how context compounds) is AgentOps'. `rpi` is never re-expressed as substrate workflow steps — the substrate dispatches the whole loop as one invocable unit.

## Rejected alternative: deprecate and keep `agentopsd` standalone

The `agentops-on-gascity` §7 position was to keep `agentopsd` as a standalone always-on runner alongside the Gas City path. Rejected because:

- **No core to protect.** A standalone daemon only earns its keep if it guards a sovereign core. AgentOps is a Gas City reference config; it has no such core. A standalone `agentopsd` would be a second orchestration substrate competing with the one AgentOps already ships as its reference — surface sprawl across the very seam this architecture draws.
- **Duplicated loop shape.** Keeping the daemon means maintaining queue-pull, dispatch, supervision, and scheduling inside AgentOps *and* documenting Gas City as the reference for the same. Two implementations of the same orchestration, both AgentOps', is the disease 3.0 set out to cure.
- **Honest product story.** "AgentOps is the in-session loop; out-of-session is the substrate's job" is a clean, defensible line. "AgentOps is the in-session loop, and also ships its own daemon, but also recommends Gas City" is not.

The deprecate-keep path keeps optionality at the cost of the architecture's central claim. Delete wins.

## Evidence

The end-to-end proof that the reference Gas City City replaces what the daemon did is captured in [`.agents/discovery/2026-05-24-gvkj6-e2e-proof.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-gvkj6-e2e-proof.md) (bead `soc-5jwah`). Per-tier verdict against a real `gc` binary on the merged reference City:

- **Demonstrated (Tiers 1/2/4/5):** the City is gc-parse-valid; the controller, supervisor, Order engine, and mayor agent come up; the AgentOps skills overlay lands in the spawned workdir (25 skills); and `ao inject` / `ao validate --gate` run in the dispatched workdir with the corpus compounding.
- **Known Gas City maturity gap (Tier 3):** order-level autonomous dispatch is *not* turnkey. A cooldown Order that pulls the next ready bead and binds it to the `rpi-dispatch` formula fails (`variable "issue" is required`), because Gas City Orders have no per-fire variable-binding mechanism. The honest, proven path is **mayor-driven dispatch** (a long-lived mayor agent runs `bd ready` → `gc sling` to a refinery worker; cron `exec` Orders handle scheduled maintenance).

The operator posture (bead `soc-5jwah` notes, 2026-05-24): accept what Gas City offers today and grow with it. The order var-binding / next-ready-dispatch gap is an **upstream Gas City contribution** the operator drives — not something AgentOps must solve and not a 3.0 blocker. The reference City ships **honestly**: dispatch is labeled mayor-driven; order-auto-dispatch is labeled a documented Gas City upstream evolution.

## Consequences

### Positive

- **Honest, defensible identity.** AgentOps is the in-session loop and the context compiler; the out-of-session story has one owner (the substrate), not two competing ones.
- **Zero-dependency sovereignty floor.** Every user gets the full loop in a plain session with no daemon, no scheduler, no cloud.
- **Smaller surface.** No queue-pull, dispatch, supervision, or scheduling code to maintain inside AgentOps.
- **Clean reference relationship.** The Gas City reference City is the single canonical way to run the loop out of session; the seam between orchestration and the loop is sharp.

### Negative (accepted tradeoffs)

- **Always-on now requires Gas City.** A user who wants unattended, queue-driven execution must adopt the Gas City substrate; AgentOps alone no longer provides it. This is the deliberate trade for the honest identity.
- **Order-level auto-dispatch is not turnkey yet.** The best achievable out-of-session dispatch today is mayor-driven; fully autonomous order-driven dispatch depends on an upstream Gas City capability (`soc-5jwah`).
- **Migration for existing daemon users.** Operators who relied on `ao daemon` / `ao schedule` / `ao overnight` migrate to the Gas City reference City or run the loop in session.

## Acceptance

This ADR is accepted when:

- `docs/adr/` contains an accepted ADR for the daemon deletion that names the rejected alternative (deprecate-keep-standalone) and the sovereignty tradeoff.
- `docs/3.0.md` states the in-session-only identity and links here.
- Live doc prose (README and friends) no longer frames an AgentOps-shipped daemon/scheduler as a current surface.
- The reference Gas City City labels its dispatch as mayor-driven and the order-auto-dispatch gap as an upstream Gas City item.

## References

- [AgentOps 3.0: the north star](../3.0.md)
- [The Canonical Loop Model](../architecture/canonical-loop-model.md)
- [ADR-0002: Hookless-First CDLC Rearchitecture](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md)
- [`.agents/discovery/2026-05-24-gvkj6-e2e-proof.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-gvkj6-e2e-proof.md) — the end-to-end proof (bead `soc-5jwah`)
- Beads: `soc-j7a5q` (this ADR), `soc-2rtm0` (the orchestration-CLI rip), `soc-5jwah` (the e2e proof + upstream-GC gap)
