---
name: crank
description: 'Hands-free epic execution. Runs until ALL children are CLOSED. Uses $swarm with runtime-native spawning (Codex sub-agents or Codex sub-agents). NO human prompts, NO stopping. Triggers: "crank", "run epic", "execute epic", "run all tasks", "hands-free execution", "crank it".'
---


# Crank Skill

> **Quick Ref:** Autonomous epic execution. `$swarm` for each wave with runtime-native spawning. Output: closed issues + final vibe.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

Autonomous execution: implement all issues until the epic is DONE.

**CLI dependencies:** bd (issue tracking), ao (knowledge flywheel). Both optional — see `skills/shared/SKILL.md` for fallback table. If bd is unavailable, use TaskList for issue tracking and skip beads sync. If ao is unavailable, skip knowledge injection/extraction.

For Codex runtime feature coverage (agents/hooks/worktree/settings), see `..$shared/references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

**Beads mode** (bd available):
```
Crank (orchestrator)           Swarm (executor)
    |                              |
    +-> bd ready (wave issues)     |
    |                              |
    +-> TaskCreate from beads  --->+-> Select spawn backend (codex sub-agents | codex sub-agents | fallback)
    |                              |
    +-> $swarm                 --->+-> Spawn workers per backend
    |                              |   (fresh context per wave)
    +-> Verify + bd update     <---+-> Workers report via backend channel
    |                              |
    +-> Loop until epic DONE   <---+-> Cleanup backend resources after wave
```

**TaskList mode** (bd unavailable):
```
Crank (orchestrator, TaskList mode)    Swarm (executor)
    |                                      |
    +-> TaskList() (wave tasks)            |
    |                                      |
    +-> $swarm                         --->+-> Select spawn backend per wave
    |                                      |
    +-> Verify via TaskList()          <---+-> Workers report via backend channel
    |                                      |
    +-> Loop until all completed       <---+-> Cleanup backend resources after wave
```

**Separation of concerns:**
- **Crank** = Orchestration, epic/task lifecycle, knowledge flywheel
- **Swarm** = Runtime-native parallel execution (Ralph Wiggum pattern via fresh worker set per wave)

Ralph alignment source: `..$shared/references/ralph-loop-contract.md` (fresh context, scheduler/worker split, disk-backed state, backpressure).

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--test-first` | off | Enable spec-first TDD: SPEC WAVE generates contracts, TEST WAVE generates failing tests, IMPL WAVES make tests pass |
| `--per-task-commits` | off | Opt-in per-task commit strategy. Falls back to wave-batch when file boundaries overlap. See `references/commit-strategies.md`. |

## Global Limits

**MAX_EPIC_WAVES = 50** (hard limit across entire epic)

This prevents infinite loops on circular dependencies or cascading failures.

**Why 50?**
- Typical epic: 5-10 issues
- With retries: ~5 waves max
- 50 = safe upper bound

## Completion Enforcement (The Sisyphus Rule)

**THE SISYPHUS RULE:** Not done until explicitly DONE.

After each wave, output completion marker:
- `<promise>DONE</promise>` - Epic truly complete, all issues closed
- `<promise>BLOCKED</promise>` - Cannot proceed (with reason)
- `<promise>PARTIAL</promise>` - Incomplete (with remaining items)

**Never claim completion without the marker.**

## Execution Steps

Given `$crank [epic-id | plan-file.md | "description"]`:

### Step 0: Load Knowledge Context (ao Integration)

**Search for relevant learnings before starting the epic:**

```bash
# If ao CLI available, pull relevant knowledge for this epic
if command -v ao &>/dev/null; then
    # Pull knowledge scoped to the epic
    ao know lookup --query "<epic-title>" --limit 5 2>/dev/null || \
      ao know search "epic execution implementation patterns" 2>/dev/null | head -20

    # Check flywheel status
    ao quality metrics flywheel status 2>/dev/null

    # Get current ratchet state
    ao work ratchet status 2>/dev/null
fi
```

If ao not available, skip this step and proceed. The knowledge flywheel enhances but is not required.

### Step 0.5: Detect Tracking Mode

```bash
if command -v bd &>/dev/null; then
  TRACKING_MODE="beads"
else
  TRACKING_MODE="tasklist"
  echo "Note: bd CLI not found. Using TaskList for issue tracking."
fi
```

**Tracking mode determines the source of truth for the rest of the workflow:**

