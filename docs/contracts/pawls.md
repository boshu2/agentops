# Retired One-Way-Door Gate Contract

> **Status:** Retired as active authority on 2026-07-14. The path remains only
> until the checked K9 consumer-deletion leaf removes it. Do not cite this file
> as lifecycle, validation, escalation, or Git authority.

## Why it was retired

The old contract began as a short reminder to obtain fresh review before an
irreversible action. It grew into a second lifecycle with separate verdict
schemas, model routing, retries, queue admission, push enforcement, and tracker
closure. That duplicated Discovery, Validate, the run governor, and repository
delivery. It also placed expensive semantic work at Git time, where upstream
movement forced repeated proof and stalled flow.

## Current authority

The [Operating Loop](../architecture/operating-loop.md) owns the complete state
model:

- Discovery shapes acceptance and consumes Premortem when plan risk warrants it;
- Crank implements one bounded tranche and freezes one candidate;
- Validate records one immutable PASS or FAIL from author-distinct fresh
  context, backed by deterministic evidence;
- Learn records one minimal consequence;
- the orchestrator classifies NOTE, REPAIR, REPLAN, HOLD, or ANDON; and
- the consumer repository owns delivery for local and cloud agents.

Ordinary check failures and REFUTED work are REPAIR or REPLAN. A no-progress or
oscillation breaker enters HOLD and receives one bounded fresh-context helper.
Only helper ESCALATE, human-only judgment, or a genuinely spent hard ceiling
becomes ANDON. Retry count alone is not an operator escalation.

## Deletion ownership

K9 removes this file's executable consumers, model-driving commands, schemas,
scripts, embedded assets, and dedicated tests in one exact candidate. K7
separately removes repository-delivery and queue machinery. Until those leaves
execute, executable leftovers are transitional facts and must not be used as
new design precedent.

## Historical record

The original text remains recoverable from Git history. Active docs and code do
not preserve a compatibility alias or restoration promise for it.
