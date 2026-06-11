---
name: red-team
description: 'Probe docs and skills. Use when: adversarially probing a doc, skill, plan, or claim for weaknesses, gaps, or unstated assumptions before it ships.'
practices:
- ai-assisted-dev
- design-by-contract
- sre
hexagonal_role: supporting
consumes:
- repo-context
produces:
- result.json
context_rel:
- kind: supplier-to
  with: vibe
skill_api_version: 1
metadata:
  tier: judgment
  stability: experimental
  dependencies:
  - council
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
output_contract: skills/council/schemas/verdict.json
---

# red-team — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/red-team/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).
