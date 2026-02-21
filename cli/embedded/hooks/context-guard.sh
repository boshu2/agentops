#!/bin/bash
# context-guard.sh - UserPromptSubmit hook: proactive context telemetry + handoff trigger
#
# Behavior:
# 1. Updates session budget telemetry from transcript usage
# 2. Emits nudge context on WARNING / CRITICAL
# 3. Writes auto-handoff marker when CRITICAL (configurable)
# 4. Optional strict mode blocks prompt submission on CRITICAL

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_CONTEXT_GUARD_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_LIB="$SCRIPT_DIR/../lib/hook-helpers.sh"
if [ -f "$HELPERS_LIB" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$HELPERS_LIB"
else
    timeout_run() {
        local seconds="$1"
        shift
        if command -v timeout >/dev/null 2>&1; then
            timeout "$seconds" "$@"
        elif command -v gtimeout >/dev/null 2>&1; then
            gtimeout "$seconds" "$@"
        else
            "$@"
        fi
    }
fi

INPUT=$(cat)
SESSION_ID="${CLAUDE_SESSION_ID:-}"
[ -z "$SESSION_ID" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // ""' 2>/dev/null)
else
    PROMPT=$(echo "$INPUT" | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

MAX_TOKENS="${AGENTOPS_CONTEXT_MAX_TOKENS:-200000}"
WATCHDOG_MINUTES="${AGENTOPS_CONTEXT_GUARD_WATCHDOG_MINUTES:-20}"
WRITE_HANDOFF="${AGENTOPS_CONTEXT_GUARD_WRITE_HANDOFF:-1}"
AUTO_RESTART_STALE="${AGENTOPS_CONTEXT_GUARD_AUTO_RESTART_STALE:-0}"
STRICT_MODE="${AGENTOPS_CONTEXT_GUARD_STRICT:-0}"

AGENT_NAME="${CLAUDE_AGENT_NAME:-}"
AO_ARGS=(context guard --session "$SESSION_ID" --max-tokens "$MAX_TOKENS" --watchdog-minutes "$WATCHDOG_MINUTES" -o json)
[ -n "$PROMPT" ] && AO_ARGS+=(--prompt "$PROMPT")
[ -n "$AGENT_NAME" ] && AO_ARGS+=(--agent-name "$AGENT_NAME")
[ "$WRITE_HANDOFF" = "1" ] && AO_ARGS+=(--write-handoff)
[ "$AUTO_RESTART_STALE" = "1" ] && AO_ARGS+=(--auto-restart-stale)

RESULT=$(timeout_run 3 ao "${AO_ARGS[@]}" 2>/dev/null) || exit 0
[ -z "$RESULT" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    ACTION=$(echo "$RESULT" | jq -r '.session.action // ""' 2>/dev/null)
    MESSAGE=$(echo "$RESULT" | jq -r '.hook_message // ""' 2>/dev/null)
else
    ACTION=$(echo "$RESULT" | grep -o '"action"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"action"[[:space:]]*:[[:space:]]*"//;s/"$//')
    MESSAGE=$(echo "$RESULT" | grep -o '"hook_message"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"hook_message"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

if [ -n "$MESSAGE" ] && [ "$MESSAGE" != "null" ]; then
    if command -v jq >/dev/null 2>&1; then
        jq -n --arg msg "$MESSAGE" '{"hookSpecificOutput":{"additionalContext":$msg}}'
    else
        safe_msg=${MESSAGE//\\/\\\\}
        safe_msg=${safe_msg//\"/\\\"}
        echo "{\"hookSpecificOutput\":{\"additionalContext\":\"$safe_msg\"}}"
    fi
fi

if [ "$STRICT_MODE" = "1" ] && [ "$ACTION" = "handoff_now" ]; then
    [ -z "$MESSAGE" ] && MESSAGE="Context is CRITICAL. End this session and continue from the auto-handoff in a fresh session."
    echo "$MESSAGE" >&2
    exit 2
fi

exit 0
