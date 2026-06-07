---
name: repeatedly-apply-skill
description: 'Iteratively apply a named skill or slash command N times with progressive deepening. Use when "apply 10 times", "keep improving", "run again", iterative polish, improvement loop, or multi-pass refinement.'
---

# repeatedly-apply-skill (Codex)

Codex-native entry point for the `repeatedly-apply-skill` operator skill.

The sibling source skill `../SKILL.md` is the source of truth for domain
behavior, commands, examples, references, and output expectations. Read it
first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant sibling references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.
