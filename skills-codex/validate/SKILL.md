---
name: validate
description: Freshly and independently judge one exact
---
# Validate

Independently judge one exact subject against one unchanged acceptance contract,
write one durable verdict, and stop. Validate is the sole verdict writer. It
never edits the subject or controls what happens next.

## Preconditions

- A schema-valid PlanPacket and CandidatePacket are supplied.
- The subject manifest still matches the subject.
- Author and validator context IDs are explicit.
- Freshness is explicitly attested with `source: runtime | caller` and an
  attester identity.

Missing, colliding, or unattested identities produce `NOT_PROVEN`. This is a
declared trust fact, not cryptographic proof that contexts were isolated.

## Workflow

1. Recompute and compare `subject-manifest.v1` using
   `python3 skills/validate/scripts/validate.py manifest`. The helper uses only
   filesystem content; Git commit/tree IDs are optional metadata.
2. Confirm acceptance and PlanPacket digests match the candidate. If the
   subject changed or complete changed-path coverage cannot be established,
   return `NOT_PROVEN`.
3. Compare proven actual changed paths with Plan `write_scope`. A proven
   out-of-scope path returns `FAIL`; incomplete scope evidence returns
   `NOT_PROVEN`.
4. Inspect the exact subject and factual evidence. Judge every acceptance
   criterion and record criterion-level results, findings, evidence references,
   `checked`, and `not_checked`.
5. Choose exactly one semantic result: `PASS`, `FAIL`, or `NOT_PROVEN`. PASS
   requires complete proof, distinct identities, and explicit freshness.
6. Persist canonical `verdict.v2` with
   `python3 skills/validate/scripts/validate.py store-verdict`. Default storage
   is `<workspace>/.agentops/verdicts/sha256/<digest>.json`; callers may provide
   `verdict_dir`.
7. Return the artifact path and digest. Stop.

The digest is SHA-256 over canonical JSON with `artifact_digest` omitted. Writes
use a same-directory temporary file, flush, fsync, and atomic rename. Identical
existing content is idempotent success; conflicting content is an integrity
failure represented by `NOT_PROVEN`.

## Boundary

Validate emits no WARN, confidence, disposition, briefing learning, owner,
next action, repair, retry, replan, helper, escalation, tracker, Git, release,
closure, or delivery state. Generic provenance may record a verdict later, but
ledger availability cannot change its validity.
