#!/bin/bash
# SubagentStop hook: capture worker output from swarm/crank executions.
# Writes final worker message + schema packet for session-start consumption.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Skip if no message
[ -z "$LAST_ASSISTANT_MSG" ] && exit 0

# Extract agent name from input if available
AGENT_NAME=""
if [ -n "$INPUT" ]; then
    if command -v jq >/dev/null 2>&1; then
        AGENT_NAME=$(echo "$INPUT" | jq -r '.agent_name // .name // ""' 2>/dev/null) || true
    fi
    if [ -z "$AGENT_NAME" ]; then
        AGENT_NAME=$(echo "$INPUT" | grep -o '"agent_name"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
            | sed 's/.*"agent_name"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
    fi
fi
[ -z "$AGENT_NAME" ] && AGENT_NAME="unknown"

# Sanitize for safe filename use (strip path separators and special chars)
AGENT_NAME=$(printf '%s' "$AGENT_NAME" | tr -cd 'a-zA-Z0-9_-')
[ -z "$AGENT_NAME" ] && AGENT_NAME="unknown"

# Write output
OUTPUT_DIR="$ROOT/.agents/ao/subagent-outputs"
mkdir -p "$OUTPUT_DIR"

TIMESTAMP=$(date -u +%Y-%m-%dT%H%M%SZ)
OUTPUT_FILE="$OUTPUT_DIR/${TIMESTAMP}_${AGENT_NAME}.md"

{
    echo "# Subagent Output: ${AGENT_NAME}"
    echo ""
    echo "**Timestamp:** $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "**Session:** ${CLAUDE_SESSION_ID:-unknown}"
    echo ""
    echo "## Final Message"
    echo ""
    echo "${LAST_ASSISTANT_MSG:0:2000}"
} > "$OUTPUT_FILE" 2>/dev/null

OUTPUT_REL=$(to_repo_relative_path "$OUTPUT_FILE")
PAYLOAD_JSON='{}'
if command -v jq >/dev/null 2>&1; then
    PAYLOAD_JSON=$(jq -n \
        --arg agent_name "$AGENT_NAME" \
        --arg summary "${LAST_ASSISTANT_MSG:0:2000}" \
        '{agent_name:$agent_name,last_assistant_message:$summary}')
fi

write_memory_packet "subagent_stop" "subagent-stop" "$PAYLOAD_JSON" "$OUTPUT_REL" >/dev/null 2>&1 || true

exit 0
