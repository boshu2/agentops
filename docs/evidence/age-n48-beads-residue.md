# age-n48 — retire beads skill residue

**Bead:** age-n48  
**Date:** 2026-06-17

## Fix

- Rewrote `evals/agentops-core/beads-issue-tracking.json` to target `beads-br` (not deleted `skills/beads`).
- Removed MOLECULES.md pointers from plan templates (skills + codex twin).

## Proof

```bash
! test -d skills/beads
! rg 'skills/beads/' evals/agentops-core/beads-issue-tracking.json skills/plan/references/templates.md
```
