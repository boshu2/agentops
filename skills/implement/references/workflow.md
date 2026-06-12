# Implement Workflow — Full Execution Steps

Given `/implement <issue-id-or-description>`:

## Step 0: Pre-Flight Checks (Resume + Gates)

**For resume protocol details, read `skills/implement/references/resume-protocol.md`.**

**For ratchet gate checks and pre-mortem gate details, read `skills/implement/references/gate-checks.md`.**

## Step 0.5: Pull Relevant Knowledge

```bash
# Pull knowledge scoped to this issue (if ao available)
ao lookup --bead <issue-id> --limit 3 2>/dev/null || true
```

**Apply retrieved knowledge (mandatory when results returned):**

If learnings or patterns are returned, do NOT just load them as passive context. For each returned item:
1. Check: does this learning apply to the current issue? (answer yes/no)
2. If yes: treat it as an implementation constraint — does it warn about an approach? suggest a pattern? flag a known pitfall?
3. Reference applicable learnings in your implementation decisions (e.g., "per learning X, avoiding approach Y")
4. Cite applicable learnings by filename in commit messages or PR descriptions

After reviewing, record each citation with the correct type:
```bash
# Only use "applied" when the learning actually influenced your output.
# Use "retrieved" for items that were loaded but not referenced in your work.
ao metrics cite "<learning-path>" --type applied 2>/dev/null || true   # influenced a decision
ao metrics cite "<learning-path>" --type retrieved 2>/dev/null || true # loaded but not used
```

**Section evidence:** When lookup results include `section_heading`, `matched_snippet`, or `match_confidence` fields, prefer the matched section over the whole file — it pinpoints the relevant portion. Higher `match_confidence` (>0.7) means the section is a strong match; lower values (<0.4) are weaker signals. Use the `matched_snippet` as the primary context rather than reading the full file.

Skip silently if ao is unavailable or returns no results.

## Step 1: Get Issue Details

**If beads issue ID provided** (e.g., `gt-123`):
```bash
bd show <issue-id> 2>/dev/null
```

**If plain description provided:** Use that as the task description.

**If no argument:** Check for ready work:
```bash
bd ready 2>/dev/null | head -3
```

## Step 2: Claim the Issue

```bash
bd update <issue-id> --status in_progress 2>/dev/null
```

## Step 2a: Build Context Briefing

```bash
if command -v ao &>/dev/null; then
    ao context assemble --task='<issue title and description>'
fi
```

This produces a 5-section briefing (GOALS, HISTORY, INTEL, TASK, PROTOCOL) at `.agents/rpi/briefing-current.md` with secrets redacted. Read it before gathering additional context.

## Step 2b: Apply Behavioral Discipline

Before exploring or editing, load the behavioral discipline standard from `/standards` and write a short execution frame for yourself:

- `Assumptions:` what is known, what is ambiguous, and which unknowns would change the solution
- `Smallest change:` the minimum patch that could satisfy the request
- `Blast radius:` which files or surfaces are in scope, plus what is explicitly out of scope
- `Verification:` the tests, commands, or gates that will prove the work is done

Rules:

- If ambiguity would materially change the implementation, ask before editing instead of silently choosing.
- If a simpler approach exists than the heavier path implied by the prompt, say so and prefer it.
- If you notice unrelated cleanup, create a bead or note it separately; do not fold it into the patch.
- Every changed line should trace back to the request or to cleanup that your change made necessary.

## Step 3: Gather Context

**USE THE TASK TOOL** to explore relevant code:

```
Tool: Task
Parameters:
  subagent_type: "Explore"
  description: "Gather context for: <issue title>"
  prompt: |
    Find code relevant to: <issue description>

    1. Search for related files (Glob)
    2. Search for relevant keywords (Grep)
    3. Read key files to understand current implementation
    4. Identify where changes need to be made

    Return:
    - Files to modify (paths)
    - Current implementation summary
    - Suggested approach
    - Any risks or concerns
```

## Step 3.5: Grep for Existing Utilities

Before implementing any new function or utility, grep the codebase for existing implementations:

