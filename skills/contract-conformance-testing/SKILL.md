---
name: contract-conformance-testing
description: |-
  Use when building conformance tests from specs, contracts, examples, or compatibility matrices.
  Triggers:
skill_api_version: 1
user-invocable: false
practices:
- tdd
- bdd-gherkin
- continuous-delivery
hexagonal_role: supporting
consumes:
- standards
- test
- validation
produces:
- conformance harness plan
- executable conformance cases
- compatibility verdict matrix
context_rel: []
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  dependencies:
  - standards
  - test
  - validation
output_contract: "A conformance harness design with executable cases, fixtures, expected verdicts, version matrix coverage, and validation commands."
---

# Testing Conformance Harnesses

Use this skill to turn an explicit authority set into an implementation-agnostic harness that proves whether one or more implementations conform across cases, platforms, and versions.

## Authority Set

Start by naming the sources that define correct behavior:

- Specifications, protocol documents, schemas, API contracts, or formal requirements.
- Canonical examples, golden transcripts, fixtures, or reference outputs.
- Version notes, compatibility promises, migration guides, and documented exceptions.
- Existing implementation behavior only when it is explicitly accepted as a reference oracle.

State precedence before designing tests. When sources conflict, the higher-precedence source controls and the conflict becomes a harness case or documented exclusion.

## Harness Shape

Build the harness around these pieces:

- **Adapter:** a narrow interface that drives each implementation without embedding product-specific behavior in assertions.
- **Case corpus:** structured cases with id, source citation, version scope, inputs, expected behavior, and tags.
- **Fixture store:** immutable inputs and expected outputs, versioned when behavior changes intentionally.
- **Oracle:** spec assertions, golden examples, reference implementations, differential checks, or metamorphic properties.
- **Runner:** deterministic execution with setup, teardown, retries only for declared nondeterminism, and isolated state.
- **Reporter:** pass, fail, skip, xfail, and unsupported verdicts with enough evidence to reproduce the result.

Keep harness code separate from implementation code. The same case should be runnable against every compatible implementation through the adapter layer.

## Case Design

Cover the behavior surface deliberately:

- Positive cases for required behavior and canonical examples.
- Negative cases for invalid input, malformed data, permission failures, and unsupported operations.
- Boundary cases for limits, empty values, ordering, precision, encoding, time, and concurrency.
- Regression cases for defects that represent conformance gaps.
- Cross-version cases for behavior that must remain stable, migrate cleanly, or intentionally diverge.
- Matrix cases for implementation, version, platform, feature flag, schema version, and protocol mode.

Each case should include a clear source, expected verdict, and reason. Avoid cases whose expected behavior is "whatever the current implementation does" unless that implementation is the declared oracle.

## Versioning Rules

Model compatibility as data:

- Define the supported version matrix before writing cases.
- Mark each case with the versions or feature states where it applies.
- Use `xfail` for known, accepted nonconformance with an owner and removal condition.
- Use `skip` only when the harness cannot run a case for a declared environmental reason.
- Preserve old fixtures when they represent stable compatibility requirements.

When behavior intentionally changes, add a new expected result scoped to the new version instead of rewriting historical expectations.

## Validation Bar

A conformance harness is ready when:

- The authority set and precedence are written down.
- Every required behavior has at least one case or a documented gap.
- The adapter can run the same corpus against at least one implementation.
- The report distinguishes conformance failures from harness failures.
- Failures include case id, source, inputs or fixture id, observed result, expected result, and reproduction command.
- The harness can be run in local development and in CI without hidden credentials unless the contract explicitly requires them.

## Output

Return the harness design and the concrete next artifact:

- Case matrix with ids, tags, versions, sources, and expected verdicts.
- Adapter interface and runner command.
- Fixture layout and golden update policy.
- Oracle strategy for each behavior class.
- CI command and report format.
- Open gaps, exclusions, and accepted divergences.
