---
name: validate
description: Freshly judge one frozen intent and exact
---
# Validate

Independently judge one frozen subject against the exact pre-minted intent,
write one durable verdict, and stop. Validate is the sole verdict writer. It
never reconstructs Plan or Candidate packets or re-reads a living intent
source. `verdict.v2` is an immutable legacy read format; all new judgments use
`verdict.v3`.

## Preconditions

- The pre-minted intent snapshot and expected exact-byte digest are supplied.
- The frozen criterion/scope index, before/final manifests, typed receipts,
  invocation ID, and author context ID are supplied.
- The final manifest matches the subject at validation start.
- Validator context identity and freshness are explicitly attested.

Missing, colliding, or unattested identities are `NOT_PROVEN`. This is a
declared trust fact, not cryptographic proof that contexts were isolated.

## Cross-model fresh validator

A caller may request a different validator model through an explicitly selected
runtime adapter. Record author and validator model identity in receipt notes.
If the requested adapter is unavailable, disclose the unsatisfied diversity
request and proceed in a distinct same-model context. Never invoke a forbidden
runtime. One fresh validator remains the default.

## Mutating-check quarantine

Classify every acceptance command as read-only or subject-mutating before
execution. Regen, sync, format, and force modes are mutating until proven
otherwise. Run a required mutating check only against a disposable copy or
stable committed subject, never the judged working tree. A validator-caused
mutation is still candidate mutation and terminates judgment.

## Workflow

1. Consume `--intent-snapshot` with `--expected-intent-digest` through
   `skills/validate/scripts/validate_v3.py`. Never re-snapshot a live source.
   Any byte drift, including whitespace or Unicode normalization, is terminal
   `NOT_PROVEN`.
2. Verify the before/final `subject-manifest.v2` artifacts and derive complete
   repository-wide changes from their identical observation policy. Recompute
   the final manifest at validation start and end. Mutation after candidate
   freeze is terminal `NOT_PROVEN`.
3. Adjudicate `effect-receipt.v1`, not a declared path list. Repository-wide
   observation includes generated companions and deletions. Proven
   out-of-scope change is `FAIL`; partial observation or incomplete coverage is
   `NOT_PROVEN`.
4. Inspect the exact subject and factual receipts. Re-execute risk-critical,
   uncertain, or insufficiently evidenced checks. Judge exactly the stable IDs
   in `scope-index.v1`. Required IDs cannot become exclusions; an unchecked
   required ID is `NOT_PROVEN`.
5. Choose exactly one semantic result: `PASS`, `FAIL`, or `NOT_PROVEN`. PASS
   requires distinct identities, freshness, complete scope, nonempty checked
   evidence, and a typed receipt for every non-excluded criterion.
6. Load the currently activated proof identity. Bind its contract digest,
   activation transition digest, and the exact verdict/report/manifest/scope/
   receipt schema digests. Refuse a candidate changing
   `docs/contracts/proof-contracts/active.json`; a candidate proof contract
   cannot activate or judge itself.
7. Persist one canonical `verdict.v3` with
   `validate_v3.py store-verdict`, using the supplied invocation and unique
   judgment IDs. Reject a second unlinked judgment for the same invocation or
   exact intent/final-subject pair. Never re-snapshot intent during storage.
8. Persist durable manifests, receipts, verdict, and `rpi-report.v2`. Return the
   verdict path and digest. Stop.

Artifact digests are SHA-256 over canonical JSON with `artifact_digest`
omitted. Writes use same-directory temporary files, flush, fsync, and atomic
rename. Identical content is idempotent. A conflicting path or duplicate
judgment is an integrity failure and stops the invocation.

## Proof-contract transition

A proof candidate never activates itself. After a candidate at epoch `N+1`
receives a `PASS` verdict.v3 under the exact active epoch `N` identity, an
external caller may run `scripts/record_proof_transition.py`. The recorder
holds a compare-and-swap lock, requires every candidate component and the next
recorder to be exact members of the judged subject, writes a content-addressed
transition, rechecks the candidate, subject, and active pointer, then replaces
the active pointer last. It accepts only an active epoch of at least 1; the
frozen bootstrap recorder remains the sole epoch 0 to 1 path.

## Freshness without duplication

Fresh validation means independent judgment over the exact subject. It does not
require replaying every author command. Verify intent identity, schema and proof
bindings, scope, receipt digests, and every required criterion. A
digest-bound deterministic receipt may prove routine facts; rerun checks whose
risk or uncertainty makes supplied evidence insufficient.

## Boundary

Validate emits no WARN, confidence, disposition, learning, owner, next action,
repair, retry, replan, helper, escalation, tracker, Git, release, closure, or
delivery state. Generic provenance may record a verdict later but cannot change
its validity.
