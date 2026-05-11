#!/usr/bin/env bash
# standards-injector.sh - PreToolUse hook: inject compact language standards.
# Reads tool_input.file_path, maps extension to language, and emits bounded
# JIT guidance. Full standards stay on disk unless explicitly requested.
# practices: [code-complete, pragmatic-programmer, design-by-contract]
set -euo pipefail

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_STANDARDS_INJECTOR_DISABLED:-}" = "1" ] && exit 0

# Read all of stdin (hook pipes JSON)
INPUT=$(cat)

# Extract file_path from tool_input
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null || true)
else
    # Fallback: grep/sed extraction
    FILE_PATH=$(printf '%s' "$INPUT" | grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"file_path"[[:space:]]*:[[:space:]]*"//;s/"$//' || true)
fi

# No file path: exit silently
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

REL_STANDARDS="skills/standards/references/${LANG}.md"

compact_standards() {
    case "$LANG" in
        go)
            cat <<EOF
# Go Standards (JIT Summary)
- Run gofmt/go test for changed packages; keep errors explicit and wrapped with context.
- Prefer small, named helpers over hidden global state; keep CLI behavior covered by focused tests.
- Watch for generated/embedded propagation when hooks, CLI docs, or runtime assets change.
Full reference: ${REL_STANDARDS}
EOF
            ;;
        python)
            cat <<EOF
# Python Standards (JIT Summary)
- Prefer typed, small functions; avoid broad exception swallowing and hidden filesystem side effects.
- Use structured parsers/APIs over ad hoc string manipulation when data has a format.
- Add focused tests for changed behavior and keep scripts deterministic in CI.
Full reference: ${REL_STANDARDS}
EOF
            ;;
        typescript)
            cat <<EOF
# TypeScript Standards (JIT Summary)
- Keep types explicit at module boundaries; avoid any unless the boundary is truly dynamic.
- Preserve existing component/state patterns and add tests around user-visible behavior.
- Keep async errors observable and avoid fire-and-forget side effects.
Full reference: ${REL_STANDARDS}
EOF
            ;;
        shell)
            cat <<EOF
# Shell Standards (JIT Summary)
- Use set -euo pipefail, quote variables, and prefer arrays for command arguments.
- Fail open only for advisory hooks; blocking hooks must emit clear reasons and exit 2.
- Keep hooks fast, kill-switchable, and safe under missing tools or malformed JSON.
Full reference: ${REL_STANDARDS}
EOF
            ;;
        javascript)
            cat <<EOF
# JavaScript Standards (JIT Summary)
- Keep data flow explicit, avoid implicit globals, and handle async failures deliberately.
- Match existing module/test style before introducing abstractions.
- Validate externally visible behavior with focused tests.
Full reference: ${REL_STANDARDS}
EOF
            ;;
        yaml)
            cat <<EOF
# YAML Standards (JIT Summary)
- Preserve indentation and quote ambiguous scalar values when type coercion would be risky.
- Keep generated/config changes paired with validation commands and docs where applicable.
- Validate schemas or loaders after editing runtime manifests.
Full reference: ${REL_STANDARDS}
EOF
            ;;
    esac
}

if [ "${AGENTOPS_STANDARDS_FULL_INJECT:-}" = "1" ]; then
    CONTENT=$(cat "$STANDARDS_FILE")
else
    CONTENT=$(compact_standards)
fi

# JSON-escape the content: backslashes, quotes, newlines, tabs, carriage returns
ESCAPED=$(printf '%s' "$CONTENT" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/	/\\t/g' | awk '{if(NR>1) printf "\\n"; printf "%s", $0}')

# Output hookSpecificOutput JSON
if command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$CONTENT" '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":$ctx}}'
else
    printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"%s"}}\n' "$ESCAPED"
fi

exit 0
