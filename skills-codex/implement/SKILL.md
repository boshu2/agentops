---
name: implement
description: 'Implement one tracked issue. Triggers: "implement", "implement one tracked issue", "implement skill".'
---
# Implement Skill

> **Quick Ref:** Execute single issue end-to-end. Output: code changes + commit + closed issue.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

## Constraints

- Freeze the claimed issue's acceptance criteria, non-goals, and write scope before editing, because every changed line must trace to the single vertical slice; route unrelated work to a follow-up.
- For behavior changes, capture a right-reason failing test before implementation and keep GREEN-mode tests immutable, because the failing proof is the slice contract rather than ceremony.
- Route a plain `REFUTED` validation result back through automatic repair and revalidation; only a circuit-breaker trip enters `HOLD` and one bounded helper pass, while helper `ESCALATE` or refusal/judgment/exhausted-budget classes reach a human.

Execute a single issue from start to finish.

**Triggers:** "implement", "implement one tracked issue", or "implement skill".

## Codex Lifecycle Guard

When this skill runs in Codex hookless mode (`CODEX_THREAD_ID` is set or
`CODEX_INTERNAL_ORIGINATOR_OVERRIDE` is `Codex Desktop`), ensure startup context
before claiming or implementing the issue:

```bash
ao codex ensure-start 2>/dev/null || true
```

`ao codex ensure-start` is the single startup guard for Codex skills. It records
startup once per thread and skips duplicate startup automatically. Leave
`ao codex ensure-stop` to dedicated closeout skills such as `$validate`,
`$post-mortem`, or `$handoff`.

## Execution Steps

Given `$implement <issue-id-or-description>`:

### Step 0: Pre-Flight Checks (Resume + Gates)

**For resume protocol details, read `references/resume-protocol.md`.**

**For ratchet gate checks and pre-mortem gate details, read `references/gate-checks.md`.**

### Step 0.5: Pull Relevant Knowledge

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

### Step 1: Get Issue Details

**If beads issue ID provided** (e.g., `gt-123`):
```bash
ao beads exec show <issue-id> 2>/dev/null
```

**If plain description provided:** Use that as the task description.

**If no argument:** Check for ready work:
```bash
br ready 2>/dev/null | head -3
```

### Step 2: Claim the Issue

```bash
ao beads exec update <issue-id> --claim 2>/dev/null
```

### Step 2a: Build Context Briefing

```bash
if command -v ao &>/dev/null; then
    ao context assemble --task='<issue title and description>'
fi
```

This produces a 5-section briefing (GOALS, HISTORY, INTEL, TASK, PROTOCOL) at `.agents/rpi/briefing-current.md` with secrets redacted. Read it before gathering additional context.

### Step 2b: Apply Behavioral Discipline

Before exploring or editing, load the behavioral discipline standard from `$standards` and write a short execution frame for yourself:

- `Assumptions:` what is known, what is ambiguous, and which unknowns would change the solution
- `Smallest change:` the minimum patch that could satisfy the request
- `Blast radius:` which files or surfaces are in scope, plus what is explicitly out of scope
- `Verification:` the tests, commands, or gates that will prove the work is done

Rules:

- If ambiguity would materially change the implementation, ask before editing instead of silently choosing.
- If a simpler approach exists than the heavier path implied by the prompt, say so and prefer it.
- If you notice unrelated cleanup, create a bead or note it separately; do not fold it into the patch.
- Every changed line should trace back to the request or to cleanup that your change made necessary.

### Step 3: Gather Context


Spawn an exploration agent (via `spawn_agent` or `codex exec`) to gather context:

