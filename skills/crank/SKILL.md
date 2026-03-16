---
name: crank
description: 'Hands-free epic execution. Runs until ALL children are CLOSED. Uses /swarm with runtime-native spawning (Codex sub-agents or Claude teams). NO human prompts, NO stopping. Triggers: "crank", "run epic", "execute epic", "run all tasks", "hands-free execution", "crank it".'
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: full
metadata:
  tier: execution
  dependencies:
    - swarm       # required - executes each wave
    - vibe        # required - final validation
    - implement   # required - individual issue execution
    - beads       # optional - issue tracking via bd CLI (fallback: TaskList)
    - post-mortem # optional - suggested for learnings extraction
---

# Crank Skill

> **Quick Ref:** Autonomous epic execution. `/swarm` for each wave with runtime-native spawning. Output: closed issues + final vibe.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

Autonomous execution: implement all issues until the epic is DONE.

**CLI dependencies:** bd (issue tracking), ao (knowledge flywheel). Both optional -- see `skills/shared/SKILL.md` for fallback table. If bd is unavailable, use TaskList for issue tracking and skip beads sync. If ao is unavailable, skip knowledge injection/extraction.

For Claude runtime feature coverage, the shared source of truth is `skills/shared/references/claude-code-latest-features.md`, mirrored locally at `references/claude-code-latest-features.md`.

## Architecture: Crank + Swarm

**Crank** = Orchestration, epic/task lifecycle, knowledge flywheel. **Swarm** = Runtime-native parallel execution (Ralph Wiggum pattern via fresh worker set per wave).

**Beads mode** (bd available): Crank discovers ready issues via `bd ready`, creates TaskList entries from them, invokes `/swarm` per wave, verifies results and updates bd, loops until epic DONE.

**TaskList mode** (bd unavailable): Crank uses TaskList directly as source of truth, invokes `/swarm` per wave, verifies via TaskList, loops until all completed.

