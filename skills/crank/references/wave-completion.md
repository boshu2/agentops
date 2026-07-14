### Step 5: Verify and Sync to Beads (MANDATORY)

**External Gate Enforcement:** After each worker completes, the orchestrator (not the worker) runs the gate command. Workers must not declare their own completion. See `external-gate-protocol.md`. Swarm executes per-task validation (see `skills/shared/validation-contract.md`); crank trusts swarm validation and focuses on beads sync.

**For verification details and failure evidence, read `skills/crank/references/team-coordination.md` and `skills/crank/references/failure-recovery.md`.**

### Step 5.5: Wave Acceptance Check (MANDATORY)

> **Principle:** Verify each wave with deterministic acceptance, evidence-schema,
> and scope checks. Independent judgment runs once in downstream Validate.

**For acceptance check details (diff computation, evidence/schema checks, scope
read, verdict handoff, and CI-policy parity), read
`skills/crank/references/wave-patterns.md`.**

**CI-Policy Parity Gate (conditional):** When a wave's diff touches `.github/workflows/*.yml`, the orchestrator MUST run `bash scripts/validate-ci-policy-parity.sh` as part of wave acceptance. On failure, the wave verdict is **FAIL**. See [wave-patterns.md](wave-patterns.md) "CI-Policy Parity Gate" for trigger pattern, worked example, and the soc-lmww1 / commit `c587b361` motivation.

### Step 5.7: Wave Checkpoint

After each wave completes (post-vibe-gate, pre-next-wave), write `.agents/crank/wave-${wave}-checkpoint.json` with fields: `schema_version`, `wave`, `timestamp`, `tasks_completed`, `tasks_failed`, `files_changed`, `git_sha`, `acceptance_verdict` (from Step 5.5), `commit_strategy`, `mutations_this_wave`, `total_mutations`, `mutation_budget` (task_added limit 5, task_reordered limit 3), and `criterion_verdicts` (per-criterion roll-up — see below). A separately admitted re-execution of the same wave replaces the checkpoint with its newer evidence.

**Per-criterion verdicts:** When the wave's beads carried an `acceptance_criteria` block (see Step 4 acceptance-criteria injection), record one verdict per criterion id:

```json
"criterion_verdicts": [
  {"id": "ac-<scope>.<n>", "status": "PASS|FAIL|SKIP", "evidence_path": "<path>", "notes": "<one-liner>"}
]
```

`evidence_path` points to a file or log line that justifies the verdict (test output, grep result, gate report). `SKIP` is reserved for criteria not exercised this wave (e.g., gated by a flag, deferred to a later wave) and must include a `notes` reason.

**Back-compat fallback (back-compat):** When the bead has no `acceptance_criteria` block, omit `criterion_verdicts` from the checkpoint and emit a WARN log line: `[deprecated] no acceptance_criteria found in packet — running vibe-only`. This is the premortem advisory fix #2 ramp: WARN until **2026-06-30**, then FAIL after that date. Tracker beads created before this rollout date are grandfathered for that window; new packets must include the block.

Immediately validate the checkpoint before using it downstream:

```bash
bash skills/crank/scripts/validate-wave-checkpoint.sh ".agents/crank/wave-${wave}-checkpoint.json"
```

The validator fails closed when `git_sha` does not resolve in the current repo, `timestamp` is invalid or more than 5 minutes in the future, or required checkpoint fields are missing/malformed. Do not proceed to Step 5.7b until this passes.

### Step 5.7b: Vibe Context Checkpoint

Copy the wave checkpoint to `.agents/vibe-context/latest-crank-wave.json` for downstream `/validate` consumption. Use file copy (not symlink) per repo conventions.

### Step 5.7c: Update Shared Task Notes (After Wave)

Harvest `## Discoveries` sections from completed worker results and append to `.agents/crank/SHARED_TASK_NOTES.md`. Also capture failed approaches from wave failures. See [shared-task-notes.md](shared-task-notes.md) for the harvest script and size management rules.

### Step 5.7d: Log Plan Mutations (After Wave)

Call `log_plan_mutation` for each plan change during this wave: DECOMPOSE → `task_removed` + `task_added` per sub-task, PRUNE → `task_removed`, scope/dependency/reorder changes → matching mutation type. See [plan-mutations.md](plan-mutations.md) for the full logging examples and budget enforcement.

### Step 5.8: Wave Status Report

Display a consolidated status table (task, subject, status, validation, duration) plus epic progress (issues closed, blocked, next wave). Informational — does not gate progression.

### Step 5.9: Refresh Worktree Base SHA (MANDATORY)

