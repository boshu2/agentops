#!/usr/bin/env bash
set -euo pipefail
# quality-signals.sh - UserPromptSubmit hook: lightweight session quality signal detection
# Detects repeated prompts and correction patterns. Logs signals to
# .agents/signals/session-quality.jsonl (append-only, advisory only).
# Non-blocking (always exit 0). Never surfaces in /status — separate task.

# Kill switches
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_QUALITY_SIGNALS_DISABLED:-}" == "1" ]] && exit 0

# Read all stdin
INPUT=$(cat)

# Extract prompt from JSON
PROMPT=""
if command -v jq >/dev/null 2>&1; then
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // ""' 2>/dev/null) || true
else
    PROMPT=$(echo "$INPUT" | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//') || true
fi

# No prompt → exit silently
[ -z "$PROMPT" ] || [ "$PROMPT" = "null" ] && exit 0

# Find repo root
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)

# State and signal directories
STATE_DIR="$ROOT/.agents/ao"
SIGNAL_DIR="$ROOT/.agents/signals"
mkdir -p "$STATE_DIR" "$SIGNAL_DIR" 2>/dev/null

LAST_PROMPT_FILE="$STATE_DIR/.last-prompt"
SIGNAL_LOG="$SIGNAL_DIR/session-quality.jsonl"

SESSION_ID="${CODEX_SESSION_ID:-${CODEX_THREAD_ID:-${CLAUDE_SESSION_ID:-unknown}}}"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

hash_prompt() {
    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s' "$1" | sha256sum | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        printf '%s' "$1" | shasum -a 256 | awk '{print $1}'
    else
        printf '%s' "$1" | cksum | awk '{print $1 ":" $2}'
    fi
}

# Helper: append a signal entry to the JSONL log
log_signal() {
    local signal_type="$1"
    local detail="$2"

    if command -v jq >/dev/null 2>&1; then
        jq -n -c \
            --arg timestamp "$TIMESTAMP" \
            --arg signal_type "$signal_type" \
            --arg detail "$detail" \
            --arg session_id "$SESSION_ID" \
            '{timestamp:$timestamp,signal_type:$signal_type,detail:$detail,session_id:$session_id}' \
            >> "$SIGNAL_LOG" 2>/dev/null || true
    else
        local esc_detail
        esc_detail=${detail//\\/\\\\}
        esc_detail=${esc_detail//\"/\\\"}
        printf '{"timestamp":"%s","signal_type":"%s","detail":"%s","session_id":"%s"}\n' \
            "$TIMESTAMP" "$signal_type" "$esc_detail" "$SESSION_ID" \
            >> "$SIGNAL_LOG" 2>/dev/null || true
    fi
}

# --- Detection 1: Repeated prompts ---
PROMPT_FINGERPRINT=$(hash_prompt "$PROMPT")
if [ -f "$LAST_PROMPT_FILE" ]; then
    LAST_PROMPT=$(cat "$LAST_PROMPT_FILE" 2>/dev/null || echo "")
    if [ "$PROMPT_FINGERPRINT" = "$LAST_PROMPT" ] && [ -n "$PROMPT" ]; then
        log_signal "repeated_prompt" "User submitted identical prompt twice in a row"
    fi
fi

# Store current prompt fingerprint for next comparison without retaining prompt text.
printf '%s' "$PROMPT_FINGERPRINT" > "$LAST_PROMPT_FILE" 2>/dev/null || true

# --- Detection 2: Correction patterns ---
# Match at start of prompt (case-insensitive)
PROMPT_LOWER=$(echo "$PROMPT" | tr '[:upper:]' '[:lower:]')
if echo "$PROMPT_LOWER" | grep -qE '^[[:space:]]*(no|wrong|not what|stop|undo|revert|that'\''s not|incorrect)\b'; then
    log_signal "correction" "Prompt starts with correction pattern"
fi

exit 0