Ralph alignment source: `../shared/references/ralph-loop-contract.md` (fresh context, scheduler/worker split, disk-backed state, backpressure).

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--test-first` | off | Spec-first TDD: SPEC WAVE generates contracts, TEST WAVE generates failing tests, IMPL WAVES make tests pass |
| `--per-task-commits` | off | Per-task commit strategy. Falls back to wave-batch when file boundaries overlap. See `references/commit-strategies.md`. |

## Global Limits

**MAX_EPIC_WAVES = 50** (hard limit). Typical epics use 5-10 waves.

## Completion Enforcement (The Sisyphus Rule)

After each wave, output one of:
- `<promise>DONE</promise>` - Epic complete, all issues closed
- `<promise>BLOCKED</promise>` - Cannot proceed (with reason)
- `<promise>PARTIAL</promise>` - Incomplete (with remaining items)

**Never claim completion without the marker.**

## Execution Steps

Given `/crank [epic-id | plan-file.md | "description"]`:

### Recovery Hooks

Register a `PostCompact` hook to auto-recover context if the session compacts mid-crank:

```json
{
  "event": "PostCompact",
  "command": "cat .agents/crank/wave-*-checkpoint.json | tail -1"
}
```

Also consider `worktree.sparsePaths` in project settings to reduce worktree size for large repos.

**Effort levels:** SPEC/TEST waves use `medium` effort. IMPL waves use `high`. Docs/chore use `low`.

### Step 0: Load Knowledge Context & Detect Tracking Mode

**Knowledge (ao integration):** If ao CLI available, run `ao lookup --query "<epic-title>" --limit 5`, `ao metrics flywheel status`, and `ao ratchet status`. If ao unavailable, skip.

**Tracking mode:** Check `command -v bd`. If available, set `TRACKING_MODE="beads"`. Otherwise, `TRACKING_MODE="tasklist"` and use TaskList for all issue tracking.

| Operation | Beads Mode | TaskList Mode |
|-----------|-----------|---------------|
| Find work | `bd ready` | `TaskList()` filtered for pending+unblocked |
| Get details | `bd show <id>` | `TaskGet(taskId)` |
| Mark complete | `bd update <id> --status closed` | `TaskUpdate(taskId, status="completed")` |
| Track retries | `bd comments add` | Task description update |

### Step 1: Identify the Epic / Work Source

**Beads mode:**
- If epic ID provided, use it directly. Do NOT ask for confirmation.
- If no epic ID: discover via `bd list --type epic --status open | head -5`.
- If multiple epics found, warn and ask user which one.

**TaskList mode:**
- Epic ID input: error -- bd required for beads epic tracking. Provide plan file or task list instead.
- Plan file (`.md`): read, decompose into TaskList tasks with `TaskCreate`, set up dependencies via `TaskUpdate(addBlockedBy)`.
- No input: check `TaskList()` for existing pending tasks; if none, ask user.
- Description string: decompose into tasks, set up dependencies.

Initialize wave counter: `wave=0`. In beads mode, also run `bd update <epic-id> --append-notes "CRANK_START: wave=0 at $(date -Iseconds)"`.

### Step 1b: Detect Test-First Mode (--test-first only)

If `--test-first` is set, classify issues into spec-eligible (feature/bug/task) and spec-skip (docs/chore/ci/epic). In beads mode, use `bd show` to get issue types. In TaskList mode, default all to spec-eligible.

If `--test-first` is NOT set, skip Steps 3b and 3c entirely.

### Step 2: Get Epic Details

**Beads mode:** `bd show <epic-id>`. **TaskList mode:** `TaskList()` to see all tasks and status/dependencies.

### Step 3: Pre-flight Checks & List Ready Issues

**Find current wave:** Beads mode uses `bd ready`, TaskList mode filters for pending+unblocked tasks.

**Pre-flight gates (all mandatory):**

1. **Issues exist:** If 0 ready issues found, STOP with error. Verify epic has at least 1 child issue (otherwise `/plan` was not run). Do NOT proceed with empty issue list.

2. **Pre-mortem required (3+ issues):** If epic has 3+ child issues, check `.agents/council/` for pre-mortem evidence. If missing, output `<promise>BLOCKED</promise>` with reason "pre-mortem required" and STOP.

3. **Changed-string grep:** Grep for every string being changed by the plan across the codebase. Matches outside the planned file set indicate scope gaps -- add those files to the epic or document as tech debt.

### Step 3b: SPEC WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each spec-eligible issue:
1. **TaskCreate** with subject `SPEC: <issue-title>`
2. Worker receives: issue description, plan boundaries, contract template (`skills/crank/references/contract-template.md`), codebase access (read-only)
3. Worker generates: `.agents/specs/contract-<issue-id>.md`
4. **Validation:** files_exist + content_check for `## Invariants` AND `## Test Cases`
5. **Wave 1 spec consistency checklist (MANDATORY):** run `skills/crank/references/wave1-spec-consistency-checklist.md` across all contracts. If any item fails, re-run SPEC workers and do NOT proceed to TEST WAVE.
6. Lead commits all specs after validation

For BLOCKED recovery and full worker prompt, read `skills/crank/references/test-first-mode.md`.

### Step 3c: TEST WAVE (--test-first only)

**Skip if `--test-first` is NOT set or if no spec-eligible issues exist.**

For each spec-eligible issue:
1. **TaskCreate** with subject `TEST: <issue-title>`
2. Worker receives: contract-<issue-id>.md + codebase types (NOT implementation code)
3. Worker generates: failing test files. Workers classify tests by pyramid level (L0-L3). If `test_levels` metadata exists, workers MUST generate tests at each required level.
4. **RED Gate:** Lead runs test suite -- ALL new tests must FAIL
5. Lead commits test harness after RED Gate passes

For RED Gate enforcement and retry logic, read `skills/crank/references/test-first-mode.md`.

### Step 3b.1: Build Context Briefing (Before Worker Dispatch)

If ao CLI available, run `ao context assemble --task='<epic title>: wave $wave'` to produce a briefing at `.agents/rpi/briefing-current.md`. Include the briefing path in each worker's TaskCreate.

- **Claude workers:** Include `Knowledge artifacts are in .agents/. See .agents/AGENTS.md for navigation. Use \`ao lookup --query "topic"\` for learnings.`
- **Codex workers:** Lead searches `.agents/learnings/` and inlines top 3 results directly in worker prompt body (no `.agents/` file access in sandbox).

