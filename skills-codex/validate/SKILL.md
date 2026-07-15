---
name: validate
description: Freshly judge exact subject content against
---
# Validate

Independently judge one exact subject against the acceptance in its existing
bead or caller source, write one durable verdict, and stop. Validate is the sole
verdict writer. It never asks the model to reconstruct Plan or Candidate
packets.

## Preconditions

- The intent source is available as a caller-owned artifact or runtime-owned
  content-addressed snapshot; its acceptance digest is derived automatically.
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
2. Confirm the intent-source digest has not changed since implementation. If
   the subject changed or complete changed-path coverage cannot be derived,
   return `NOT_PROVEN`.
3. Compare runtime-derived actual changed paths with the source write scope. A proven
   out-of-scope path returns `FAIL`; incomplete scope evidence returns
   `NOT_PROVEN`.
4. Inspect the exact subject and factual evidence. Judge every acceptance
   criterion and record criterion-level results, findings, evidence references,
   `checked`, and `not_checked`.
5. Choose exactly one semantic result: `PASS`, `FAIL`, or `NOT_PROVEN`. PASS
   requires distinct identities, explicit freshness, nonempty checked scope,
   top-level evidence, and evidence for every criterion.
6. Persist canonical `verdict.v2` with `store-verdict --draft <draft.json>
   --intent-source <resolved-intent> --subject-manifest <manifest.json>
   --author-context-id <id> --scope-result <PASS|FAIL|NOT_PROVEN>`. The helper
   snapshots the exact resolved intent under
   `<workspace>/.agentops/intents/sha256/<digest>.intent`, then computes and
   injects intent and subject digests. Identity and changed-path facts come from
   runtime-derived manifests and receipts, not model transcription. Verdict
   storage defaults to `<workspace>/.agentops/verdicts/sha256/<digest>.json`;
   callers may provide `verdict_dir`.
7. Return the artifact path and digest. Stop.

The digest is SHA-256 over canonical JSON with `artifact_digest` omitted. Writes
use a same-directory temporary file, flush, fsync, and atomic rename. Identical
existing content is idempotent success; conflicting content is an integrity
failure represented by `NOT_PROVEN`.

## Freshness without duplication

Fresh validation means independent judgment over the exact subject. It does not
require mechanically replaying every author command. Verify intent identity,
scope, evidence digests, and every acceptance criterion; independently rerun
the risk-critical, uncertain, or insufficiently evidenced checks. A
digest-bound deterministic receipt may prove routine facts. Replay an expensive
full suite only when acceptance requires that result or the supplied receipt
cannot establish it.

## Boundary

Validate emits no WARN, confidence, disposition, briefing learning, owner,
next action, repair, retry, replan, helper, escalation, tracker, Git, release,
closure, or delivery state. Generic provenance may record a verdict later, but
ledger availability cannot change its validity.
