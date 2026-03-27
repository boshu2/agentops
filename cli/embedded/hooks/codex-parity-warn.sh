#!/usr/bin/env bash
# codex-parity-warn.sh - PreToolUse hook (matcher: Edit)
# Warns when editing skills/ files that have a skills-codex/ counterpart needing sync.
# Non-blocking (always exit 0). Warning via additionalContext injection.
set -euo pipefail

# Kill switches
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_CODEX_PARITY_DISABLED:-}" == "1" ]] && exit 0

# Only fire on Edit tool
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [[ -z "$TOOL_NAME" ]]; then
    INPUT=$(cat)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[[ "$TOOL_NAME" != "Edit" ]] && exit 0

# Find repo root
ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

# Extract file path being edited
FILE_PATH="${CLAUDE_TOOL_INPUT_FILE_PATH:-}"
if [[ -z "$FILE_PATH" ]]; then
    [[ -z "${INPUT:-}" ]] && INPUT=$(cat)
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null) || exit 0
fi
[[ -z "$FILE_PATH" ]] && exit 0

# Normalize to relative path
REL_PATH="${FILE_PATH#"$ROOT/"}"

# Only fire for skills/ edits (not skills-codex/)
[[ "$REL_PATH" == skills/* ]] || exit 0
[[ "$REL_PATH" == skills-codex/* ]] && exit 0

# Extract skill name: skills/<name>/...
SKILL_NAME=$(echo "$REL_PATH" | cut -d'/' -f2)
[[ -z "$SKILL_NAME" ]] && exit 0

# Check if codex counterpart exists
CODEX_DIR="$ROOT/skills-codex/$SKILL_NAME"
[[ -d "$CODEX_DIR" ]] || exit 0

# Determine what kind of file was edited
EDITED_FILE=$(basename "$REL_PATH")
CODEX_COUNTERPART="$CODEX_DIR/$EDITED_FILE"
if [[ "$REL_PATH" == */references/* ]]; then
    REF_FILE="${REL_PATH/skills\//skills-codex/}"
    CODEX_COUNTERPART="$ROOT/$REF_FILE"
fi

# Build warning message
WARNING="Codex parity: skills-codex/$SKILL_NAME/ exists and may need sync."
if [[ -f "$CODEX_COUNTERPART" ]]; then
    WARNING="$WARNING File exists at both locations — copy changes after editing."
fi
WARNING="$WARNING Run: scripts/regen-codex-hashes.sh after sync."

cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"$WARNING"}}
EOF
