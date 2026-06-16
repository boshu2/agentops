---
name: cc-hooks
description: Run hook policy workflows.
---

# cc-hooks (Codex)

Codex-native entry point for the `cc-hooks` operator skill.

The AgentOps source skill `../../skills/cc-hooks/SKILL.md` is the source of truth
for domain behavior, commands, examples, references, and output expectations.
Read it first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant source references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.

## Opt-in guards

- **Installed-skill-edit guard** — a PreToolUse `Edit|Write` guard that routes an
  edit of an installed skill copy (`*/.claude/skills/**`, `.codex`, `.gemini`)
  back to the repo source of truth `skills/<name>/`. Ships inert; activate with
  `scripts/install-installed-skill-edit-guard.sh`. See
  `references/INSTALLED-SKILL-EDIT-GUARD.md`.

  **Value-proof:** on each fire the guard appends exactly one gate-blind JSONL
  line — `{ts, session, token_class, path_sha256}` — to
  `${AGENTOPS_HOME:-~/.agentops}/guardrail-telemetry.jsonl`. The path is SHA-256
  hashed, never raw (privacy); nothing is written on the happy path; the sensor
  is inert until installed. Pre-registered methodology (metric = declining
  fire-ATTEMPT rate over time; min N; null-at-small-N acceptable) satisfies
  ADR-0002 l.58. See `references/GUARDRAIL-VALUE-PROOF.md`.
