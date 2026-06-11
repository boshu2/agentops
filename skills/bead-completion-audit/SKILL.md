---
name: bead-completion-audit
description: |-
  Use when auditing closed beads for real shipped evidence, acceptance proof, and truthful closeout.
  Triggers:
skill_api_version: 1
user-invocable: false
practices:
- design-by-contract
- evidence-over-assertion
- cmm-process-maturity
hexagonal_role: supporting
consumes:
- closed-beads
produces:
- compliance-report.md
context_rel:
- kind: customer-of
  with: beads-br
- kind: supplier-to
  with: post-mortem
metadata:
  tier: judgment
  dependencies:
  - beads-br
  stability: experimental
context:
  window: fork
---

# bead-completion-audit — moved to Mount Olympus (2026-06-10)

This skill encodes independent-verdict machinery and now lives with the outer
gate product. Canonical: `~/dev/mt-olympus/.claude/skills/bead-completion-audit/SKILL.md` —
read and follow that file. This stub preserves fleet routing until the
using-agentops catalog closer updates the registry (skill-prune Lane A,
evidence/skill-prune-recon.md).
