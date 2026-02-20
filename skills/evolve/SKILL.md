---
name: evolve
description: Goal-driven fitness-scored improvement loop. Measures goals, picks worst gap, runs /rpi (or parallel /swarm of /rpi cycles), compounds via knowledge flywheel.
metadata:
  tier: execution
  dependencies:
    - rpi         # required - executes each improvement cycle
    - post-mortem # required - auto-runs at teardown to harvest learnings
  triggers:
    - evolve
    - improve everything
    - autonomous improvement
    - run until done
---

# /evolve — Goal-Driven Compounding Loop

> **Purpose:** Measure what's wrong. Fix the worst thing. Measure again. Compound.

Thin fitness-scored loop over `/rpi`. The knowledge flywheel provides compounding — each cycle loads learnings from all prior cycles.

## Compaction Resilience

The evolve loop MUST survive context compaction. Every cycle commits its
artifacts to git before proceeding. The `cycle-history.jsonl` file is the
recovery point — on session restart, read it to determine cycle number
and resume from Step 1.

## Known Good Properties

- Severity-based selection naturally orders: code health → architecture →
  testing → documentation → cleanup. This is the correct ordering.
  Do not add special-case logic to front-load doc fixes.

**Dormancy is success.** When all goals pass and no harvested work remains, the system enters dormancy — a valid, healthy state. The system does not manufacture work to justify its existence. Nothing to do means everything is working.

## Quick Start

```bash
/evolve                      # Run forever until kill switch or stagnation
/evolve --max-cycles=5       # Cap at 5 improvement cycles
/evolve --dry-run            # Measure fitness, show what would be worked on, don't execute
```

## Execution Steps

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

### Step 0: Setup

```bash
mkdir -p .agents/evolve
```

Load accumulated learnings (COMPOUNDING):
```bash
ao inject 2>/dev/null || true
```

Parse flags:
- `--max-cycles=N` (default: **unlimited**) — optional hard cap. Without this flag, the loop runs **forever** until kill switch or stagnation.
- `--dry-run` — measure and report only, no execution
- `--skip-baseline` — skip the Step 0.5 baseline sweep
- `--parallel` — enable parallel goal execution via /swarm per cycle (default: off, sequential)
- `--max-parallel=N` (default: **3**, cap: **5**) — max goals to improve in parallel per cycle. Only meaningful with `--parallel`.

**Capture session-start SHA** (for multi-commit revert):
```bash
SESSION_START_SHA=$(git rev-parse HEAD)
```

Initialize state:
```
evolve_state = {
  cycle: 0,
  max_cycles: <from flag, or Infinity if not set>,
  dry_run: <from flag, default false>,
  test_first: <from flag, default false>,
  parallel: <from --parallel flag, default false>,
  max_parallel: <from --max-parallel flag, default 3, cap 5>,
  session_start_sha: $SESSION_START_SHA,
  idle_streak: 0,         # consecutive cycles with nothing to do
  max_idle_streak: 3,     # stop after this many consecutive idle cycles
  history: []
}
```

### Step 0.5: Cycle-0 Baseline Sweep

Capture a baseline fitness snapshot before the first cycle so every later cycle
has a comparison anchor.  Skipped on resume (idempotent).

```
if [ "$SKIP_BASELINE" = "true" ]; then
  log "Skipping baseline sweep (--skip-baseline flag set)"
  exit 0
fi

if ! [ -f .agents/evolve/fitness-0-baseline.json ]; then
  # **Preferred (when ao CLI available):**
  if command -v ao &>/dev/null; then
    ao goals measure --json > .agents/evolve/fitness-0-baseline.json
  fi

  # **Fallback (no ao CLI):**
  baseline = MEASURE_FITNESS()            # run every GOALS.yaml goal
  baseline.cycle = 0
  write ".agents/evolve/fitness-0-baseline.json" baseline

  # Baseline report
  failing = [g for g in baseline.goals if g.result == "fail"]
  failing.sort(by=weight, descending)
  cat > .agents/evolve/cycle-0-report.md << EOF
  # Cycle-0 Baseline
  **Total goals:** ${len(baseline.goals)}
  **Passing:** ${len(baseline.goals) - len(failing)}
  **Failing:** ${len(failing)}
  $(for g in failing: "- [weight ${g.weight}] ${g.id}: ${g.result}")
  EOF

  log "Baseline captured: ${len(failing)}/${len(baseline.goals)} goals failing"
fi

# Wiring closure check: every check-*.sh must appear in GOALS.yaml
unwired=$(comm -23 \
  <(ls scripts/check-*.sh 2>/dev/null | xargs -I{} basename {} | sort) \
  <(grep -oP 'scripts/check-\S+\.sh' GOALS.yaml | xargs -I{} basename {} | sort))
if [ -n "$unwired" ]; then
  for script in $unwired; do
    add_to_next_work("Unwired script: $script — wire to GOALS.yaml or delete",
                     severity="high", type="tech-debt")
  done
  log "Found $(echo "$unwired" | wc -l | tr -d ' ') unwired scripts — added to next-work"
fi
```

