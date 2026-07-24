---
name: implement
description: 'Execute one bounded RED to GREEN experiment from an exact intent snapshot; freeze the candidate and factual receipts. Triggers: "implement", "build this plan", "run the experiment".'
practices:
- tdd
- refactoring
- small-batch-flow
hexagonal_role: driving-adapter
consumes:
- intent-snapshot.sha256
- scope-index.v1
produces:
- subject-manifest.v2
- check-receipt.v1
- effect-receipt.v1
context_rel:
- kind: customer-of
  with: plan
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: execution
  dependencies: []
  capabilities: [execute_one_experiment, collect_factual_evidence]
  effects: [modify_declared_subject, derive_subject_manifest]
  canonical_status: canonical
  disposition: keep
---

# Implement

Execute exactly one bounded experiment from the frozen intent. Implement owns
subject edits and factual receipts. It does not revise acceptance, create a
candidate packet, retry, or judge meaning.

## Workflow

1. Consume the pre-minted intent snapshot with its expected digest. Never read
   or reserialize the living source. Read acceptance IDs and scope classes from
   the frozen `scope-index.v1`.
2. Before editing, derive `subject-manifest.v2` over the repository root. Only
   narrow runtime-owned intent, verdict, and report stores may be excluded;
   write scope is not an observation boundary.
3. Run the declared first check. RED-first applies to behavioral acceptance.
   Relocations, documentation, and pure refactors record an honest green
   pre-change baseline instead.
4. Make the smallest authorized change satisfying the active behavior. Stop on
   a live consumer outside frozen scope; do not amend scope, revise intent, or
   start a repair candidate inside this invocation.
5. Run targeted checks and persist `check-receipt.v1` facts bound to the
   observed subject. A reported exit code without a typed receipt is a claim.
6. Refactor only while the same acceptance check remains green. Self-audit for
   mocks, placeholders, TODO stubs, and hard-coded fixture values standing in
   for real behavior.
7. Derive the final `subject-manifest.v2` with the identical observation
   policy. Derive changes, deletions, generated companions, undeclared paths,
   check references, and completeness in `effect-receipt.v1`; the model never
   transcribes those facts.
8. Freeze the candidate. Return durable before/final manifests, receipts, and
   author context identity. Any later subject mutation is terminal. Stop.

## Evidence proportionality

During editing, run the smallest deterministic checks that can falsify the
active behavior. Reuse exact-input receipts while their subject and tool
identity still match. Run expensive integration checks at the integration
boundary or when acceptance explicitly requires them.

## Boundary

- Do not commit, push, claim, close, release, land, reserve, retry, revise
  intent, or invoke a semantic validator.
- Do not silently expand acceptance or scope.
- A failed check is evidence for the caller, not permission to create another
  candidate or validation loop.
