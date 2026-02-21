#!/bin/bash
# prompt-nudge.sh - UserPromptSubmit hook: ratchet-aware one-liner nudges
# Checks prompt keywords against RPI ratchet state. Injects reminders.
# Cap: one nudge line, < 200 bytes. No directory scanning.
# Nudge priority: ratchet-advance.sh (PostToolUse, PRIMARY) > prompt-nudge.sh (UserPromptSubmit, SECONDARY) > session-start.sh (SessionStart, CROSS-SESSION)
# This hook suppresses if ratchet-advance already fired recently (dedup via .ratchet-advance-fired flag).

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

run_ao_quick() {
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "${AGENTOPS_PROMPT_NUDGE_TIMEOUT:-2}" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

# Read all stdin
INPUT=$(cat)

# Extract prompt from JSON
if command -v jq >/dev/null 2>&1; then
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // ""' 2>/dev/null)
else
    PROMPT=$(echo "$INPUT" | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# No prompt → exit silently
[ -z "$PROMPT" ] || [ "$PROMPT" = "null" ] && exit 0

# Find repo root
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo ".")

# Nudge dedup: suppress if ratchet-advance already fired recently
DEDUP_FLAG="$ROOT/.agents/ao/.ratchet-advance-fired"
if [ -f "$DEDUP_FLAG" ]; then
    # Check if flag is fresh (written in last 10 minutes)
    if find "$DEDUP_FLAG" -mmin -10 2>/dev/null | grep -q .; then
        # ratchet-advance already fired recently — suppress this nudge
        exit 0
    else
        # Stale flag — clean up and continue with normal nudge logic
        rm -f "$DEDUP_FLAG" 2>/dev/null
    fi
fi

# Cold start: no ratchet chain = no nudging
[ ! -f "$ROOT/.agents/ao/chain.jsonl" ] && exit 0

# Check ao availability
command -v ao >/dev/null 2>&1 || exit 0

# Get ratchet status as JSON
RATCHET=$(run_ao_quick ratchet status -o json) || exit 0
[ -z "$RATCHET" ] && exit 0

# Parse steps (requires jq for JSON parsing)
command -v jq >/dev/null 2>&1 || exit 0

# Helper: check if a step is pending
# Note: ao ratchet status normalizes to CANONICAL field names ("step", "status"),
# so no dual-schema grep needed here (unlike push-gate.sh which parses chain.jsonl directly).
step_pending() {
    echo "$RATCHET" | jq -e ".steps[] | select(.step == \"$1\" and .status == \"pending\")" >/dev/null 2>&1
}

# Lowercase prompt for matching
PROMPT_LOWER=$(echo "$PROMPT" | tr '[:upper:]' '[:lower:]')

NUDGE=""

# Check prompt keywords against ratchet state
if echo "$PROMPT_LOWER" | grep -qE '(implement|build|code|fix|create|add)'; then
    if step_pending "pre-mortem"; then
        NUDGE="Reminder: pre-mortem hasn't been run on your plan."
    fi
elif echo "$PROMPT_LOWER" | grep -qE '(commit|push|ship|deploy|release)'; then
    if step_pending "vibe"; then
        NUDGE="Reminder: run /vibe before pushing."
    fi
elif echo "$PROMPT_LOWER" | grep -qE '(done|finished|wrap|complete|close)'; then
    if step_pending "post-mortem"; then
        NUDGE="Reminder: run /post-mortem to capture learnings."
    fi
fi

# No nudge needed → exit silently
[ -z "$NUDGE" ] && exit 0

# Output nudge as additionalContext
jq -n --arg nudge "$NUDGE" '{"hookSpecificOutput":{"additionalContext":$nudge}}'

exit 0
