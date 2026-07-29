---
name: goals
description: 'Compatibility alias — renamed to fitness. Use fitness to measure declared project fitness goals. Triggers: "goals" (deprecated).'
practices:
- dora-metrics
hexagonal_role: domain
consumes: []
produces: []
context_rel:
- kind: alias-of
  with: fitness
skill_api_version: 1
user-invocable: true
metadata:
  capabilities: []
  effects: []
  canonical_status: canonical
  disposition: keep_off_path
  tier: product
  dependencies: []
output_contract: none — delegates to fitness
---
# Goals — compatibility alias for fitness

This skill was renamed to `fitness` on 2026-07-29. Everything it did lives
there unchanged; the `ao goals` CLI command family keeps its name.

When invoked, apply `skills/fitness/SKILL.md` exactly as written — this
alias adds no behavior, grants no authority, and produces nothing of its
own. It exists so existing references and habits keep resolving, and it is
deliberately not advertised as a destination.

Physical deletion follows the observed-zero policy: this alias is removed
only after a declared observation window shows no remaining use.
