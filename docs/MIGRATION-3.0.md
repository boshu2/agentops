# Migrating to AgentOps 3.0

> **3.2 update:** the `ao rpi` / `ao evolve` CLI surface described below was removed in v3.2 and archived behind the `legacy` build tag (`AGENTOPS_LEGACY=1 make build` restores it). The in-session path is the operating loop + skills. Living map: [MIGRATION.md](MIGRATION.md).

3.0 is a deliberate narrowing. AgentOps is now the **in-session** agent operating loop and context compiler: skills, the operating loop (formerly the `ao rpi`/`ao evolve` verbs, removed in 3.2), crank/swarm, and the `.agents/` corpus. Everything that tried to be an always-on orchestration runtime has been removed. Out-of-session orchestration is delegated to a substrate you choose. AgentOps adopts a reference trio — **NTM** (a local tmux swarm) + **MCP** (`ao mcp serve`, shipped) + **managed-agents** (`ao agent`) — or you can run **Olympus** (the full-custom take). AgentOps owns none of it.

This guide lists what was removed in 3.0 and what to use instead.

## Removed: all hooks (the repo is hookless)

`ao hooks install`, `--with-hooks`, and the embedded hook surface are gone. AgentOps no longer installs or runs hooks.

**Use instead:** skills + the `ao` CLI guide the workflow; the **installed local cockpit pre-push gate is routine release authority** (`scripts/install-pre-push-gate.sh` -> `.git/hooks/pre-push` -> `scripts/hooks/pre-push.local`), while `.github/workflows/validate.yml` remains tag/PR/manual backstop telemetry. What a PreToolUse/PostToolUse hook used to enforce locally is now enforced by the explicit local gate before main pushes. For the in-session knowledge loop, the skills (`/rpi`, `/evolve`, `/post-mortem`) call the `ao` commands directly instead of relying on hook side effects.

## Removed: the daemon (`ao daemon` / `agentopsd`)

The always-on daemon — the queue, supervisor, scheduler, job executors, and the dream/overnight compounding runner — is deleted. AgentOps has no out-of-session runtime.

**Use instead:**
- **In-session:** run the loop yourself (pre-3.2: `ao rpi`/`ao evolve`; now the operating loop + the `/rpi` skill), with `crank`/`swarm` for in-session agent teams. This is the zero-dependency path; it needs no extra binary.
- **Out-of-session (always-on):** run the loop under a substrate AgentOps adopts but does not own — an **NTM** tmux swarm (workers each run the loop per bead; a lead agent or operator slings ready beads), **MCP** via `ao mcp serve` (exposes the `ao` tool surface to any MCP-aware harness), or **managed-agents** via `ao agent` (hosted, scheduled drivers). The substrate supplies supervision, crash-recovery, and agent spawning; the agents inherit the AgentOps skills and run the AgentOps loops.
- Or use **Olympus** for a full-custom out-of-session implementation.

## Removed: scheduling (`ao schedule`, `ao plans`, `ao watch`, `ao overnight`)

These drove the daemon's cron lane and are gone with it. `docs/scheduling.md` and the `.agents/schedule.yaml` contract no longer apply.

**Use instead:** schedule the substrate you adopt — cron-triggered NTM dispatch or a managed-agents schedule that runs `ao compile` / `ao maturity` / corpus maintenance — or run the work in-session on demand.

## Removed: the factory command (`ao factory`) and its contract corpus

**Use instead:** the factory *is* the loop — `crank`/`swarm` in-session, or substrate-driven dispatch (NTM / managed-agents) out-of-session. There is no separate factory binary.

## Removed: the `runtime=gc` phased-engine bridge

The CLI gc-bridge glue is severed. `ao rpi` (the command, since removed in 3.2) kept its non-gc backends (`auto`/`direct`/`stream`/`tmux`); `runtime=gc` is no longer a valid mode. A substrate now dispatches whole loops as one unit rather than driving the phased engine through a bridge.

## New in 3.0

- **`ao validate --gate`** — deterministic exit-code verdict (0 pass/warn, 1 fail, 2 internal error; `--strict` flips WARN→1). The retry hook for a substrate's `check` step and for CI; composes the existing ratchet validator, no network or LLM.
- **The reference out-of-session substrate** — AgentOps adopts **NTM** + **MCP** (`ao mcp serve`, shipped) + **managed-agents** (`ao agent`) to run the loop out of session. None of it is AgentOps-owned; the substrate dispatches a whole operating-loop run (the `ao rpi`/`ao evolve` verbs since removed in 3.2) as one unit and never drives its insides.

## What stays (the in-session core)

`crank`, `swarm`, `ao doctor`, `ao goals`, and the `.agents/` corpus remain in the default in-session core. The `ao rpi`/`ao evolve` verbs were removed in 3.2 (restore: `AGENTOPS_LEGACY=1 make build`), and the corpus/flywheel commands (`ao corpus`, `ao harvest`, `ao mind`, …) moved behind the `flywheel` build tag (restore: `make build-flywheel`). The in-session loop is the product, and it is proven end-to-end.

## One-line summary

If you ran AgentOps **in a session**, nothing you rely on changed. If you ran the **daemon** for always-on work, move that lane to a substrate (NTM + MCP + managed-agents) or Olympus — AgentOps itself is now in-session only.
