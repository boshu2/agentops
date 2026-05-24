# Tool-Call-Based Compaction Signals

> Suggest context compaction at strategic points based on tool call count, not time or token usage.

## Problem

Auto-compaction happens at arbitrary points (95% context fill), often mid-task. This destroys working memory at the worst possible time. Time-based compaction doesn't correlate with context complexity.

## Solution: Tool-Call Counter with Strategic Signals

Track tool calls per session. Signal compaction at logical breakpoints.

### Signal Schedule

| Tool Calls | Signal | Message |
|-----------|--------|---------|
| 50 (threshold) | First signal | "50 tool calls reached — consider /compact if transitioning phases" |
| 75 | Recurring | "75 tool calls — good checkpoint for /compact" |
| 100 | Recurring | "100 tool calls — strongly recommend /compact before next major task" |
| Every 25 after threshold | Recurring | Repeating signal |

### Why 50?

- Typical exploration phase: 15-25 tool calls (reads, greps, globs)
- Typical implementation phase: 20-40 tool calls (reads, edits, writes, bash)
- Transition point (exploration → implementation): ~40-60 tool calls
- Compacting at the phase boundary preserves implementation context

### Implementation (Hook Pattern)

```bash
#!/usr/bin/env bash
set -euo pipefail

[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0

# Session-specific counter
SESSION_ID="${CLAUDE_SESSION_ID:-default}"
COUNTER_FILE="/tmp/agentops-toolcount-${SESSION_ID}"
THRESHOLD="${COMPACT_THRESHOLD:-50}"

# Increment counter
if [[ -f "$COUNTER_FILE" ]]; then
    COUNT=$(( $(cat "$COUNTER_FILE") + 1 ))
else
    COUNT=1
fi
echo "$COUNT" > "$COUNTER_FILE"

# Signal at threshold, then every 25
if [[ "$COUNT" -eq "$THRESHOLD" ]]; then
    echo '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"[StrategicCompact] '"$THRESHOLD"' tool calls reached — consider /compact if transitioning between exploration and implementation phases."}}'
elif [[ "$COUNT" -gt "$THRESHOLD" ]] && [[ $(( (COUNT - THRESHOLD) % 25 )) -eq 0 ]]; then
    echo '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"[StrategicCompact] '"$COUNT"' tool calls — good checkpoint for /compact if context is getting stale."}}'
fi

exit 0
```

### Why Manual Over Auto-Compact

| Auto-Compact | Strategic Compact |
|-------------|-------------------|
| Fires at 95% context fill | Fires at phase transitions |
| Loses working memory mid-task | Preserves working memory for current task |
| No user awareness | User chooses when to compact |
| Can't save important context | User can save critical notes before compact |

### When to Compact (User Guide)

**Good times:**
- After finishing exploration, before starting implementation
- After completing a crank wave, before starting the next
- After reading many files, before writing code
- At any natural milestone

**Bad times:**
- Mid-implementation (lose edit context)
- During council deliberation (lose judge context)
- While debugging (lose error reproduction context)

### Integration with RPI Phases

| Phase Transition | Compact? | Reason |
|-----------------|----------|--------|
| Discovery → Implementation | Yes | Fresh context for coding |
| Between crank waves | Optional | If context feels stale |
| Implementation → Validation | Yes | Fresh context for review |
| Post-mortem → Next RPI | Yes | Clean slate |

### Configuration

Environment variables:
- `COMPACT_THRESHOLD=50` — First signal threshold (default: 50)
- `COMPACT_INTERVAL=25` — Recurring signal interval (default: 25)

### Counter Management

```bash
# Reset counter (on /compact or session start)
rm -f "/tmp/agentops-toolcount-${CLAUDE_SESSION_ID:-default}"

# Check current count
cat "/tmp/agentops-toolcount-${CLAUDE_SESSION_ID:-default}" 2>/dev/null || echo "0"
```

The counter file is session-specific and lives in `/tmp`, so it auto-cleans on reboot.