### Step 1: Kill Switch Check

Check at the TOP of every cycle iteration:

```bash
# External kill (outside repo — can't be accidentally deleted by agents)
if [ -f ~/.config/evolve/KILL ]; then
  echo "KILL SWITCH ACTIVE: $(cat ~/.config/evolve/KILL)"
  # Write acknowledgment
  echo "{\"killed_at\": \"$(date -Iseconds)\", \"cycle\": $CYCLE}" > .agents/evolve/KILLED.json
  exit 0
fi

# Local convenience stop
if [ -f .agents/evolve/STOP ]; then
  echo "STOP file detected: $(cat .agents/evolve/STOP 2>/dev/null)"
  exit 0
fi
```

If either file exists, log reason and **stop immediately**. Do not proceed to measurement.

### Step 2: Measure Fitness (MEASURE_FITNESS)

Read `GOALS.yaml` from repo root.

**Preferred (when ao CLI available):**
```bash
if command -v ao &>/dev/null; then
  ao goals measure --json > .agents/evolve/fitness-${CYCLE}-snapshot.json
fi
```

**Fallback (no ao CLI):**

For each goal:

```bash
# Run the check command
if eval "$goal_check" > /dev/null 2>&1; then
  # Exit code 0 = PASS
  result = "pass"
else
  # Non-zero = FAIL
  result = "fail"
fi
```

Record results with **continuous values** (not just pass/fail):
```bash
# Write fitness snapshot
cat > .agents/evolve/fitness-${CYCLE}.json << EOF
{
  "cycle": $CYCLE,
  "timestamp": "$(date -Iseconds)",
  "cycle_start_sha": "$(git rev-parse HEAD)",
  "goals": [
    {"id": "$goal_id", "result": "$result", "weight": $weight, "value": $metric_value, "threshold": $threshold},
    ...
  ]
}
EOF
```

For goals with measurable metrics, extract the continuous value:
- `go-coverage-floor`: parse `go test -cover` output → `"value": 85.7, "threshold": 80`
- `doc-coverage`: count skills with references/ → `"value": 20, "threshold": 16`
- `shellcheck-clean`: count of warnings → `"value": 0, "threshold": 0`
- Other goals: `"value": null` (binary pass/fail only)

**Snapshot enforcement (HARD GATE):** After writing the snapshot, validate it:
```bash
if ! jq empty ".agents/evolve/fitness-${CYCLE}.json" 2>/dev/null; then
  echo "ERROR: Fitness snapshot write failed or invalid JSON. Refusing to proceed."
  exit 1
fi
```
Do NOT proceed to Step 3 without a valid fitness snapshot.

**Bootstrap mode:** If a check command fails to execute (command not found, permission denied), mark that goal as `"result": "skip"` with a warning. Do NOT block the entire loop because one check is broken.

### Step 3: Select Work

