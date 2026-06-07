---
name: implementation-pattern-mining
description: |-
  Use when mining repeated codebase patterns and turning them into reusable implementation guidance.
  Triggers:
skill_api_version: 1
user-invocable: false
hexagonal_role: supporting
practices:
- pragmatic-programmer
- docs-as-code
- hexagonal-architecture
consumes:
- codebase
- implementation-examples
- tests
produces:
- pattern-inventory
- implementation-guidance
- convention-candidates
context_rel:
- kind: partnership
  with: research
- kind: partnership
  with: standards
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: judgment
  stability: experimental
  dependencies:
  - research
  - standards
output_contract: A concise implementation-pattern report with evidence-backed repeated patterns, reusable conventions, counterexamples, and guidance that names where the convention applies and where it does not.
---

# Implementation Pattern Mining

Use this skill to identify repeated implementation patterns in a codebase,
extract reusable conventions, and turn them into guidance without overfitting.
The goal is not to write a style guide from taste. The goal is to describe
implementation moves that the codebase already repeats for good reasons.

## When To Use

Use this when a task asks for codebase conventions, local patterns, reusable
implementation guidance, onboarding notes for how code is built here, or a
pattern inventory before implementing similar work.

Do not use this to justify a preferred design that is not already present in the
codebase. Do not promote one example into a convention unless you can explain
why it generalizes.

## Mining Workflow

1. Define the scope.
   Name the subsystem, language, framework, and artifact types being mined.
   Keep the scope narrow enough that examples are comparable.

2. Find candidate repetitions.
   Use repository search and code navigation to collect examples of recurring
   implementation moves: module boundaries, constructors, adapters, validation,
   error handling, persistence, tests, configuration, CLI flags, API shapes, and
   naming schemes.

3. Build evidence sets.
   For each candidate pattern, collect at least three independent examples when
   possible. If there are only two examples, mark the result as a weak signal
   unless those examples are central, recent, and deliberately parallel.

4. Separate convention from coincidence.
   Ask what stays stable across the examples and what varies. Preserve only the
   stable implementation move as the convention. Treat names, ordering, helper
   choice, and file layout as incidental unless the evidence shows they carry
   meaning.

5. Check counterexamples.
   Search for code that solves the same problem differently. A counterexample
   does not kill the pattern by itself; it defines the boundary where the
   convention applies, reveals migration drift, or exposes a competing local
   convention.

6. Extract guidance.
   Turn each accepted pattern into a short rule of use: when to apply it, how to
   implement it, which example is the clearest exemplar, what to avoid, and what
   would be overfitting.

7. Verify against likely future work.
   Test the guidance against a plausible new implementation. If following the
   guidance would force irrelevant details from the examples, narrow the rule.

## Pattern Quality Bar

A mined pattern is strong when it has repeated independent examples, a clear
reason to exist, consistent behavior, and useful boundaries. It should help a
future implementer make fewer local-design decisions without copying irrelevant
details.

A mined pattern is weak when it is based on one file, one author's habit, old
code that newer code no longer follows, generated output, tests that intentionally
use odd shapes, or examples that share only superficial structure.

## Overfitting Guardrails

- Prefer behavior and intent over syntax trivia.
- Name confidence: strong, medium, or weak.
- Include counterexamples and explain whether they are exceptions, drift, or a
  separate pattern.
- Avoid universal words like always and never unless the repository provides
  broad evidence.
- Preserve useful variation. If examples differ safely, the guidance should say
  what can vary.
- Do not require new abstractions just because examples share a shape.

## Report Format

Return a compact report with these sections:

```markdown
# Implementation Pattern Mining

Scope: <subsystem, language, artifact types>

## Pattern Inventory

### <Pattern Name>

Confidence: strong|medium|weak
Evidence:
- <file or symbol>: <what it shows>
- <file or symbol>: <what it shows>

Convention:
<one or two sentences describing the reusable implementation move>

Guidance:
<how to apply it to new work>

Boundaries:
<where it applies, where it does not, and known counterexamples>

Overfitting Risk:
<details from the examples that should not be copied blindly>

## Candidate Patterns Not Promoted

- <candidate>: <why evidence was insufficient or too context-specific>
```

## Handoff Rules

When handing the result to an implementer, keep the guidance executable:
reference concrete examples, state the confidence level, and call out what is
optional. If the output will become durable documentation, remove weak patterns
or label them explicitly as observations rather than conventions.
