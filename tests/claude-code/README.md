# Claude Code Functional Tests

These tests exercise live Claude Code CLI behavior against the AgentOps plugin.
They are intentionally outside the default fast lane and are reached through
`tests/run-all.sh --tier=3` or direct invocation.

Run the suite:

```bash
bash tests/claude-code/run-all.sh
```

When `claude` is absent from `PATH`, the suite exits successfully with a
`SKIPPED` message.

Cost and runtime guards:

- `MAX_BUDGET_USD`: max budget per Claude invocation; defaults to `1.00`.
- `MAX_TURNS`: max agentic turns per invocation; defaults to `3`, with a few
  complex tests overriding to `5`.
- `DEFAULT_TIMEOUT`: default timeout per invocation; defaults to `120` seconds.
- `ALLOWED_TOOLS`: optional comma-separated tool allowlist; when unset, tests use
  `--dangerously-skip-permissions` for plugin access.