```
failing_goals = [g for g in goals if g.result == "fail"]

if not failing_goals:
  # Cycle-0 Comprehensive Sweep (optional full-repo scan)
  # Before consuming harvested work, optionally discover items the harvest missed.
  # This is because manual sweeps have found issues automated harvests didn't catch.

  if ! [ -f .agents/evolve/last-sweep-date ] || \
     [ $(date +%s) -gt $(( $(stat -f %m .agents/evolve/last-sweep-date) + 604800 )) ]; then
    # Sweep is stale (> 7 days) or missing — run lightweight scan
    log "Running cycle-0 comprehensive sweep (stale/missing: .agents/evolve/last-sweep-date)"

    # Lightweight sweep: shellcheck, go vet, known anti-patterns
    shellcheck hooks/*.sh 2>&1 | grep -v "^$" | while read line; do
      add_to_next_work("shellcheck finding: $line", severity="medium", type="bug")
    done

    go vet ./cli/... 2>&1 | grep -v "^$" | while read line; do
      add_to_next_work("go vet finding: $line", severity="medium", type="bug")
    done

    # grep for known anti-patterns (e.g., hardcoded secrets, TODO markers)
    grep -r "TODO|FIXME|XXX" --include="*.go" --include="*.sh" . 2>/dev/null | while read line; do
      add_to_next_work("code marker: $line", severity="low", type="tech-debt")
    done

    # Coverage floor single-pass: scan ALL packages at once
    # When discovering coverage floor gaps, process everything in one cycle.
    if [ -f GOALS.yaml ] && grep -q "coverage-floor" GOALS.yaml; then
      # Run coverage for ALL packages in a single pass
      go test -cover ./... 2>/dev/null | grep -E "^ok|^FAIL" | while read line; do
        pkg=$(echo "$line" | awk '{print $2}')
        cov=$(echo "$line" | grep -oP '\d+\.\d+%' | tr -d '%')
        if [ -n "$cov" ]; then
          # Check if package has a floor in GOALS.yaml
          # If not tracked, add it; if tracked with >3% headroom, tighten it
          add_to_next_work("Coverage floor check: $pkg at ${cov}%",
                           severity="medium", type="coverage-floor")
        fi
      done
      log "Coverage floor single-pass complete for all packages"
    fi

    # Mark sweep complete
    touch .agents/evolve/last-sweep-date
    log "Cycle-0 sweep complete. New findings added to next-work.jsonl"
  fi

  # Coverage floor processing guidance:
  # When the explore agent or sweep finds coverage floor headroom:
  # - Run coverage for ALL packages in a single pass
  # - Compare ALL floors to actual in one table
  # - Tighten ALL floors with >3% headroom in a single cycle
  # - Add ALL untracked packages in the same cycle
  # Do NOT split floor-tightening across multiple cycles.

  # All goals pass — check harvested work from prior /rpi cycles
  if [ -f .agents/rpi/next-work.jsonl ]; then
    # Detect current repo for filtering
    CURRENT_REPO=$(bd config --get prefix 2>/dev/null \
      || basename "$(git remote get-url origin 2>/dev/null)" .git 2>/dev/null \
      || basename "$(pwd)")

    all_items = read_unconsumed(next-work.jsonl)  # entries with consumed: false
    # Filter by target_repo: include items where target_repo matches
    # CURRENT_REPO, target_repo is "*" (cross-repo), or field is absent (backward compat).
    # Skip items whose target_repo names a different repo.
    items = [i for i in all_items
             if i.target_repo in (CURRENT_REPO, "*", None)]
    if items:
      evolve_state.idle_streak = 0  # reset — we found work
      selected_item = max(items, by=severity)  # highest severity first
      log "All goals met. Picking harvested work: {selected_item.title}"
      # Execute as an /rpi cycle (Step 4), then mark consumed
      /rpi "{selected_item.title}" --auto --max-cycles=1 --test-first   # if --test-first set
      /rpi "{selected_item.title}" --auto --max-cycles=1                 # otherwise
      mark_consumed(selected_item)  # set consumed: true, consumed_by, consumed_at
      # Skip Steps 4-5 (already executed above), go to Step 6 (log cycle)
      log_cycle(cycle, goal_id="next-work:{selected_item.title}", result="harvested")
      continue loop  # → Step 1 (kill switch check)

  # Nothing to do THIS cycle — but don't quit yet
  evolve_state.idle_streak += 1
  log "All goals met, no harvested work. Idle streak: {idle_streak}/{max_idle_streak}"

  if evolve_state.idle_streak >= evolve_state.max_idle_streak:
    log "Stagnation: {max_idle_streak} consecutive idle cycles. Nothing left to improve."
    STOP → go to Teardown

  # NOT stagnant yet — re-measure next cycle (external changes, new harvested work)
  log "Re-measuring next cycle in case conditions changed..."
  continue loop  # → Step 1 (kill switch check)

# Meta-goal guidance: after pruning any allowlist, add a meta-goal that
# prevents re-accumulation. The meta-goal should fail if allowlist entries
# have callers. Allowlists without meta-goals are technical debt magnets.
# See references/goals-schema.md for the meta-goal pattern.

# We have failing goals — reset idle streak
evolve_state.idle_streak = 0

# Sort by weight (highest priority first)
failing_goals.sort(by=weight, descending)

# Simple strike check: skip goals that failed the last 3 consecutive cycles
eligible_goals = []
for goal in failing_goals:
  recent = last_3_cycles_for(goal.id)
  if all(r.result == "regressed" for r in recent):
    log "Skipping {goal.id}: regressed 3 consecutive cycles. Needs human attention."
    continue
  eligible_goals.append(goal)

if not eligible_goals:
  log "All failing goals have regressed 3+ times. Human intervention needed."
  STOP → go to Teardown

if evolve_state.parallel and len(eligible_goals) > 1:
  # PARALLEL MODE: Select top N independent failing goals
  selected_goals = select_parallel_goals(eligible_goals, max=evolve_state.max_parallel)

  # select_parallel_goals logic:
  # 1. Start with highest-weight eligible goal (already weight-sorted)
  # 2. For each remaining goal:
  #    a. Check if its check command references the same scripts/paths
  #       as any already-selected goal (heuristic independence check)
  #    b. If independent: add to selection (up to max_parallel)
  #    c. If overlapping: skip (will be handled in next cycle)
  # 3. This is a heuristic — true conflicts are caught by the regression
  #    gate (Step 5), which reverts the entire parallel wave if any
  #    goal regresses another.

  log "Parallel mode: selected {len(selected_goals)} goals for this cycle"
  for g in selected_goals:
    log "  - [{g.weight}] {g.id}"

  # Proceed to Step 4 (parallel execution)
else:
  # SEQUENTIAL MODE: Select single worst goal (existing behavior)
  selected = eligible_goals[0]
```

