---
name: cross-vendor-trust-gate
description: "Run the skill-factory final trust gate: operate trust-gate.sh, read skill.trust.json, and enforce --require-cross."
practices:
- measure-before-land
- evidence-driven-iteration
- cross-vendor-parity-as-a-gate
hexagonal_role: domain
consumes:
- skill
produces:
- trust-artifact
- stdout
context_rel:
- kind: shared-kernel
  with: heal-skill
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  external_dependencies:
  - trust-gate.sh
  - jq
output_contract: 'artifacts: skill.trust.json (trust_level + trust_score) + a pass/fail verdict by exit code'
user-invocable: false
---

# cross-vendor-trust-gate — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/cross-vendor-trust-gate/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).
