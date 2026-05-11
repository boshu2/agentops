#!/usr/bin/env bash
# edit-knowledge-surface.sh - PreToolUse hook (matcher: Edit)
# Surfaces relevant learnings BEFORE editing files that appear in knowledge artifacts.
# Uses grep -l for fast path matching (<100ms on 155 files).
# Non-blocking (always exit 0). Knowledge via additionalContext injection.
# practices: [wiki-knowledge-surface, design-by-contract]
set -euo pipefail

# Kill switches
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_EDIT_KNOWLEDGE_DISABLED:-}" == "1" ]] && exit 0

# Only fire on Edit tool
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [[ -z "$TOOL_NAME" ]]; then
    INPUT=$(cat)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[[ "$TOOL_NAME" != "Edit" ]] && exit 0

# --- SKILL.md line-count guard (warn at 240, fail at 250) ---
_check_skill_line_limit() {
    local fpath="$1"
    [[ "$fpath" == */SKILL.md ]] || return 0
    [[ -f "$fpath" ]] || return 0
    local lines
    lines=$(wc -l < "$fpath")
    if (( lines >= 250 )); then
        cat <<WARN_EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"WARNING: ${fpath##*/} is ${lines} lines (limit 248). skill-lint will FAIL in CI. Move content to references/ or scripts/."}}
WARN_EOF
        exit 0
    elif (( lines >= 240 )); then
        cat <<WARN_EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"CAUTION: ${fpath##*/} is ${lines} lines — approaching 248-line CI limit. Consider moving content to references/."}}
WARN_EOF
        exit 0
    fi
}

# Find repo root
ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
LEARNINGS_DIR="$ROOT/.agents/learnings"
[[ -d "$LEARNINGS_DIR" ]] || exit 0

# Extract file path being edited
FILE_PATH="${CLAUDE_TOOL_INPUT_FILE_PATH:-}"
if [[ -z "$FILE_PATH" ]]; then
    [[ -z "${INPUT:-}" ]] && INPUT=$(cat)
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null) || exit 0
fi
[[ -z "$FILE_PATH" ]] && exit 0

# SKILL.md line-count guard
_check_skill_line_limit "$FILE_PATH"

# Normalize to relative path for matching
REL_PATH="${FILE_PATH#"$ROOT/"}"
# Extract just the filename for broader matching
FILENAME=$(basename "$REL_PATH")

# Try relative path first (precise match), fall back to filename (broader)
MATCHES=""
MATCHES=$(grep -rl "$REL_PATH" "$LEARNINGS_DIR/" 2>/dev/null | head -3) || true
if [[ -z "$MATCHES" ]]; then
    MATCHES=$(grep -rl "$FILENAME" "$LEARNINGS_DIR/" 2>/dev/null | head -3) || true
fi
[[ -z "$MATCHES" ]] && exit 0

# Build summary of matched learnings (read just the title line from each)
SUMMARIES=""
while IFS= read -r match_file; do
    [[ -z "$match_file" ]] && continue
    # Extract learning title (first # heading or filename)
    TITLE=$(grep -m1 '^# ' "$match_file" 2>/dev/null | sed 's/^# //' || basename "$match_file")
    BASENAME=$(basename "$match_file")
    SUMMARIES="${SUMMARIES}- ${BASENAME}: ${TITLE}\n"
done <<< "$MATCHES"

[[ -z "$SUMMARIES" ]] && exit 0

# Output as hook context
cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"Relevant learnings for $(basename "$REL_PATH"):\n${SUMMARIES}Review these before making changes to this file."}}
EOF
