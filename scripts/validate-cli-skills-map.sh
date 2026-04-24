#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MAP_PATH="${CLI_SKILLS_MAP_PATH:-$REPO_ROOT/docs/cli-skills-map.md}"
COMMANDS_PATH="${CLI_COMMANDS_PATH:-$REPO_ROOT/cli/docs/COMMANDS.md}"

errors=0

fail() {
  echo "CLI_SKILLS_MAP: $*"
  errors=$((errors + 1))
}

if [[ ! -f "$MAP_PATH" ]]; then
  fail "map not found: $MAP_PATH"
fi

if [[ ! -f "$COMMANDS_PATH" ]]; then
  fail "CLI reference not found: $COMMANDS_PATH"
fi

if [[ "$errors" -eq 0 ]]; then
  generated_count="$(grep -Ec '^### `ao ' "$COMMANDS_PATH" || true)"
  declared_count="$(sed -nE 's/.* ([0-9]+) generated CLI command headings.*/\1/p' "$MAP_PATH" | head -n 1)"

  if [[ -z "$declared_count" ]]; then
    fail "top audit line must declare '<N> generated CLI command headings'"
  elif [[ "$declared_count" != "$generated_count" ]]; then
    fail "declared generated CLI command headings=$declared_count, cli/docs/COMMANDS.md has $generated_count"
  fi

  if grep -Fq 'tests/rpi-e2e/run-full-rpi.sh' "$MAP_PATH"; then
    fail "map references removed tests/rpi-e2e/run-full-rpi.sh"
  fi

  if grep -Fq '`ao gate check`' "$MAP_PATH"; then
    fail "map still lists phantom subcommand ao gate check"
  fi

  if grep -Fq '`ao forge index`' "$MAP_PATH"; then
    fail "map still lists phantom subcommand ao forge index"
  fi

  for hook in session-start.sh ao-inject.sh; do
    if ! awk -v hook="$hook" '
      /^## Hooks → Commands/ { in_hooks = 1; next }
      in_hooks && /^---$/ { exit }
      in_hooks && index($0, hook) && index($0, "SessionStart") { found = 1 }
      END { exit found ? 0 : 1 }
    ' "$MAP_PATH"; then
      fail "SessionStart hook table must include $hook"
    fi
  done
fi

if [[ "$errors" -gt 0 ]]; then
  exit 1
fi

echo "CLI_SKILLS_MAP: PASS (generated CLI command headings: $generated_count)"