### Step 4: Execute

**If `--dry-run`:** Report the selected goal (or harvested item) and stop.

```
log "Dry run: would work on '{selected.id}' (weight: {selected.weight})"
log "Description: {selected.description}"
log "Check command: {selected.check}"

# Also show queued harvested work (filtered to current repo)
if [ -f .agents/rpi/next-work.jsonl ]; then
  all_items = read_unconsumed(next-work.jsonl)
  items = [i for i in all_items
           if i.target_repo in (CURRENT_REPO, "*", None)]
  if items:
    log "Harvested work queue ({len(items)} items):"
    for item in items:
      log "  - [{item.severity}] {item.title} ({item.type})"

STOP → go to Teardown
```

**Otherwise:** Run improvement cycle(s) on the selected goal(s).

```
if evolve_state.parallel and len(selected_goals) > 1:
  # PARALLEL EXECUTION: Use /swarm to run multiple /rpi cycles concurrently
  log "Parallel execution: spawning /swarm for {len(selected_goals)} goals"

  # Artifact isolation: each /rpi worker gets its own artifact directory
  # to prevent collision on .agents/rpi/ state files
  for goal in selected_goals:
    mkdir -p .agents/evolve/parallel-rpi/{goal.id}

  # Results directory
  mkdir -p .agents/evolve/parallel-results

  # Create TaskList tasks for each goal
  for goal in selected_goals:
    TaskCreate(
      subject="Improve {goal.id}: {goal.description}",
      description="""
        You are improving a single fitness goal as part of a parallel /evolve cycle.

        Run a full /rpi cycle:
        /rpi "Improve {goal.id}: {goal.description}" --auto --max-cycles=1

        Goal check command: {goal.check}
        Current value: {goal.value}
        Threshold: {goal.threshold}

        IMPORTANT - Artifact isolation:
        Set RPI_ARTIFACT_DIR=.agents/evolve/parallel-rpi/{goal.id}
        Write all /rpi phase summaries and artifacts to that directory.

        Write result summary to .agents/evolve/parallel-results/{goal.id}.md
        Include: what changed, files modified, whether goal check now passes.
      """,
      activeForm="Improving {goal.id}"
    )

  # No dependencies between parallel goals (pre-filtered as independent)

  # Invoke /swarm — uses worktree isolation to prevent git conflicts
  # Each worker operates in its own worktree (/tmp/evolve-{goal.id})
  # /swarm handles team creation, worker spawning, validation, cleanup
  /swarm --worktrees

  # After /swarm completes, read results from each worker
  for goal in selected_goals:
    if [ -f .agents/evolve/parallel-results/{goal.id}.md ]; then
      result = read(".agents/evolve/parallel-results/{goal.id}.md")
      log "Goal {goal.id}: completed"
    else:
      log "Goal {goal.id}: no result file (worker may have failed)"

  # Proceed to Step 5 (regression gate applies to ALL parallel goals at once)

else:
  # SEQUENTIAL EXECUTION: Run a single /rpi cycle (existing behavior)
  /rpi "Improve {selected.id}: {selected.description}" --auto --max-cycles=1 --test-first   # if --test-first set
  /rpi "Improve {selected.id}: {selected.description}" --auto --max-cycles=1                 # otherwise
```

