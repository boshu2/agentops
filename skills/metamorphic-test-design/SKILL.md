---
name: metamorphic-test-design
description: |-
  Use when designing metamorphic tests for oracle-poor behavior using invariants and input relations.
  Triggers:
practices:
- pragmatic-programmer
skill_api_version: 1
hexagonal_role: supporting
metadata:
  tier: judgment
  stability: stable
  dependencies: []
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
output_contract: skills/metamorphic-test-design/skill.spec.json
user-invocable: false
---

# Metamorphic Testing

Use this skill when a normal assertion is weak because the exact expected output is unavailable, too expensive to compute, or too brittle to maintain. The goal is to test relations between executions: if the input is transformed in a controlled way, the output must transform, remain stable, or preserve a property. Treat invariants as contracts about what must survive the transform.

## Inputs

Collect the behavior under test, its input domain, known preconditions, available seed examples, and the risk you need evidence for. If an exact oracle exists for a narrow part of the domain, keep it as a calibration check, but do not make the plan depend on exact outputs everywhere.

## Relation Selection

Choose relations that encode a real contract, not a convenient coincidence. Prefer relations that are explainable to a domain owner and cheap enough to run many times.

Common relation families:

- Invariance: formatting, ordering of independent inputs, or harmless metadata changes must not alter the relevant result.
- Equivalence: two different representations of the same meaning must produce equivalent outputs.
- Round trip or inverse: encode then decode, normalize then parse, or apply an inverse transform and recover the original property.
- Monotonicity: increasing an input, permission, budget, or threshold cannot decrease a corresponding output property.
- Decomposition and aggregation: solving parts independently and combining them must match solving the whole where the domain permits it.
- Idempotence: applying normalization, repair, deduplication, or canonicalization twice must match applying it once.
- Commutativity or associativity: operation order must not matter when the domain says it should not matter.
- Scaling or shifting: numeric outputs should change predictably when units, offsets, or scale factors change.
- Differential relation: two independent implementations, modes, or APIs must agree on a shared property even if their exact formats differ.

Reject a relation if it depends on undocumented behavior, hides a known lossy step, or only passes because the generator avoids hard cases.

## Generator Design

Start with seeds that cover normal, boundary, and historically broken inputs. Add transformations that preserve the chosen relation and state each transform's precondition.

For each generator, define:

- Seed source: fixtures, production-shaped examples, reduced bug cases, or structured random builders.
- Transform: the exact mutation applied to the seed.
- Guard: conditions that must hold before the relation is valid.
- Observable: the output property compared across executions.
- Shrink strategy: how to reduce a failing pair to the smallest seed and transform that still fails.

Make generators produce paired or grouped cases, not isolated inputs. The harness should record the seed, transform, relation name, guard result, compared observables, and any tolerances used.

## Harness Pattern

Structure each test as source execution, transformed execution, relation assertion. Keep relation logic separate from input generation so failed cases can be replayed deterministically.

Recommended test shape:

1. Build or load a seed input.
2. Apply one named transform.
3. Skip only when the relation guard is false.
4. Execute both inputs through the same public behavior.
5. Compare the declared observable, using explicit tolerance for nondeterministic or numeric systems.
6. Persist the minimal failing seed and transform metadata when the assertion fails.

## Failure Triage

When a relation fails, triage in this order:

1. Guard failure: the precondition was wrong or incomplete.
2. Generator failure: the transform created an invalid or unintended input.
3. Observation failure: the compared property is too broad, too narrow, or includes unstable data.
4. Tolerance failure: the relation needs an explicit numeric, timing, ordering, or concurrency tolerance.
5. Relation failure: the supposed invariant is not a product contract.
6. Product failure: the implementation violates a real relation and needs a fix.

Record the decision. A fixed test should explain whether the change repaired the system, narrowed the generator, changed the guard, or replaced the relation.

## Output Format

Return a concise test design with:

- Risk statement: the behavior and why exact expected outputs are hard.
- Relation matrix: relation name, input transform, guard, observable, assertion, and risk covered.
- Generator plan: seed source, transform strategy, boundary cases, replay method, and shrink strategy.
- Harness notes: framework hooks, determinism controls, tolerances, and artifact capture.
- Failure triage rubric: how to classify relation failures before changing product code.

## Quality Bar

A good metamorphic test fails for a meaningful contract violation, not for incidental representation differences. It runs often enough to find regressions, logs enough metadata to reproduce a failing relation, and keeps each relation narrow enough that one failure tells the maintainer what kind of contract broke.
