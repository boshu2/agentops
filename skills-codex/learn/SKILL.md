---
name: learn
description: Consume an immutable Validate verdict
---
# Learn

> **Purpose:** Bookkeep what an immutable verdict observed and return a bounded
> learning receipt to the orchestrator.

## Critical Constraints

- The input verdict is immutable. Bind it by `input_verdict_ref` and
  `input_verdict_digest`; never author, edit, reinterpret, or replace its value.
- Consume only structured observations already present in the verdict. Missing
  evidence remains missing; Learn does not manufacture a lesson.
- Classify bookkeeping outcomes such as `record`, `candidate`, or `no_change`,
  but do not promote a rule or alter the remaining plan in this mode.
- Postmortem is optional and runs only for retrospective causal analysis. Learn
  may request that specialization; the caller decides whether to invoke it.
- Emit observations plus one Learn receipt. Do not operate proof, repository,
  tracker, delivery, or Premortem authority.
- `DONE` requires a schema-valid receipt and phase summary. Unreadable proof is
  `BLOCKED`; incomplete bookkeeping is `PARTIAL`.

## Workflow

1. Resolve the Validate verdict, verify its schema, and compute or confirm its
   SHA-256 digest.
2. Copy structured observations into the Learn receipt without changing their
   `kind`, `summary`, or `evidence_ref`.
3. Record a bounded disposition for each observation: `record`, `candidate`, or
   `no_change`. This is bookkeeping, not promotion.
4. If an explicit retrospective causal question exists, emit a Postmortem
   request as `next_action`; do not perform the retrospective inline.
5. Write `learn-receipt.json` and `.agents/rpi/phase-4-summary.md`.
6. Append the ordered RPI completion receipt:

```json
{
  "phase": "learn",
  "skill": "learn",
  "status": "DONE",
  "artifact": ".agents/rpi/phase-4-summary.md"
}
```

## Output Specification

- **Artifacts:** `learn-receipt.json` and `.agents/rpi/phase-4-summary.md`.
- **Schema:** [learn-receipt.schema.json](schemas/learn-receipt.schema.json).
- **Validator:** `bash skills/learn/scripts/validate.sh`.
- **Downstream:** the orchestrator consumes the receipt and alone decides
  whether to continue, re-plan, stop, or route a causal-analysis request.

Executable behavior is in [learn.feature](references/learn.feature). The
post-verdict ownership map is in
[post-verdict-actions.md](references/post-verdict-actions.md).