After committing a wave, verify HEAD advanced past `WAVE_START_SHA`. Next wave's worktrees must branch from this new SHA to prevent cross-wave file collisions. Before spawning the next wave, cross-reference next wave's file manifests (`metadata.files`) against `git diff --name-only "${WAVE_START_SHA}..HEAD"` — log any overlap so workers are aware of prior-wave changes in their worktree base.

### Step 6: Report Remaining Work

After completing a wave, check for newly unblocked issues (beads: `bd ready`,
TaskList: `TaskList()`). Record `remaining_work` and return PARTIAL when another
wave exists. Do not loop to Step 4. The wave evidence must pass through
Validate and Learn before the orchestrator may invoke Crank again. If no work
remains, proceed to the final evidence summary.

**For detailed check and return-boundary logic, read `skills/crank/references/team-coordination.md`.**

### Step 6.5: De-Sloppify Pass (Optional)

If implementation waves produced significant output (>200 lines changed), run an optional cleanup pass before final validation. This uses a separate focused worker — see `de-sloppify.md` for the full pattern.

**De-sloppify targets:** coverage-padding tests, debug logging, commented-out code, over-defensive error handling, dead imports. Does NOT touch business logic or behavioral tests.

**Skip if:** Total changes < 50 lines, or epic is docs/chore only.

```bash
# Quick slop scan before deciding whether to de-sloppify
SLOP_COUNT=$(git diff --name-only "${FIRST_WAVE_SHA}..HEAD" | xargs grep -l 'fmt\.Println\|console\.log\|# TODO\|// TODO\|commented out' 2>/dev/null | wc -l | tr -d ' ')
if [[ "$SLOP_COUNT" -gt 0 ]]; then
    echo "De-sloppify: $SLOP_COUNT files with potential slop detected"
    # Spawn single cleanup worker (no parallelism needed)
fi
```

> *(orchestrator-owned: this pre-scan grep + wc is an inline gate decision, not worker-delegated. The orchestrator checks slop presence to decide whether to spawn a de-sloppify worker. Do NOT move pattern matching into a separate `Skill()` call.)*

### Step 6.9: Pre-Vibe Lifecycle Checks

Skip if `--no-lifecycle` is set.

```
a) if dependency files changed (go.mod, go.sum, package.json, package-lock.json,
     requirements.txt, poetry.lock, Cargo.toml, Cargo.lock, Gemfile, Gemfile.lock):
     Skill(skill="deps", args="vuln --quick")
     CRITICAL vulns (CVSS >= 9.0): BLOCK (treat as test failure — fix before vibe).
     All others: WARN, append to phase summary.

b) Skill(skill="test", args="coverage --quick")
     Append coverage report to vibe context.
```

### Step 7: Final Evidence Handoff

When all issues complete, assemble the acceptance roll-up and changed-surface
evidence for one final Validate invocation by the caller/orchestrator. Crank
does not invoke Validate itself.

If hooks or `lib/hook-helpers.sh` were modified, verify embedded copies are in sync: `cd cli && make sync-hooks`.

**For detailed validation steps, read `skills/crank/references/failure-recovery.md`.**

### Step 8: Write Phase-2 Summary

Before extracting learnings, write a phase-2 summary for downstream `/validate` consumption:

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

This summary is consumed by `/validate` for scope reconciliation. Validate
does not run learning or Premortem inline.

### Step 8.5: Emit Wave Evidence

Return the phase summary and evidence references to Validate. Do not extract or
promote learnings here; Learn owns post-verdict bookkeeping after Validate.

### Step 8.6: Archive Shared Task Notes

Archive `.agents/crank/SHARED_TASK_NOTES.md` to `.agents/crank/archives/` as
evidence for downstream Validate and Learn. See
[shared-task-notes.md](shared-task-notes.md) for the archive script.

### Step 8.7: Scope-Completion Check (Pre-Close Gate)

Before marking the epic DONE, verify planned acceptance criteria are met:

1. Read the plan from `.agents/plans/` (most recent matching the epic)
2. Extract acceptance criteria from each issue's `## Acceptance` section
3. For each criterion, check current state:
   - `files_exist`: verify file paths exist
   - `content_check`: grep for expected patterns
   - `command`: run verification commands
4. Report results:
   - All criteria met → proceed to Step 9
   - Any criteria NOT met → **WARN** with list of unmet criteria (do not block — validation phase catches remaining gaps)

Example: `PLAN_FILE=$(ls -t .agents/plans/*.md 2>/dev/null | head -1)` then extract and verify each acceptance criterion from the plan.

**Opt-out:** `--no-scope-check` flag.

> *(orchestrator-owned: this gate runs file-existence checks, grep patterns, and verification commands directly. Do NOT delegate acceptance-criterion validation to workers — the orchestrator evaluates closure readiness before emitting `<promise>DONE</promise>`.)*
