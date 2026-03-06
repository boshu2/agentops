---
name: evolve
description: 'Goal-driven fitness-scored improvement loop. Measures goals, picks worst gap, runs $rpi, compounds via knowledge flywheel. Also pulls from open beads when goals all pass.'
---


# $evolve — Goal-Driven Compounding Loop

> Measure what's wrong. Fix the worst thing. Measure again. Compound.

Thin fitness-scored loop over `$rpi`. Three work sources in priority order:
1. **Failing GOALS.yaml goals** (highest-weight first)
2. **Open beads issues** (`bd ready` — when all goals pass)
3. **Harvested next-work.jsonl** (from prior $rpi post-mortems)

**Dormancy is success.** When all sources are empty, stop. Don't manufacture work.

```bash
$evolve                      # Run until kill switch or stagnation
$evolve --max-cycles=5       # Cap at 5 cycles
$evolve --dry-run            # Show what would be worked on, don't execute
$evolve --beads-only         # Skip goals measurement, work beads backlog only
$evolve --quality            # Quality-first mode: prioritize post-mortem findings
$evolve --quality --max-cycles=10  # Quality mode with cycle cap
$evolve --athena             # Mine → Defrag warmup before first cycle
$evolve --athena --max-cycles=5  # Warm knowledge base then run 5 cycles
$evolve --test-first         # Default strict-quality $rpi execution path
$evolve --no-test-first      # Explicit opt-out from test-first mode
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Stop after `N` completed cycles |
| `--dry-run` | off | Show planned cycle actions without executing |
| `--beads-only` | off | Skip goal measurement and run backlog-only selection |
| `--skip-baseline` | off | Skip first-run baseline snapshot |
| `--quality` | off | Prioritize harvested post-mortem findings |
| `--athena` | off | Run `ao mine` + `ao defrag` warmup before cycle 1 |
| `--test-first` | on | Pass strict-quality defaults through to `$rpi` |
| `--no-test-first` | off | Explicitly disable test-first passthrough to `$rpi` |

## Execution Steps

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.**

### Step 0: Setup

```bash
mkdir -p .agents/evolve
ao know inject 2>/dev/null || true
```

Recover cycle number and idle streak from disk (survives context compaction):
```bash
if [ -f .agents/evolve/cycle-history.jsonl ]; then
  CYCLE=$(( $(tail -1 .agents/evolve/cycle-history.jsonl | jq -r '.cycle // 0') + 1 ))
else
  CYCLE=1
fi
SESSION_START_SHA=$(git rev-parse HEAD)

# Recover idle streak from disk (not in-memory — survives compaction)
# Portable: forward-scanning awk counts trailing idle run without tac (unavailable on stock macOS)
IDLE_STREAK=$(awk '/"result"\s*:\s*"(idle|unchanged)"/{streak++; next} {streak=0} END{print streak+0}' \
  .agents/evolve/cycle-history.jsonl 2>/dev/null)

PRODUCTIVE_THIS_SESSION=0

# Circuit breaker: stop if last productive cycle was >60 minutes ago
LAST_PRODUCTIVE_TS=$(grep -v '"idle"\|"unchanged"' .agents/evolve/cycle-history.jsonl 2>/dev/null \
  | tail -1 | jq -r '.timestamp // empty')
if [ -n "$LAST_PRODUCTIVE_TS" ]; then
  NOW_EPOCH=$(date +%s)
  LAST_EPOCH=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "$LAST_PRODUCTIVE_TS" +%s 2>/dev/null \
    || date -d "$LAST_PRODUCTIVE_TS" +%s 2>/dev/null || echo 0)
  if [ "$LAST_EPOCH" -gt 1000000000 ] && [ $((NOW_EPOCH - LAST_EPOCH)) -ge 3600 ]; then
    echo "CIRCUIT BREAKER: No productive work in 60+ minutes. Stopping."
    # go to Teardown
  fi
fi

# Track oscillating goals (improved→fail→improved→fail) to avoid burning cycles
declare -A QUARANTINED_GOALS  # goal_id → true if oscillation count >= 3

# Pre-populate quarantine list from cycle history (lightweight local scan)
if [ -f .agents/evolve/cycle-history.jsonl ]; then
  while IFS= read -r goal; do
    QUARANTINED_GOALS[$goal]=true
    echo "Quarantined oscillating goal: $goal"
  done < <(
    jq -r '.target' .agents/evolve/cycle-history.jsonl 2>/dev/null \
    | awk '{
        if (prev != "" && prev != $0) transitions[$0]++
        prev = $0
      }
      END {
        for (g in transitions) if (transitions[g] >= 3) print g
      }'
  )
