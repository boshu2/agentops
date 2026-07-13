---
name: learn
description: Capture bounded observations after an
---
# Learn

> **Purpose:** Turn a completed Validate verdict into a bounded, inspectable
> learning handoff without reopening proof or taking delivery authority.

## Critical Constraints

- Consume an existing verdict artifact; never author, edit, or reinterpret its
  PASS/WARN/FAIL value.
- Emit observations and one Learn phase receipt only. Do not commit, push,
  close tracker work, or decide whether delivery proceeds.
- Keep every observation linked to evidence. An unsupported lesson is omitted,
  not promoted by confidence or repetition in prose.
- Return `DONE` only when the receipt and phase summary validate. Missing input
  produces `BLOCKED`; partial observation coverage produces `PARTIAL`.

## Workflow

1. Resolve the immutable Validate verdict and its artifact digest or commit.
2. Extract only evidence-backed observations that could change future work.
3. Write `learn-receipt.json` using [the receipt schema](schemas/learn-receipt.schema.json).
4. Write `.agents/rpi/phase-4-summary.md` with intent, evidence, observations,
   constraints, and next action.
5. Append the ordered RPI receipt:

```json
{
  "phase": "learn",
  "skill": "learn",
  "status": "DONE",
  "artifact": ".agents/rpi/phase-4-summary.md"
}
```

The executable scenarios are in [references/learn.feature](references/learn.feature).
S3 owns the deeper Validate/Learn authority boundary; this slice establishes
the receipt and handoff shape only.

## Output Specification

- **Artifact:** `learn-receipt.json` plus `.agents/rpi/phase-4-summary.md`.
- **Schema:** [schemas/learn-receipt.schema.json](schemas/learn-receipt.schema.json).
- **Validator:** `bash skills/learn/scripts/validate.sh`.
- **Downstream:** RPI may report or re-plan after recording the Learn receipt;
  delivery remains outside this skill.

## Quality Checklist

- [ ] Input verdict reference is nonempty and unchanged.
- [ ] Observations cite evidence and do not mutate proof.
- [ ] Phase, skill, status, and artifact identify the fourth umbrella.
- [ ] The validator exits zero before RPI reports completion.
