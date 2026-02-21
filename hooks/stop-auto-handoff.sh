#!/bin/bash
# Stop hook: capture last_assistant_message for session continuity.
# Writes markdown handoff + schema packet for session-start.sh packet-first recovery.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Skip if no message to capture
[ -z "$LAST_ASSISTANT_MSG" ] && exit 0

# Gather context
RATCHET_STATE=$(timeout_run 1 ao ratchet status -o json 2>/dev/null || echo "")
ACTIVE_BEAD=$(timeout_run 1 bd current 2>/dev/null || echo "")
TIMESTAMP=$(date -u +%Y-%m-%dT%H%M%SZ)

# Write markdown handoff artifact (legacy-friendly human-readable context).
HANDOFF_DIR="$ROOT/.agents/handoff"
mkdir -p "$HANDOFF_DIR" 2>/dev/null || exit 0

HANDOFF_FILE="$HANDOFF_DIR/stop-${TIMESTAMP}.md"
{
    echo "# Auto-Handoff (Stop)"
    echo ""
    echo "**Timestamp:** $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "**Session:** ${CLAUDE_SESSION_ID:-unknown}"
    echo ""
    echo "## Last Assistant Message"
    echo "${LAST_ASSISTANT_MSG:0:2000}"
    echo ""
    echo "## Ratchet State"
    if [ -n "$RATCHET_STATE" ]; then
        echo '```json'
        echo "$RATCHET_STATE"
        echo '```'
    else
        echo "none"
    fi
    echo ""
    echo "## Active Work"
    if [ -n "$ACTIVE_BEAD" ]; then
        echo "$ACTIVE_BEAD"
    else
        echo "none"
    fi
} > "$HANDOFF_FILE" 2>/dev/null || exit 0

HANDOFF_REL=$(to_repo_relative_path "$HANDOFF_FILE")
PAYLOAD_JSON='{}'
if command -v jq >/dev/null 2>&1; then
    PAYLOAD_JSON=$(jq -n \
        --arg message "${LAST_ASSISTANT_MSG:0:2000}" \
        --arg ratchet "$RATCHET_STATE" \
        --arg bead "$ACTIVE_BEAD" \
        '{last_assistant_message:$message,ratchet_state:$ratchet,active_bead:$bead}')
fi

# Packet write is best-effort; fail-open hook semantics remain.
write_memory_packet "stop" "stop-auto-handoff" "$PAYLOAD_JSON" "$HANDOFF_REL" >/dev/null 2>&1 || true

exit 0