fi
```

Parse flags: `--max-cycles=N` (default unlimited), `--dry-run`, `--beads-only`, `--skip-baseline`, `--quality`, `--athena`.

Track cycle-level execution state:

```text
evolve_state = {
  cycle: <current cycle number>,
  mode: <standard|quality|beads-only>,
  test_first: <true by default; false only when --no-test-first>
}
```

### Step 0.2: Athena Warmup (--athena only)

Skip if `--athena` was not passed or if `--dry-run`.

Run the mechanical half of the Athena cycle to surface fresh signal before the first evolve cycle:

```bash
mkdir -p .agents/mine .agents/defrag
echo "Athena warmup: mining signal..."
ao mine --since 26h --quiet 2>/dev/null || echo "(ao mine unavailable — skipping)"

echo "Athena warmup: defrag sweep..."
ao defrag --prune --dedup --quiet 2>/dev/null || echo "(ao defrag unavailable — skipping)"
```

Then read `.agents/mine/latest.json` and `.agents/defrag/latest.json` and note (in 1-2 sentences each):
- Any **orphaned research** files that look relevant to current goals
- Any **code hotspots** (high-CC functions with recent edits) that may be the root cause of failing goals
- Any **duplicate learnings** merged by defrag — context on what's been cleaned up

These notes inform work selection throughout the evolve session. Store them in a session variable (in-memory), not a file.

### Step 0.5: Baseline (first run only)

Skip if `--skip-baseline` or `--beads-only` or baseline already exists.

```bash
if [ ! -f .agents/evolve/fitness-0-baseline.json ]; then
  ao work goals measure --json --timeout 60 > .agents/evolve/fitness-0-baseline.json
fi
```

### Step 1: Kill Switch Check

Run at the TOP of every cycle:

```bash
CYCLE_START_SHA=$(git rev-parse HEAD)
[ -f ~/.config/evolve/KILL ] && echo "KILL: $(cat ~/.config/evolve/KILL)" && exit 0
[ -f .agents/evolve/STOP ] && echo "STOP: $(cat .agents/evolve/STOP 2>/dev/null)" && exit 0
```

### Step 2: Measure Fitness

Skip if `--beads-only`.

```bash
ao work goals measure --json --timeout 60 > .agents/evolve/fitness-latest.json
```

**Do NOT write per-cycle `fitness-{N}-pre.json` files.** The rolling file is sufficient for work selection and regression detection.

This writes a fitness snapshot to `.agents/evolve/`. If `ao work goals measure` is unavailable or fails, stop with error. The CLI is required for fitness measurement.

### Step 3: Select Work

**Step 3.0: Emergency gates** (skip if `--beads-only`):
```bash
# Check for failing gates with weight >= 5
FAILING=$(jq -r '.goals[] | select(.result=="fail" and .weight>=5) | .id' .agents/evolve/fitness-latest.json | head -1)
```
Pick highest-weight failing goal. Skip quarantined goals (oscillation ≥ 3).

**Oscillation check:** Before working a failing goal, check if it has oscillated (improved→fail transitions ≥ 3 times in cycle-history.jsonl). If so, quarantine it and try the next failing goal. See `references/oscillation.md`.
```bash
# Count improved→fail transitions for this goal
OSC_COUNT=$(jq -r "select(.target==\"$FAILING\") | .result" .agents/evolve/cycle-history.jsonl \
  | awk 'prev=="improved" && $0=="fail" {count++} {prev=$0} END {print count+0}')
if [ "$OSC_COUNT" -ge 3 ]; then
  QUARANTINED_GOALS[$FAILING]=true
  echo "{\"cycle\":${CYCLE},\"target\":\"${FAILING}\",\"result\":\"quarantined\",\"oscillations\":${OSC_COUNT},\"timestamp\":\"$(date -Iseconds)\"}" >> .agents/evolve/cycle-history.jsonl