### Step 4: Execute Wave via Swarm

**GREEN mode (--test-first only):** If SPEC/TEST waves completed, include in each spec-eligible worker's TaskCreate: `"Failing tests exist at <test-file-paths>. Make them pass. Do NOT modify test files. See GREEN Mode rules in /implement SKILL.md."` Docs/chore/ci issues use standard prompts unchanged.

**Required metadata for every TaskCreate:**

- **`metadata.issue_type`:** Feeds constraint applicability and validation policy. Do not omit.
- **`metadata.files`:** File manifest for pre-spawn conflict detection. Two workers claiming the same file get serialized or worktree-isolated.
- **`metadata.grep_check`:** For new function issues ("create"/"add"/"implement"), include the function name pattern. Workers MUST grep for existing implementations before writing new code.
- **`metadata.validation`:** For `feature|bug|task`, include `tests` plus at least one structural check (`files_exist` or `content_check`). For `docs|chore|ci`, use test-exempt path with structural/lint checks. Carry forward `test_levels` from `/plan` into `metadata.validation.test_levels`. See test pyramid standard (`test-pyramid.md` in standards skill).

**Language Standards Injection (code tasks):** Detect project language from repo root markers (`go.mod`, `pyproject.toml`, `Cargo.toml`, `package.json`). Lead Reads the applicable standards file and includes the Testing section verbatim in worker task descriptions. For test-modifying issues, also inject file naming conventions, assertion quality rules, and pre-commit verification commands.

**Validation block extraction (beads mode):** Extract `validation` fenced blocks from `bd show` output for each issue. If present, use as `metadata.validation`. If absent, generate fallback `files_exist` check from file paths mentioned in the issue body.

```
TaskCreate(
  subject="ag-1234: Add auth middleware",
  description="...",
  activeForm="Implementing ag-1234",
  metadata={
    "issue_type": "feature",
    "files": ["src/middleware/auth.py", "tests/test_auth.py"],
    "validation": {
      "tests": "pytest tests/test_auth.py -v",
      "files_exist": ["src/middleware/auth.py", "tests/test_auth.py"]
    }
  }
)
```

**File ownership verification:** Before spawning, verify the ownership map (from swarm Step 1.5) has zero unresolved conflicts. If conflicts > 0, do NOT invoke `/swarm` -- serialize conflicting tasks into sub-waves or merge scope.

**Before each wave:**
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
fi
```

**Pre-Spawn: Spec Consistency Gate:** If `.agents/specs/contract-*.md` exist, run `bash scripts/spec-consistency-gate.sh .agents/specs/` -- hard failures block spawn, WARN-level issues do not.

**Cross-cutting constraint injection (SDD):** If the plan has a `## Boundaries` section, extract "Always" boundaries and inject into every TaskCreate's `metadata.validation.cross_cutting`. "Ask First" boundaries: log as annotation in auto mode, prompt in `--interactive` mode.

**For wave execution details (beads sync, TaskList bridging, swarm invocation), read `skills/crank/references/team-coordination.md`.**

**Cross-cutting validation (SDD):** After per-task validation passes, run cross-cutting checks across all files modified in the wave (`git diff --name-only "${WAVE_START_SHA}..HEAD"`).

### Step 5: Verify, Checkpoint, and Report

> Swarm executes per-task validation (see `skills/shared/validation-contract.md`). Crank trusts swarm validation and focuses on beads sync and wave acceptance.

**For verification details, retry logic, and failure escalation, read `skills/crank/references/team-coordination.md` and `skills/crank/references/failure-recovery.md`.**

**Wave Acceptance Check:** Verify each wave meets acceptance criteria using lightweight inline judges. No skill invocations -- prevents context explosion. **For details, read `skills/crank/references/wave-patterns.md`.**

**Wave Checkpoint:** After each wave completes, write checkpoint:

```bash
mkdir -p .agents/crank .agents/vibe-context

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

# Copy for downstream /vibe consumption (file copy, not symlink)
cp ".agents/crank/wave-${wave}-checkpoint.json" .agents/vibe-context/latest-crank-wave.json
```