```bash
# Search for the function name pattern you're about to create
grep -rn "<function-name-pattern>" --include="*.go" --include="*.py" --include="*.ts" .
```

**Why:** In context-orchestration-leverage, a worker created a duplicate `estimateTokens` function that already existed in `context.go`. A 5-second grep would have prevented the duplication and the rework needed to consolidate it.

If you find an existing implementation, reuse it. If it needs modification, modify it in place rather than creating a parallel version.

## Step 3.6: Write Failing Tests First (TDD-First Default)

Before implementing, write tests that define the expected behavior:

1. **Write tests covering:** happy path, one error path, one edge case
2. **Run tests to confirm they FAIL** (RED confirmation)
   - If tests pass → feature already exists or tests are wrong. Investigate before proceeding.
3. **Proceed to Step 4** with failing tests as the implementation target

```bash
# Run tests - ALL new tests must FAIL
# Python: pytest tests/test_<feature>.py -v
# Go: go test ./path/to/... -run TestNew
# Node: npm test -- --grep "new feature"
```

**Test level selection:** Classify each test by pyramid level (see the test pyramid standard (`test-pyramid.md` in the standards skill)):
- **L0 (Contract):** Write if the issue touches spec boundaries, file existence, or registration
- **L1 (Unit):** Write always for feature/bug issues — happy path, one error path, one edge case
- **L2 (Integration):** Write if the change crosses module boundaries or involves multiple components
- **L3 (Component):** Write if the change affects a full subsystem workflow (with mocked external deps)

If the issue includes `test_levels` metadata from `/plan`, use those levels. Otherwise, default to L1 + any applicable higher levels from the decision tree above.
When delegating to `/test`, carry those selected levels and any BF expectations into the request context. `--quick` is not permission to collapse to L1-only coverage.

**Bug-Finding Level Selection (alongside L0–L3):**

If the implementation touches external boundaries (APIs, databases, file I/O):
- Add BF4 chaos test: mock the boundary to fail, verify graceful error handling
- This catches the bugs that L1 unit tests mock away

If the implementation includes data transformations (parse, render, serialize):
- Add BF1 property test: randomize inputs with hypothesis/gopter/fast-check
- This catches edge cases no human would write

If the implementation generates output files (configs, reports, manifests):
- Add BF2 golden test: generate canonical output, save as golden file, assert match

Reference: the test pyramid standard in `/standards` for full tooling matrix.

**RED Verification Gate (mechanical):**
After writing tests, run the test suite and verify ALL new tests FAIL:
- If exit code == 0 (all tests PASS before implementation): **BLOCK** with "Tests pass before implementation -- either feature already exists or tests don't test new behavior. Investigate."
- If exit code != 0 (tests fail as expected): proceed to Step 4
- **Skip if:** `--no-tdd` flag is set, GREEN mode is active, or issue type is `chore`, `docs`, or `ci`

**Skip conditions (any of these bypasses Step 3.5):**
- GREEN mode is active (invoked by `/crank --test-first` — tests already exist)
- Issue type is `chore`, `docs`, or `ci`
- `--no-tdd` flag is set
- No test framework detected in the project

**Note:** Tests written here are MUTABLE — unlike GREEN mode's immutable tests, you may adjust these tests during implementation if you discover the initial test design was wrong. The goal is to think about behavior before code, not to be rigid.

## Step 3.6a: Auto-Generate Tests via /test (lifecycle integration)

If skip conditions above are NOT met AND `--no-lifecycle` is NOT set:

```
Skill(skill="test", args="generate <feature-scope> --quick")
```

The generated test request must preserve the selected `test_levels` and BF expectations from Step 3.6. Review the generated tests. Adjust as needed (tests are MUTABLE in this context). If `/test` fails to produce useful output or is unavailable, fall back to manual test writing in Step 3.6 above.

**Skip if:** `--no-lifecycle` flag, GREEN mode active, issue type is chore/docs/ci, or `/test` is unavailable.