This internally runs the full lifecycle (per goal):
- `/research` — understand the problem
- `/plan` — decompose into issues
- `/pre-mortem` — validate the plan
- `/crank` — implement (spawns workers)
- `/vibe` — validate the code
- `/post-mortem` — extract learnings + `ao forge` (COMPOUNDING)

**Wait for all /rpi cycles to complete before proceeding to Step 5.**

In parallel mode, each worker runs a complete /rpi cycle independently. The regression gate (Step 5) runs once after ALL parallel goals complete — not per-goal. See `references/parallel-execution.md` for architecture details.

### Step 5: Full-Fitness Regression Gate

**CRITICAL: Re-run ALL goals, not just the target.**

After /rpi completes, re-run MEASURE_FITNESS on **every goal** (same as Step 2). Write result to `fitness-{CYCLE}-post.json`.

Compare the pre-cycle snapshot (`fitness-{CYCLE}.json`) against the post-cycle snapshot (`fitness-{CYCLE}-post.json`) for **ALL goals**:

```
# Load pre-cycle results
pre_results = load("fitness-{CYCLE}.json")

# Re-measure ALL goals (writes fitness-{CYCLE}-post.json)
post_results = MEASURE_FITNESS()

# PARALLEL MODE: handle multiple goals
if evolve_state.parallel and len(selected_goals) > 1:
  # Count how many target goals improved
  improved = [g for g in selected_goals if post_results.find(g.id).result == "pass" and pre_results.find(g.id).result == "fail"]
  unchanged = [g for g in selected_goals if post_results.find(g.id).result == pre_results.find(g.id).result]

  # FULL REGRESSION CHECK: compare ALL goals (same as sequential)
  newly_failing = []
  for goal in post_results.goals:
    pre = pre_results.find(goal.id)
    if pre.result == "pass" and goal.result == "fail":
      newly_failing.append(goal.id)

  if newly_failing:
    outcome = "regressed"
    log "REGRESSION in parallel wave: {newly_failing} started failing"
    log "Targeted goals: {[g.id for g in selected_goals]}"

    # Revert ALL commits since cycle start (entire parallel wave)
    cycle_start_sha = pre_results.cycle_start_sha
    commit_count = $(git rev-list --count ${cycle_start_sha}..HEAD)
    if commit_count == 0:
      log "No commits to revert"
    elif commit_count == 1:
      git revert HEAD --no-edit
    else:
      git revert --no-commit ${cycle_start_sha}..HEAD
      git commit -m "revert: evolve cycle ${CYCLE} parallel wave regression in {newly_failing}"
    log "Reverted entire parallel wave (${commit_count} commits). All {len(selected_goals)} goal improvements rolled back."
  else:
    outcome = "improved" if improved else "unchanged"
    log "Parallel wave: {len(improved)} improved, {len(unchanged)} unchanged"

else:
  # SEQUENTIAL MODE: existing single-goal logic
  if selected_goal.post_result == "pass":
    outcome = "improved"
  else:
    outcome = "unchanged"

  # FULL REGRESSION CHECK: compare ALL goals, not just the target
  newly_failing = []
  for goal in post_results.goals:
    pre = pre_results.find(goal.id)
    if pre.result == "pass" and goal.result == "fail":
      newly_failing.append(goal.id)

  if newly_failing:
    outcome = "regressed"
    log "REGRESSION: {newly_failing} started failing after fixing {selected.id}"

    # Multi-commit revert using cycle start SHA
    cycle_start_sha = pre_results.cycle_start_sha
    commit_count = $(git rev-list --count ${cycle_start_sha}..HEAD)
    if commit_count == 0:
      log "No commits to revert"
    elif commit_count == 1:
      git revert HEAD --no-edit
    else:
      git revert --no-commit ${cycle_start_sha}..HEAD
      git commit -m "revert: evolve cycle ${CYCLE} regression in {newly_failing}"
    log "Reverted ${commit_count} commits. Moving to next goal."
```

