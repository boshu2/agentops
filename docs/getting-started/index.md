# Getting started

Give your native coding agent (Claude Code, Codex, Cursor, or any other
runtime) one concrete behavior with acceptance examples. No AgentOps skill
installation, bootstrap or `ao` setup is required. The agent implements the
change and runs the repository's checks. For an ordinary change those checks
and CI are the gate; one fresh author-distinct read happens only when you ask,
a mistake cannot be cheaply undone after it lands, or no deterministic check
covers the changed behavior. The [agent workflow
reference](../agent-workflow-reference.md) states these steps.

Skills are optional guidance. Install them into your agent (the Claude Code or
Codex plugin, or `npx skills@latest add boshu2/agentops` for any other agent)
when you want them. To select the optional RPI workflow, invoke it **in that
agent's chat** with one behavior:

```text
/rpi Add an edge-case-safe parser for the supplied format.
```

RPI keeps the exact resolved intent source, a runtime-derived subject manifest
and check receipts, and reports its result with checked and not_checked scope.
The invocation stops after reporting.

For direct use, invoke Plan, Implement, or Validate separately. Keep the same
resolved intent bytes across phases, let the runtime derive subject identity and
changed paths, and give Validate the exact subject facts and check receipts. See
the [skill router](../SKILL-ROUTER.md) and [first value
path](../first-value-path.md).
