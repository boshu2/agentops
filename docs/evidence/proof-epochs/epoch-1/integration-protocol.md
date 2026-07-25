---
id: skill-system-overhaul-integration-protocol
proof_epoch: 1
status: active
plan_ref: docs/plans/2026-07-24-skill-system-overhaul.md
duel_ref: docs/audits/skill-system-overhaul-duel-2026-07-24/README.md
fable_program_review_sha256: 147f114a9950ec4033f7d1e7561d35a5d4efcfceacf6de30c7351e0deaf23453
---

# Skill-system overhaul integration protocol

This protocol turns the frozen tranche intents into safe landing order. It
does not revise their acceptance, grant lifecycle authority, or replace their
fresh verdicts.

## Evidence roles

1. A lane author may produce a candidate and factual check receipts.
2. An author-distinct review may approve or reject code quality. A review is
   advisory and cannot write the semantic verdict.
3. Only a fresh Validate context may write `PASS | FAIL | NOT_PROVEN`.
4. A lane verdict does not automatically compose after cherry-pick or rebase.
   Binding tranche judgment is over the exact post-integration subject.
5. A landing gate cited as evidence must have a durable, subject-bound record.
   An orchestrator message or `/tmp` report alone cannot gate landing.

Every candidate descriptor and verdict records distinct author and validator
context identities. Validators re-derive manifests, changed paths, receipts,
and criterion evidence; they do not transcribe author reports.

## Serialized landing order

Each numbered step leaves a checkable resting state. A failed step stops before
the next mutation.

1. **T2 publisher:** obtain a zero-finding author-distinct review and fresh
   exact-subject verdict for the final compiler, strict reader, and
   transactional publisher. T2 must also close any compiler/schema defect that
   prevents a skill from truthfully distinguishing evidence-artifact writes
   from candidate-subject mutation; a compiling declaration that hides a real
   write or grants excess mutation authority is not acceptable. Land the T2
   source and durable review evidence.
1a. **T1 kernel contract-v3 completion:** after the final T2 grammar is present,
   author truthful `contract_v3` declarations and owned proofs for `plan`,
   `implement`, `rpi`, and `validate`. All four are required: a partial kernel
   cannot satisfy the 49-skill cutover matrix. Obtain a zero-finding
   author-distinct review and a fresh exact-subject verdict before continuing.
2. **Go G0-G2 repair:** preserve every prior FAIL, validate the final repair,
   land it, and rerun the exact integrated Go lifecycle, race, vet, lint, and
   supported cross-build checks. The known-failed Go landing tip is not an
   acceptable resting state.
3. **T3 product/campaign:** validate and land the frozen T3 subject plus every
   subject-bound rejection and repair-intent record.
4. **T7b neutral-contract foundation:** land only the runtime-neutral model
   dispatch contract under `docs/contracts/model-dispatch.md`. Do not retire
   `shared` at this step.
5. **T4, T5, T6, and T7a source tranches:** land one source-only tranche at a
   time and obtain a fresh verdict over each exact integrated subject. T4 owns
   the `council` and `idea-genie` consumer repoints. T7a owns
   `agent-native`, `codex-exec`, and `ntm` adapter mechanics and consumer
   repoints. No peer skill may import another canonical skill's private proof
   code.
6. **T7b support and retirement:** after the T4/T7a repoints resolve, land the
   remaining support contracts, conformance test, compatibility tombstone, and
   semantic `shared` retirement. Physical deletion still waits for the
   declared observed-zero window.
7. **T8 cutover:** only after all 49 source skills have fresh tranche lineage,
   execute the one global contract/catalog activation, transactional
   publication, completion matrix, and final fresh exact-subject verdict.
8. **Main:** reconcile the validated subject with current `origin/main`
   without content mutation, rerun the required final gates, then verify local
   and remote `main` identity.

## Cross-lane ownership

The T7b neutral-contract foundation is deliberately separate because its
consumers live in other tranche scopes.

| Surface | Owner |
|---|---|
| `docs/contracts/model-dispatch.md` | T7b foundation |
| `tests/integration/test_multi_model_dispatch.bats` | T7b final support tranche |
| `skills/council/**`, `skills/idea-genie/**` consumer references | T4 |
| `skills/agent-native/**`, `skills/codex-exec/**`, `skills/ntm/**` adapter mechanics and consumer references | T7a |
| `skills/shared/**` compatibility and retirement behavior | T7b final support tranche |

The foundation does not claim that a consumer has migrated. T4 or T7a cannot
claim the neutral contract exists until the exact foundation content is in its
judged subject. T7b cannot claim all consumers migrated until the exact T4 and
T7a content is in its judged subject.

## Publication and generated surfaces

- Worker lanes never write the registry, catalog, router, readiness ledger,
  Codex/Gemini projections, runtime images, or publication state.
- Source-tranche checks may generate only in disposable state.
- Shared publication is serialized through the reviewed transactional
  publisher.
- T8 performs the single portfolio WRITE, requires the first post-write CHECK
  to be `CLEAN`, repeats WRITE for byte-idempotence, and records the exact
  owner map, manifest, and receipt.
- Unknown residue, path collision, partial ownership, incomplete cleanup, or a
  changed subject stops publication.

## Required platform disclosures

Verdicts state rather than hide unexecuted platform facts, including:

- macOS power-loss durability beyond ordinary `fsync`;
- raw Darwin `waitid` shim behavior;
- Linux process-group behavior on a real restricted `/proc`; and
- live Windows process-tree behavior when only cross-build evidence exists.

An allowed platform exclusion is `not_checked`; it is never converted into a
claim of live execution.

## Stop conditions

Stop integration when any of these is true:

- the candidate or intent digest changed during judgment;
- actual changed-path coverage is incomplete or outside frozen scope;
- a review has an unresolved critical or warning finding;
- a required tranche verdict is not `PASS`;
- any canonical skill cannot truthfully express its authority and effects in
  the active contract grammar;
- a durable gate record is missing;
- a consumer points to an owner that is not present in the judged subject;
- regeneration or publication is dirty, non-idempotent, or incompletely
  recoverable; or
- final landing would change the validated content.
