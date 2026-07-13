#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DEPRECATED_COMMANDS_GO="$ROOT/cli/internal/quality/stale_refs.go"
SKILL_ROOTS=("$ROOT/skills" "$ROOT/skills-codex")

failures=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

print_matches() {
  local matches="$1"
  while IFS= read -r line; do
    printf '  %s\n' "$line" >&2
  done <<< "$matches"
}

require_path() {
  local path="$1"
  [[ -e "$path" ]] || {
    echo "Missing required path: $path" >&2
    exit 1
  }
}

require_path "$DEPRECATED_COMMANDS_GO"
for root in "${SKILL_ROOTS[@]}"; do
  require_path "$root"
done

echo "=== Skill runtime parity validation ==="

mapfile -t deprecated_commands < <(
  sed -n '/var DeprecatedCommands/,/^}/p' "$DEPRECATED_COMMANDS_GO" \
    | grep '"ao ' \
    | sed 's/.*"\(ao [^"]*\)".*:.*"\(ao [^"]*\)".*/\1|\2/' \
    | cut -d'|' -f1 \
    | sort -u
)

echo "--- Deprecated ao command scan ---"
for cmd in "${deprecated_commands[@]}"; do
  [[ -n "$cmd" ]] || continue
  if matches="$(rg -n -F "$cmd" "${SKILL_ROOTS[@]}" 2>/dev/null || true)" && [[ -n "$matches" ]]; then
    fail "deprecated command reference found: $cmd"
    print_matches "$matches"
  fi
done

echo "--- Hook install claim scan ---"
declare -a hook_patterns=(
  'all 8 events|hook coverage count is stale; current local source of truth is full 12-event coverage by default'
  'SessionStart \+ Stop|minimal hooks now install SessionStart + SessionEnd + Stop'
  'ao init --hooks --full|ao init --hooks is already the full install path; use --minimal-hooks for lightweight mode'
)

for entry in "${hook_patterns[@]}"; do
  pattern="${entry%%|*}"
  message="${entry#*|}"
  if matches="$(rg -n --pcre2 "$pattern" "${SKILL_ROOTS[@]}" 2>/dev/null || true)" && [[ -n "$matches" ]]; then
    fail "$message"
    print_matches "$matches"
  fi
done

echo "--- Summary ---"
if [[ "$failures" -gt 0 ]]; then
  echo "Skill runtime parity validation FAILED ($failures finding(s))." >&2
  exit 1
fi

echo "Skill runtime parity validation passed."
exit 0
