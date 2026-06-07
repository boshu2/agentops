---
name: agent-mail
description: 'MCP Agent Mail for multi-agent coordination. Use when agents need file locks, messaging, inboxes, or conflict prevention. Handles macro_start_session, file_reservation_paths, send_message, threading, pre-commit guards.'
---

# agent-mail (Codex)

Codex-native entry point for the `agent-mail` operator skill.

The sibling source skill `../SKILL.md` is the source of truth for domain
behavior, commands, examples, references, and output expectations. Read it
first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant sibling references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
