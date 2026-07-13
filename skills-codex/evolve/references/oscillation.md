# Oscillation Detection

## What Is Oscillation?

A goal **oscillates** when it alternates between passing and failing across
evolve cycles. Typically this means the fix for the goal causes a side-effect
that breaks something else, which gets reverted, which re-exposes the original
failure — creating a loop.

Example from cycle-history.jsonl:
```
cycle 5:  goal-X → improved
cycle 8:  goal-X → fail
cycle 12: goal-X → improved
cycle 15: goal-X → fail
```

## Detection

Count **improved→fail transitions** for the same `target` in
`.agents/evolve/cycle-history.jsonl`:

```bash
# Count oscillations for a given goal — reads via BC3 LoopReaderPort
# (soc-y5vh.4 wrapper) instead of inline JSONL slurp.
scripts/evolve-read-cycle-history.sh recent 0 \
  | jq -r "select(.target==\"$GOAL_ID\") | .result" \
  | awk 'prev=="improved" && $0=="fail" {count++} {prev=$0} END {print count+0}'
```

## Threshold

**3 oscillations** (improved→fail transitions) within a single session
triggers quarantine. The goal is skipped in Step 3 selection.

## Effect

- Quarantined goals are skipped during work selection (Step 3)
- Skipping a quarantined goal counts as idle (no actionable work found)
- The quarantine is session-scoped — a new session resets the count
- Quarantine events are logged in cycle-history.jsonl with `"result": "quarantined"`

## Recovery

1. Human identifies the root cause (usually a conflict between two goals)
2. Fix the underlying issue manually
3. Start a new evolve session (quarantine resets)
4. Or: remove the quarantine by deleting the goal from the skip list

## Why Not Just Increase the Skip Threshold?

The 3-consecutive-regression skip in Step 3 only catches monotonic failure.
Oscillation is worse — it burns cycles alternating between "fixed" and "broken"
without the stagnation detector ever triggering (because the goal intermittently
passes). The oscillation detector catches this pattern explicitly.
