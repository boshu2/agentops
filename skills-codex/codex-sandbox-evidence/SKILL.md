---
name: codex-sandbox-evidence
description: |
  Run codex exec under a least-privilege sandbox and emit a machine-checkable JSONL proof surface for the validator.

  Triggers: "codex exec", "codex sandbox", "least-privilege codex", "codex evidence", "codex JSONL proof", "--output-schema", "codex worker dispatch", "read-only codex", "workspace-write", "events.jsonl", "verifiable codex run", "factory/flywheel Codex worker", "harden a codex invocation".

  Use when:
  - Dispatching a Codex worker in the factory/flywheel loop that must produce verifiable evidence, not just a final answer
  - A validator (human or agent) needs to confirm WHAT a Codex run did — its event stream, tool calls, and structured last message
  - Hardening a Codex invocation to read-only / workspace-write so it cannot escape its lane

  Perfect for: ACFS / Mount Olympus worker dispatch where every run must leave a proof artifact on disk.
  Not ideal for: interactive Codex sessions (use the TUI), or Claude work (use NTM panes / subagents — `claude -p` is banned).
---

# codex-sandbox-evidence (Codex)

Codex-native parity wrapper. The full skill content — overview, critical
constraints, the four-phase workflow, output spec, quality rubric, examples, and
troubleshooting — lives in the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
