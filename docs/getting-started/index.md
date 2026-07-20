# Getting started

AgentOps runs as skills inside a coding agent — Claude Code, Codex, Cursor, or
any runtime that loads skills. Install the skills into your agent
(`npx skills@latest add boshu2/agentops --all -g`, a runtime plugin, or a
source checkout), then invoke RPI **in that agent's chat** with one behavior:

```text
/rpi Add an edge-case-safe parser for the supplied format.
```

The expected evidence is the exact resolved intent source, a runtime-derived
subject manifest and check receipts, one fresh independent verdict, and the
final RPI report. The loop stops after reporting.

For direct use, invoke Plan, Implement, or Validate separately. Keep the same
resolved intent bytes across phases, let the runtime derive subject identity and
changed paths, and give Validate the exact subject facts and check receipts. See
the [skill router](../SKILL-ROUTER.md) and [first value
path](../first-value-path.md).