| | Beads Mode | TaskList Mode |
|---|---|---|
| **Source of truth** | `bd` (beads issues) | TaskList (Claude-native) |
| **Find work** | `bd ready` | `TaskList()` → pending, unblocked |
| **Get details** | `bd show <id>` | `TaskGet(taskId)` |
| **Mark complete** | `bd update <id> --status closed` | `TaskUpdate(taskId, status="completed")` |
| **Track retries** | `bd comments add` | Task description update |
| **Epic tracking** | `bd update <epic-id> --append-notes` | In-memory wave counter |

### Step 1: Identify the Epic / Work Source

**Beads mode:**

**If epic ID provided:** Use it directly. Do NOT ask for confirmation.

**If no epic ID:** Discover it:
```bash
bd list --type epic --status open 2>/dev/null | head -5
```

**Single-Epic Scope Check (WARN):**
If `bd list --type epic --status open` returns more than one epic, log a warning:
```
WARN: Multiple open epics detected. $crank operates on a single epic.
Use --allow-multi-epic to suppress this warning.
```
This is a WARN, not a FAIL — cranking with focused scope is a best practice but not a hard requirement.

If multiple epics found, ask user which one.

**TaskList mode:**

If input is an epic ID → Error: "bd CLI required for beads epic tracking. Install bd or provide a plan file / task list."

If input is a plan file path (`.md`):
1. Read the plan file
2. Decompose into TaskList tasks (one `TaskCreate` per distinct work item)
3. Set up dependencies via `TaskUpdate(addBlockedBy)`
4. Proceed to Step 3

If no input:
1. Check `TaskList()` for existing pending tasks
2. If tasks exist, use them as the work items
3. If no tasks, ask user what to work on

If input is a description string:
1. Decompose into tasks (`TaskCreate` for each)
2. Set up dependencies
3. Proceed to Step 3

### Step 1a: Initialize Wave Counter

**Beads mode:**
```bash
# Initialize crank tracking in epic notes
bd update <epic-id> --append-notes "CRANK_START: wave=0 at $(date -Iseconds)" 2>/dev/null
```

**TaskList mode:** Track wave counter in memory only. No external state needed.

Track in memory: `wave=0`

### Step 1b: Detect Test-First Mode (--test-first only)

```bash
# Check for --test-first flag
if [[ "$TEST_FIRST" == "true" ]]; then
    # Classify issues by type
    # spec-eligible: feature, bug, task → SPEC + TEST waves apply
    # skip: docs, chore, ci, epic → standard implementation waves only
    SPEC_ELIGIBLE=()
    SPEC_SKIP=()

    if [[ "$TRACKING_MODE" == "beads" ]]; then
        for issue in $READY_ISSUES; do
            ISSUE_TYPE=$(bd show "$issue" 2>/dev/null | grep "Type:" | head -1 | awk '{print tolower($NF)}')
            case "$ISSUE_TYPE" in
                feature|bug|task) SPEC_ELIGIBLE+=("$issue") ;;
                docs|chore|ci|epic) SPEC_SKIP+=("$issue") ;;
                *)
                    echo "WARNING: Issue $issue has unknown type '$ISSUE_TYPE'. Defaulting to spec-eligible."
                    SPEC_ELIGIBLE+=("$issue")
                    ;;
            esac
        done
    else
        # TaskList mode: no bd available, default all to spec-eligible
        SPEC_ELIGIBLE=($READY_ISSUES)
        echo "TaskList mode: all ${#SPEC_ELIGIBLE[@]} issues defaulted to spec-eligible (no bd type info)"
    fi
    echo "Test-first mode: ${#SPEC_ELIGIBLE[@]} spec-eligible, ${#SPEC_SKIP[@]} skipped (docs/chore/ci/epic)"
fi
```

If `--test-first` is NOT set, skip Steps 3b and 3c entirely — behavior is unchanged.

### Step 2: Get Epic Details

**Beads mode:**
```bash
bd show <epic-id> 2>/dev/null
```

**TaskList mode:** `TaskList()` to see all tasks and their status/dependencies.

### Step 3: List Ready Issues (Current Wave)

**Beads mode:**

Find issues that can be worked on (no blockers):
```bash
bd ready 2>/dev/null
```

**`bd ready` returns the current wave** - all unblocked issues. These can be executed in parallel because they have no dependencies on each other.

**TaskList mode:**