```
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

### Step 3.5: Grep for Existing Utilities

Before implementing any new function or utility, grep the codebase for existing implementations:

```bash
# Search for the function name pattern you're about to create
grep -rn "<function-name-pattern>" --include="*.go" --include="*.py" --include="*.ts" .
```

**Why:** In context-orchestration-leverage, a worker created a duplicate `estimateTokens` function that already existed in `context.go`. A 5-second grep would have prevented the duplication and the rework needed to consolidate it.

If you find an existing implementation, reuse it. If it needs modification, modify it in place rather than creating a parallel version.

### Step 3.6: Write Failing Tests First (TDD-First Default)

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

If the issue includes `test_levels` metadata from `$plan`, use those levels. Otherwise, default to L1 + any applicable higher levels from the decision tree above.
When delegating to `$test`, carry those selected levels and any BF expectations into the request context. `--quick` is not permission to collapse to L1-only coverage.

**Bug-Finding Level Selection (alongside L0–L3):**

If the implementation touches external boundaries (APIs, databases, file I/O):
- Add BF4 chaos test: mock the boundary to fail, verify graceful error handling
- This catches the bugs that L1 unit tests mock away

If the implementation includes data transformations (parse, render, serialize):
- Add BF1 property test: randomize inputs with hypothesis/gopter/fast-check
- This catches edge cases no human would write

If the implementation generates output files (configs, reports, manifests):
- Add BF2 golden test: generate canonical output, save as golden file, assert match

Reference: the test pyramid standard in `$standards` for full tooling matrix.

Or use `$test <feature>` to auto-generate test candidates, then hand-refine.

**Captured RED is mandatory for every behavior change.** GREEN input already
contains the failing contract and records it as captured RED. With no test
framework, write a minimal executable shell/contract harness. Behavior-changing
CI is behavior. `--no-tdd` cannot authorize closure.

Only a mechanically derived `docs-only` diff or independently reviewed `pure-refactor`
slice may use `red.kind=waived`; pure refactor must bind and rerun the same green
baseline before and after. An issue label alone never waives behavior RED.

Persist the exact RED command, nonzero exit, full output, and SHA-256 of every
contract test file under
`.agents/evidence/implement/<issue-id>/attempts/red-<n>.log`.
The RED command must be byte-for-byte one of the bead's canonical executable
acceptance commands—the same exact command set rerun for GREEN. The captured
output evidence is a nonempty JSON envelope containing that exact command, exit,
and SHA-256 of the combined replay output bytes; the envelope itself is digest-bound.
Exit 2 (shell/syntax), 126/127
(not executable/not found), and signal exits are invalid RED evidence.

**Test-contract rule:** after the first RED receipt, changing a test contract requires a new slice and a new RED receipt; GREEN-mode contracts are always immutable.

Before the first RED receipt, correct a malformed provisional test. Once that
receipt exists, never silently revise it: preserve the superseded evidence,
define a new slice, and earn a new RED receipt before implementation resumes.

### Step 3.6a: Auto-Generate Tests via $test (lifecycle integration)

If skip conditions above are NOT met AND `--no-lifecycle` is NOT set:

```
$test generate <feature-scope> --quick
```

The generated test request must preserve the selected `test_levels` and BF expectations from Step 3.6. Review generated tests before the first RED receipt. Any later contract change starts a new slice and RED receipt. If `$test` fails to produce useful output or is unavailable, fall back to manual test writing in Step 3.6 above.

**Skip if:** `--no-lifecycle` flag, GREEN mode active, issue type is chore/docs/ci, or `$test` is unavailable.

**CI-safe tests:** If the function under test shells out to an external CLI (`br`, `ao`, `gh`), do NOT test the wrapper. Instead, test the underlying function that performs the testable work (event emission, state mutation, file I/O). See the Go standards (Testing section) for examples.

### Step 4: Implement the Change

**GREEN Mode check:** If test files were provided (invoked by $crank --test-first):
1. Read all provided test files FIRST
2. Read the contract for invariants
3. Implement to make tests pass (do NOT modify test files)
4. Skip to Step 5 verification

Based on the context gathered:

1. **Edit existing files** using apply_patch (preferred)
2. **Write new files** only if necessary
3. **Follow existing patterns** in the codebase
4. **Keep changes minimal** - don't over-engineer

### Step 4a: Build Verification (CLI repos only)

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

### Step 4.5: Security Verification

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

### Step 5: Verify the Change

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

Persist each final proving command, zero exit, and full output under
`.agents/evidence/implement/<issue-id>/attempts/green-<n>.log`; Step 7 binds
these paths to the committed SHA.

**If no tests exist:** Manual verification required:
- [ ] Syntax check passes (file compiles/parses)
- [ ] Imports resolve correctly
- [ ] Can reproduce expected behavior manually
- [ ] Edge cases identified during implementation are handled

**If verification fails:** Do NOT proceed to Step 5a. Fix the issue first.

### Step 5.5: Binary-Deployment Gate (CLI/Hook Bug Fixes) — MANDATORY

**For the full gate spec (rationale, mtime check, plugin-cache check, remediation), read `references/binary-deployment-gate.md`.**

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
STALE=$(find ~/.codex/plugins/cache \
    -name '<hook-name>.sh' -path '*agentops*' \
    -exec grep -L "<MARKER>" {} \; 2>/dev/null)
[ -n "$STALE" ] && { echo "BLOCK: stale plugin-cache hook copies: $STALE"; exit 1; }
```

