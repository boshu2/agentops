# GREEN Mode (Test-First Implementation)

When invoked by /crank with `--test-first`, the worker receives:
- **Failing tests** (immutable — DO NOT modify)
- **Contract** (contract-{issue-id}.md)
- **Issue description**

**GREEN Mode Rules:**

1. **Read failing tests FIRST** — understand what must pass
2. **Read contract** — understand invariants and failure modes
3. **Implement ONLY enough** to make all tests pass
4. **Do NOT modify test files** — tests are immutable in GREEN mode
5. **Do NOT add features** beyond what tests require
6. **Diff check (mechanical):** After implementation, verify no test files were modified:
   ```bash
   MODIFIED_TESTS=$(git diff --name-only -- '*_test.go' '*_test.py' '*.test.ts' '*.test.js' '*.spec.ts' '*.spec.js')
   if [ -n "$MODIFIED_TESTS" ]; then
     echo "BLOCK: GREEN mode violation: test file modified: $MODIFIED_TESTS"
     # Revert test changes and re-implement without modifying tests
   fi
   ```
   **Opt-out:** `--allow-test-modification` flag (for cases where test fixtures need updating)
7. **BLOCKED if spec error** — if contract contradicts tests or is incomplete, write BLOCKED with reason

**Verification (GREEN Mode):**
1. Run test suite → ALL tests must PASS
2. Standard Iron Law (Step 5a) still applies — fresh verification evidence required
3. No untested code — every line must be reachable by a test

**Test Immutability Enforcement:**
- Workers may ADD new test files but MUST NOT modify existing test files provided by the TEST WAVE
- If a test appears wrong, write BLOCKED with the specific test and reason — do NOT fix it
