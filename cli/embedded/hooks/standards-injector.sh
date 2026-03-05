#!/usr/bin/env bash
# standards-injector.sh - PreToolUse hook: inject language standards as context
# Reads tool_input.file_path, maps extension to language, injects standards reference.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# Read all of stdin (hook pipes JSON)
INPUT=$(cat)

# Extract file_path from tool_input
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null)
else
    # Fallback: grep/sed extraction
    FILE_PATH=$(echo "$INPUT" | grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"file_path"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# No file path → exit silently
if [ -z "$FILE_PATH" ] || [ "$FILE_PATH" = "null" ]; then
    exit 0
fi

# Extract extension
EXT="${FILE_PATH##*.}"
# Handle no-extension case (FILE_PATH equals EXT means no dot)
if [ "$EXT" = "$FILE_PATH" ]; then
    exit 0
fi

# Map extension to language (6 entries only)
case "$EXT" in
    py)        LANG="python" ;;
    go)        LANG="go" ;;
    ts|tsx)    LANG="typescript" ;;
    sh)        LANG="shell" ;;
    js)        LANG="javascript" ;;
    yaml|yml)  LANG="yaml" ;;
    *)         exit 0 ;;
esac

# Resolve script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Read standards file (reject symlinks to prevent arbitrary file reads)
STANDARDS_FILE="$SCRIPT_DIR/../skills/standards/references/${LANG}.md"
if [ ! -f "$STANDARDS_FILE" ] || [ -L "$STANDARDS_FILE" ]; then
    exit 0
fi

# Verify resolved path is within expected directory
RESOLVED=$(cd "$(dirname "$STANDARDS_FILE")" && pwd)/$(basename "$STANDARDS_FILE")
case "$RESOLVED" in
    */skills/standards/references/*) ;; # expected location
    *) exit 0 ;;
esac

CONTENT=$(cat "$STANDARDS_FILE")

# JSON-escape the content: backslashes, quotes, newlines, tabs, carriage returns
ESCAPED=$(printf '%s' "$CONTENT" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/	/\\t/g' | awk '{if(NR>1) printf "\\n"; printf "%s", $0}')

# Output hookSpecificOutput JSON
printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"%s"}}\n' "$ESCAPED"

exit 0
