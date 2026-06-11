---
name: council
description: 'Run multi-judge consensus. Use when: an irreversible or high-stakes decision needs independent judges before committing — architecture forks, one-way doors, scoring options.'
practices:
- llm-eval-harness
- ai-assisted-dev
- design-by-contract
hexagonal_role: domain
consumes:
- standards
produces:
- result.json
- verdict.json
context_rel:
- kind: shared-kernel
  with: standards
skill_api_version: 1
context:
  window: isolated
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  tier: judgment
  dependencies:
  - standards
  replaces: judge
output_contract: skills/council/schemas/verdict.json
---

# council — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/council/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).

## Examples

- `/council should we swap the policy engine to Cedar?` — runs at the canonical
  location; this stub forwards. Read
  `~/dev/mt-olympus/.claude/skills/council/SKILL.md`.

## Troubleshooting

- **Skill seems empty / missing scripts:** the body moved to Mount Olympus
  (2026-06-10). Use the canonical path above; this stub exists only to keep
  fleet routing alive until the catalog closer updates the registry.

## Runtime Contract

Multi-judge runs still bind to the shared Claude runtime surface:
[claude-code-latest-features.md](../shared/references/claude-code-latest-features.md).
