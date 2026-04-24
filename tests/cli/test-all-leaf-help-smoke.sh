#!/usr/bin/env bash
# test-all-leaf-help-smoke.sh - Execute --help for every generated ao leaf command.
#
# This is the broadest safe CLI smoke surface: every leaf command listed in
# cli/docs/COMMANDS.md must be runnable as `ao <leaf> --help` without panics,
# empty output, or Cobra help regressions.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMMANDS_MD="$REPO_ROOT/cli/docs/COMMANDS.md"
AO="$REPO_ROOT/cli/bin/ao"
SKIP_BUILD=false
TIMEOUT_SECONDS="${AO_LEAF_HELP_TIMEOUT:-10}"

usage() {
  cat <<USAGE
Usage: bash tests/cli/test-all-leaf-help-smoke.sh [--skip-build] [--binary <path>]

Options:
  --skip-build     Use the existing ao binary.
  --binary <path>  Test this ao binary instead of cli/bin/ao.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    --binary)
      if [[ $# -lt 2 ]]; then
        echo "--binary requires a path" >&2
        usage >&2
        exit 2
      fi
      AO="$2"
      SKIP_BUILD=true
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$COMMANDS_MD" ]]; then
  echo "FATAL: command reference not found: $COMMANDS_MD" >&2
  exit 2
fi

if [[ "$SKIP_BUILD" != true ]]; then
  (cd "$REPO_ROOT/cli" && make build >/dev/null)
fi

if [[ ! -x "$AO" ]]; then
  echo "FATAL: ao binary not executable: $AO" >&2
  exit 2
fi

mapfile -t LEAF_COMMANDS < <(python3 - "$COMMANDS_MD" <<'PY'
import re
import sys
from pathlib import Path

commands = set()
for line in Path(sys.argv[1]).read_text().splitlines():
    match = re.match(r"^#{3,5} `ao ([^`]+)`", line)
    if match:
        commands.add(match.group(1).strip())

for command in sorted(commands):
    if not any(other != command and other.startswith(command + " ") for other in commands):
        print(command)
PY
)

if [[ "${#LEAF_COMMANDS[@]}" -eq 0 ]]; then
  echo "FATAL: no leaf commands parsed from $COMMANDS_MD" >&2
  exit 1
fi

run_with_timeout() {
  if command -v timeout >/dev/null 2>&1; then
    timeout "$TIMEOUT_SECONDS" "$@"
  else
    "$@"
  fi
}

failures=0
passed=0

check_output_for_crash() {
  local label="$1"
  local output="$2"
  if grep -qE '(^panic:|runtime error:|^goroutine [0-9]+ \[)' <<<"$output"; then
    echo "FAIL $label - panic/crash marker found"
    grep -E '(^panic:|runtime error:|^goroutine [0-9]+ \[)' <<<"$output" | head -5 | sed 's/^/  /'
    return 1
  fi
  return 0
}

check_help_output() {
  local label="$1"
  local output="$2"
  if [[ -z "$(printf '%s' "$output" | tr -d '[:space:]')" ]]; then
    echo "FAIL $label - empty help output"
    return 1
  fi
  if ! grep -qiE '(Usage:|usage:|Available Commands:|Flags:)' <<<"$output"; then
    echo "FAIL $label - help output missing Usage/Commands/Flags"
    printf '%s\n' "$output" | head -8 | sed 's/^/  /'
    return 1
  fi
  return 0
}

echo "=== ao leaf help smoke ==="
echo "Binary: $AO"
echo "Leaf commands: ${#LEAF_COMMANDS[@]}"

for leaf in "${LEAF_COMMANDS[@]}"; do
  read -r -a args <<<"$leaf"
  label="ao $leaf --help"
  output=""
  rc=0
  output="$(run_with_timeout "$AO" "${args[@]}" --help 2>&1)" || rc=$?

  if ! check_output_for_crash "$label" "$output"; then
    failures=$((failures + 1))
    continue
  fi
  if [[ "$rc" -ne 0 ]]; then
    echo "FAIL $label - exit $rc"
    printf '%s\n' "$output" | head -8 | sed 's/^/  /'
    failures=$((failures + 1))
    continue
  fi
  if ! check_help_output "$label" "$output"; then
    failures=$((failures + 1))
    continue
  fi
  passed=$((passed + 1))
done

echo "Passed: $passed  Failed: $failures"
exit "$((failures > 0 ? 1 : 0))"