On retry of the same wave, the file is overwritten (same path).

**Wave Status Report:** Display consolidated status table after each wave:

```
Wave $wave Status:
| Task | Subject | Status | Validation | Duration |
Epic Progress: Issues closed: X/Y (wave N of est. M), Blocked: ..., Next wave: ...
```

This table is informational -- it does not gate progression.

**Refresh Worktree Base SHA (MANDATORY):** After committing a wave, verify HEAD advanced past `WAVE_START_SHA`. Next wave's worktrees MUST branch from the new HEAD. Cross-reference next wave's file manifests against files changed in the current wave to detect overlap -- overlapping files will include current wave changes since worktrees branch from the updated SHA.

### Step 6: Check for More Work

After completing a wave, check for newly unblocked issues (beads: `bd ready`, TaskList: `TaskList()`). Loop back to Step 4 if work remains, or proceed to Step 7 when done.

**For detailed check/retry logic, read `skills/crank/references/team-coordination.md`.**

### Step 7: Final Batched Validation

When all issues complete, run ONE comprehensive vibe on recent changes. Fix CRITICAL issues before completion.

If hooks or `lib/hook-helpers.sh` were modified, verify embedded copies are in sync: `cd cli && make sync-hooks`.

**For detailed validation steps, read `skills/crank/references/failure-recovery.md`.**

### Step 8: Summary and Learnings

**Phase-2 summary:** Write to `.agents/rpi/phase-2-summary-$(date +%Y-%m-%d)-crank.md` with epic ID, waves completed, issues completed, files modified count, status, completion marker, and timestamp. Consumed by `/post-mortem` Step 2.2.

**Extract learnings (ao integration):** If ao CLI available, run `ao forge transcript`, `ao flywheel close-loop --quiet`, `ao metrics flywheel status`, and `ao pool list --status=pending`. If ao unavailable, skip and recommend `/post-mortem` manually.

### Step 9: Report Completion

Tell the user: epic ID/title, issues completed count, iterations used (of 50 max), final vibe results, flywheel status (if ao available). Suggest running `/post-mortem`.

**Output completion marker:**
```
<promise>DONE</promise>
Epic: <epic-id>
Issues completed: N
Iterations: M/50
Flywheel: <status from ao metrics flywheel status>
```

If stopped early:
```
<promise>BLOCKED</promise>
Reason: <global limit reached | unresolvable blockers>
Issues remaining: N
Iterations: M/50
```

## The FIRE Loop

Crank follows FIRE (Find -> Ignite -> Reap -> Vibe -> Escalate) for each wave. Loop until all issues are CLOSED (beads) or all tasks are completed (TaskList).

**For FIRE loop details, parallel wave models, and wave acceptance check, read `skills/crank/references/wave-patterns.md`.**

## Key Rules

- **Auto-detect tracking** - check for `bd` at start; use TaskList if absent
- **Plan files as input** - `/crank plan.md` decomposes plan into tasks automatically
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
- **Beads <-> TaskList sync** - in beads mode, crank bridges beads issues to TaskList for swarm

### Verb Disambiguation for Worker Prompts

| Verb | Clarified Instruction |
|------|----------------------|
| "Extract (file)" | "Remove from source AND write to new file. Source line count must decrease." |
| "Extract (spec)" | "Generate a specification document from issue/task metadata. Source is unchanged." |
| "Remove" | "Delete the content. Verify it no longer appears in the file." |
| "Update" | "Change [specific field] from [old] to [new]." |
| "Consolidate" | "Merge from [A, B] into [C]. Delete [A, B] after merge." |

Include `wc -l` assertions in task metadata when content moves between files.

## Examples

**User says:** `/crank ag-m0r` -- Beads epic: loads learnings, swarm per wave, loops until all closed, final vibe.

**User says:** `/crank .agents/plans/auth-refactor.md` -- Plan file: decomposes into tasks, swarm per wave, final vibe.
**User says:** `/crank --test-first ag-xj9` -- SPEC -> TEST -> RED Gate -> GREEN IMPL. See `references/test-first-mode.md`.

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "No ready issues found" | Epic has no children or all blocked | Run `/plan` first or check deps with `bd show <id>` |
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
