---
name: codex-exec
description: |
  Drive worker/validator agents with `codex exec` on the ChatGPT Pro subscription (OAuth) — never an API-billed path.

  Triggers: "codex exec", "spawn a codex worker", "run codex non-interactively", "codex validator", "headless codex", "codex resume", "drive codex from a script", "factory worker on Codex".

  Use when:
  - Dispatching a non-interactive Codex worker or validator from a loop, script, or NTM pane
  - Resuming a prior Codex session to continue work
  - Piping a prompt into Codex via stdin from an orchestrator

  Perfect for: factory/loop turns that need a non-Claude vendor lane on the Pro sub.
  Not ideal for: interactive pair-coding (use the `codex` TUI) or Claude work (use NTM panes / subagents — never `claude -p`).
---

# codex-exec (Codex)

Codex-native parity wrapper. The full skill content — overview, critical
constraints, the three-phase workflow, output spec, exit codes, quality rubric,
examples, and troubleshooting — lives in the sibling base file `../SKILL.md`.
Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