`TaskList()` → filter for status=pending, no blockedBy (or all blockers completed). These are the current wave.

### Step 3a: Pre-flight Check - Issues Exist

**Verify there are issues to work on:**

**If 0 ready issues found (beads mode) or 0 pending unblocked tasks (TaskList mode):**
```
STOP and return error:
  "No ready issues found for this epic. Either:
   - All issues are blocked (check dependencies)
   - Epic has no child issues (run $plan first)
   - All issues already completed"
```

Also verify: epic has at least 1 child issue total. An epic with 0 children means $plan was not run.

Do NOT proceed with empty issue list - this produces false "epic complete" status.

### Step 3a.1: Pre-flight Check - Pre-Mortem Required (3+ issues)

**If the epic has 3 or more child issues, require pre-mortem evidence before proceeding.**

```bash
# Count child issues (beads mode)
if [[ "$TRACKING_MODE" == "beads" ]]; then
    CHILD_COUNT=$(bd show "$EPIC_ID" 2>/dev/null | grep -c "↳")
else
    CHILD_COUNT=$(TaskList | grep -c "pending\|in_progress")
fi

if [[ "$CHILD_COUNT" -ge 3 ]]; then
    # Look for pre-mortem report in .agents/council/
    PRE_MORTEM=$(ls -t .agents/council/*pre-mortem* 2>/dev/null | head -1)
    if [[ -z "$PRE_MORTEM" ]]; then
        echo "STOP: Epic has $CHILD_COUNT issues but no pre-mortem evidence found."
        echo "Run '$pre-mortem' first to validate the plan before cranking."
        echo "<promise>BLOCKED</promise>"
        echo "Reason: pre-mortem required for epics with 3+ issues"
        # STOP - do not continue
        exit 1
    fi
    echo "Pre-mortem evidence found: $PRE_MORTEM"
fi
```

**Why:** 7 consecutive epics (ag-oke through ag-9ad) showed positive ROI from pre-mortem validation. For epics with 3+ issues, the cost of a pre-mortem (~2 min) is negligible compared to the cost of cranking a flawed plan.

### Step 3a.2: Pre-flight Check - Changed-String Grep

**Before spawning workers, grep for every string being changed by the plan.**

This catches stale cross-references that the plan missed. In na-lz0, 2 HIGH findings (stale "7-category" refs in agent-prompts.md and post-mortem/SKILL.md) were caught by council review, not by planning.

```bash
# PSEUDO-CODE
# Read the plan to identify strings being changed
PLAN_FILE=$(ls -t .agents/plans/*.md 2>/dev/null | head -1)
if [[ -n "$PLAN_FILE" ]]; then
    echo "Pre-flight: Checking for stale cross-references..."
    # For each key term/string being modified, grep all skills
    # Example: if changing "7-category" to "8-category":
    #   grep -rn "7-category" skills/
    # Any matches outside the planned file set = scope gap
    echo "Review matches outside planned files. Add to scope if stale."
fi
```

**Action on findings:**
- If grep finds matches in files NOT in the plan's file manifest, either:
  1. Add those files to the current epic (create new issue), or
  2. Document as known tech debt with a follow-up beads issue

**Why:** This is a P0 process fix. Both HIGH findings from na-lz0 would have been caught by this check.

### Step 3b: SPEC WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each spec-eligible issue (feature/bug/task):
1. **TaskCreate** with subject `SPEC: <issue-title>`
2. Worker receives: issue description, plan boundaries, contract template (`skills/crank/references/contract-template.md`), codebase access (read-only)
3. Worker generates: `.agents/specs/contract-<issue-id>.md`
4. **Validation:** files_exist + content_check for `## Invariants` AND `## Test Cases`
5. **Wave 1 spec consistency checklist (MANDATORY):** run `skills/crank/references/wave1-spec-consistency-checklist.md` across all contracts in this wave. If any item fails, re-run SPEC workers for affected issues and do NOT proceed to TEST WAVE.
6. Lead commits all specs after validation

For BLOCKED recovery and full worker prompt, read `skills/crank/references/test-first-mode.md`.

### Step 3c: TEST WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each spec-eligible issue:
1. **TaskCreate** with subject `TEST: <issue-title>`
2. Worker receives: contract-<issue-id>.md + codebase types (NOT implementation code)
3. Worker generates: failing test files in appropriate location
4. **RED Gate:** Lead runs test suite — ALL new tests must FAIL
5. Lead commits test harness after RED Gate passes

