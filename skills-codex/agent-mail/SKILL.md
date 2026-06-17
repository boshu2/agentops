---
name: agent-mail
description: "Use Agent Mail from Codex for file leases, notifications, inboxes, and conflict prevention."
---

# agent-mail (Codex)

Codex-native entry point for the `agent-mail` operator skill.

The AgentOps source skill `../../skills/agent-mail/SKILL.md` is the source of truth
for domain behavior, commands, examples, references, and output expectations.
Read it first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant source references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Keep durable task state, evidence, and closure in BR/beads; use Agent Mail as the side channel for leases and pings.
- Verify command syntax from local `--help` or checked-in references before acting.
- Health-check with `am robot health` (CLI/direct SQLite — works without the HTTP server). The `curl …:8765/health` form only answers when the HTTP MCP server is running; CLI-only deploys have no `:8765` listener.
- Fork maintenance: `am` is a fork (`boshu2/mcp_agent_mail_rust`); pull upstream via `make fork-status`/`fork-preview`/`fork-sync` in `~/dev/mcp_agent_mail_rust` (never rebase main by hand). Divergence facts → FORKS-MAP F-3.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
