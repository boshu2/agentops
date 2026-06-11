---
name: eval-outcomes
description: Grade agent or model output against Outcomes for holdout-safe evals and runtime comparisons.
skill_api_version: 1
practices:
- continuous-delivery
- ddd
hexagonal_role: supporting
consumes:
- validate
- ratchet
- council
produces:
- skills/council/schemas/verdict.json
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: execution
  dependencies: [validate, ratchet]
  stability: experimental
output_contract: "skills/council/schemas/verdict.json (one council verdict record)"
---

# eval-outcomes — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/eval-outcomes/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).