**CI-safe tests:** If the function under test shells out to an external CLI (`bd`, `ao`, `gh`), do NOT test the wrapper. Instead, test the underlying function that performs the testable work (event emission, state mutation, file I/O). See the Go standards (Testing section) for examples.

## Step 4: Implement the Change

**GREEN Mode check:** If test files were provided (invoked by /crank --test-first):
1. Read all provided test files FIRST
2. Read the contract for invariants
3. Implement to make tests pass (do NOT modify test files)
4. Skip to Step 5 verification

Based on the context gathered:

1. **Edit existing files** using the Edit tool (preferred)
2. **Write new files** only if necessary using the Write tool
3. **Follow existing patterns** in the codebase
4. **Keep changes minimal** - don't over-engineer

## Step 4a: Build Verification (CLI repos only)

If the project has a Go `cmd/` directory or a Makefile with a `build` target, run build verification before proceeding to tests:

```bash
# Detect CLI repo
if [ -f go.mod ] && ls cmd/*/main.go &>/dev/null; then
    echo "CLI repo detected — running build verification..."

    # Build
    go build ./cmd/... 2>&1
    if [ $? -ne 0 ]; then
        echo "BUILD FAILED — fix compilation errors before proceeding"
        # Do NOT proceed to Step 5
    fi

    # Vet
    go vet ./cmd/... 2>&1

    # Smoke test: run the binary with --help
    BINARY=$(ls -t cmd/*/main.go | head -1 | xargs dirname | xargs basename)
    if [ -f "bin/$BINARY" ]; then
        ./bin/$BINARY --help > /dev/null 2>&1
        echo "Smoke test: $BINARY --help passed"
    fi
fi
```

**If build fails:** Fix compilation errors and re-run before proceeding. Do NOT skip to verification with a broken build.

**If not a CLI repo:** This step is a no-op — proceed directly to Step 5.

## Step 4.5: Security Verification

Before proceeding to functional verification, check for common security issues in modified code:

| Check | What to Look For | Action |
|-------|------------------|--------|
| Input validation | User/external input used without validation | Add validation at entry points |
| Output escaping | Raw data in HTML/templates (innerHTML, document.write, dangerouslySetInnerHTML) | Use framework auto-escaping or explicit sanitization |
| Path safety | Path traversal via `..` sequences; file paths from user input without sanitization | Reject `..`, absolute paths; use `filepath.Clean()` or equivalent; verify path stays within allowed directory |
| Auth gates | Endpoints/handlers missing authentication or authorization checks | Add middleware or guard clauses |
| Content-Type | HTTP responses without explicit Content-Type headers | Set Content-Type to prevent MIME-sniffing attacks |
| CORS | Overly permissive CORS configuration (`*` origin, credentials: true) | Restrict to known origins; never combine wildcard with credentials |
| CSRF tokens | State-changing endpoints (POST/PUT/DELETE) without anti-CSRF tokens | Add anti-CSRF token validation; do not rely solely on cookies for auth |
| Rate limiting | Authentication, API, and upload endpoints without rate limits | Add rate-limit middleware; return 429 with Retry-After header |

**Skip when:** The change does not involve HTTP handlers, user-facing input, file system operations, or template rendering. Pure internal refactors, test-only changes, and documentation edits skip this step.

**If issues found:** Fix before proceeding to Step 5. Log fixes in the commit message.

## Step 5: Verify the Change

**Success Criteria (all must pass):**
- [ ] All existing tests pass (no new failures introduced)
- [ ] New code compiles/parses without errors
- [ ] No new linter warnings (if linter available)
- [ ] Change achieves the stated goal

Check for test files and run them:
```bash
# Find tests
ls *test* tests/ test/ __tests__/ 2>/dev/null | head -5

# Run tests (adapt to project type)
# Python: pytest
# Go: go test ./...
# Node: npm test
# Rust: cargo test
```

**If tests exist:** All tests must pass. Any failure = verification failed.

**If no tests exist:** Manual verification required:
- [ ] Syntax check passes (file compiles/parses)
- [ ] Imports resolve correctly
- [ ] Can reproduce expected behavior manually
- [ ] Edge cases identified during implementation are handled

