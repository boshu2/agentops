---
name: eval-outcomes
description: Grade agent or model output against Outcomes for holdout-safe evals and runtime comparisons.
  Also the fold target for the retired scenario skill — Manage holdout scenarios; author and
  manage holdout scenarios with measurable acceptance vectors and satisfaction scoring in
  .agents/holdout/ for behavioral validation.
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

## Folded-In Trigger Surface (scenario)

eval-outcomes is the fold target for the retired standalone `scenario` skill
(skill-prune phase 2). Fire this skill for its use-cases:

- **Scenario — Manage holdout scenarios.** Author and manage holdout scenarios
  for behavioral validation: scenarios define **what** the system should do in
  narrative form, with measurable acceptance vectors and satisfaction scoring.
  They live in `.agents/holdout/*.json` so implementing agents cannot see them
  during development. When asked to author, manage, or score holdout scenarios,
  fire this skill.
