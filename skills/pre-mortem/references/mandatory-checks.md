# Mandatory Council Checks

> Extracted from pre-mortem/SKILL.md on 2026-04-11.
>
> These checks run during or alongside the council validation step. Steps 2.4–2.8 are documented here to keep SKILL.md within its line budget.

## Step 2.4: Temporal Interrogation (`--deep` and `--temporal`)

**Included automatically with `--deep`.** Also available via `--temporal` flag for quick reviews.

Walk through the plan's implementation timeline to surface time-dependent risks:

| Phase | Questions |
|-------|-----------|
| **Hour 1: Setup** | What blocks the first meaningful code change? Are dependencies available? |
| **Hour 2: Core** | Which files change in what order? Are there circular dependencies? |
| **Hour 4: Integration** | What fails when components connect? Which error paths are untested? |
| **Hour 6+: Ship** | What "should be quick" but historically isn't? What context is lost overnight? |

Add to each judge's prompt when temporal interrogation is active:

```
TEMPORAL INTERROGATION: Walk through this plan's implementation timeline.
For each phase (Hour 1, 2, 4, 6+), identify:
1. What blocks progress at this point?
2. What fails silently at this point?
3. What compounds if not caught at this point?
Report temporal findings in a separate "Timeline Risks" section.
```

**Auto-triggered** (even without `--deep`) when the plan has 5+ files or 3+ sequential dependencies.

**Retro history correlation:** When `.agents/retro/index.jsonl` has 2+ entries, load the last 5 retros and check for recurring timeline-phase failures. Auto-escalate severity for phases that caused issues in prior retros.

Temporal findings appear in the report as a `## Timeline Risks` table. See [temporal-interrogation.md](temporal-interrogation.md) for the full framework.

## Step 2.5: Error & Rescue Map (Mandatory for plans with external calls)

When the plan introduces methods, services, or codepaths that can fail, the council packet MUST include an Error & Rescue Map. If the plan omits one, generate it during review.

Include in the council packet as `context.error_map`:

| Method/Codepath | What Can Go Wrong | Exception/Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-----------------|----------|---------------|-----------|
| `ServiceName#method` | API timeout | `TimeoutError` | Y/N | Retry 2x, then raise | "Service unavailable" |

**Rules:**

- Every external call (API, database, file I/O) must have at least one row
- `rescue StandardError` or bare `except:` is always a smell — name specific exceptions
- Every rescued error must: retry with backoff, degrade gracefully, OR re-raise with context
- For LLM/AI calls: map malformed response, empty response, hallucinated JSON, and refusal as separate failure modes
- Each GAP (unrescued error) is a finding with severity=significant

See [error-rescue-map-template.md](error-rescue-map-template.md) for the full template with worked examples.

## Step 2.6: Council FAIL Pattern Check (Mandatory)

Evaluate the plan against the top 8 council FAIL patterns (see [council-fail-patterns.md](council-fail-patterns.md)): missing mechanical verification, self-assessment, context rot, propagation blindness, plan oscillation, dead infrastructure activation, missing rollback map, and four-surface closure gap. Each pattern violation is a finding with severity based on the calibration table in the reference.

Add to each judge's prompt:

```
COUNCIL FAIL PATTERN CHECK: Review this plan for the top 8 council FAIL patterns:
1. Missing mechanical verification — are all gates automated?
2. Self-assessment — is validation external to the implementer?
3. Context rot — are phase boundaries enforced with fresh sessions?
4. Propagation blindness — is the full change surface enumerated?
5. Plan oscillation — is direction validated before propagation?
6. Dead infrastructure activation — does the plan provision anything without activation tests?
7. Missing rollback map — does any production-state change lack a rollback procedure?
8. Four-surface closure — does the plan address Code + Docs + Examples + Proof for every feature?
Report FAIL pattern findings in a "FAIL Pattern Risks" section.
```

**Auto-triggered** for all plans (both `--quick` and `--deep` modes).

## Step 2.7: Test Pyramid Coverage Check (Mandatory)