**Pass criteria:** both checks clean (or trigger is empty). Only then proceed to Step 5a. Failure modes, fallbacks, and remediation steps are in the references doc.

### Step 5a: Verification Gate (MANDATORY)

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

#### Rationalization Table

| Excuse | Reality |
|--------|---------|
| "Too simple to verify" | Simple code breaks. Verification takes 10 seconds. |
| "I just ran it" | Run it AGAIN. Fresh output only. |
| "Tests passed earlier" | Run them NOW. State changes. |
| "It's obvious it works" | Nothing is obvious. Evidence or silence. |
| "The edit looks correct" | Looking != working. Run the code. |

**Store checkpoint:**
```bash
br comments add <issue-id> "CHECKPOINT: Step 5a verification passed at $(date -Iseconds)" 2>/dev/null
```

### GREEN Mode (Test-First Implementation)

When invoked by $crank with `--test-first`, the worker receives:
- **Failing tests** (immutable — DO NOT modify)
- **Contract** (contract-{issue-id}.md)
- **Issue description**

**GREEN Mode Rules:**

1. **Read failing tests FIRST** — understand what must pass
2. **Read contract** — understand invariants and failure modes
3. **Implement ONLY enough** to make all tests pass
4. **Do NOT modify test files** — tests are immutable in GREEN mode
5. **Do NOT add features** beyond what tests require
6. **BLOCKED if spec error** — if contract contradicts tests or is incomplete, write BLOCKED with reason

**Verification (GREEN Mode):**
1. Run test suite → ALL tests must PASS
2. Standard Iron Law (Step 5a) still applies — fresh verification evidence required
3. No untested code — every line must be reachable by a test

**Test Immutability Enforcement:**
- Workers may ADD new test files but MUST NOT modify existing test files provided by the TEST WAVE
- If a test appears wrong, write BLOCKED with the specific test and reason — do NOT fix it

### Step 5b: Autonomous Quality Loop (Pre-Commit)

Before committing, run a fix-verify loop on all files modified in this session (max 3 iterations):

**Iteration N:**

1. **List modified files:** `git diff --name-only HEAD`
2. **Read each modified file completely** — do not skim
3. **Check for defects:**
   - Wrong variable references (copy-paste errors, stale names)
   - Silent error swallowing (`_ = err` or empty catch blocks)
   - Hardcoded values that should be configurable or constants
   - Missing edge cases identified during implementation
   - Inconsistencies with existing patterns in the codebase
   - Unused imports or variables
   - Complexity budget violations (function cyclomatic complexity >15)
4. **Report findings** as a numbered list with severity (HIGH/MEDIUM/LOW)
5. **HIGH findings:** Fix immediately, re-run tests, re-sweep (next iteration)
   - If a fix causes test regression: **revert the fix**, report as unresolvable, proceed
6. **MEDIUM/LOW findings:** Report in commit message, proceed

**Loop termination:**
- 0 HIGH findings → exit loop, proceed to Step 6
- 3 iterations exhausted with HIGH findings remaining → **BLOCK commit**. Report remaining HIGHs and stop. Do NOT proceed to Step 6.
  - Override: `--force-commit` allows proceeding with documented HIGHs (explicit opt-in only)

**Output:** Record iteration count, findings per iteration, and remaining items.

If no modified files or sweep finds zero issues on first pass, proceed directly to Step 5c.

### Step 5c: Generate Behavioral Spec

**Skip if:** `--no-spec` flag, or issue type is `docs`/`chore`/`ci`.

