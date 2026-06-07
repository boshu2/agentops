---
name: gemini-skills-extensions
description: |
  Install, link, enable, disable, and validate AgentOps skills and Gemini CLI
  extensions for the Gemini image.

  Triggers: "gemini skills install", "gemini skills link", "Gemini extensions",
  "install AgentOps into Gemini", "Gemini skill bundle", "gemini extensions
  validate", "Gemini image setup", "link local skills into Gemini".
practices:
- release-engineering
- docs-as-code
hexagonal_role: driven-adapter
consumes:
- skill-bundle
- gemini-extension
produces:
- gemini-skill-install
- gemini-extension-install
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
  dependencies: []
output_contract: "A verified Gemini skill/extension install or link, with list/validate output and rollback commands."
---

# gemini-skills-extensions

Manage the Gemini CLI distribution surface for AgentOps: `gemini skills` for
agent skills and `gemini extensions` for broader packaged capabilities. Use this
when bringing the Gemini image up to parity with the Claude, Codex, and AGY
images.

Ground truth on this host: `gemini skills` supports `list`, `enable`,
`disable`, `install`, `link`, and `uninstall`; `gemini extensions` supports
`install`, `uninstall`, `list`, `update`, `disable`, `enable`, `link`, `new`,
`validate`, and `config`.

## Critical Constraints

- **Prefer `link` for local development.** Use `gemini skills link <path>` or
  `gemini extensions link <path>` while iterating. **Why:** edits are reflected
  immediately and avoid stale installed copies.
- **Validate extensions before install.** Run `gemini extensions validate
  <path>` for local extension trees. **Why:** malformed package metadata breaks
  the image at activation time.
- **List after every mutation.** Run `gemini skills list --all` or `gemini
  extensions list` after install/link/enable/disable. **Why:** the command exit
  is not enough; confirm the runtime discovery surface.
- **Keep source of truth in AgentOps.** Do not edit managed copies under
  `~/.gemini/skills` as the durable source. **Why:** installed runtime copies
  drift and are hard to review.
- **Record rollback.** Every install or link should name the matching
  `uninstall`/`disable` command. **Why:** image setup must be reversible.

## Quick Start

```bash
gemini skills list --all
gemini skills link /Users/bo/dev/agentops/skills/gemini-native
gemini skills list --all

gemini extensions validate /path/to/extension
gemini extensions link /path/to/extension
gemini extensions list
```

## Workflow

### Phase 1: Inspect the image

Run:

```bash
gemini skills list --all
gemini extensions list
```

Checkpoint: record what is already installed, linked, enabled, and disabled.

### Phase 2: Choose install vs link

Use `link` for local AgentOps development and `install` for released artifacts or
remote repositories. Do not mix both for the same skill/extension without first
removing the old one.

### Phase 3: Apply the change

For a skill:

```bash
gemini skills link /path/to/skill
gemini skills enable <name>
```

For an extension:

```bash
gemini extensions validate /path/to/extension
gemini extensions link /path/to/extension
gemini extensions enable <name>
```

### Phase 4: Verify and record rollback

Run list commands again and save the output. Record rollback:

```bash
gemini skills disable <name>
gemini skills uninstall <name>
gemini extensions disable <name>
gemini extensions uninstall <name>
```

## Output Specification

Return:

- target name and path/source
- commands run and exit codes
- before/after list output
- rollback commands
- any disabled or conflicting older install

## Quality Rubric

- Local development uses link, not stale copies.
- Extension trees validate before activation.
- The installed/enabled state is verified after mutation.
- Rollback commands are included.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Skill does not appear | Wrong source path or disabled state | Run `gemini skills list --all`, then enable |
| Extension install fails | Invalid package metadata | Run `gemini extensions validate <path>` |
| Edits are not reflected | Installed copy instead of linked source | Uninstall, then `link` the source path |
| Two copies conflict | Installed and linked variants coexist | Disable/uninstall one copy |

## See Also

- [gemini-native](../gemini-native/SKILL.md)
- [gemini-mcp-hooks](../gemini-mcp-hooks/SKILL.md)