**If verification fails:** Do NOT proceed to Step 5a. Fix the issue first.

## Step 5.5: Binary-Deployment Gate (CLI/Hook Bug Fixes) — MANDATORY

**For the full gate spec (rationale, mtime check, plugin-cache check, remediation), read `skills/implement/references/binary-deployment-gate.md`.**

**This gate BLOCKS declaring "done" when the diff touches CLI/hook surfaces.** It is not a warning. Council finding (`.agents/council/2026-05-01-evolution-cycle-council.md`, finding 1, action item A; 6/6 judges): a fix shipped to source while the deployed runtime is pre-fix keeps reproducing the bug during its own post-mortem. Captured failure mode: `.agents/learnings/2026-05-01-fix-shipped-binary-stale.md`.

**Trigger** — gate fires if the diff touches `cli/cmd/**`, `hooks/**`, or `cli/embedded/hooks/**`:

```bash
CHANGED=$(git diff --name-only HEAD~1 2>/dev/null; git diff --name-only --cached; git diff --name-only)
TRIGGERS=$(printf '%s\n' "$CHANGED" | grep -E '^(cli/cmd/|hooks/|cli/embedded/hooks/)' | sort -u)
[ -z "$TRIGGERS" ] && echo "Binary-deployment gate: no CLI/hook surfaces touched, skipping" || echo "Binary-deployment gate FIRES on: $TRIGGERS"
```

**When fired, both checks below MUST pass before Step 5a.**

**Check A — deployed binary mtime ≥ source-fix commit timestamp** (per binary under `cli/cmd/<bin>/`):

```bash
BIN=<binary-name>            # e.g., ao
DEPLOYED=$(command -v "$BIN") || { echo "BLOCK: $BIN not on PATH"; exit 1; }
DEPLOYED_MTIME=$(stat -c %Y "$DEPLOYED" 2>/dev/null || stat -f %m "$DEPLOYED")  # Linux | macOS
SOURCE_MTIME=$(git log -1 --format=%ct -- "cli/cmd/$BIN/")
[ "$DEPLOYED_MTIME" -lt "$SOURCE_MTIME" ] && { echo "BLOCK: deployed $BIN is pre-fix — rebuild & redeploy"; exit 1; }
```

**Check B — plugin-cache hook copies reflect the fix** (for any `hooks/` or `cli/embedded/hooks/` change, substitute the marker string introduced by the fix, e.g., `AGENTOPS_STARTUP_CLOSE_LOOP`):

```bash
STALE=$(find ~/.claude/plugins/cache ~/.codex/plugins/cache \
    -name '<hook-name>.sh' -path '*agentops*' \
    -exec grep -L "<MARKER>" {} \; 2>/dev/null)
[ -n "$STALE" ] && { echo "BLOCK: stale plugin-cache hook copies: $STALE"; exit 1; }
```

**Pass criteria:** both checks clean (or trigger is empty). Only then proceed to Step 5a. Failure modes, fallbacks, and remediation steps are in the references doc.

## Step 5a: Verification Gate (MANDATORY)

**THE IRON LAW:** NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE

Before reporting success, you MUST:

1. **IDENTIFY** - What command proves this claim works?
2. **RUN** - Execute the FULL command (fresh, not cached output)
3. **READ** - Check full output AND exit code
4. **VERIFY** - Does output actually confirm the claim?
5. **ONLY THEN** - Make the completion claim

**Forbidden phrases without fresh verification evidence:**
- "should work", "probably fixed", "seems to be working"
- "Great!", "Perfect!", "Done!" (without output proof)
- "I just ran it" (must run it AGAIN, fresh)

### Rationalization Table

| Excuse | Reality |
|--------|---------|
| "Too simple to verify" | Simple code breaks. Verification takes 10 seconds. |
| "I just ran it" | Run it AGAIN. Fresh output only. |
| "Tests passed earlier" | Run them NOW. State changes. |
| "It's obvious it works" | Nothing is obvious. Evidence or silence. |
| "The edit looks correct" | Looking != working. Run the code. |

