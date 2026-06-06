---
name: codex-mcp-plugins
description: |
  Wire MCP servers and plugins into the Codex CLI — the harness-native path
  to reach flywheel binaries (Agent-Mail, br/beads) and to ship the AgentOps
  skill bundle to Codex.

  Triggers: "add an MCP server to Codex", "wire Agent-Mail into codex",
  "reach br/beads from codex", "ship the skill bundle to Codex", "install the
  AgentOps plugin in codex", "codex mcp add/list/get/login/logout", "codex plugin
  add/list/marketplace", "skill_mcp_dependency_install", "plugin_sharing",
  "connect Codex to a tool server", standing up a Codex lane in the flywheel.

  Perfect for:
  - Operator setup of a Codex lane in the flywheel (caam profile, bushido node, Mac cockpit)
  - Making a freshly-built MCP server reachable from `codex exec` workers

  Not ideal for:
  - Client-facing AI Partner work (this drives operator binaries — keep it backstage)
  - Claude Code MCP/plugin config (use that harness's own surfaces, not codex CLI)
---

# codex-mcp-plugins (Codex)

Codex-native parity wrapper. The full skill content — overview, constraints, the
four-phase workflow, output spec, quality rubric, examples, and troubleshooting —
lives in the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