**Snapshot enforcement:** Validate `fitness-{CYCLE}-post.json` was written and is valid JSON before proceeding.

### Step 6: Log Cycle

Append to `.agents/evolve/cycle-history.jsonl`:

```jsonl
{"cycle": 1, "goal_id": "test-pass-rate", "result": "improved", "commit_sha": "abc1234", "goals_passing": 18, "goals_total": 23, "timestamp": "2026-02-11T21:00:00Z"}
{"cycle": 2, "goal_id": "doc-coverage", "result": "regressed", "commit_sha": "def5678", "reverted_to": "abc1234", "goals_passing": 17, "goals_total": 23, "timestamp": "2026-02-11T21:30:00Z"}
```

For parallel cycles, use `goal_ids` array instead of single `goal_id`:
```jsonl
{"cycle": 3, "goal_ids": ["test-pass-rate", "doc-coverage", "lint-clean"], "result": "improved", "commit_sha": "ghi9012", "goals_passing": 22, "goals_total": 23, "goals_added": 0, "parallel": true, "timestamp": "2026-02-11T22:00:00Z"}
```

**MANDATORY fields in every cycle log entry:**
- `goals_passing`: count of goals with result "pass"
- `goals_total`: total goals measured
- `goals_added`: count of new goals added this cycle (0 if none)

These enable fitness trajectory plotting across cycles.

**Telemetry logging (end of each cycle):**
```bash
bash scripts/log-telemetry.sh evolve cycle-complete cycle=${CYCLE} score=${SCORE} goals_passing=${PASSING} goals_total=${TOTAL}
```

**Compaction-proofing: commit after every cycle.**
Uncommitted state does not survive context compaction. ALWAYS commit cycle
artifacts before starting the next cycle:

```bash
git add .agents/evolve/cycle-history.jsonl .agents/evolve/fitness-*.json
if evolve_state.parallel and len(selected_goals) > 1:
  git commit -m "evolve: cycle ${CYCLE} — parallel wave [${goal_ids}] ${outcome}" --allow-empty
else:
  git commit -m "evolve: cycle ${CYCLE} — ${selected.id} ${outcome}" --allow-empty
```

### Step 7: Loop or Stop

```
evolve_state.cycle += 1

# Only stop for max-cycles if the user explicitly set one
if evolve_state.max_cycles != Infinity and evolve_state.cycle >= evolve_state.max_cycles:
  log "Max cycles ({max_cycles}) reached."
  STOP → go to Teardown

# Otherwise: loop back to Step 1 (kill switch check) — run forever
```

### Teardown

**Auto-run /post-mortem on the full evolution session:**

```
/post-mortem "evolve session: $CYCLE cycles, goals improved: X, harvested: Y"
```

This captures learnings from the ENTIRE evolution run (all cycles, all /rpi invocations) in one council review. The post-mortem harvests follow-up items into `next-work.jsonl`, feeding the next `/evolve` session.

**Compute session fitness trajectory:**

```bash
# Check if both baseline and final snapshot exist
if [ -f .agents/evolve/fitness-0-baseline.json ] && [ -f .agents/evolve/fitness-${CYCLE}.json ]; then
  baseline = load(".agents/evolve/fitness-0-baseline.json")
  final = load(".agents/evolve/fitness-${CYCLE}.json")

  # Compute delta — goals that flipped between baseline and final
  improved_count = 0
  regressed_count = 0
  unchanged_count = 0
  delta_rows = []

  for final_goal in final.goals:
    baseline_goal = baseline.goals.find(g => g.id == final_goal.id)
    baseline_result = baseline_goal ? baseline_goal.result : "unknown"
    final_result = final_goal.result

    if baseline_result == "fail" and final_result == "pass":
      delta = "improved"
      improved_count += 1
    elif baseline_result == "pass" and final_result == "fail":
      delta = "regressed"
      regressed_count += 1
    else:
      delta = "unchanged"
      unchanged_count += 1

    delta_rows.append({goal_id: final_goal.id, baseline_result, final_result, delta})

  # Write session-fitness-delta.md with trajectory table
  cat > .agents/evolve/session-fitness-delta.md << EOF
  # Session Fitness Trajectory

  | goal_id | baseline_result | final_result | delta |
  |---------|-----------------|--------------|-------|
  $(for row in delta_rows: "| ${row.goal_id} | ${row.baseline_result} | ${row.final_result} | ${row.delta} |")

  **Summary:** ${improved_count} improved, ${regressed_count} regressed, ${unchanged_count} unchanged
  EOF

  # Include delta summary in user-facing teardown report
  log "Fitness trajectory: ${improved_count} improved, ${regressed_count} regressed, ${unchanged_count} unchanged"
fi
```

