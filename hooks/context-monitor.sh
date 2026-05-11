#!/usr/bin/env bash
set -euo pipefail

# context-monitor.sh - PostToolUse hook: context window usage monitoring via bridge pattern
# practices: [sre, distributed-tracing]
#
# Reads context metrics from statusline bridge file and injects agent-facing
# warnings at 35% and 25% remaining context.
#
# Bridge file: /tmp/claude-ctx-{CLAUDE_SESSION_ID}.json
# Written by statusline command (user-configured).
#
# Env vars:
#   AGENTOPS_HOOKS_DISABLED=1          - disable all hooks
#   AGENTOPS_CONTEXT_MONITOR_DISABLED=1 - disable this hook only
#   AGENTOPS_CONTEXT_WARN_PERCENT=35   - warning threshold (default 35)
#   AGENTOPS_CONTEXT_CRIT_PERCENT=25   - critical threshold (default 25)
#
# ── Bridge Setup ──────────────────────────────────────────────────────────────
#
# This hook reads a JSON bridge file that YOUR statusline command writes.
# Without this bridge file, the hook silently exits (no error, no output).
#
# Bridge file location: /tmp/claude-ctx-{CLAUDE_SESSION_ID}.json
# Required JSON format:
#
#   {
#     "remaining_percent": 65,
#     "total_tokens": 200000,
#     "used_tokens": 70000
#   }
#
# Only "remaining_percent" is required. The other fields are informational.
# The file must be updated within the last 5 minutes or it is treated as stale.
#
# Option 1: Claude Code statusline command
#
#   Add to ~/.claude/settings.json:
#
#   {
#     "statusline": {
#       "command": "echo '{\"remaining_percent\": '$REMAINING'}' > /tmp/claude-ctx-$CLAUDE_SESSION_ID.json"
#     }
#   }
#
#   Replace $REMAINING with however your statusline calculates the percentage.
#   The statusline command runs periodically and has CLAUDE_SESSION_ID in env.
#
# Option 2: Manual bridge script
#
#   Save as ~/bin/ctx-bridge.sh and run in a separate terminal:
#
#   #!/usr/bin/env bash
#   # Usage: ctx-bridge.sh <session-id> [poll-seconds]
#   SESSION_ID="${1:?Usage: ctx-bridge.sh <session-id> [poll-seconds]}"
#   POLL="${2:-30}"
#   BRIDGE="/tmp/claude-ctx-${SESSION_ID}.json"
#   while true; do
#     # Replace this with your actual token-counting logic
#     TOTAL=200000
#     USED=$(wc -c < "/tmp/claude-session-${SESSION_ID}.log" 2>/dev/null || echo 0)
#     PCT=$(( 100 - (USED * 100 / TOTAL) ))
#     printf '{"remaining_percent":%d,"total_tokens":%d,"used_tokens":%d}\n' \
#       "$PCT" "$TOTAL" "$USED" > "$BRIDGE"
#     sleep "$POLL"
#   done
#
# ──────────────────────────────────────────────────────────────────────────────

# Kill switches
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_CONTEXT_MONITOR_DISABLED:-}" == "1" ]] && exit 0

# Consume stdin (required even if unused)
cat > /dev/null

SESSION_ID="${CLAUDE_SESSION_ID:-}"
[[ -z "$SESSION_ID" ]] && exit 0

BRIDGE_FILE="/tmp/claude-ctx-${SESSION_ID}.json"
[[ ! -f "$BRIDGE_FILE" ]] && exit 0

# Bridge file must be recent (< 5 min old) to avoid stale data
if command -v stat >/dev/null 2>&1; then
    if [[ "$(uname)" == "Darwin" ]]; then
        FILE_AGE=$(( $(date +%s) - $(stat -f %m "$BRIDGE_FILE") ))
    else
        FILE_AGE=$(( $(date +%s) - $(stat -c %Y "$BRIDGE_FILE") ))
    fi
    [[ "$FILE_AGE" -gt 300 ]] && exit 0
fi

# Read remaining percent from bridge file
if command -v jq >/dev/null 2>&1; then
    REMAINING=$(jq -r '.remaining_percent // ""' "$BRIDGE_FILE" 2>/dev/null) || exit 0
else
    REMAINING=$(grep -o '"remaining_percent"[[:space:]]*:[[:space:]]*[0-9]*' "$BRIDGE_FILE" 2>/dev/null | grep -o '[0-9]*$') || exit 0
fi

[[ -z "$REMAINING" ]] && exit 0

WARN_THRESHOLD="${AGENTOPS_CONTEXT_WARN_PERCENT:-35}"
CRIT_THRESHOLD="${AGENTOPS_CONTEXT_CRIT_PERCENT:-25}"

MSG=""

if [[ "$REMAINING" -le "$CRIT_THRESHOLD" ]]; then
    MSG="Context window at ${REMAINING}% remaining. Plan final steps carefully — room for ~1-2 more significant operations. Consider /handoff."
elif [[ "$REMAINING" -le "$WARN_THRESHOLD" ]]; then
    MSG="Context window at ${REMAINING}% remaining. Consider wrapping up current work within ~2-3 tasks."
fi

[[ -z "$MSG" ]] && exit 0

# Emit agent-facing warning
if command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$MSG" '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":$ctx}}'
else
    safe_msg=${MSG//\\/\\\\}
    safe_msg=${safe_msg//\"/\\\"}
    echo "{\"hookSpecificOutput\":{\"hookEventName\":\"PostToolUse\",\"additionalContext\":\"$safe_msg\"}}"
fi

exit 0