Behavior work may not skip: it must store the spec as contained digest-bound
evidence. `skipped_reason` is valid only for a non-behavior waiver lane.

After verification passes, produce a behavioral spec documenting what the implementation
does. This feeds Stage 4 behavioral validation (STEP 1.8 in `$validate`).

```bash
mkdir -p .agents/specs
cat > .agents/specs/<issue-id>.json <<'SPEC'
{
    "id": "auto-<issue-id>",
    "version": 1,
    "date": "<YYYY-MM-DD>",
    "goal": "<one-line: what user outcome this implementation serves>",
    "narrative": "<2-3 sentences: what the implementation does and how a user interacts with it>",
    "expected_outcome": "<what a satisfied user observes when this works correctly>",
    "acceptance_vectors": [
        {
            "dimension": "<name: correctness|performance|usability|security|...>",
            "threshold": <0.0-1.0>,
            "check": "<optional: mechanical check command>"
        }
    ],
    "satisfaction_threshold": 0.7,
    "scope": {
        "files": ["<list of modified files>"],
        "functions": ["<key functions added/modified>"],
        "behaviors": ["<behavioral descriptions>"]
    },
    "source": "agent",
    "status": "active"
}
SPEC
```

**Guidelines:**
- `acceptance_vectors` should capture the BEHAVIORAL contract, not test assertions.
- Include at least 2 acceptance vectors (correctness + one other dimension).
- `scope.files` must match the files you actually modified (not planned files).

**If skipped:** Log "Behavioral spec skipped (reason: <flag|issue-type>)" and proceed.

### Step 6: Commit the Change

If the change is complete and verified:
```bash
git add <modified-files>
git commit -m "<descriptive message>

Implements: <issue-id>"
```

### Step 7: Persist the Implementation Receipt

Bind the exact RED/GREEN evidence and changed files to the full committed SHA.

**Receipt path:** `.agents/evidence/implement/<issue-id>/<full-sha>/<issue-id>-<full-sha>-receipt.json`
**Receipt schema:** `schemas/implementation-receipt.schema.json`

The receipt contains `base_sha` and `head_sha`, `work_class`, acceptance ids,
and the exact `base_sha..head_sha` changed-file set. Every evidence item is a
contained `{path,sha256}` object. Behavior uses `red.kind=captured` with the RED
commit, reproducible nonzero command, and immutable test-file digests. Only
`docs-only` or `pure-refactor` may waive RED; pure refactor binds
before/after green baselines. Every command evidence file uses the same
`{command,exit_code,output_sha256}` envelope and is checked against fresh replay.

RED/GREEN may first be captured under `<issue-id>/attempts/` before the final
commit exists. Step 7 copies each envelope byte-for-byte into
`<issue-id>/<head_sha>/evidence/`, hashes the archived bytes, and makes the
receipt reference only those contained head-root paths.

```bash
HEAD_SHA=$(git rev-parse HEAD)
RECEIPT=.agents/evidence/implement/<issue-id>/$HEAD_SHA/<issue-id>-$HEAD_SHA-receipt.json
mkdir -p "$(dirname "$RECEIPT")"
python3 -m jsonschema -i "$RECEIPT" skills-codex/implement/schemas/implementation-receipt.schema.json
```

Do not rewrite an earlier RED record in place. A corrected contract is a new
slice/attempt with a new receipt; preserve the superseded evidence.

### Step 8: Independent Validation and Pawl Routing

Run `$validate` in a fresh context against the exact `head_sha`, receipt,
acceptance ids, changed files, and commands. Write its evidence path and
disposition back to the receipt. Do not close the issue before this route
returns `CONFIRMED`.

Leave `independent_validation` pending here. The Step 9 close wrapper first runs
the pinned repository pawl against the canonical verdict and canonical evidence,
then snapshots those exact bytes into the receipt tree and records their hashes.

**Canonical-first invariant:** run the pinned pawl on canonical verdict/evidence before any archive copy or receipt rewrite.
**Head-root archive invariant:** receipt evidence paths resolve only beneath `.agents/evidence/implement/<issue-id>/<head_sha>/`.

