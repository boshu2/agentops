---
name: codex-goals
description: |
  Drive Codex's native iterate-until-done loop with the stable Goals feature — define the objective once, let Codex own the loop.

  Triggers: "codex goals", "iterate until done", "codex autodev", "define the goal and let it run", "Codex-native operating loop", "the codex /goals slash command", "first-class loop primitive for codex".

  Use when:
  - You want a Codex-native AUTODEV/operating loop: state the objective once, Codex drives iterate → check → continue until done.
  - Replacing a hand-rolled bash while-loop or `codex exec` re-prompt harness with a first-class loop primitive.
  - Running an unattended/background Codex worker against a durable objective on the flywheel.

  Perfect for: operator-side flywheel/factory workers (NTM panes, spawned codex-exec jobs) converging on a stated outcome.
  Not ideal for: client-facing content (operator-side only), or open-ended exploration with no done-condition.
---

# codex-goals (Codex)

Codex-native parity wrapper. The full skill content — overview, critical
constraints, the five-phase workflow, output spec, quality rubric, examples, and
troubleshooting — lives in the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