For RED Gate enforcement and retry logic, read `skills/crank/references/test-first-mode.md`.

**Summary:** SPEC WAVE generates contracts from issues → TEST WAVE generates failing tests from contracts → RED Gate verifies all new tests fail before proceeding. Docs/chore/ci issues bypass both waves.

### Step 3b.1: Build Context Briefing (Before Worker Dispatch)

```bash
if command -v ao &>/dev/null; then
    ao work context assemble --task='<epic title>: wave $wave'
fi
```

This produces a 5-section briefing (GOALS, HISTORY, INTEL, TASK, PROTOCOL) at `.agents/rpi/briefing-current.md` with secrets redacted. Include the briefing path in each worker's TaskCreate description so workers start with full project context.

### Step 4: Execute Wave via Swarm

**GREEN mode (--test-first only):** If `--test-first` is set and SPEC/TEST waves have completed, modify worker prompts for spec-eligible issues:
- Include in each worker's TaskCreate: `"Failing tests exist at <test-file-paths>. Make them pass. Do NOT modify test files. See GREEN Mode rules in $implement SKILL.md."`
- Workers receive: failing tests (immutable), contract, issue description
- Workers follow GREEN Mode rules from `$implement` SKILL.md
- Docs/chore/ci issues (skipped by SPEC/TEST waves) use standard worker prompts unchanged

**File manifests (REQUIRED):** Include a `metadata.files` array in every TaskCreate listing the files that worker will modify. This feeds into swarm's pre-spawn conflict detection -- two workers claiming the same file in the same wave get serialized or worktree-isolated automatically. Derive file lists from the issue description, plan, or codebase exploration during planning.

**Grep-for-existing-functions (REQUIRED for new function issues):** When an issue description says "create", "add", or "implement" a new function/utility, include `metadata.grep_check` with the function name pattern. Workers MUST grep the codebase for existing implementations before writing new code. This prevents utility duplication (e.g., `estimateTokens` was duplicated in context-orchestration-leverage because no grep check was specified).

**Validation metadata policy (REQUIRED):** For implementation tasks typed `feature|bug|task`, include `metadata.validation.tests` plus at least one structural check (`files_exist` or `content_check`). `docs|chore|ci` use an explicit test-exempt path and should still include applicable structural and/or command/lint checks.

**Validation block extraction (beads mode):** Before building TaskCreate calls, extract validation metadata from each issue's description. `$plan` embeds conformance checks as fenced `validation` blocks in issue bodies:

```bash
# For each issue in the current wave, extract validation JSON from bd show output
ISSUE_BODY=$(bd show "$ISSUE_ID" 2>/dev/null)
VALIDATION_JSON=$(echo "$ISSUE_BODY" | sed -n '/^```validation$/,/^```$/{ /^```/d; p }')

if [[ -n "$VALIDATION_JSON" ]]; then
    # Use extracted validation as metadata.validation in TaskCreate
    echo "Extracted validation block for $ISSUE_ID"
else
    # Fallback: generate default validation from files mentioned in description
    # Use files_exist check for any file paths found in the issue body
    MENTIONED_FILES=$(echo "$ISSUE_BODY" | grep -oE '[a-zA-Z0-9_/.-]+\.(go|py|ts|sh|md|yaml|json)' | sort -u)
    VALIDATION_JSON="{\"files_exist\": [$(echo "$MENTIONED_FILES" | sed 's/.*/"&"/' | paste -sd,)]}"
    echo "WARNING: No validation block in $ISSUE_ID — using fallback files_exist check"
fi
```

Inject the extracted or fallback `VALIDATION_JSON` into the `metadata.validation` field of each worker's TaskCreate. This closes the plan-to-crank validation pipeline: `$plan` writes conformance checks → bd stores them → `$crank` extracts and enforces them.

```
TaskCreate(
  subject="ag-1234: Add auth middleware",
  description="...",
  activeForm="Implementing ag-1234",
  metadata={
    "files": ["src/middleware/auth.py", "tests/test_auth.py"],
    "validation": {
      "tests": "pytest tests/test_auth.py -v",
      "files_exist": ["src/middleware/auth.py", "tests/test_auth.py"]
    }
  }
)
```

**Display file-ownership table (from swarm Step 1.5):**

Before spawning, verify the ownership map has zero unresolved conflicts:

```
File Ownership Map (Wave $wave):
┌─────────────────────────────┬──────────┬──────────┐
│ File                        │ Owner    │ Conflict │
├─────────────────────────────┼──────────┼──────────┤
│ (populated by swarm)        │          │          │
└─────────────────────────────┴──────────┴──────────┘
Conflicts: 0
```

**If conflicts > 0:** Do NOT invoke `$swarm`. Resolve by serializing conflicting tasks into sub-waves or merging task scope before proceeding.

**BEFORE each wave:**
```bash
wave=$((wave + 1))
WAVE_START_SHA=$(git rev-parse HEAD)

