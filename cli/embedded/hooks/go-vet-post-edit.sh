#!/usr/bin/env bash
# PostToolUse hook: run go vet on modified Go files after edits.
# Non-blocking (exit 0) — logs warnings to stderr for visibility.
# practices: [continuous-integration, code-complete]
set -euo pipefail

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || TOOL_NAME=""
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null) || FILE_PATH=""

# Only trigger on Edit/Write
case "$TOOL_NAME" in
  Edit|Write) ;;
  *) exit 0 ;;
esac

# Only .go files
[[ "$FILE_PATH" == *.go ]] || exit 0

# Need go compiler
command -v go &>/dev/null || exit 0

# Find the package directory for the modified file
PKG_DIR="$(dirname "$FILE_PATH")"
[[ -d "$PKG_DIR" ]] || exit 0

# Run go vet on the package
VETOUT=$(cd "$PKG_DIR" && go vet ./... 2>&1) || true
if [[ -n "$VETOUT" ]]; then
  echo "go vet warning in $(basename "$PKG_DIR"):" >&2
  echo "$VETOUT" | head -20 >&2
fi

# Assertion density check for test files
if [[ "$FILE_PATH" == *_test.go ]] && [[ -f "$FILE_PATH" ]]; then
  EMPTY_TESTS=""
  while IFS= read -r func_name; do
    [[ -z "$func_name" ]] && continue
    # Extract ~50 lines after function declaration, check for assertion patterns
    if ! grep -A 50 "func ${func_name}" "$FILE_PATH" 2>/dev/null | \
         grep -qE '(t\.(Fatal|Fatalf|Error|Errorf|Fail|FailNow)|assert\.|require\.|want.*got|got.*want|expected.*actual|actual.*expected)'; then
      EMPTY_TESTS="${EMPTY_TESTS}  ${func_name}\n"
    fi
  done < <(grep -oE 'func (Test[A-Za-z0-9_]+)' "$FILE_PATH" 2>/dev/null | awk '{print $2}')

  if [[ -n "$EMPTY_TESTS" ]]; then
    DENSITY_MSG="Assertion density warning: test functions with no assertions detected:
$(printf '%b' "$EMPTY_TESTS")  These tests will pass regardless of code behavior."
    if command -v jq >/dev/null 2>&1; then
      jq -n --arg ctx "$DENSITY_MSG" '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":$ctx}}'
    else
      echo "$DENSITY_MSG" >&2
    fi
  fi
fi

exit 0
