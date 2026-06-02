# Migrating to AgentOps 3.0

3.0 is a deliberate narrowing. AgentOps is now the **skill-session** operating layer and context compiler: skills, `ao` support commands, crank/swarm patterns, provenance, and the `.agents/` corpus. Everything that tried to be an always-on orchestration runtime has been removed. Out-of-session orchestration is NTM background agents: Claude/Codex tmux sessions supervised by NTM, coordinated through mcp-agent-mail, and equipped with MCP/`ao` tools. AgentOps owns the skills and context, not a daemon.

This guide lists what was removed in 3.0 and what to use instead.

## Removed: all hooks (the repo is hookless)

`ao hooks install`, `--with-hooks`, and the embedded hook surface are gone. AgentOps no longer installs or runs hooks.

**Use instead:** skills + the `ao` CLI guide the workflow; **CI is the authoritative gate** (`.github/workflows/validate.yml`). What a PreToolUse/PostToolUse hook used to enforce locally, a CI job now enforces on push. Skill sessions call `ao session bootstrap`, `ao inject`, `ao validate`, provenance, and corpus commands directly instead of relying on hook side effects.

## Removed: the daemon (`ao daemon` / `agentopsd`)

The always-on daemon — the queue, supervisor, scheduler, job executors, and the dream/overnight compounding runner — is deleted. AgentOps has no out-of-session runtime.

**Use instead:**
- **In-session:** run a skill-guided Claude/Codex session yourself. This is the zero-dependency path; it needs no extra binary beyond the runtime, git, and the recommended `ao`/`bd`.
- **Out-of-session (always-on):** keep Claude/Codex sessions warm under **NTM**. A lead agent or operator slings ready beads, mcp-agent-mail carries assignments/reservations/check-ins, and the worker sessions inherit AgentOps skills. MCP via `ao mcp serve` exposes the tool surface to MCP-aware harnesses.
- Or use **Olympus** for a full-custom out-of-session implementation.

## Removed: scheduling (`ao schedule`, `ao plans`, `ao watch`, `ao overnight`)

These drove the daemon's cron lane and are gone with it. `docs/scheduling.md` and the `.agents/schedule.yaml` contract no longer apply.

**Use instead:** schedule or supervise NTM background sessions, or run the work in-session on demand. Background sessions can run corpus maintenance skills/commands (`ao compile`, `ao maturity`) when explicitly assigned.

## Removed: the factory command (`ao factory`) and its contract corpus

**Use instead:** the factory is NTM background agents — Claude/Codex skill sessions kept ready by NTM and coordinated through mcp-agent-mail. There is no separate factory binary.

## Removed: the `runtime=gc` phased-engine bridge

The CLI gc-bridge glue is severed. Legacy `ao rpi` keeps its non-gc backends (`auto`/`direct`/`stream`/`tmux`) for compatibility; `runtime=gc` is no longer a valid mode. New background-agent work should start NTM-supervised skill sessions rather than driving the phased engine through any bridge.

## New in 3.0

- **`ao validate --gate`** — deterministic exit-code verdict (0 pass/warn, 1 fail, 2 internal error; `--strict` flips WARN→1). The retry hook for a substrate's `check` step and for CI; composes the existing ratchet validator, no network or LLM.
- **The reference out-of-session substrate** — AgentOps uses **NTM** + **mcp-agent-mail** + **MCP** (`ao mcp serve`, shipped) to run background skill sessions. NTM supervises Claude/Codex agents; mcp-agent-mail coordinates work; AgentOps supplies skills, context, validation, and provenance.

## What stays (the in-session core)

Skills, `crank`, `swarm`, `ao harvest`, `ao forge`, `ao mine`, `ao compile`, `ao wiki`, `ao doctor`, `ao goals`, and the `.agents/` corpus stay. `ao rpi` and `ao evolve` remain compatibility surfaces while the active product direction moves to skill sessions.

## One-line summary

If you ran AgentOps **in a session**, keep using skills and `ao` support commands. If you ran the **daemon** for always-on work, move that lane to NTM background agents coordinated by mcp-agent-mail — AgentOps itself is now skills + context + validation + provenance, not a daemon.
