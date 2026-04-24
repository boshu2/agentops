#!/bin/bash
# commit-review-gate.sh - PreToolUse hook: inject staged diff before git commit
# Forces Claude to see its own changes before committing.
# Non-blocking (always exit 0). Injects diff as additionalContext.

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_COMMIT_REVIEW_DISABLED:-}" = "1" ] && exit 0

# Read stdin
INPUT=$(cat)

redact_sensitive_diff() {
    sed -E \
        -e 's/(([A-Za-z0-9_-]*([Aa][Pp][Ii][_-]?[Kk][Ee][Yy]|[Tt][Oo][Kk][Ee][Nn]|[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]|[Pp][Aa][Ss][Ss][Ww][Dd]|[Ss][Ee][Cc][Rr][Ee][Tt])[A-Za-z0-9_-]*)[[:space:]]*[:=][[:space:]]*)[^[:space:]"'\''`]+/\1[REDACTED]/g' \
        -e 's/(([Aa]uthorization|AUTHORIZATION)[[:space:]]*:[[:space:]]*([Bb]earer|[Bb]asic)[[:space:]]+)[^[:space:]"'\''`]+/\1[REDACTED]/g'
}

# Extract tool name and command
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$TOOL_NAME" ] || [ -z "$COMMAND" ]; then
    # Single jq call to extract both fields (avoids double-parse of stdin)
    IFS=$'\t' read -r _jq_tool _jq_cmd < <(echo "$INPUT" | jq -r '[.tool_name // "", .tool_input.command // ""] | @tsv' 2>/dev/null) || exit 0
    [ -z "$TOOL_NAME" ] && TOOL_NAME="$_jq_tool"
    [ -z "$COMMAND" ] && COMMAND="$_jq_cmd"
fi

# Only fire on Bash + git commit
[ "$TOOL_NAME" = "Bash" ] || exit 0
echo "$COMMAND" | grep -q 'git commit' || exit 0

# Don't fire on --amend with no new changes (just message edit)
echo "$COMMAND" | grep -qE '\-\-amend.*\-\-no-edit' && exit 0

# Capture staged diff summary (separate call — different output format)
DIFF_STAT=$(git diff --cached --stat 2>/dev/null)
[ -z "$DIFF_STAT" ] && exit 0

# Capture full diff once and derive metrics (TOCTOU fix: index can change between calls)
FULL_DIFF=$(git diff --cached 2>/dev/null)
FILE_COUNT=$(printf '%s\n' "$FULL_DIFF" | grep -c '^diff --git' 2>/dev/null || echo 0)
[ "$FILE_COUNT" = "0" ] && exit 0

DIFF_LINES=$(printf '%s\n' "$FULL_DIFF" | wc -l | tr -d ' ')
DIFF_CONTENT=$(printf '%s\n' "$FULL_DIFF" | head -200 | redact_sensitive_diff)
TRUNCATED=""
if [ "$DIFF_LINES" -gt 200 ]; then
    TRUNCATED=" (showing first 200 of $DIFF_LINES lines — run 'git diff --cached' for full diff)"
fi

# Build review context
REVIEW_MSG="SELF-REVIEW before committing ($FILE_COUNT files changed):
Check for: wrong variable references, changed defaults, removed error handling, silent data loss, YAML syntax errors.

Staged changes:
$DIFF_STAT
${TRUNCATED}

${DIFF_CONTENT}"

# Inject as additionalContext
if command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$REVIEW_MSG" '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":$ctx}}'
else
    # Fallback: escape for JSON
    safe_msg=${REVIEW_MSG//\\/\\\\}
    safe_msg=${safe_msg//\"/\\\"}
    safe_msg=$(echo "$safe_msg" | tr '\n' ' ')
    echo "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"additionalContext\":\"$safe_msg\"}}"
fi

exit 0
