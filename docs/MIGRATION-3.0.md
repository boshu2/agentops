# Migrating to AgentOps 3.0

3.0 is a deliberate narrowing. AgentOps is now the **in-session** agent operating loop and context compiler: skills, the rpi/evolve loops, crank/swarm, and the `.agents/` corpus. Everything that tried to be an always-on orchestration runtime has been removed. Out-of-session orchestration is delegated to a substrate you choose — **Gas City** (AgentOps ships a reference City) or **Olympus** (the full-custom take).

This guide lists what was removed in 3.0 and what to use instead.

## Removed: all hooks (the repo is hookless)

`ao hooks install`, `--with-hooks`, and the embedded hook surface are gone. AgentOps no longer installs or runs hooks.

**Use instead:** skills + the `ao` CLI guide the workflow; **CI is the authoritative gate** (`.github/workflows/validate.yml`). What a PreToolUse/PostToolUse hook used to enforce locally, a CI job now enforces on push. For the in-session knowledge loop, the skills (`/rpi`, `/evolve`, `/post-mortem`) call the `ao` commands directly instead of relying on hook side effects.

## Removed: the daemon (`ao daemon` / `agentopsd`)

The always-on daemon — the queue, supervisor, scheduler, job executors, and the dream/overnight compounding runner — is deleted. AgentOps has no out-of-session runtime.

**Use instead:**
- **In-session:** run the loop yourself — `ao rpi` (one cycle), `ao evolve` (many cycles), `crank`/`swarm` for in-session agent teams. This is the zero-dependency path; it needs no extra binary.
- **Out-of-session (always-on):** run the AgentOps **reference Gas City** (`city.toml` + `packs/agentops`). Gas City supplies the controller, supervision, crash-recovery, and agent spawning; the agents inherit the AgentOps skills and run the AgentOps loops. Dispatch is **mayor-driven** (a long-lived mayor agent pulls ready beads and slings them to workers); scheduled maintenance runs as Gas City cron exec orders.
- Or use **Olympus** for a full-custom out-of-session implementation.

## Removed: scheduling (`ao schedule`, `ao plans`, `ao watch`, `ao overnight`)

These drove the daemon's cron lane and are gone with it. `docs/scheduling.md` and the `.agents/schedule.yaml` contract no longer apply.

**Use instead:** Gas City **Orders** for out-of-session scheduling (cron exec orders for `ao compile` / `ao maturity` / corpus maintenance), or run the work in-session on demand.

## Removed: the factory command (`ao factory`) and its contract corpus

**Use instead:** the factory *is* the loop — `crank`/`swarm` in-session, or the mayor-driven Gas City City out-of-session. There is no separate factory binary.

## Removed: the `runtime=gc` phased-engine bridge

The CLI gc-bridge glue is severed. `ao rpi` keeps its non-gc backends (`auto`/`direct`/`stream`/`tmux`); `runtime=gc` is no longer a valid mode. Gas City now dispatches whole loops (`ao rpi`) as one unit rather than driving the phased engine through a bridge.

## New in 3.0

- **`ao validate --gate`** — deterministic exit-code verdict (0 pass/warn, 1 fail, 2 internal error; `--strict` flips WARN→1). The retry hook for Gas City `check` and for CI; composes the existing ratchet validator, no network or LLM.
- **The AgentOps reference Gas City** — `city.toml` + `packs/agentops` (mayor + refinery agents, a curated skills overlay, thin Orders). The reference for running the loop out-of-session. Gas City's order-level *autonomous* dispatch is still maturing upstream (tracked at `soc-5jwah`); today dispatch is mayor-driven.

## What stays (the in-session core)

`ao rpi`, `ao evolve`, `crank`, `swarm`, `ao harvest`, `ao forge`, `ao mine`, `ao compile`, `ao wiki`, `ao doctor`, `ao goals`, and the `.agents/` corpus are all unchanged. The in-session loop is the product, and it is proven end-to-end.

## One-line summary

If you ran AgentOps **in a session**, nothing you rely on changed. If you ran the **daemon** for always-on work, move that lane to Gas City (reference City provided) or Olympus — AgentOps itself is now in-session only.
