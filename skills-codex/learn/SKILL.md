---
name: learn
description: Consume an immutable Validate verdict and
---
# Learn

> **Purpose:** Bookkeep what an immutable verdict observed and return a bounded
> learning receipt to the orchestrator.

## Critical Constraints

- The input verdict is immutable. Because Learn cannot grade its own input, bind
  it by `input_verdict_ref` and `input_verdict_digest`; never author, edit,
  reinterpret, or replace its value.
- Consume only structured observations already present in the verdict because
  missing evidence remains missing; Learn does not manufacture a lesson.
- Classify bookkeeping outcomes such as `record`, `candidate`, or `no_change`,
  but do not promote a rule or alter the remaining plan in this mode.
- Postmortem is optional and runs only for retrospective causal analysis. Learn
  may request that specialization; the caller decides whether to invoke it.
- Because proof, repository, tracker, delivery, and Premortem ports have
  separate owners, emit observations plus one Learn receipt without operating
  those authorities.
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

- **Artifact directory:** the invocation root plus `.agents/rpi/`.
- **Filename convention:** `learn-receipt.json` and `phase-4-summary.md`.
- **Serialization/schema format:** JSON follows
  [learn-receipt.schema.json](schemas/learn-receipt.schema.json); summary is Markdown.
- **Validator command:** `bash skills/learn/scripts/validate.sh`.
- **Downstream handoff:** the orchestrator consumes the receipt and alone decides
  whether to continue, re-plan, stop, or route a causal-analysis request.

## Quality Checklist

- [ ] The receipt binds the immutable verdict reference and digest.
- [ ] Every observation is copied without semantic mutation.
- [ ] Every disposition remains bookkeeping rather than promotion.
- [ ] The phase summary and receipt pass the validator command.

Executable behavior is in [learn.feature](references/learn.feature). The
post-verdict ownership map is in
[post-verdict-actions.md](references/post-verdict-actions.md).
