# Teardown Procedure

**Auto-run $post-mortem on the full evolution session:**

```
$post-mortem "evolve session: $CYCLE cycles, goals improved: X, harvested: Y"
```

This captures learnings from the ENTIRE evolution run (all cycles, all $rpi invocations) in one council review. The post-mortem harvests follow-up items into `next-work.jsonl`, feeding the next `$evolve` session.

**Compute session fitness trajectory:**

```bash
# Check if both current-era baseline and final snapshot exist
GOALS_FILE=""
if [ -f GOALS.md ]; then
  GOALS_FILE="GOALS.md"
elif [ -f GOALS.yaml ]; then
  GOALS_FILE="GOALS.yaml"
fi

ACTIVE_BASELINE_PATH=""
if [ -n "$GOALS_FILE" ]; then
  ERA_ID="goals-$(shasum -a 256 "$GOALS_FILE" | awk '{print substr($1, 1, 12)}')"
  ACTIVE_BASELINE_PATH="$(ls -t ".agents/evolve/fitness-baselines/$ERA_ID"/*.json 2>/dev/null | head -1 || true)"
fi

if [ -n "$ACTIVE_BASELINE_PATH" ] && [ -f "$ACTIVE_BASELINE_PATH" ] && [ -f .agents/evolve/fitness-latest.json ]; then
  baseline = load("$ACTIVE_BASELINE_PATH")
  final = load(".agents/evolve/fitness-latest.json")

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
# $evolve Session Summary

**Date:** $(date -Iseconds)
**Cycles:** $CYCLE of $MAX_CYCLES
**Goals measured:** $(wc -l < GOALS.yaml goals)

## Cycle History
$(cat .agents/evolve/cycle-history.jsonl)

## Final Fitness
$(cat .agents/evolve/fitness-latest.json)

## Post-Mortem
<path to post-mortem report from above>

## Next Steps
- Run \`$evolve\` again to continue improving
- Run \`$evolve --dry-run\` to check current fitness without executing
- Create \`~/.config/evolve/KILL\` to prevent future runs
- Create \`.agents/evolve/STOP\` for a one-time local stop
EOF
```

Report to user:
```
## $evolve Complete

Cycles: N of M
Goals improved: X
Goals regressed: Y (reverted)
Goals unchanged: Z
Post-mortem: <verdict> (see <report-path>)

Run `$evolve` again to continue improving.
```

## Release-shaped teardown

Release-shaped branches (`release/*`, `v*-prep`, `v*-evolve-run`, `v\d+\.\d+*`) must not recommend `$release` from a per-cycle `--fast` smoke test. The operator runs the full gate and confirms green before tagging.

```text
## Pre-release checklist — REQUIRED before $release

[ ] 1. Regenerate derived surfaces if any Cobra command or flag changed:
       bash scripts/regen-all.sh          # regenerates COMMANDS.md, registry.json, and maps
       # Adding an `ao` command also requires both cobra_commands_test.go
       # expectedCmds lists and the CLI-command-surface count updates documented in
       # references/ao-command-landing.md.
       git diff cli/docs/COMMANDS.md registry.json   # commit if non-empty
[ ] 2. Full release gate:
       ao gate check --full --workflow-coverage --require-workflow-parity
[ ] 3. Smoke $evolve --dry-run --max-cycles=1 if BC port wire-ups changed.

Only after [1]–[2] pass: $release <version>
```

The handoff artifact must reproduce this full checklist verbatim and unchecked. “Ready to tag” means the boxes are checked, not merely that an evolve cycle ran cleanly.
