#!/bin/bash
# ratchet-advance.sh - PostToolUse hook: suggest next RPI skill after ratchet record
# Fires on successful `ao ratchet record <step>`. Injects additionalContext suggestion.
# Kill switches: AGENTOPS_AUTOCHAIN=0, AGENTOPS_HOOKS_DISABLED=1

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_AUTOCHAIN:-}" = "0" ] && exit 0
AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

run_ao_quick() {
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "${AGENTOPS_RATCHET_ADVANCE_TIMEOUT:-2}" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

# Read stdin JSON
INPUT=$(cat)

# Extract command and exit code from tool input/response
if command -v jq >/dev/null 2>&1; then
    CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null)
    EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_response.exit_code // ""' 2>/dev/null)
else
    CMD=$(echo "$INPUT" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"command"[[:space:]]*:[[:space:]]*"//;s/"$//')
    EXIT_CODE=$(echo "$INPUT" | grep -o '"exit_code"[[:space:]]*:[[:space:]]*[0-9]*' | head -1 | sed 's/.*:[[:space:]]*//')
fi

# Hot-path exit: only care about `ao ratchet record`
echo "$CMD" | grep -q 'ao ratchet record' || exit 0

# Check exit code — only suggest on success
[ "$EXIT_CODE" != "0" ] && [ -n "$EXIT_CODE" ] && exit 0

# Extract step name: first positional arg after "record"
STEP=$(echo "$CMD" | sed -n 's/.*ao ratchet record[[:space:]]\{1,\}\([a-z_-]*\).*/\1/p')
[ -z "$STEP" ] && exit 0

# Map step → next skill
# Try new structured command first
if command -v jq >/dev/null 2>&1 && run_ao_quick ratchet next --help >/dev/null 2>&1; then
    next_json=$(run_ao_quick ratchet next -o json)
    if [ -n "$next_json" ]; then
        NEXT=$(echo "$next_json" | jq -r '.skill // ""')
        COMPLETE=$(echo "$next_json" | jq -r '.complete // false')
        if [ "$COMPLETE" = "true" ]; then
            NEXT="Cycle complete"
        fi
    fi
fi

# Fallback: original case statement if new command unavailable or failed
if [ -z "$NEXT" ]; then
    case "$STEP" in
        research)    NEXT="/plan" ;;
        plan)        NEXT="/pre-mortem" ;;
        pre-mortem)  NEXT="/implement or /crank" ;;
        implement)   NEXT="/vibe" ;;
        crank)       NEXT="/vibe" ;;
        vibe)        NEXT="/post-mortem" ;;
        post-mortem) NEXT="Cycle complete" ;;
        *)           exit 0 ;;  # Unknown step, no suggestion
    esac
fi

# Extract --output artifact path if present
ARTIFACT=$(echo "$CMD" | sed -n 's/.*--output[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p')
# Sanitize: relative paths only, no ".." traversal
if [ -n "$ARTIFACT" ]; then
    echo "$ARTIFACT" | grep -q '^\.\.' && ARTIFACT=""
    echo "$ARTIFACT" | grep -q '^\/' && ARTIFACT=""
fi

# Idempotency: check if next step already recorded in chain.jsonl
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo ".")
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHAIN_LIB="$SCRIPT_DIR/../lib/chain-parser.sh"
if [ -f "$CHAIN_LIB" ]; then
    # shellcheck source=../lib/chain-parser.sh
    . "$CHAIN_LIB"
else
    chain_find_entry() {
        local chain_file="$1" step_name="$2"
        grep -E "\"(step|gate)\"[[:space:]]*:[[:space:]]*\"${step_name}\"" "$chain_file" 2>/dev/null | tail -1
    }
    chain_is_done() {
        local entry="$1"
        [ -z "$entry" ] && return 1
        if echo "$entry" | grep -qE '"status"[[:space:]]*:[[:space:]]*"(locked|skipped)"'; then
            return 0
        fi
        if echo "$entry" | grep -qE '"locked"[[:space:]]*:[[:space:]]*true'; then
            return 0
        fi
        return 1
    }
fi
CHAIN="$ROOT/.agents/ao/chain.jsonl"
if [ -f "$CHAIN" ]; then
    # Determine the next step NAME (not the skill name)
    case "$STEP" in
        research)    NEXT_STEP="plan" ;;
        plan)        NEXT_STEP="pre-mortem" ;;
        pre-mortem)  NEXT_STEP="implement" ;;
        implement)   NEXT_STEP="vibe" ;;
        crank)       NEXT_STEP="vibe" ;;
        vibe)        NEXT_STEP="post-mortem" ;;
        *)           NEXT_STEP="" ;;
    esac
    if [ -n "$NEXT_STEP" ]; then
        ENTRY=$(chain_find_entry "$CHAIN" "$NEXT_STEP")
        if [ -n "$ENTRY" ] && chain_is_done "$ENTRY"; then
            exit 0  # Already done, suppress
        fi
    fi
fi

# Write dedup flag file for prompt-nudge coordination
FLAG_DIR="$ROOT/.agents/ao"
mkdir -p "$FLAG_DIR" 2>/dev/null
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $STEP" > "$FLAG_DIR/.ratchet-advance-fired"

# Build suggestion message
if [ "$NEXT" = "Cycle complete" ]; then
    MSG="RPI auto-advance: ${STEP} completed. Cycle complete — all RPI steps done."
elif [ -n "$ARTIFACT" ]; then
    MSG="RPI auto-advance: ${STEP} completed. Suggested next: ${NEXT} ${ARTIFACT}"
else
    MSG="RPI auto-advance: ${STEP} completed. Suggested next: ${NEXT}"
fi

# Output as additionalContext
printf '{"hookSpecificOutput":{"additionalContext":"%s"}}\n' "$MSG"
exit 0
