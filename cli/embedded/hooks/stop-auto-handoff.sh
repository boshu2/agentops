#!/bin/bash
# Stop hook: capture last_assistant_message for session handoff
# Writes handoff to .agents/handoff/pending/ for session-start.sh to consume

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Skip if no message to capture
[ -z "$LAST_ASSISTANT_MSG" ] && exit 0

# Gather context
RATCHET_STATE=$(timeout 1 ao ratchet status -o json 2>/dev/null || echo "")
ACTIVE_BEAD=$(timeout 1 bd current 2>/dev/null || echo "")
TIMESTAMP=$(date -u +%Y-%m-%dT%H%M%SZ)

# Write handoff
HANDOFF_DIR="$ROOT/.agents/handoff/pending"
mkdir -p "$HANDOFF_DIR"

HANDOFF_FILE="$HANDOFF_DIR/${TIMESTAMP}-stop.json"

if command -v jq >/dev/null 2>&1; then
    jq -n \
        --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --arg type "stop" \
        --arg last_msg "${LAST_ASSISTANT_MSG:0:2000}" \
        --arg ratchet "$RATCHET_STATE" \
        --arg bead "$ACTIVE_BEAD" \
        --arg session "${CLAUDE_SESSION_ID:-unknown}" \
        '{ts:$ts,type:$type,last_assistant_message:$last_msg,ratchet_state:$ratchet,active_bead:$bead,session_id:$session}' \
        > "$HANDOFF_FILE" 2>/dev/null
else
    # Fallback without jq — escape for JSON safety
    ESC_MSG=$(json_escape_value "${LAST_ASSISTANT_MSG:0:2000}")
    ESC_RATCHET=$(json_escape_value "$RATCHET_STATE")
    ESC_BEAD=$(json_escape_value "$ACTIVE_BEAD")
    printf '{"ts":"%s","type":"stop","last_assistant_message":"%s","ratchet_state":"%s","active_bead":"%s","session_id":"%s"}\n' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$ESC_MSG" "$ESC_RATCHET" "$ESC_BEAD" "${CLAUDE_SESSION_ID:-unknown}" \
        > "$HANDOFF_FILE" 2>/dev/null
fi

exit 0
