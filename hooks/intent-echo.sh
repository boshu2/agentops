#!/bin/bash
# intent-echo.sh - UserPromptSubmit hook: force intent confirmation for high-stakes operations
# Detects destructive/scoping keywords and injects echo protocol.
# Non-blocking (always exit 0). Injects reminder as additionalContext.

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_INTENT_ECHO_DISABLED:-}" = "1" ] && exit 0

# Read stdin
INPUT=$(cat)

# Extract prompt
if command -v jq >/dev/null 2>&1; then
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // ""' 2>/dev/null)
else
    PROMPT=$(echo "$INPUT" | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# No prompt → exit silently
[ -z "$PROMPT" ] || [ "$PROMPT" = "null" ] && exit 0

# Find repo root for state files
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
STATE_DIR="$ROOT/.agents/ao"
mkdir -p "$STATE_DIR" 2>/dev/null

# Dedup: don't fire if already fired recently (10-min TTL)
DEDUP_FLAG="$STATE_DIR/.intent-echo-fired"
if [ -f "$DEDUP_FLAG" ]; then
    if find "$DEDUP_FLAG" -mmin -10 2>/dev/null | grep -q .; then
        exit 0
    else
        rm -f "$DEDUP_FLAG" 2>/dev/null
    fi
fi

# Lowercase prompt for matching
PROMPT_LOWER=$(echo "$PROMPT" | tr '[:upper:]' '[:lower:]')

# Check for high-stakes keywords
TRIGGERED=0

# Destructive/scoping verbs
if echo "$PROMPT_LOWER" | grep -qE '\b(remove|delete|refactor|rewrite|replace|migrate|restructure|eliminate|strip|extract|consolidate|overhaul|rip out|tear down|gut)\b'; then
    TRIGGERED=1
fi

# Scope-limiting words that signal ambiguous intent ("just rename", "only update")
if [ "$TRIGGERED" = "0" ]; then
    if echo "$PROMPT_LOWER" | grep -qE '\b(just|only)\b'; then
        TRIGGERED=1
    fi
fi

# Scope-ambiguous phrases in long prompts (>150 chars)
if [ "$TRIGGERED" = "0" ] && [ "${#PROMPT}" -gt 150 ]; then
    if echo "$PROMPT_LOWER" | grep -qE '\b(all|everything|entire|whole|every)\b'; then
        TRIGGERED=1
    fi
fi

# Not triggered → exit silently
[ "$TRIGGERED" = "0" ] && exit 0

# Write dedup flag
touch "$DEDUP_FLAG" 2>/dev/null

# Inject echo protocol
ECHO_MSG="HIGH-STAKES OPERATION DETECTED. Before proceeding:
1. Restate the user's intent in ONE sentence
2. List what WILL change (specific files/functions)
3. List what will NOT change
4. State your success criteria
Proceed ONLY after this confirmation step."

if command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$ECHO_MSG" '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":$ctx}}'
else
    safe_msg=${ECHO_MSG//\\/\\\\}
    safe_msg=${safe_msg//\"/\\\"}
    safe_msg=$(echo "$safe_msg" | tr '\n' ' ')
    echo "{\"hookSpecificOutput\":{\"hookEventName\":\"UserPromptSubmit\",\"additionalContext\":\"$safe_msg\"}}"
fi

exit 0