fi
```

**Step 3.1: Directive gap** (skip if `--beads-only`):
```bash
# Get directives from GOALS.md
DIRECTIVES=$(ao work goals measure --directives 2>/dev/null)
```
If directives exist, assess the top-priority directive (lowest number, non-quarantined):
- Check git log for recent commits addressing it
- If gap detected → generate `suggested_work` description from the directive
- Use this as the work item for Step 4

**Step 3.2: Harvested work** from `.agents/rpi/next-work.jsonl` (unconsumed entries).

**Step 3.3: Open beads**:
```bash
READY_ISSUE=$(bd ready -n 1 2>/dev/null | head -1 | awk '{print $2}')
```
Pick highest-priority unblocked issue. If `bd ready` fails or returns empty, fall through. Do not treat bd failure as idle.

**Step 3.4: Next directive** — assess priority 2, 3, etc. directives for gaps.

**Step 3.5: Lower-weight failing gates** — failing goals with weight < 5.

**Quality mode (`--quality`)** — inverted cascade (findings before directives):

Step 3.0q: Unconsumed high-severity post-mortem findings:
```bash
HIGH=$(jq -r 'select(.consumed==false) | .items[] | select(.severity=="high") | .title' \
  .agents/rpi/next-work.jsonl 2>/dev/null | head -1)
```

Step 3.1q: Unconsumed medium-severity findings.

Step 3.2q: Emergency gates (weight >= 5).

Step 3.3q: Top directive gap.

Step 3.4q: Open beads (`bd ready`).

Step 3.5q: Next directive.

This inverts the standard cascade: findings BEFORE goals and directives.
Rationale: harvested work has 100% first-attempt success rate.

When evolve picks a finding, mark it consumed in next-work.jsonl:
- Set `consumed: true`, `consumed_by: "evolve-quality:cycle-N"`, `consumed_at: "<timestamp>"`
- If the $rpi cycle fails (regression), un-mark the finding (set consumed back to false).

See `references/quality-mode.md` for scoring and full details.

**Nothing found?** HARD GATE — re-derive idle streak from disk:

```bash
# Count trailing idle/unchanged entries in cycle-history.jsonl (portable, no tac)
IDLE_STREAK=$(awk '/"result"\s*:\s*"(idle|unchanged)"/{streak++; next} {streak=0} END{print streak+0}' \
  .agents/evolve/cycle-history.jsonl 2>/dev/null)

if [ "$IDLE_STREAK" -ge 2 ]; then
  # This would be the 3rd consecutive idle cycle — STOP
  echo "Stagnation reached (3 idle cycles). Dormancy is success."
  # go to Teardown — do NOT log another idle entry
fi
```

If IDLE_STREAK < 2: this is idle cycle 1 or 2. Go to Step 6 (idle path).

A cycle is idle if NO work source returned actionable work. A cycle that targeted an oscillating goal and skipped it counts as idle.

If `--dry-run`: report what would be worked on and go to Teardown.

### Step 4: Execute

For a **failing goal**:
```
Invoke $rpi "Improve {goal_id}: {description}" --auto --max-cycles=1
```

For a **beads issue**:
```
Invoke $implement {issue_id}
```
Or for an epic with children: `Invoke $crank {epic_id}`.

For **harvested work**:
```
Invoke $rpi "{item_title}" --auto --max-cycles=1
```
Then mark the item consumed in next-work.jsonl.

### Step 5: Regression Gate

After execution, verify nothing broke:

```bash
# Detect and run project build+test
if [ -f Makefile ]; then make test
elif [ -f package.json ]; then npm test
elif [ -f go.mod ]; then go build ./... && go vet ./... && go test ./... -count=1 -timeout 120s
elif [ -f Cargo.toml ]; then cargo build && cargo test
elif [ -f pyproject.toml ] || [ -f setup.py ]; then python -m pytest
else echo "No recognized build system found"; fi

# Cross-cutting constraint check (catches wiring regressions)
if [ -f scripts/check-wiring-closure.sh ]; then
  bash scripts/check-wiring-closure.sh
else
  echo "WARNING: scripts/check-wiring-closure.sh not found — skipping wiring check"
fi
```

If not `--beads-only`, also re-measure to produce a post-cycle snapshot:
```bash
ao work goals measure --json --timeout 60 --goal $GOAL_ID > .agents/evolve/fitness-latest-post.json