if [[ "$TRACKING_MODE" == "beads" ]]; then
    bd update <epic-id> --append-notes "CRANK_WAVE: $wave at $(date -Iseconds)" 2>/dev/null
fi

# CHECK GLOBAL LIMIT
if [[ $wave -ge 50 ]]; then
    echo "<promise>BLOCKED</promise>"
    echo "Global wave limit (50) reached."
    # STOP - do not continue
fi
```

**Pre-Spawn: Spec Consistency Gate**

Prevents workers from implementing inconsistent or incomplete specs. Hard failures (missing frontmatter, bad structure, scope conflicts) block spawn; WARN-level issues (terminology, implementability) do not.

```bash
if [ -d .agents/specs ] && ls .agents/specs/contract-*.md &>/dev/null 2>&1; then
    bash scripts/spec-consistency-gate.sh .agents/specs/ || {
        echo "⚠️ Spec consistency check failed — fix contract files before spawning workers"
        exit 1
    }
fi
```

**Cross-cutting constraint injection (SDD):**

Before spawning workers, check for cross-cutting constraints:

```bash
# PSEUDO-CODE
# Guard clause: skip if plan has no boundaries (backward compat)
PLAN_FILE=$(ls -t .agents/plans/*.md 2>/dev/null | head -1)
if [[ -n "$PLAN_FILE" ]] && grep -q "## Boundaries" "$PLAN_FILE"; then
    # Extract "Always" boundaries and convert to cross_cutting checks
    # Read the plan's ## Cross-Cutting Constraints section or derive from ## Boundaries
    # Inject into every TaskCreate's metadata.validation.cross_cutting
fi
# "Ask First" boundaries: in auto mode, log as annotation only (no blocking)
# In --interactive mode, prompt before proceeding
```

When creating TaskCreate for each wave issue, include cross-cutting constraints in metadata:
```json
{
  "validation": {
    "files_exist": [...],
    "content_check": {...},
    "cross_cutting": [
      {"name": "...", "type": "content_check", "file": "...", "pattern": "..."}
    ]
  }
}
```

**For wave execution details (beads sync, TaskList bridging, swarm invocation), read `skills/crank/references/team-coordination.md`.**

**Cross-cutting validation (SDD):**

After per-task validation passes, run cross-cutting checks across all files modified in the wave:

```bash
# Only if cross_cutting constraints were injected
if [[ -n "$CROSS_CUTTING_CHECKS" ]]; then
    WAVE_FILES=$(git diff --name-only "${WAVE_START_SHA}..HEAD")
    for check in $CROSS_CUTTING_CHECKS; do
        run_validation_check "$check" "$WAVE_FILES"
    done
fi
```

### Step 5: Verify and Sync to Beads (MANDATORY)

> Swarm executes per-task validation (see `skills/shared/validation-contract.md`). Crank trusts swarm validation and focuses on beads sync.

**For verification details, retry logic, and failure escalation, read `skills/crank/references/team-coordination.md` and `skills/crank/references/failure-recovery.md`.**

### Step 5.5: Wave Acceptance Check (MANDATORY)

> **Principle:** Verify each wave meets acceptance criteria using lightweight inline judges. No skill invocations — prevents context explosion in the orchestrator loop.

**For acceptance check details (diff computation, inline judges, verdict gating), read `skills/crank/references/wave-patterns.md`.**

### Step 5.7: Wave Checkpoint

After each wave completes (post-vibe-gate, pre-next-wave), write a checkpoint file:

```bash
mkdir -p .agents/crank

cat > ".agents/crank/wave-${wave}-checkpoint.json" <<EOF
{
  "schema_version": 1,
  "wave": ${wave},
  "timestamp": "$(date -Iseconds)",
  "tasks_completed": $(echo "$COMPLETED_IDS" | jq -R 'split(" ")'),
  "tasks_failed": $(echo "$FAILED_IDS" | jq -R 'split(" ")'),
  "files_changed": $(git diff --name-only "${WAVE_START_SHA}..HEAD" | jq -R . | jq -s .),
  "git_sha": "$(git rev-parse HEAD)",
  "acceptance_verdict": "<PASS|WARN|FAIL>",
  "commit_strategy": "<per-task|wave-batch|wave-batch-fallback>"
}
EOF
```

- `COMPLETED_IDS` / `FAILED_IDS`: space-separated issue IDs from the wave results.
- `acceptance_verdict`: verdict from the Wave Acceptance Check (Step 5.5). Used by final validation to skip redundant $vibe on clean epics.
- On retry of the same wave, the file is overwritten (same path).

### Step 5.7b: Vibe Context Checkpoint

After writing the wave checkpoint, copy it for downstream `$vibe` consumption:

```bash
mkdir -p .agents/vibe-context
cp ".agents/crank/wave-${wave}-checkpoint.json" .agents/vibe-context/latest-crank-wave.json
```

This provides `$vibe` a stable path to read the latest crank state without scanning wave checkpoint files. Uses file copy (not symlink) per repo conventions.

### Step 5.8: Wave Status Report

After each wave checkpoint, display a consolidated status table:

```
Wave $wave Status:
┌────────┬──────────────────────────────┬───────────┬────────────┬──────────┐
│ Task   │ Subject                      │ Status    │ Validation │ Duration │
├────────┼──────────────────────────────┼───────────┼────────────┼──────────┤
│ #1     │ Add auth middleware           │ completed │ PASS       │ 2m 14s   │
│ #2     │ Fix rate limiting             │ completed │ PASS       │ 1m 47s   │
│ #3     │ Update config schema          │ failed    │ FAIL       │ 3m 02s   │
└────────┴──────────────────────────────┴───────────┴────────────┴──────────┘

Epic Progress:
  Issues closed: 5/12 (wave 3 of est. 5)
  Blocked:       1 (#8, waiting on #7)
  Next wave:     #6, #7 (2 tasks, 0 conflicts)
```

This table is informational — it does not gate progression. Step 6 handles the loop decision.

### Step 5.9: Refresh Worktree Base SHA (MANDATORY)

**After committing a wave, refresh the base SHA before spawning the next wave.** This prevents cross-wave file collisions where later-wave worktrees overwrite earlier-wave fixes.

```bash
# After wave commit completes:
WAVE_COMMIT_SHA=$(git rev-parse HEAD)

# Verify the commit landed
if [[ "$WAVE_COMMIT_SHA" == "$WAVE_START_SHA" ]]; then
    echo "WARNING: Wave commit did not advance HEAD. Check for commit failures."
fi

# Next wave's worktrees MUST branch from this SHA, not the original base.
# The swarm pre-spawn step uses HEAD as the worktree base, so this is
# automatic IF the wave commit happens BEFORE the next $swarm invocation.
echo "Wave $wave committed at $WAVE_COMMIT_SHA. Next wave branches from here."
```

**Cross-wave shared file check:**

Before spawning the next wave, cross-reference the next wave's file manifests against files changed in the current wave:

```bash
# Files modified by the just-completed wave
WAVE_CHANGED=$(git diff --name-only "${WAVE_START_SHA}..HEAD")

# Files planned for next wave (from TaskCreate metadata.files)
NEXT_WAVE_FILES=(<next wave file manifests>)

# Check for overlap
OVERLAP=$(comm -12 <(echo "$WAVE_CHANGED" | sort) <(printf '%s\n' "${NEXT_WAVE_FILES[@]}" | sort))
if [[ -n "$OVERLAP" ]]; then
    echo "Cross-wave file overlap detected:"
    echo "$OVERLAP"
    echo "These files were modified in Wave $wave and are planned for Wave $((wave+1))."
    echo "Worktrees will include Wave $wave changes (branched from $WAVE_COMMIT_SHA)."
fi
```

**Why:** In na-vs9, Wave 2 worktrees were created from pre-Wave-1 SHA. A Wave 2 agent overwrote Wave 1's `.md→.json` fix in `rpi_phased_test.go` because its worktree predated the fix. Refreshing the base SHA between waves eliminates this class of collision.

### Step 6: Check for More Work

After completing a wave, check for newly unblocked issues (beads: `bd ready`, TaskList: `TaskList()`). Loop back to Step 4 if work remains, or proceed to Step 7 when done.

**For detailed check/retry logic, read `skills/crank/references/team-coordination.md`.**

### Step 7: Final Batched Validation

When all issues complete, run ONE comprehensive vibe on recent changes. Fix CRITICAL issues before completion.

If hooks or `lib/hook-helpers.sh` were modified, verify embedded copies are in sync: `cd cli && make sync-hooks`.

**For detailed validation steps, read `skills/crank/references/failure-recovery.md`.**

### Step 8: Write Phase-2 Summary

Before extracting learnings, write a phase-2 summary for downstream `$post-mortem` consumption:

```bash
mkdir -p .agents/rpi
cat > ".agents/rpi/phase-2-summary-$(date +%Y-%m-%d)-crank.md" <<PHASE2
# Phase 2 Summary: Implementation

- **Epic:** <epic-id>
- **Waves completed:** ${wave}
- **Issues completed:** <completed-count>/<total-count>
- **Files modified:** $(git diff --name-only "${WAVE_START_SHA}..HEAD" | wc -l | tr -d ' ')
- **Status:** <DONE|PARTIAL|BLOCKED>
- **Completion marker:** <promise marker from Step 9>
- **Timestamp:** $(date -Iseconds)
PHASE2
```

This summary is consumed by `$post-mortem` Step 2.2 for scope reconciliation.

### Step 8.5: Extract Learnings (ao Integration)

If ao CLI available: run `ao know forge transcript`, `ao quality flywheel close-loop --quiet`, `ao quality metrics flywheel status`, and `ao quality pool list --status=pending` to extract and review learnings. If ao unavailable, skip and recommend `$post-mortem` manually.

### Step 9: Report Completion

Tell the user:
1. Epic ID and title
2. Number of issues completed
3. Total iterations used (of 50 max)
4. Final vibe results
5. Flywheel status (if ao available)
6. Suggest running `$post-mortem` to review and promote learnings

**Output completion marker:**
```
<promise>DONE</promise>
Epic: <epic-id>
Issues completed: N
Iterations: M/50
Flywheel: <status from ao quality metrics flywheel status>
```

If stopped early:
```
<promise>BLOCKED</promise>
Reason: <global limit reached | unresolvable blockers>
Issues remaining: N
Iterations: M/50
```

## The FIRE Loop

Crank follows FIRE (Find → Ignite → Reap → Vibe → Escalate) for each wave. Loop until all issues are CLOSED (beads) or all tasks are completed (TaskList).

**For FIRE loop details, parallel wave models, and wave acceptance check, read `skills/crank/references/wave-patterns.md`.**

## Key Rules

- **Auto-detect tracking** - check for `bd` at start; use TaskList if absent
- **Plan files as input** - `$crank plan.md` decomposes plan into tasks automatically
- **If epic ID given, USE IT** - don't ask for confirmation (beads mode only)
- **Swarm for each wave** - delegates parallel execution to swarm
- **Fresh context per issue** - swarm provides Ralph pattern isolation
- **Batch validation at end** - ONE vibe at the end saves context
- **Fix CRITICAL before completion** - address findings before reporting done
- **Loop until done** - don't stop until all issues closed / tasks completed
- **Autonomous execution** - minimize human prompts
- **Respect wave limit** - STOP at 50 waves (hard limit)
- **Output completion markers** - DONE, BLOCKED, or PARTIAL (required)
- **Knowledge flywheel** - load learnings at start, forge at end (ao optional)
- **Beads ↔ TaskList sync** - in beads mode, crank bridges beads issues to TaskList for swarm

### Verb Disambiguation for Worker Prompts

Ambiguous verbs cause workers to implement the wrong operation. Use explicit instructions:

| Verb | Clarified Instruction |
|------|----------------------|
| "Extract" | "Remove from source AND write to new file. Source line count must decrease." |
| "Remove" | "Delete the content. Verify it no longer appears in the file." |
| "Update" | "Change [specific field] from [old] to [new]." |
| "Consolidate" | "Merge from [A, B] into [C]. Delete [A, B] after merge." |

Include `wc -l` assertions in task metadata when content moves between files.

---

## Examples

### Execute Epic with Beads Tracking

**User says:** `$crank ag-m0r`

Loads learnings (`ao know inject`), gets epic details (`bd show`), finds unblocked issues (`bd ready`), creates TaskList, invokes `$swarm` per wave with runtime-native spawning. Workers execute in parallel; lead verifies, commits per wave. Loops until all issues closed, then batched vibe + `ao know forge transcript`.

### Execute from Plan File (TaskList Mode)

**User says:** `$crank .agents/plans/auth-refactor.md`

Reads plan file, decomposes into TaskList tasks with dependencies. Invokes `$swarm` per wave, lead verifies and commits. Loops until all tasks completed, then final vibe.

### Test-First Epic with Contract-Based TDD

**User says:** `$crank --test-first ag-xj9`

Runs: classify issues → SPEC WAVE (contracts) → TEST WAVE (failing tests, no impl access) → RED Gate (tests must fail) → GREEN IMPL WAVES (make tests pass) → final vibe. See `skills/crank/references/test-first-mode.md`.

### Recovery from Blocked State

If all remaining issues are blocked (e.g., circular dependencies), crank outputs `<promise>BLOCKED</promise>` with the blocking chains and exits cleanly. See `skills/crank/references/failure-recovery.md`.

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "No ready issues found" | Epic has no children or all blocked | Run `$plan` first or check deps with `bd show <id>` |
| "Global wave limit (50) reached" | Excessive retries or circular deps | Review `.agents/crank/wave-N-checkpoint.json`, fix blockers manually |
| Wave vibe gate fails repeatedly | Workers producing non-conforming code | Check `.agents/council/` vibe reports, refine constraints |
| Workers complete but files missing | Permission errors or wrong paths | Check swarm output files, verify write permissions |
| RED Gate passes (tests don't fail) | Test wave workers wrote implementation | Re-run TEST WAVE with no-implementation-access prompt |
| TaskList mode can't find epic | bd CLI required for beads tracking | Provide plan file (`.md`) instead, or install bd |

See `skills/crank/references/troubleshooting.md` for extended troubleshooting.

---

## References

- **Wave patterns:** `skills/crank/references/wave-patterns.md`
- **Team coordination:** `skills/crank/references/team-coordination.md`
- **Failure recovery:** `skills/crank/references/failure-recovery.md`
- **Failure Taxonomy:** `references/failure-taxonomy.md`
- **FIRE Protocol:** `references/fire.md`

## Reference Documents

- [references/claude-code-latest-features.md](references/claude-code-latest-features.md)
- [references/commit-strategies.md](references/commit-strategies.md)
- [references/worktree-per-worker.md](references/worktree-per-worker.md)
- [references/contract-template.md](references/contract-template.md)
- [references/failure-recovery.md](references/failure-recovery.md)
- [references/failure-taxonomy.md](references/failure-taxonomy.md)
- [references/fire.md](references/fire.md)
- [references/ralph-loop-contract.md](references/ralph-loop-contract.md)
- [references/taskcreate-examples.md](references/taskcreate-examples.md)
- [references/team-coordination.md](references/team-coordination.md)
- [references/test-first-mode.md](references/test-first-mode.md)
- [references/troubleshooting.md](references/troubleshooting.md)
- [references/uat-integration-wave.md](references/uat-integration-wave.md)
- [references/wave1-spec-consistency-checklist.md](references/wave1-spec-consistency-checklist.md)
- [references/wave-patterns.md](references/wave-patterns.md)

## Local Resources

### references/

- [references/claude-code-latest-features.md](references/claude-code-latest-features.md)
- [references/commit-strategies.md](references/commit-strategies.md)
- [references/contract-template.md](references/contract-template.md)
- [references/failure-recovery.md](references/failure-recovery.md)
- [references/failure-taxonomy.md](references/failure-taxonomy.md)
- [references/fire.md](references/fire.md)
- [references/ralph-loop-contract.md](references/ralph-loop-contract.md)
- [references/taskcreate-examples.md](references/taskcreate-examples.md)
- [references/team-coordination.md](references/team-coordination.md)
- [references/test-first-mode.md](references/test-first-mode.md)
- [references/troubleshooting.md](references/troubleshooting.md)
- [references/uat-integration-wave.md](references/uat-integration-wave.md)
- [references/wave-patterns.md](references/wave-patterns.md)
- [references/wave1-spec-consistency-checklist.md](references/wave1-spec-consistency-checklist.md)
- [references/worktree-per-worker.md](references/worktree-per-worker.md)

### scripts/

- `scripts/validate.sh`


