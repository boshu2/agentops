---
name: Factory
kind: concept
status: draft
see-also: [loop, context-compiler]
---
# Factory

The **out-of-session placement** for AgentOps work: a set of NTM-supervised Claude/Codex background sessions that can take work from a bead queue, coordinate through mcp-agent-mail, load skills, and hand results back for operator review. Factory is not an AgentOps daemon and no longer means running `ao rpi`/`ao evolve` unattended.

## The substrate owns it, not AgentOps

AgentOps 3.0 ships **no** always-on daemon, scheduler, or overnight runner — those surfaces were **deleted** in the 3.0 rearchitecture (see [`docs/adr/ADR-0009-daemon-deletion-in-session-only.md`](../../../docs/adr/ADR-0009-daemon-deletion-in-session-only.md)). The Factory driver is the NTM substrate's job. NTM holds the long-lived tmux sessions; mcp-agent-mail holds coordination, reservations, and handoff; MCP (`ao mcp serve`) exposes tools. AgentOps supplies the skills, session profiles, context, validation, and provenance.

## Skill-session dispatch (current target)

On the reference substrate, dispatch is **NTM-driven**: a lead agent or operator runs `bd ready`, assigns the next bead through mcp-agent-mail, reserves files, and starts/resumes a Claude or Codex worker session with the right skills loaded. The worker session calls `ao session bootstrap`, pulls context with `ao inject`, follows the relevant skills, validates, and records provenance. The substrate never re-expresses AgentOps practice as substrate workflow steps.

## When to use

- Use **Factory** for unattended, queue-driven background agents. Do not use it for an AgentOps-shipped daemon; that surface was deleted when out-of-session orchestration moved to NTM.
- The substrate starts and supervises skill sessions. It never drives the session's internal practice steps.

## Bounded context

The Factory *driver* is **substrate-owned (orchestration)**. The *skill session and context* are **AgentOps-owned**. The seam between them is the load-bearing DDD boundary: orchestration (when/where/who-supervises) versus practice/context (what the agent does, how context compounds).

## See also

- `loop.md` — the historical umbrella and vocabulary
- `context-compiler.md` — the context surface background sessions pull from
