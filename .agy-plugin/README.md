# AgentOps Antigravity Core Plugin

This bundle packages the AgentOps core operator skills for Google
Antigravity. It is intentionally separate from the canonical source corpus:
the source of truth remains `skills/<name>/SKILL.md`; `.agy-plugin/skills/*`
is the installable AGY bundle.

Contents:

- `plugin.json`: AGY plugin manifest. The `skills` field points to the bundled
  portable `SKILL.md` set, and `mcpServers.agent-mail` points at
  `am serve-stdio`.
- `mcp_config.json`: sidecar MCP payload for Antigravity/Gemini config import.
- `hooks.json`: sidecar hooks payload carrying the AgentOps destructive-command
  guard and a non-mutating closeout/evidence surface check.
- `skills/*/SKILL.md`: the core AgentOps tool/operator skills copied from the
  canonical source tree.

Validate before install:

```bash
bash scripts/validate-agy-plugin.sh
```

Install locally:

```bash
agy plugin validate .agy-plugin
agy plugin install .agy-plugin
agy plugin enable agentops-antigravity-core
```

Gemini CLI uses the same portable `SKILL.md` files directly via
`gemini skills install` or `gemini skills link`; no content conversion is
required for Gemini or AGY.