| From | To | Required action |
| --- | --- | --- |
| `CONFIRMED` | `CLOSE` | Independent evidence authorizes Step 9. |
| `REFUTED` | `AUTO-REDO` | Repair from findings, write new GREEN evidence, update the receipt, and rerun Step 8; do not close or consult a helper. |
| `CIRCUIT-BREAKER-TRIP` | `HOLD` | Freeze mutation and preserve the receipt plus blocker evidence. |
| `HOLD` | `HELPER` | Run exactly one bounded helper consultation for this blocker class. |
| `HELPER-UNSTUCK` | `AUTO-REDO` | Apply the concrete next action, reset the breaker for the new approach, and re-earn `CONFIRMED`. |
| `HELPER-ESCALATE` | `HUMAN` | Hand back the receipt, blocker evidence, and helper verdict. |
| `REFUSAL-LANE / EXPLICIT-JUDGMENT / BUDGET-EXHAUSTED` | `HUMAN` | Skip the helper and ask the operator. |

`UNSTUCK` resumes work; it never authorizes closure. A changed implementation
requires fresh GREEN evidence on the new SHA and another independent verdict.

### Step 9: Close the Issue with Confirmed Evidence

Fail closed unless the executable verifier proves the bead, canonical path,
base ancestry, exact Git change set, evidence bytes, reproducible RED/GREEN,
immutable tests, and real `CONFIRMED` pawl verdict:

**Closure verifier:** `scripts/verify-implementation-receipt.sh --issue <issue-id> --receipt "$RECEIPT"`
**Close wrapper:** `scripts/close-with-implementation-receipt.sh --issue <issue-id> --receipt "$RECEIPT"`

```bash
scripts/close-with-implementation-receipt.sh --issue <issue-id> --receipt "$RECEIPT"
```

### Step 10: Record Implementation in Ratchet Chain

After confirmed closure, record the same SHA and files:

```bash
ao ratchet record implement --input "$RECEIPT" --output "$HEAD_SHA" 2>/dev/null || true
```

If ratchet recording fails, report that bookkeeping defect; do not falsify the
independent verdict or reopen the implementation contract.

### Step 11: Report to User

Tell the user:
1. What was changed (files modified)
2. How it was verified (with actual command output)
3. Receipt path, independent `CONFIRMED` evidence, and issue status
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

## Key Rules

- **Captured RED for behavior** - every behavior change closes only with a reproducible failing contract at a pre-implementation commit; GREEN input counts as captured RED, and no-framework work uses a minimal executable harness. `--no-tdd` cannot authorize behavior closure. Only mechanically derived docs-only and independently reviewed pure-refactor lanes may waive RED; pure refactor proves canonical acceptance green before and after with unchanged test drivers.
- **Refactor after every green — the load-bearing move.** Refactor under green as its own commit after each behavior, never deferred to one final pass. **Never let a refactor step change a test** (a test change during refactor = a new slice, not a refactor).
- **One behavior per cycle (small batch)** - implement one behavior, keep green, refactor, move on.
- **Explore first** - understand before changing
- **Edit, don't rewrite** - prefer targeted edits over full file rewrites
- **Follow patterns** - match existing code style
- **Verify changes** - run tests or sanity checks
- **Commit with context** - reference the issue ID
- **Close the issue** - update status when done, then run `$crank`'s Close checkpoint (Step 6.5): a closed bead is a sensor reading — if what it taught falsifies an assumption the remaining plan depends on, surface it for re-planning instead of silently proceeding (age-cysr)

## Without Beads

If br CLI not available:
1. Skip the claim/close status updates
2. Use the description as the task
3. Still commit with descriptive message
4. Report completion to user

## Output Specification

- **Path:** modify only issue-approved product/test paths; store evidence under `.agents/evidence/implement/<issue-id>/` and the final receipt under its `<full-sha>/` directory.
- **Filename:** product/tests use repository-native names; the receipt is exactly `<issue-id>-<full-sha>-receipt.json`.
- **Format:** product files use native formats; the receipt is JSON conforming to `schemas/implementation-receipt.schema.json` and binds immutable RED plus fresh GREEN evidence to the full SHA.
- **Validation command:** run issue acceptance and relevant gates, `scripts/validate-workflow-contract.sh codex`, then `scripts/verify-implementation-receipt.sh --issue <issue-id> --receipt <canonical-path>`; closure requires the verifier and canonical pawl check to pass.
- **Downstream handoff:** pass the receipt and exact SHA to `$validate`; `REFUTED` auto-repairs, breaker `HOLD` consults one helper, and only `CONFIRMED` authorizes closure.

