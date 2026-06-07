---
name: fuzz-test-design
description: |-
  Use when designing fuzz, property, randomized, or corpus-based tests and replaying failures.
  Triggers:
practices:
- property-based-testing
- chaos-testing
- tdd
hexagonal_role: supporting
consumes:
- repo-context
- test-target
- failure-report
produces:
- fuzz-test-plan
- minimized-repro
- regression-test
- ci-budget
context_rel: []
skill_api_version: 1
user-invocable: false
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
  stability: stable
  dependencies: []
output_contract: skills/fuzz-test-design/skill.spec.json
---

# Testing Fuzzing

Use this skill when the work calls for fuzz tests, property tests, randomized scenario tests, model-based tests, or replayable failure reduction. The goal is not "more random tests"; it is to expose input classes and state transitions that example tests miss, then turn each useful failure into a deterministic regression.

## Operating Rules

1. Start with the target boundary. Identify the parser, serializer, validator, protocol handler, state machine, scheduler, allocator, CLI surface, API contract, or workflow being stressed.
2. State the risk. Name the defect class the randomized test is meant to find: panic, crash, data loss, invalid state transition, auth bypass, resource blow-up, nontermination, incorrect round trip, incompatible decode, or divergent implementation behavior.
3. Define the oracle before the generator. Random input without a verdict function creates noise.
4. Record every random seed, corpus path, command, timeout, and environment detail needed to reproduce a failure.
5. Minimize failures before fixing them. Keep the minimized case as a regression test and, when useful, as a seed corpus entry.

## Oracle Selection

Choose one or more oracles that can be evaluated automatically:

- Invariant oracle: properties that must always hold, such as sorted output, balance conservation, monotonic counters, valid UTF-8, authorization boundaries, or schema validity.
- Round-trip oracle: encode/decode, parse/print/parse, serialize/deserialize, import/export, or normalize/idempotence checks.
- Metamorphic oracle: transformed input should preserve or predictably transform output, such as reordering independent records or adding irrelevant whitespace.
- Differential oracle: compare independent implementations, old and new code paths, optimized and reference versions, or local and remote validators.
- Stateful model oracle: compare command sequences against a small model of allowed states and transitions.
- Safety oracle: the target must not panic, crash, leak secrets, hang, exhaust memory, corrupt state, or accept invalid privileged actions.

If no strong oracle exists, build a smoke oracle first: no crash, bounded runtime, valid error shape, and deterministic replay. Mark it as weak and add a follow-up to improve it.

## Test Design Workflow

1. Inventory target inputs.
   List public input types, file formats, request bodies, CLI args, environment variables, stored records, message frames, generated code, and internal command sequences.

2. Partition the input space.
   Cover valid inputs, invalid inputs, edge cases, boundary sizes, nested structures, duplicate fields, unknown versions, missing fields, mixed encodings, extreme numbers, time zones, concurrency interleavings, and resource limits.

3. Choose the testing style.
   Use property tests for pure or mostly deterministic functions. Use coverage-guided fuzzing for parsers, decoders, protocol handlers, and unsafe boundaries. Use randomized scenario tests for workflows, schedulers, stores, and state machines. Use model-based tests when command ordering matters.

4. Build generators deliberately.
   Prefer structured generators over raw bytes when the target consumes structured data. Mix valid and invalid cases. Keep constraints narrow enough to reach meaningful code and broad enough to discover missed classes. Add shrinking or minimization support where the framework allows it.

5. Seed the corpus.
   Include hand-picked examples, historical incidents, production-shaped fixtures, boundary cases, minimized regressions, and small representative valid files. Deduplicate corpus entries and avoid committing large or sensitive data.

6. Run in layers.
   Start with short local runs to validate the harness, then increase iterations, coverage time, input size, and parallelism. Keep each command replayable with an explicit seed or corpus artifact.

7. Triage each failure.
   Classify it as product bug, test bug, oracle bug, generator bug, environment flake, or resource-budget issue. Do not merge randomized tests that fail without a replay path.

8. Convert discoveries into durable tests.
   Add a focused regression test for every real bug. Add the minimized input to the corpus only when it improves future discovery or protects a format boundary.

## Corpus Management

Keep corpora small, intentional, and reproducible.

- Store source-controlled seeds under an existing test fixture or fuzz corpus convention for the project.
- Put large generated corpora in CI artifacts or external storage, not in the normal source tree.
- Name corpus entries by behavior or issue when possible, not by random hashes alone.
- Scrub secrets, customer data, credentials, and host-specific paths.
- Add corpus entries only after deduplication and minimization.
- Preserve replay commands next to failure reports or in test comments when the framework does not embed them.

## Minimization And Replay

Every useful randomized failure needs a path back to determinism:

- Capture seed, framework version, command, target, timeout, platform, and corpus revision.
- Minimize the input with the framework shrinker, reducer, or a manual delta-debugging loop.
- Re-run the minimized input at least twice before changing production code.
- Add a deterministic regression that does not depend on broad random search.
- Keep the broad fuzz/property harness after the regression lands; the regression guards the found bug, the harness searches for the next class.

## CI Budget

Use budgets that match the gate:

- Pull request smoke: fixed seed or checked-in corpus, short timeout, deterministic replay, low parallelism, no network dependency.
- Main branch or nightly: longer duration, coverage growth, corpus refresh, sanitizer builds where applicable, artifact upload for failures and new corpus candidates.
- Release hardening: targeted longer runs on high-risk parsers, migration paths, compatibility boundaries, unsafe code, concurrency-sensitive state machines, and security-relevant validators.

Randomized tests in CI must have bounded runtime, stable replay, and failure artifacts. If a harness is valuable but too expensive for pull requests, wire a small smoke target into PR CI and move the deep run to a scheduled gate.

## Output Format

Return or commit the smallest useful set of artifacts:

- Target and risk summary.
- Oracle matrix with property, verdict, and expected failure signal.
- Generator strategy and corpus policy.
- Commands, seeds, iteration counts, timeouts, and CI budgets.
- Failure triage with minimized replay command when a failure is found.
- Deterministic regression tests for confirmed bugs.
- Residual risk and recommended next fuzzing budget.

## Completion Checklist

- The oracle is explicit.
- The generator covers valid, invalid, and boundary-shaped inputs.
- Seeds and corpus entries are small, scrubbed, and justified.
- Every discovered failure has a minimized replay path.
- Confirmed bugs have deterministic regression tests.
- CI runs are bounded and split between pull-request smoke and deeper scheduled fuzzing when needed.
