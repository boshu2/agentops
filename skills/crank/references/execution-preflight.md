### Recovery Hooks

Register a `PostCompact` hook: `"command": "cat .agents/crank/wave-*-checkpoint.json | tail -1"` to auto-recover wave state after compaction. Consider `worktree.sparsePaths` to reduce worktree size.

**Effort levels per worker type:**

| Worker Role | Recommended Effort | Rationale |
|-------------|-------------------|-----------|
| SPEC wave (contracts) | `medium` | Balanced reasoning for spec generation |
| TEST wave (failing tests) | `medium` | Test scaffolding needs moderate depth |
| IMPL wave (make tests pass) | `high` | Deep reasoning for correct implementation |
| Docs/chore tasks | `low` | Fast execution for simple tasks |

**Effort-to-Tier Mapping:** high→opus, medium→sonnet, low→haiku. Used for council calls (wave acceptance, final vibe). Override with `--tier=<name>` flag or `models.skill_overrides.crank` in `.agentops/config.yaml`.

### Step 0: Load Knowledge Context (ao Integration)

If ao CLI available, pull relevant knowledge: `ao lookup --query "<epic-title>" --limit 5`, `ao metrics flywheel status`, `ao ratchet status`. Apply retrieved learnings as implementation constraints and cite with `ao metrics cite "<path>" --type applied` (influenced decision) or `--type retrieved` (loaded but not referenced). Prefer `matched_snippet` over full files when lookup results include section evidence. If ao unavailable, skip and proceed.

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
| **Record failure evidence** | `bd comments add` | Task description update |
| **Run admission** | Persistent RPI governor state | Persistent RPI governor state |

### Step 0.6: Select Dispatch Backend (NTM > runtime-native > beads floor)

Select the wave-dispatch backend per the canonical ladder in `skills/shared/SKILL.md` ("Selection policy"): if `AGENTOPS_ORCHESTRATION=off`, degrade to the **beads floor** (create issues, validate; no spawn). Otherwise prefer **NTM** (capability-probed via `ntm --robot-capabilities`), then **runtime-native** (Claude Native Teams / Codex sub-agents) via `/swarm`. Output-contract parity is unchanged on every tier: workers write `.agents/swarm/results/*.json`, crank verifies-then-trusts.

> **gc pool is NOT selected (DEPRECATION).** gc tier removed (soc-2rtm0); retained for historical reference only — NOT selected. The Gas City (`gc`) CLI bridge was removed and `runtime=gc` is rejected by the CLI (see `agentops/CLAUDE.md`). [gc-pool-dispatch.md](gc-pool-dispatch.md) documents the old gc pool dispatch shape for archival purposes only — the top tier is **NTM**.

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
WARN: Multiple open epics detected. /crank operates on a single epic.
Use --allow-multi-epic to suppress this warning.
```
If multiple epics found, ask user which one (WARN, not FAIL).

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

### Step 1.5: Branch Isolation Gate

Before wave-1 commit, refuse to crank on `main`/`master`. Cut `crank/<epic-id>` to prevent parallel-session reset clobbers. See [branch-isolation.md](branch-isolation.md) for the gate script and override flag.

### Step 1a: Load The Persistent Run Governor

RPI initializes one run before any phase dispatch. Crank receives the stable
`RPI_RUN_ID` and `RPI_GOVERNOR_STATE_DIR`; it never initializes or resets
either. Before selecting a dispatch backend, require both values and require
the existing run state to pass the governor's fail-closed validation. The same
state is resumed in beads and TaskList modes and across fresh processes.

The actual charge is recorded by the atomic admission in
[wave-dispatch.md](wave-dispatch.md). Preflight must not add a second counter in
epic notes, task descriptions, memory, or checkpoint files.

### Step 1a.1: Initialize Plan Mutation Audit Trail

```bash
mkdir -p .agents/rpi
: > .agents/rpi/plan-mutations.jsonl
```

Initialize the `log_plan_mutation` helper for audit evidence only. See
[plan-mutations.md](plan-mutations.md) for its JSONL schema and mutation types;
mutation logging does not authorize dispatch or create a budget.

### Step 1a.2: Initialize Shared Task Notes

```bash
mkdir -p .agents/crank
cat > .agents/crank/SHARED_TASK_NOTES.md <<EOF
# Shared Task Notes — Epic ${EPIC_ID:-unknown}
> Cross-wave context for workers. Read before starting.
EOF
```

See [shared-task-notes.md](shared-task-notes.md) for the full pattern, size management, and worker integration.

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
   - Epic has no child issues (run /plan first)
   - All issues already completed"
```

Also verify: epic has at least 1 child issue total. An epic with 0 children means /plan was not run.

Do NOT proceed with empty issue list - this produces false "epic complete" status.

### Step 3a.1: Pre-flight Check - Premortem Required (3+ issues)

If the epic has 3+ child issues, look for its Premortem report. If none exists,
emit `<promise>BLOCKED</promise>` and stop — run `/premortem` first. Premortems
have positive ROI for 3+ issue epics; cost (~2 min) is negligible.

### Step 3a.2: Pre-flight Check - Bead Audit (Stale/Fixed/Consolidatable)

Run `scripts/bd-audit.sh --json` (beads mode only) before wave execution to avoid burning compute on dead work. **WARNING gate** — warns on any flagged beads, **blocks at >50%** flagged. Use `--skip-audit` to bypass. If blocked, clean up with `scripts/bd-audit.sh --auto-close` and `scripts/bd-cluster.sh --auto-merge`, then re-run crank.

### Step 3a.3: Pre-flight Check - Changed-String Grep

**Before spawning workers, grep for every string being changed by the plan.**

This catches stale cross-references that the plan missed. Grep for each key term being modified across the codebase. Matches outside the planned file set indicate scope gaps — add those files to the epic or document as tech debt.

> *(orchestrator-owned: this scan is intentionally inline, not a `Skill()` delegation. Do NOT spawn a worker for this check.)*