## Quality Checklist

- Acceptance fidelity: every changed line maps to one acceptance example or necessary cleanup, with non-goals unchanged.
- Test fidelity: the first failing proof fails for missing behavior, final tests pass fresh, and refactor commits do not modify the behavioral contract.
- Scope fidelity: changed paths remain inside the issue write scope; unrelated findings become follow-ups instead of hitchhiking.
- Evidence fidelity: commit, tracker closure, changed files, and validation commands identify the same final implementation SHA.

## Examples

### Implement Specific Issue

**User says:** `$implement ag-5k2`

**What happens:**
1. Agent reads issue from beads: "Add JWT token validation middleware"
2. Explore agent finds relevant auth code and middleware patterns
3. Agent edits `middleware/auth.go` to add token validation
4. Runs `go test ./middleware/...` — all tests pass
5. Commits with message "Add JWT token validation middleware\n\nImplements: ag-5k2"
6. Closes only via `scripts/close-with-implementation-receipt.sh --issue ag-5k2 --receipt <canonical-receipt>`

**Result:** Issue implemented, verified, committed, and closed. Ratchet recorded.

### Pick Up Next Available Work

**User says:** `$implement`

**What happens:**
1. Agent runs `ao beads exec ready` — finds `ag-3b7` (first unblocked issue)
2. Claims issue via `ao beads exec update ag-3b7 --claim`
3. Implements and verifies
4. Closes through the receipt wrapper after canonical pawl confirmation

**Result:** Autonomous work pickup and completion from ready queue.

### GREEN Mode (Test-First)

**User says:** `$implement ag-8h3` (invoked by `$crank --test-first`)

**What happens:**
1. Agent receives failing tests (immutable) and contract
2. Reads tests to understand expected behavior
3. Implements ONLY enough to make tests pass
4. Does NOT modify test files
5. Verification: all tests pass with fresh output

**Result:** Minimal implementation driven by tests, no over-engineering.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Issue not found | Issue ID doesn't exist or local state looks stale | Run `ao beads exec show <id>` to verify; use `br sync --status` only if you need tracker sync state |
| GREEN mode violation | Edited a file not related to the issue scope | Revert unrelated changes. GREEN mode restricts edits to files relevant to the issue |
| Verification gate fails | Tests fail or build breaks after implementation | Read the verification output, fix the specific failures, re-run verification |
| "BLOCKED" status | Contract contradicts tests or is incomplete in GREEN mode | Write BLOCKED with specific reason, do NOT modify tests |
| Fresh verification missing | Agent claims success without running verification command | MUST run verification command fresh with full output before claiming completion |
| Ratchet record failed | ao CLI unavailable or chain.jsonl corrupted | Implementation still closes via br, but ratchet chain needs manual repair |

## See Also

- [test](../test/SKILL.md) — Test generation, coverage analysis, and TDD workflow

## Reference Documents

- [references/binary-deployment-gate.md](references/binary-deployment-gate.md)
- [references/gate-checks.md](references/gate-checks.md)
- [references/resume-protocol.md](references/resume-protocol.md)

## Local Resources

### references/

- [references/binary-deployment-gate.md](references/binary-deployment-gate.md)
- [references/gate-checks.md](references/gate-checks.md)
- [references/resume-protocol.md](references/resume-protocol.md)

### scripts/

- `scripts/validate.sh`
- `scripts/validate-workflow-contract.sh`
- `scripts/verify-implementation-receipt.sh`
- `scripts/verify-implementation-receipt.bash`
- `scripts/test-implementation-receipt.sh`
- `scripts/close-with-implementation-receipt.sh`
- `scripts/close-with-implementation-receipt.bash`

### schemas/

- `schemas/implementation-receipt.schema.json`

<!-- Lifecycle integration wired: 2026-03-28. See skills/implement/SKILL.md for canonical -->