# Extract goal counts for cycle history entry
PASSING=$(jq '[.goals[] | select(.result=="pass")] | length' .agents/evolve/fitness-latest-post.json 2>/dev/null || echo 0)
TOTAL=$(jq '.goals | length' .agents/evolve/fitness-latest-post.json 2>/dev/null || echo 0)
```

**If regression detected** (previously-passing goal now fails):
```bash
git revert HEAD --no-edit  # single commit
# or for multiple commits:
git revert --no-commit ${CYCLE_START_SHA}..HEAD && git commit -m "revert: evolve cycle ${CYCLE} regression"
```
Set outcome to "regressed".

### Step 6: Log Cycle + Commit

Two paths: productive cycles get committed, idle cycles are local-only.

**PRODUCTIVE cycles** (result is improved, regressed, or harvested):

```bash
# Quality mode: compute quality_score BEFORE writing the JSONL entry
QUALITY_SCORE_FIELD=""
if [ "$QUALITY_MODE" = "true" ]; then
  REMAINING_HIGH=$(jq -r 'select(.consumed==false) | .items[] | select(.severity=="high")' \
    .agents/rpi/next-work.jsonl 2>/dev/null | wc -l | tr -d ' ')
  REMAINING_MEDIUM=$(jq -r 'select(.consumed==false) | .items[] | select(.severity=="medium")' \
    .agents/rpi/next-work.jsonl 2>/dev/null | wc -l | tr -d ' ')
  QUALITY_SCORE=$((100 - (REMAINING_HIGH * 10) - (REMAINING_MEDIUM * 3)))
  [ "$QUALITY_SCORE" -lt 0 ] && QUALITY_SCORE=0
  QUALITY_SCORE_FIELD=",\"quality_score\":${QUALITY_SCORE}"
fi

# Append to cycle history (atomic write)
# Note: flock is Linux-native. On macOS use plain >> append if single-process.
ENTRY="{\"cycle\":${CYCLE},\"target\":\"${TARGET}\",\"result\":\"${OUTCOME}\",\"sha\":\"$(git rev-parse --short HEAD)\",\"timestamp\":\"$(date -Iseconds)\",\"goals_passing\":${PASSING},\"goals_total\":${TOTAL}${QUALITY_SCORE_FIELD}}"
flock .agents/evolve/cycle-history.jsonl -c "echo '${ENTRY}' >> .agents/evolve/cycle-history.jsonl" 2>/dev/null \
  || echo "${ENTRY}" >> .agents/evolve/cycle-history.jsonl

# Verify write
LAST=$(tail -1 .agents/evolve/cycle-history.jsonl | jq -r '.cycle')
[ "$LAST" != "$CYCLE" ] && echo "FATAL: cycle log write failed" && exit 1

# Telemetry
bash scripts/log-telemetry.sh evolve cycle-complete cycle=${CYCLE} goal=${TARGET} outcome=${OUTCOME} 2>/dev/null || true

# Check if this cycle changed real code (not just artifacts)
# Note: Using ${CYCLE_START_SHA} instead of HEAD~1 safely handles sub-skill multi-commit cases
REAL_CHANGES=$(git diff --name-only ${CYCLE_START_SHA}..HEAD -- ':!.agents/*' ':!GOALS.yaml' 2>/dev/null | wc -l | tr -d ' ')

if [ "$REAL_CHANGES" -gt 0 ]; then
  # Full commit: real code was changed
  git add .agents/evolve/cycle-history.jsonl
  git commit -m "evolve: cycle ${CYCLE} -- ${TARGET} ${OUTCOME}"
else
  # Artifact-only cycle: stage JSONL but don't create a standalone commit
  # The $rpi or $implement sub-skill already committed its own artifact changes
  git add .agents/evolve/cycle-history.jsonl
  # Do NOT create a standalone commit for artifact-only work
fi

PRODUCTIVE_THIS_SESSION=$((PRODUCTIVE_THIS_SESSION + 1))
```

**IDLE cycles** (nothing found):

```bash
# Append locally — NOT committed (disposable if compaction occurs)
flock .agents/evolve/cycle-history.jsonl -c \
  "echo '{\"cycle\":${CYCLE},\"target\":\"idle\",\"result\":\"unchanged\",\"timestamp\":\"$(date -Iseconds)\"}' >> .agents/evolve/cycle-history.jsonl" \
  2>/dev/null \
  || echo "{\"cycle\":${CYCLE},\"target\":\"idle\",\"result\":\"unchanged\",\"timestamp\":\"$(date -Iseconds)\"}" >> .agents/evolve/cycle-history.jsonl
