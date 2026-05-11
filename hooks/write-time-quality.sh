#!/usr/bin/env bash
# PostToolUse hook: lightweight code quality checks after Write/Edit.
# Non-blocking (always exit 0) — outputs warnings as JSON.
# practices: [code-complete, refactoring]
set -euo pipefail

# Kill switch
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || TOOL_NAME=""
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""' 2>/dev/null) || FILE_PATH=""

# Only trigger on Edit/Write
case "$TOOL_NAME" in
  Edit|Write) ;;
  *) exit 0 ;;
esac

# Need a real file
[[ -n "$FILE_PATH" ]] || exit 0
[[ -f "$FILE_PATH" ]] || exit 0

# Detect language from extension
EXT="${FILE_PATH##*.}"
LANG=""
case "$EXT" in
  go)          LANG="go" ;;
  py)          LANG="python" ;;
  sh|bash)     LANG="shell" ;;
  *) exit 0 ;;  # unsupported language — skip silently
esac

# Skip test files for some checks
IS_TEST=false
case "$LANG" in
  go)     [[ "$FILE_PATH" == *_test.go ]] && IS_TEST=true ;;
  python) [[ "$FILE_PATH" == *test_* ]] || [[ "$FILE_PATH" == *_test.py ]] && IS_TEST=true ;;
  shell)  [[ "$FILE_PATH" == *test* ]] && IS_TEST=true ;;
esac

WARNINGS=()

# --- Go checks ---
if [[ "$LANG" == "go" ]]; then
  # Missing error checks: lines assigning err without a nearby check
  if grep -nE '(err[[:space:]]*[:=])' "$FILE_PATH" 2>/dev/null | head -20 | while read -r line; do
    lineno=$(echo "$line" | cut -d: -f1)
    nextlines=$(sed -n "$((lineno+1)),$((lineno+3))p" "$FILE_PATH" 2>/dev/null || true)
    if ! echo "$nextlines" | grep -qE '(if[[:space:]]+err|return.*err|errors\.)'; then
      echo "found"
      break
    fi
  done | grep -q "found"; then
    WARNINGS+=("go: possible unchecked error — verify all err assignments are handled")
  fi

  # Bare fmt.Println in library code (not test, not main)
  if [[ "$IS_TEST" == "false" ]]; then
    FMT_COUNT=$(grep -cE '^[[:space:]]*fmt\.Print(ln|f)?\(' "$FILE_PATH" 2>/dev/null || true)
    if [[ "$FMT_COUNT" -gt 0 ]]; then
      if ! grep -qE '^package[[:space:]]+main' "$FILE_PATH" 2>/dev/null; then
        WARNINGS+=("go: ${FMT_COUNT} fmt.Print call(s) in library code — use structured logging")
      fi
    fi
  fi
fi

# --- Python checks ---
if [[ "$LANG" == "python" ]]; then
  # Bare except
  BARE_EXCEPT=$(grep -cE '^[[:space:]]*except[[:space:]]*:' "$FILE_PATH" 2>/dev/null || true)
  if [[ "$BARE_EXCEPT" -gt 0 ]]; then
    WARNINGS+=("python: ${BARE_EXCEPT} bare except: clause(s) — specify exception type")
  fi

  # eval/exec usage
  EVAL_COUNT=$(grep -cE '(^|[^a-zA-Z0-9_])(eval|exec)[[:space:]]*\(' "$FILE_PATH" 2>/dev/null || true)
  if [[ "$EVAL_COUNT" -gt 0 ]] && [[ "$IS_TEST" == "false" ]]; then
    WARNINGS+=("python: ${EVAL_COUNT} eval/exec call(s) — security risk with untrusted input")
  fi

  # Missing type hints on public functions (def without ->)
  if [[ "$IS_TEST" == "false" ]]; then
    MISSING_HINTS=$(grep -cE '^def[[:space:]]+[a-z][a-zA-Z0-9_]*[[:space:]]*\([^)]*\)[[:space:]]*:' "$FILE_PATH" 2>/dev/null || true)
    if [[ "$MISSING_HINTS" -gt 0 ]]; then
      WARNINGS+=("python: ${MISSING_HINTS} public function(s) without return type hint (missing ->)")
    fi
  fi
fi

# --- Shell checks ---
if [[ "$LANG" == "shell" ]]; then
  # Missing set -euo pipefail (check first 5 lines)
  HEAD=$(head -5 "$FILE_PATH" 2>/dev/null || true)
  if ! echo "$HEAD" | grep -qE 'set[[:space:]]+-euo[[:space:]]+pipefail'; then
    WARNINGS+=("shell: missing 'set -euo pipefail' in header")
  fi

  # Unquoted variables (simple heuristic — flag obvious cases)
  # Look for $VAR not inside double quotes on non-comment lines
  UNQUOTED=$(grep -nE '\$[A-Za-z_][A-Za-z0-9_]*' "$FILE_PATH" 2>/dev/null | \
    grep -vE '(^[[:space:]]*#|"\$|\{\$)' | head -5 | wc -l || true)
  UNQUOTED=$(echo "$UNQUOTED" | tr -d ' ')
  if [[ "$UNQUOTED" -gt 0 ]]; then
    WARNINGS+=("shell: possible unquoted variable(s) — use \"\$VAR\" or \"\${VAR}\"")
  fi
fi

# Output results
WARN_COUNT=${#WARNINGS[@]}
if [[ "$WARN_COUNT" -eq 0 ]]; then
  # Clean — no output needed
  exit 0
fi

# Build JSON output
if command -v jq &>/dev/null; then
  WARNINGS_JSON=$(printf '%s\n' "${WARNINGS[@]}" | jq -R . | jq -s .)
  jq -n \
    --arg event "write_time_quality" \
    --arg file "$FILE_PATH" \
    --arg lang "$LANG" \
    --argjson count "$WARN_COUNT" \
    --argjson warnings "$WARNINGS_JSON" \
    '{"hookSpecificOutput":{"hookEventName":$event,"file":$file,"language":$lang,"warning_count":$count,"warnings":$warnings}}'
else
  # Fallback: minimal JSON without jq
  echo "{\"hookSpecificOutput\":{\"hookEventName\":\"write_time_quality\",\"file\":\"${FILE_PATH}\",\"warning_count\":${WARN_COUNT}}}"
fi

# Log to stderr for visibility
echo "write-time-quality: ${WARN_COUNT} warning(s) in $(basename "$FILE_PATH"):" >&2
for w in "${WARNINGS[@]}"; do
  echo "  - $w" >&2
done

exit 0
