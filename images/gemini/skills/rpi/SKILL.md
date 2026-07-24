---
name: rpi
description: 'Run one bounded Plan, Implement, and fresh Validate experiment, then report and stop. Triggers: "run rpi", "feed this through the loop", "research-plan-implement".'
practices:
- bdd-gherkin
- tdd
- design-by-contract
hexagonal_role: domain
consumes:
- plan
- implement
- validate
produces:
- rpi-report.v2
context_rel:
- kind: customer-of
  with: plan
- kind: customer-of
  with: implement
- kind: customer-of
  with: validate
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: meta
  dependencies: [plan, implement, validate]
  capabilities: [orchestrate_once, report]
  effects: [dispatch_core_phases]
  canonical_status: canonical
  disposition: keep
output_contract: 'rpi-report.v2 machine artifact plus a concise human-readable interactive summary'
---

# RPI

Run one experiment from the caller's existing intent through three
responsibilities and stop:

```text
Plan -> Implement -> fresh Validate -> report
```

RPI dispatches each core phase at most once. It does not own retries, repair
revisions, budgets, queues, campaigns, claims, leases, Git, delivery, release,
closure, or the caller's next decision. The pure
[`scripts/run_once.py`](scripts/run_once.py) behavior makes exact-byte
transport, phase cardinality, reporting, and stop semantics executable without
Git, `ao`, or a tracker.

## Contract

1. Invoke Plan once with the existing bead or caller intent. Plan returns the
   exact resolved bytes, or no usable intent. The runtime mints those bytes
   once under their SHA-256 digest. If no usable intent can be frozen, report
   `NOT_PLANNED` and stop.
2. Invoke Implement once with only the immutable intent reference and expected
   digest. Implement performs one bounded experiment and returns durable
   before/final manifests plus typed check and effect receipts. If no subject
   is built, report `NOT_BUILT` and stop.
3. Invoke Validate once in a context distinct from the author's. Pass the
   pre-minted snapshot identity, frozen criterion/scope index, exact manifests,
   receipts, invocation ID, validator identity, and freshness attestation.
   Never pass a living intent source for re-derivation.
4. Return the durable `verdict.v3` reference in `rpi-report.v2`. Stop regardless
   of `PASS`, `FAIL`, or `NOT_PROVEN`.

`NOT_PLANNED` and `NOT_BUILT` are report statuses, never semantic verdicts. A
caller or Goal may select another experiment after this invocation stops; RPI
does not carry campaign attempts, continuation envelopes, retry state, or next
actions.

The caller may supply one opaque `correlation` object for external joins. RPI
preserves it byte-for-byte in meaning and never interprets it. The object is
limited to eight scalar string entries, 64-character keys, 256-character
values, and 2048 canonical JSON bytes; oversize or nested data is rejected.

## Invariants

- Plan, Implement, and Validate are each dispatched zero or one time.
- Every phase uses the SHA-256 of the same exact snapshot bytes. Whitespace,
  Unicode normalization, or serialization changes produce a different intent.
- Repository-wide observation derives all actual changes, including generated
  companions and deletions. Partial observation is `NOT_PROVEN`.
- Proven change outside frozen write-scope classes is `FAIL`.
- Frozen required criterion IDs cannot be reclassified as exclusions.
- Candidate mutation after Implement freezes the final manifest is terminal.
- Invocation and judgment identities are unique. A second unlinked judgment
  over the same exact intent/final-subject pair is rejected.
- Opaque correlation is size-bounded, preserved on every terminal report, and
  grants no campaign or continuation authority.
- PASS requires nonempty distinct author and validator context IDs, freshness,
  checked scope, and receipt-backed evidence for every required criterion.
- The verdict binds the activated proof identity and exact schema digests. A
  candidate proof contract cannot activate or judge itself.
- Optional strategies, specialists, factories, trackers, and runtimes do not
  alter phase order or core outcomes. Learn remains an optional later consumer.

Delegate only the frozen intent identity and established facts needed by a
lane, not orchestration conversation. Shared writer or regeneration surfaces
serialize; that is runtime safety, not RPI lifecycle state.

## Report

RPI has two surfaces:

1. **Machine artifact:** return or persist the exact
   [`rpi-report.v2`](../../schemas/rpi-report.v2.schema.json).
2. **Interactive response:** lead with status and one sentence describing the
   caller-visible outcome, then only the strongest proof, material unchecked
   scope, and a clickable verdict reference.

The machine artifact remains behind the report link. Emit full JSON only when
the caller requests machine-readable output or an adapter consumes it. Do not
append a next action. The caller owns continuation.
