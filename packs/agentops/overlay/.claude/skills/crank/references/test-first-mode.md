# Test-First Mode (--test-first)

> Reference for crank's `--test-first` flag. Covers SPEC WAVE, TEST WAVE, and RED Gate enforcement.

## SPEC WAVE

> **Purpose:** Generate contracts that ground implementation in verified requirements.

**Skip this step if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each **spec-eligible** issue (feature/bugfix/refactor):

1. **TaskCreate** with subject `SPEC: <issue-title>`
2. **Worker prompt:**
   ```
   You are a spec writer. Generate a contract for this issue.

   FIRST: Explore the codebase to understand existing patterns, types, and interfaces
   relevant to this issue. Use Glob and Read to examine the code.

   THEN: Read the contract template at:
   skills/crank/references/contract-template.md

   Generate a contract following the template. Include:
   - At least 3 invariants
   - At least 3 test cases mapped to invariants
   - Concrete types and interfaces from the actual codebase

   If inputs are missing or the issue is underspecified, write BLOCKED with reason.

   Output: .agents/specs/contract-<issue-id>.md
   ```
3. **Worker receives:** Issue description, plan boundaries, contract template, codebase access (read-only)
4. **Validation:** files_exist + content_check for `## Invariants` AND `## Test Cases`
5. **Lead commits** all specs after validation: `git add .agents/specs/ && git commit -m "spec: contracts for <issue-ids>"`
6. **Wave 1 consistency checklist (MANDATORY):** run `skills/crank/references/wave1-spec-consistency-checklist.md` across the full SPEC wave set before advancing to TEST WAVE.

**Category-based skip:** Issues categorized as docs/chore/ci bypass SPEC and TEST waves entirely and proceed directly to standard implementation waves.

### Wave 1 Consistency Gate

Run the checklist once per SPEC wave:

```bash
# Mechanical gate: all contracts in this wave satisfy checklist criteria
# (frontmatter completeness, invariant/test-case minimums, and consistency checks)
cat skills/crank/references/wave1-spec-consistency-checklist.md
```

If any checklist item fails:
1. Re-run SPEC worker(s) for affected issue(s)
2. Re-validate the full SPEC wave
3. Do not start TEST WAVE until checklist passes

### SPEC WAVE BLOCKED Recovery

If a spec worker writes `BLOCKED` instead of a contract:

1. **Read the BLOCKED reason** from the worker output
2. **Add context to the issue:**
   ```bash
   bd comments add <issue-id> "SPEC BLOCKED: <reason>. Retrying with additional context..." 2>/dev/null
   ```
3. **Retry once** with enriched prompt (include the BLOCKED reason + additional codebase context)
4. **If still BLOCKED after 2 attempts**, escalate:
   ```bash
   bd update <issue-id> --labels BLOCKER 2>/dev/null
   bd comments add <issue-id> "ESCALATED: Spec generation failed 2x. Reason: <reason>. Human review required." 2>/dev/null
   ```
   Remove the issue from spec-eligible list and continue with remaining issues. Do NOT block the entire wave.

## TEST WAVE

> **Purpose:** Generate failing tests from contracts. Tests must FAIL (RED confirmation).

**Skip this step if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each **spec-eligible** issue:

1. **TaskCreate** with subject `TEST: <issue-title>`
2. **Worker prompt:**
   ```
   You are a test writer. Generate FAILING tests from the contract.

   Read ONLY the contract at .agents/specs/contract-<issue-id>.md.
   You may read codebase structure (imports, types, interfaces) but NOT existing
   implementation details.

   Generate tests that:
   - Cover ALL test cases from the contract's Test Cases table
   - Cover ALL invariants (at least one test per invariant)
   - All tests MUST FAIL when run (RED state)
   - Follow existing test patterns in the codebase

   Do NOT read or reference existing implementation code.
   Do NOT write implementation code.

   Output: test files in the appropriate location for the project's test framework.
   ```
3. **Worker receives:** contract-<issue-id>.md + codebase structure (imports, types) but NOT existing implementations
4. **Validation:** test files exist + RED confirmation (lead runs test suite, all new tests must fail)
5. **RED Gate:** Lead runs the test suite. ALL new tests must FAIL:
   ```bash
   # Run tests — expect failures for new tests
   # If any new test PASSES, the test is not meaningful (validates existing behavior, not new)
   ```
6. **Lead commits** test harness: `git add <test-files> && git commit -m "test: failing tests for <issue-ids> (RED)"`

## RED Gate Enforcement

After TEST WAVE, the lead **must** verify RED state before proceeding:

```bash
# Run the test suite and capture results
TEST_OUTPUT=$(<test-command> 2>&1) || true
TEST_EXIT=$?

# Parse for unexpected passes among new test files
UNEXPECTED_PASSES=()
for test_file in $NEW_TEST_FILES; do
    # Check if tests in this file passed (framework-specific detection)
    if echo "$TEST_OUTPUT" | grep -q "PASS.*$(basename $test_file)"; then
        UNEXPECTED_PASSES+=("$test_file")
    fi
done

if [[ ${#UNEXPECTED_PASSES[@]} -gt 0 ]]; then
    echo "RED GATE FAILED: ${#UNEXPECTED_PASSES[@]} test file(s) passed unexpectedly:"
    printf '  - %s\n' "${UNEXPECTED_PASSES[@]}"
fi
```

**Decision tree for unexpected passes:**

| Condition | Action |
|-----------|--------|
| All new tests FAIL | PASS — proceed to IMPL wave |
| Some tests pass, some fail | Retry: re-generate passing tests with explicit "must fail" constraint |
| All new tests PASS | BLOCKED — tests validate existing behavior, not new requirements. Escalate to human. |

**On retry (max 2 attempts):**
1. Add the unexpected-pass context to the worker prompt
2. Re-spawn test writer with: "These tests passed unexpectedly: <list>. They must fail against current code. Rewrite them to test NEW behavior described in the contract."
3. If still passing after 2 retries, mark issue as BLOCKER and skip to standard IMPL

## Test Framework Detection

> Spec workers use this heuristic when the issue doesn't specify a test framework. First match wins.

**Detection priority (check in order, first match wins):**

| Priority | File Present | Check | Framework | Test Command | Contract `framework:` |
|----------|-------------|-------|-----------|-------------|----------------------|
| 1 | `Cargo.toml` | file exists | Rust | `cargo test` | `rust` |
| 2 | `go.mod` | file exists | Go | `go test ./...` | `go` |
| 3 | `pyproject.toml` or `pytest.ini` | file exists | pytest | `pytest` | `python` |
| 4 | `package.json` | `devDependencies.vitest` key exists | Vitest | `npx vitest run` | `typescript` |
| 5 | `package.json` | `devDependencies.jest` key exists | Jest | `npx jest` | `typescript` |
| 6 | `package.json` | file exists (no jest/vitest) | Node | `npm test` | `typescript` |
| 7 | `*.test.sh` or `tests/*.sh` | glob match | Shell | `bash <test-file>` | `shell` |

**For SPEC WAVE workers:** Detect the project framework using the heuristic above. Set `framework:` in the contract YAML frontmatter.

**For TEST WAVE workers:** Read the `framework:` field from the contract to determine which test runner to use. Generate tests following the project's existing test patterns.

**Fallback:** If no framework detected, spec worker writes `framework: unknown` and TEST WAVE skips that issue (falls back to standard IMPL without TDD).

**Polyglot repos:** If multiple frameworks match (e.g., Go backend + Node tooling), use the framework that matches the issue's target files. If ambiguous, use the highest-priority match.
