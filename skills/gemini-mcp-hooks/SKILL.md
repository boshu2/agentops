---
name: gemini-mcp-hooks
description: |-
  Use when wiring MCP servers, hooks, and scoped tool policy into the Gemini CLI image.
  Triggers:
practices:
- data-contracts
- least-privilege
hexagonal_role: driven-adapter
consumes:
- mcp-server
- hook-policy
produces:
- gemini-mcp-config
- gemini-hook-config
context_rel:
- kind: customer-of
  with: gemini-native
skill_api_version: 1
user-invocable: false
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  dependencies: [agent-mail, beads-br, dcg]
output_contract: "A verified Gemini MCP/hook setup with before/after list output, enabled server names, and any hook migration notes."
---

# gemini-mcp-hooks

Connect Gemini CLI to the AgentOps tool substrate. Use `gemini mcp` for tool
servers such as Agent Mail or bead bridges, and `gemini hooks migrate` only when
you intentionally port an existing hook policy into Gemini's hook system.

Ground truth on this host: `gemini mcp` supports `add`, `remove`, `list`,
`enable`, and `disable`; `gemini hooks` currently exposes `migrate`.

## Critical Constraints

- **List before and after.** Run `gemini mcp list` before mutation and after
  mutation. **Why:** the effective tool surface is the list output, not the
  command you thought you ran.
- **Add only scoped servers.** Register only the MCP servers the Gemini task
  actually needs. **Why:** Gemini can call registered tools; unnecessary servers
  widen blast radius.
- **Disable before removing when uncertain.** Prefer `gemini mcp disable <name>`
  as a reversible first step. **Why:** removing a working server destroys
  configuration that may belong to another lane.
- **Hook migration is not a blind port.** `gemini hooks migrate` can translate
  another runtime's hook shape, but the resulting policy must be inspected.
  **Why:** hook events and trust semantics differ by runtime.
- **Do not inline secrets in commands or tracked files.** Use env-var backed MCP
  configuration when a token is required. **Why:** runtime config is easy to
  leak across machines and repos.

## Quick Start

```bash
gemini mcp list
gemini mcp add agent-mail <command-or-url> <args...>
gemini mcp enable agent-mail
gemini mcp list
```

For hook migration:

```bash
gemini hooks migrate
# inspect generated settings before relying on them
```

## Workflow

### Phase 1: Inventory current access

Run:

```bash
gemini mcp list
```

Record server names, enabled state, and any unknown servers before changing the
image.

### Phase 2: Add or enable a server

Use `gemini mcp add <name> <commandOrUrl> [args...]` for new servers. Use
`enable`/`disable` for existing servers. Keep names stable and descriptive.

Checkpoint: the server appears in `gemini mcp list` with the expected enabled
state.

### Phase 3: Migrate hooks only when needed

Run `gemini hooks migrate` when moving an existing hook policy into Gemini.
Inspect the generated config and verify the hook still enforces the intended
boundary.

Checkpoint: hook policy is understood in Gemini terms, not merely converted.

### Phase 4: Smoke test the tool surface

Run a read-only Gemini prompt with `--approval-mode plan` asking it to inspect
available context or call the intended safe read operation. Capture output and
exit code.

## Output Specification

Return:

- before/after `gemini mcp list`
- server names added, enabled, disabled, or removed
- hook migration path and review notes
- smoke-test command and exit code
- rollback commands

## Quality Rubric

- MCP access is scoped to the task.
- Hook migration is inspected, not assumed correct.
- Secret material is not stored in tracked files.
- Rollback is explicit.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Server missing from Gemini | Add failed or wrong config scope | Re-run `gemini mcp list`, then add/enable |
| Tool calls fail | Server command/url wrong | Test server outside Gemini, then update |
| Hook blocks valid work | Blind migration mismatch | Inspect migrated policy and narrow the rule |
| Unknown server appears | Shared local config | Disable first, then ask before removing |

## See Also

- [gemini-skills-extensions](../gemini-skills-extensions/SKILL.md)
- [gemini-native](../gemini-native/SKILL.md)