**Then write session summary:**

```bash
cat > .agents/evolve/session-summary.md << EOF
# /evolve Session Summary

**Date:** $(date -Iseconds)
**Cycles:** $CYCLE of $MAX_CYCLES
**Goals measured:** $(wc -l < GOALS.yaml goals)

## Cycle History
$(cat .agents/evolve/cycle-history.jsonl)

## Final Fitness
$(cat .agents/evolve/fitness-${CYCLE}.json)

## Post-Mortem
<path to post-mortem report from above>

## Next Steps
- Run \`/evolve\` again to continue improving
- Run \`/evolve --dry-run\` to check current fitness without executing
- Create \`~/.config/evolve/KILL\` to prevent future runs
- Create \`.agents/evolve/STOP\` for a one-time local stop
EOF
```

Report to user:
```
## /evolve Complete

Cycles: N of M
Goals improved: X
Goals regressed: Y (reverted)
Goals unchanged: Z
Post-mortem: <verdict> (see <report-path>)

Run `/evolve` again to continue improving.
```

---

Read `references/compounding.md` for details on how the knowledge flywheel and work harvesting compound across cycles.

---

## Kill Switch

Two paths, checked at every cycle boundary:

| File | Purpose | Who Creates It |
|------|---------|---------------|
| `~/.config/evolve/KILL` | Permanent stop (outside repo) | Human |
| `.agents/evolve/STOP` | One-time local stop | Human or automation |

To stop /evolve:
```bash
echo "Taking a break" > ~/.config/evolve/KILL    # Permanent
echo "done for today" > .agents/evolve/STOP       # Local, one-time
```

To re-enable:
```bash
rm ~/.config/evolve/KILL
rm .agents/evolve/STOP
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Optional hard cap. Without this, loop runs forever. |
| `--test-first` | off | Pass `--test-first` through to `/rpi` → `/crank` |
| `--dry-run` | off | Measure fitness and show plan, don't execute |
| `--skip-baseline` | off | Skip cycle-0 baseline sweep |
| `--parallel` | off | Enable parallel goal execution via /swarm per cycle |
| `--max-parallel=N` | 3 | Max goals to fix in parallel (cap: 5). Only with `--parallel`. |

---

Read `references/goals-schema.md` for the GOALS.yaml format.

---

## Artifacts

See `references/artifacts.md` for the full list of generated files and their purposes.

---

## Examples

**User says:** "Run evolve until goals are green."

**Do:**
```bash
/evolve
```

**User says:** "Run evolve in dry-run mode for two cycles."

**Do:**
```bash
/evolve --dry-run --max-cycles=2
```

See `references/examples.md` for detailed examples including infinite improvement, dry-run mode, and regression with revert.

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `/evolve` exits immediately with "KILL SWITCH ACTIVE" | Kill switch file exists | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` to re-enable |
| "No goals to measure" error | GOALS.yaml missing or empty | Create GOALS.yaml in repo root with fitness goals (see references/goals-schema.md) |
| Cycle completes but fitness unchanged | Goal check command is always passing or always failing | Verify check command logic in GOALS.yaml produces exit code 0 (pass) or non-zero (fail) |
| Regression revert fails | Multiple commits in cycle or uncommitted changes | Check cycle-start SHA in fitness snapshot, commit or stash changes before retrying |
| Harvested work never consumed | All goals passing but `next-work.jsonl` not read | Check file exists and has `consumed: false` entries. Agent picks harvested work after goals met. |
| Loop stops after N cycles | `--max-cycles` was set (or old default of 10) | Omit `--max-cycles` flag — default is now unlimited. Loop runs until kill switch or 3 idle cycles. |

---

## See Also

- `skills/rpi/SKILL.md` — Full lifecycle orchestrator (called per cycle)
- `skills/vibe/SKILL.md` — Code validation (called by /rpi)
- `skills/council/SKILL.md` — Multi-model judgment (called by /rpi)
- `GOALS.yaml` — Fitness goals for this repo
