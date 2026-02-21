#!/bin/bash
# ConfigChange hook: security auditing for mid-session configuration changes
# Logs config changes and optionally blocks critical changes in strict mode
#
# NOTE: This hook uses AGENTOPS_HOOKS_DISABLED as its kill switch.
# If someone disables hooks via a config change, this hook is also disabled —
# a known chicken-and-egg limitation. The hook still fires for config FILE
# changes since env vars are set before the process starts.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Skip if no input
[ -z "$INPUT" ] && exit 0

# Extract config change fields
CONFIG_KEY=""
OLD_VALUE=""
NEW_VALUE=""

if command -v jq >/dev/null 2>&1; then
    CONFIG_KEY=$(echo "$INPUT" | jq -r '.config_key // .key // ""' 2>/dev/null) || true
    OLD_VALUE=$(echo "$INPUT" | jq -r '.old_value // ""' 2>/dev/null) || true
    NEW_VALUE=$(echo "$INPUT" | jq -r '.new_value // ""' 2>/dev/null) || true
else
    CONFIG_KEY=$(echo "$INPUT" | grep -o '"config_key"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
        | sed 's/.*"config_key"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
    OLD_VALUE=$(echo "$INPUT" | grep -o '"old_value"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
        | sed 's/.*"old_value"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
    NEW_VALUE=$(echo "$INPUT" | grep -o '"new_value"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
        | sed 's/.*"new_value"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
fi

# Skip if no key identified
[ -z "$CONFIG_KEY" ] && exit 0

# Classify severity
SEVERITY="info"
case "$CONFIG_KEY" in
    AGENTOPS_HOOKS_DISABLED|approval_policy|sandbox_mode|disableAllHooks)
        SEVERITY="critical"
        ;;
    AGENTOPS_WORKER|AGENTOPS_CONTEXT_*|AGENTOPS_EVICTION_*|AGENTOPS_*_DISABLED)
        SEVERITY="warn"
        ;;
esac

# Log to config-changes.jsonl
LOG_DIR="$ROOT/.agents/ao"
mkdir -p "$LOG_DIR"

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
AGENT_NAME="${CLAUDE_AGENT_NAME:-${USER:-unknown}}"

if command -v jq >/dev/null 2>&1; then
    jq -n --compact-output \
        --arg ts "$TIMESTAMP" \
        --arg agent "$AGENT_NAME" \
        --arg key "$CONFIG_KEY" \
        --arg old "$OLD_VALUE" \
        --arg new "$NEW_VALUE" \
        --arg severity "$SEVERITY" \
        --arg session "${CLAUDE_SESSION_ID:-unknown}" \
        '{ts:$ts,agent:$agent,config_key:$key,old_value:$old,new_value:$new,severity:$severity,session_id:$session}' \
        >> "$LOG_DIR/config-changes.jsonl" 2>/dev/null
else
    ESC_OLD=$(json_escape_value "$OLD_VALUE")
    ESC_NEW=$(json_escape_value "$NEW_VALUE")
    ESC_KEY=$(json_escape_value "$CONFIG_KEY")
    ESC_AGENT=$(json_escape_value "$AGENT_NAME")
    printf '{"ts":"%s","agent":"%s","config_key":"%s","old_value":"%s","new_value":"%s","severity":"%s","session_id":"%s"}\n' \
        "$TIMESTAMP" "$ESC_AGENT" "$ESC_KEY" "$ESC_OLD" "$ESC_NEW" "$SEVERITY" "${CLAUDE_SESSION_ID:-unknown}" \
        >> "$LOG_DIR/config-changes.jsonl" 2>/dev/null
fi

# Strict mode: block critical changes
if [ "${AGENTOPS_CONFIG_GUARD_STRICT:-}" = "1" ] && [ "$SEVERITY" = "critical" ]; then
    write_failure "config_change_guard" "$CONFIG_KEY" 2 "Critical config change blocked: $CONFIG_KEY from '$OLD_VALUE' to '$NEW_VALUE'"
    echo "BLOCKED: Critical configuration change '$CONFIG_KEY' requires approval. Set AGENTOPS_CONFIG_GUARD_STRICT=0 to allow." >&2
    exit 2
fi

exit 0
