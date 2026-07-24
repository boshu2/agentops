---
name: plan
description: Shape the existing bead or caller intent and
---
# Plan

Turn the caller's intent into one bounded, testable behavior in the place that
already owns the work, then mint exactly one snapshot of the resolved bytes.
Prefer the caller's tracker when one exists; otherwise use supplied issue or
conversation bytes. Later phases receive only the snapshot reference and
expected SHA-256 digest. They never re-read the living source.

## Workflow

1. Resolve the source and choose one active behavior. Apply any authorized
   refinement in that source before freeze. Otherwise return a typed proposed
   amendment and stop without pretending it was accepted.
2. Name the work type and ground truth from **Ground-truth routing**. Inspect
   only enough real context to make interfaces, paths, effects, and evidence
   concrete.
3. Give each acceptance criterion a stable, unique ID. Record important
   non-goals, scope classes, and caller-declared optional exclusions. A
   required criterion cannot be excluded.
4. Name the first useful acceptance check. Enumerate generated companions,
   parity twins, and tests that assert on the changed surface before freeze.
5. Mint the exact resolved bytes once with
   `python3 skills/plan/scripts/mint_intent.py --source <source>
   --intent-dir <dir>`. Retain both `intent_ref` and `intent_digest`, then
   freeze the IDs, statement digests, scope classes, and prior exclusions in
   `scope-index.v1`.

Planning produces no AgentOps plan packet and no campaign graph. The
content-addressed snapshot is runtime-derived identity, not a model-authored
restatement. Whitespace, Unicode normalization, or serialization changes create
a different digest and therefore a different intent.

## Scope admission

Write scope limits authorized mutation; it never limits observation. Implement
observes the repository root with only the kernel's narrow runtime-artifact
exclusions. An unrelated mutation therefore remains visible and fails scope.

For generated surfaces, write scope names a class: hand-edited sources plus all
outputs of the owning regeneration commands. Hand enumeration is falsified
when a generator rewrites an unlisted companion. If a live consumer or
generated twin is discovered after freeze, the current invocation stops; Plan
does not silently amend or mint another intent.

## Ground-truth routing

| Work type | Ground truth | Control experiment | Deviation ledger |
|---|---|---|---|
| Integrate an external substrate, runtime, tracker, or service | vendor docs plus stock behavior | pinned vanilla quickstart with zero local code, before design | every deviation from documented flow and every local replacement for a native component |
| Extend this project | existing repository patterns and behavior spec | simplest version satisfying acceptance, and why it is insufficient | each new abstraction, dependency, or pattern |
| Greenfield | reference experience and domain prior art | walking skeleton | each deviation from the boring default |

The integration control applies only to integration-class work. Routine feature
work uses the Extend row and its normal behavior-first RED-to-GREEN discipline.

Plan is done only when a cold context can act from the exact snapshot without
the planning conversation or mutable source. Move any missing execution fact
into the source before the single mint.
