---
name: system-performance-remediation
description: 'Restore machine responsiveness via safe, selective process cleanup. Use when system unresponsive, high CPU/load average, IO pressure, filesystem cache bloat, memory pressure from btrfs/ext4, stuck tests, competing cargo builds, confused agents in loops, swap thrashing, disk full, systemd-oomd kills, or tmux/zellij session sprawl.'
---

# system-performance-remediation (Codex)

Codex-native entry point for the `system-performance-remediation` operator skill.

The sibling source skill `../SKILL.md` is the source of truth for domain
behavior, commands, examples, references, and output expectations. Read it
first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant sibling references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
