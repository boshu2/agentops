# Wave 1 Spec Consistency Checklist

Use this checklist after SPEC WAVE and before TEST WAVE.

## Required Per-Contract Checks

For every `contract-<issue-id>.md` produced in the current SPEC wave:

1. Frontmatter completeness:
- [ ] `issue` is present and matches the issue being implemented.
- [ ] `framework` is present (`go|python|typescript|rust|shell|unknown`).
- [ ] `category` is present (`feature|bugfix|refactor|docs|chore|ci`).

2. Structural completeness:
- [ ] `## Invariants` exists with at least 3 numbered invariants.
- [ ] `## Test Cases` exists with at least 3 rows.
- [ ] Every test case has a non-empty `Validates Invariant` value.

3. Implementability:
- [ ] Inputs/outputs reference concrete codebase concepts (not placeholders).
- [ ] Failure modes describe expected behavior, not only symptoms.

## Wave-Level Consistency Checks

Across all contracts in the wave:

1. Scope consistency:
- [ ] No contract combines multiple issue IDs.
- [ ] Each spec-eligible issue has exactly one contract.

2. Terminology consistency:
- [ ] Shared domain terms are used consistently between contracts.
- [ ] Conflicting invariants are resolved before TEST WAVE starts.

3. Test readiness:
- [ ] Every contract includes at least one success-path and one error-path test case.
- [ ] No contract is marked `BLOCKED` without a corresponding issue comment/escalation.

## Gate Rule

If any required item fails:
1. Re-run SPEC worker(s) for affected issue(s).
2. Re-run this checklist across the full wave.
3. Do NOT proceed to TEST WAVE until all required checks pass.