**Store checkpoint:**
```bash
bd update <issue-id> --append-notes "CHECKPOINT: Step 5a verification passed at $(date -Iseconds)" 2>/dev/null
```

## GREEN Mode (Test-First Implementation)

**For GREEN mode rules and verification details, read `skills/implement/references/green-mode.md`.**

## Step 5b: Autonomous Quality Loop (Pre-Commit)

**For the full pre-commit fix-verify loop spec, read `skills/implement/references/quality-loop.md`.**

## Step 5c: Generate Behavioral Spec (Optional)

**For the behavioral spec format and guidelines, read `skills/implement/references/behavioral-spec.md`.**

## Step 6: Commit the Change

If the change is complete and verified:
```bash
git add <modified-files>
git commit -m "<descriptive message>

Implements: <issue-id>"
```

## Step 7: Close the Issue with Evidence

Close with scoped evidence so the closure-integrity audit can resolve
without parser_miss/timing_miss. The close reason must cite the commit
and changed files from Step 6.

```bash
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANGED_FILES=$(git diff --name-only HEAD~1 2>/dev/null | head -10 | tr '\n' ' ' | sed 's/ $//')
bd close <issue-id> --reason "commit:${COMMIT_SHA} files:[${CHANGED_FILES}]" 2>/dev/null
```

If `bd close` is unavailable, fall back to `bd update <issue-id> --status closed`.

## Step 7a: Record Implementation in Ratchet Chain

**After successful issue closure, record in ratchet:**

```bash
# Check if ao CLI is available
if command -v ao &>/dev/null; then
  # Reuse commit evidence from Step 7
  COMMIT_HASH=$(git rev-parse HEAD 2>/dev/null || echo "")
  CHANGED_FILES=$(git diff --name-only HEAD~1 2>/dev/null | tr '\n' ',' | sed 's/,$//')

  if [ -n "$COMMIT_HASH" ]; then
    # Record successful implementation
    # Determine TDD mode for ratchet tracking
    # Values: red (wrote failing tests), green (GREEN mode from crank),
    #         skipped (skip conditions met), no-tdd (explicitly disabled)
    TDD_MODE="red"  # default when TDD was followed
    # Override based on context:
    # GREEN mode → "green", skip conditions → "skipped", --no-tdd → "no-tdd"

    ao ratchet record implement \
      --tdd-mode "$TDD_MODE" \
      --output "$COMMIT_HASH" \
      --files "$CHANGED_FILES" \
      --issue "<issue-id>" \
      2>&1 | tee -a .agents/flywheel.log

    if [ $? -eq 0 ]; then
      echo "Ratchet: Implementation recorded (commit: ${COMMIT_HASH:0:8})"
    else
      echo "Ratchet: Failed to record - chain.jsonl may need repair"
    fi
  else
    echo "Ratchet: No commit found - skipping record"
  fi
else
  echo "Ratchet: ao CLI not available - implementation NOT recorded"
  echo "  Run manually: ao ratchet record implement --output <commit>"
fi
```

**On failure/blocker:** Record the blocker in ratchet:

```bash
if command -v ao &>/dev/null; then
  ao ratchet record implement \
    --status blocked \
    --reason "<blocker description>" \
    2>/dev/null
fi
```

**Fallback:** If ao is not available, the issue is still closed via bd but won't be tracked in the ratchet chain. The skill continues normally.

## Step 7b: Post-Implementation Ratchet Record

After implementation is complete:

```bash
if command -v ao &>/dev/null; then
  ao ratchet record implement --output "<issue-id>" 2>/dev/null || true
fi
```

Tell user: "Implementation complete. Run /validate to validate before pushing."

## Step 8: Report to User

Tell the user:
1. What was changed (files modified)
2. How it was verified (with actual command output)
3. Issue status (closed)
4. Any follow-up needed
5. **Ratchet status** (implementation recorded or skipped)

**Output completion marker:**
```
<promise>DONE</promise>
```

If blocked or incomplete:
```
<promise>BLOCKED</promise>
Reason: <why blocked>
```

```
<promise>PARTIAL</promise>
Remaining: <what's left>
```
