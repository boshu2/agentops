# Cycle History Format and Recovery Protocol

## Compaction Resilience

The evolve loop MUST survive context compaction. Every cycle commits its
artifacts to git before proceeding. The `cycle-history.jsonl` file is the
recovery point -- on session restart, read it to determine cycle number
and resume from Step 1.

## Cycle History JSONL Format

Append one line per cycle to `.agents/evolve/cycle-history.jsonl`.

**Sequential cycle entry:**
```jsonl
{"cycle": 1, "goal_id": "test-pass-rate", "result": "improved", "commit_sha": "abc1234", "goals_passing": 18, "goals_total": 23, "timestamp": "2026-02-11T21:00:00Z"}
{"cycle": 2, "goal_id": "doc-coverage", "result": "regressed", "commit_sha": "def5678", "reverted_to": "abc1234", "goals_passing": 17, "goals_total": 23, "timestamp": "2026-02-11T21:30:00Z"}
```

**Parallel cycle entry** (use `goal_ids` array instead of single `goal_id`):
```jsonl
{"cycle": 3, "goal_ids": ["test-pass-rate", "doc-coverage", "lint-clean"], "result": "improved", "commit_sha": "ghi9012", "goals_passing": 22, "goals_total": 23, "goals_added": 0, "parallel": true, "timestamp": "2026-02-11T22:00:00Z"}
```

### Mandatory Fields

Every cycle log entry MUST include:

| Field | Description |
|-------|-------------|
| `cycle` | Cycle number (0-indexed) |
| `goal_id` or `goal_ids` | Target goal(s) for this cycle |
| `result` | One of: `improved`, `regressed`, `unchanged`, `harvested` |
| `commit_sha` | Git SHA after cycle commit |
| `goals_passing` | Count of goals with result "pass" |
| `goals_total` | Total goals measured |
| `goals_added` | Count of new goals added this cycle (0 if none) |
| `timestamp` | ISO 8601 timestamp |

These enable fitness trajectory plotting across cycles.

### Telemetry

Log telemetry at the end of each cycle:
```bash
bash scripts/log-telemetry.sh evolve cycle-complete cycle=${CYCLE} score=${SCORE} goals_passing=${PASSING} goals_total=${TOTAL}
```

### Compaction-Proofing: Commit After Every Cycle

Uncommitted state does not survive context compaction. ALWAYS commit cycle
artifacts before starting the next cycle:

```bash
git add .agents/evolve/cycle-history.jsonl .agents/evolve/fitness-*.json
if evolve_state.parallel and len(selected_goals) > 1:
  git commit -m "evolve: cycle ${CYCLE} -- parallel wave [${goal_ids}] ${outcome}" --allow-empty
else:
  git commit -m "evolve: cycle ${CYCLE} -- ${selected.id} ${outcome}" --allow-empty
```

## Recovery Protocol

On session restart or after compaction:

1. Read `.agents/evolve/cycle-history.jsonl` to find last completed cycle number
2. Set `evolve_state.cycle` to last cycle + 1
3. Resume from Step 1 (kill switch check)
4. The baseline snapshot (`fitness-0-baseline.json`) is preserved -- do not regenerate

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

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--max-cycles=N` | unlimited | Optional hard cap. Without this, loop runs forever. |
| `--test-first` | off | Pass `--test-first` through to `/rpi` -> `/crank` |
| `--dry-run` | off | Measure fitness and show plan, don't execute |
| `--skip-baseline` | off | Skip cycle-0 baseline sweep |
| `--parallel` | off | Enable parallel goal execution via /swarm per cycle |
| `--max-parallel=N` | 3 | Max goals to fix in parallel (cap: 5). Only with `--parallel`. |

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `/evolve` exits immediately with "KILL SWITCH ACTIVE" | Kill switch file exists | Remove `~/.config/evolve/KILL` or `.agents/evolve/STOP` to re-enable |
| "No goals to measure" error | GOALS.yaml missing or empty | Create GOALS.yaml in repo root with fitness goals (see goals-schema.md) |
| Cycle completes but fitness unchanged | Goal check command is always passing or always failing | Verify check command logic in GOALS.yaml produces exit code 0 (pass) or non-zero (fail) |
| Regression revert fails | Multiple commits in cycle or uncommitted changes | Check cycle-start SHA in fitness snapshot, commit or stash changes before retrying |
| Harvested work never consumed | All goals passing but `next-work.jsonl` not read | Check file exists and has `consumed: false` entries. Agent picks harvested work after goals met. |
| Loop stops after N cycles | `--max-cycles` was set (or old default of 10) | Omit `--max-cycles` flag -- default is now unlimited. Loop runs until kill switch or 3 idle cycles. |