# No git add, no git commit, no fitness snapshot write
```

### Step 7: Loop or Stop

```bash
CYCLE=$((CYCLE + 1))
# Stop if max-cycles reached
# Otherwise: go to Step 1
```

Push only when productive work has accumulated:
```bash
if [ $((PRODUCTIVE_THIS_SESSION % 5)) -eq 0 ] && [ "$PRODUCTIVE_THIS_SESSION" -gt 0 ]; then
  git push
fi
```

### Teardown

1. Commit any staged but uncommitted cycle-history.jsonl (from artifact-only cycles):
```bash
if git diff --cached --name-only | grep -q cycle-history.jsonl; then
  git commit -m "evolve: session teardown -- artifact-only cycles logged"
fi
```
2. Run `$post-mortem "evolve session: ${CYCLE} cycles"` to harvest learnings.
3. Push only if unpushed commits exist:
```bash
UNPUSHED=$(git log origin/main..HEAD --oneline 2>/dev/null | wc -l)
[ "$UNPUSHED" -gt 0 ] && git push
```
4. Report summary:

```
## $evolve Complete
Cycles: N | Productive: X | Regressed: Y (reverted) | Idle: Z
Stop reason: stagnation | circuit-breaker | max-cycles | kill-switch
```

In quality mode, the report includes additional fields:
```
## $evolve Complete (quality mode)
Cycles: N | Findings resolved: X | Goals fixed: Y | Idle: Z
Quality score: start → end (delta)
Remaining unconsumed: H high, M medium
Stop reason: stagnation | circuit-breaker | max-cycles | kill-switch
```

## Examples

**Basic:** `$evolve --max-cycles=5` — measures goals, fixes highest-weight failure, gates, repeats for 5 cycles.

**Beads only:** `$evolve --beads-only` — skips goals measurement, works through `bd ready` backlog.

**Dry run:** `$evolve --dry-run` — shows what would be worked on without executing.

**Athena warmup:** `$evolve --athena` — runs `ao mine` + `ao defrag` at session start to surface fresh signal (orphaned research, code hotspots, oscillating goals) before the first evolve cycle. Use before a long autonomous run or after a burst of development activity.

See `references/examples.md` for detailed walkthroughs.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Loop exits immediately | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` |
| Stagnation after 3 idle cycles | All work sources empty — this is success |
| `ao work goals measure` hangs | Use `--timeout 30` flag or `--beads-only` to skip |
| Regression gate reverts | Review reverted changes, narrow scope, re-run |

See `references/cycle-history.md` for advanced troubleshooting.

## References

- `references/cycle-history.md` — JSONL format, recovery protocol, kill switch
- `references/compounding.md` — Knowledge flywheel and work harvesting
- `references/goals-schema.md` — GOALS.yaml format and continuous metrics
- `references/parallel-execution.md` — Parallel $swarm architecture
- `references/teardown.md` — Trajectory computation and session summary
- `references/examples.md` — Detailed usage examples
- `references/artifacts.md` — Generated files registry
- `references/oscillation.md` — Oscillation detection and quarantine
- `references/quality-mode.md` — Quality-first mode: scoring, priority cascade, artifacts

## See Also

- `skills/rpi/SKILL.md` — Full lifecycle orchestrator (called per cycle)
- `skills/crank/SKILL.md` — Epic execution (called for beads epics)
- `GOALS.yaml` — Fitness goals for this repo

## Reference Documents

- [references/artifacts.md](references/artifacts.md)
- [references/compounding.md](references/compounding.md)
- [references/cycle-history.md](references/cycle-history.md)
- [references/examples.md](references/examples.md)
- [references/goals-schema.md](references/goals-schema.md)
- [references/oscillation.md](references/oscillation.md)
- [references/parallel-execution.md](references/parallel-execution.md)
- [references/quality-mode.md](references/quality-mode.md)
- [references/teardown.md](references/teardown.md)

## Local Resources

### references/

- [references/artifacts.md](references/artifacts.md)
- [references/compounding.md](references/compounding.md)
- [references/cycle-history.md](references/cycle-history.md)
- [references/examples.md](references/examples.md)
- [references/goals-schema.md](references/goals-schema.md)
- [references/oscillation.md](references/oscillation.md)
- [references/parallel-execution.md](references/parallel-execution.md)
- [references/quality-mode.md](references/quality-mode.md)
- [references/teardown.md](references/teardown.md)

### scripts/

- `scripts/validate.sh`