Validate that the plan includes appropriate test levels per the test pyramid standard (`test-pyramid.md` in the standards skill).

Check each issue in the plan:

| Question | Expected | Finding if Missing |
|----------|----------|--------------------|
| Does any issue touching external APIs include L0 (contract) tests? | Yes | severity=significant: "Missing contract tests for API boundary" |
| Does every feature/bug issue include L1 (unit) tests? | Yes | severity=significant: "Missing unit tests for feature/bug issue" |
| Do cross-module changes include L2 (integration) tests? | Yes | severity=moderate: "Missing integration tests for cross-module change" |
| Are L4+ levels deferred to human gate (not agent-planned)? | Yes | severity=low: "Agent planning L4+ tests — these require human-defined scenarios" |
| For any skip/dedup/consumed/idempotency/regression guard test, does the fixture round-trip the **real persisted shape** (not a hand-built in-memory constructor)? | Yes | severity=significant: "Guard-test fixture uses a shape production never emits — false-green risk (cf. ag-mjlg / PR #652)" |
| Does any guard marker (`consumed`/`skip`/`dedup`) get set at the granularity the on-disk artifact uses (batch/parent/envelope vs item)? | Yes | severity=significant: "Guard marker set at item-level when persisted artifact marks it at batch-level" |

Add to each judge's prompt when test pyramid check is active:

```
TEST PYRAMID CHECK: Review the plan's test coverage against the L0-L7 pyramid.
For each issue, verify:
1. Are the right test levels specified? (L0 for boundaries, L1 for behavior, L2 for integration)
2. Are there gaps where tests should exist but aren't planned?
3. Are any agent-autonomous levels (L0-L3) missing from code-change issues?
Report test pyramid findings in a "Test Coverage Gaps" section.
```

**Auto-triggered** when any issue in the plan modifies source code files (`.go`, `.py`, `.ts`, `.rs`, `.js`).

## Step 2.8: Input Validation Check (Mandatory for enum-like fields)

When the plan introduces or modifies fields with a bounded set of valid values (enums, tier names, mode strings, status codes), verify the plan includes validation logic.

| Question | Expected | Finding if Missing |
|----------|----------|--------------------|
| Does every new enum-like field have a validation guard? | Yes | severity=significant: "No validation for enum field — invalid values pass silently" |
| Is there a defined fallback for unrecognized values? | Yes | severity=moderate: "No fallback behavior specified for invalid input" |
| Are valid values defined as a constant set (not inline strings)? | Yes | severity=low: "Valid values are inline strings — extract to named constant set" |

**Auto-triggered** when the plan introduces struct fields with comments mentioning valid values, config fields with bounded options, or string fields parsed from user input.

## Step 2.9: Regex Scope Predicate Check (Mandatory when plan introduces a regex/glob/grep filter)

When the plan introduces a regex, glob, or grep pattern that classifies inputs into "in scope" / "out of scope" (goal gates that scan files, lint rules that classify code, orchestrators that filter work, search/inject filters), the plan MUST enumerate ≥3 positive cases and ≥3 negative cases.

| Question | Expected | Finding if Missing |
|----------|----------|--------------------|
| Does the plan list ≥3 positive cases the predicate MUST match? | Yes | severity=significant: "Regex predicate has no positive case list — risk of too-narrow first iteration" |
| Does the plan list ≥3 negative cases the predicate MUST NOT match? | Yes | severity=significant: "Regex predicate has no negative case list — risk of too-broad first iteration (false positives)" |
| Does the implementation include a unit test covering both lists? | Yes | severity=moderate: "Regex predicate has cases listed in plan but no unit test — predicate semantics drift after first edit" |

**Auto-triggered** when any plan issue mentions: a goal gate scanning `scripts/**`, `docs/**`, or any glob; a regex assigned to a variable; a `grep -E` invocation in a new gate or lint script; an orchestrator that filters which files to dispatch; a search filter that decides which records to surface.

See [scope-predicate-positive-negative-cases.md](scope-predicate-positive-negative-cases.md) for the full rationale, the C3 incident that motivated this check, and the pseudocode-fix template.
