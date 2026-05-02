#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   scripts/validate-cli-skills-map.sh         # Validate; exit 1 if drifted
#   scripts/validate-cli-skills-map.sh --fix   # Rewrite the count line in
#                                              # docs/cli-skills-map.md to
#                                              # match the generated count.

FIX=false
if [[ "${1:-}" == "--fix" ]]; then
  FIX=true
fi

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
    if $FIX; then
      # Portable sed -i: BSD sed (macOS) needs an explicit empty arg, GNU
      # sed accepts -i alone. Use a temp-file rewrite to avoid the split.
      tmp_map="$(mktemp)"
      sed -E "s/([^0-9])${declared_count}( generated CLI command headings)/\\1${generated_count}\\2/" "$MAP_PATH" > "$tmp_map"
      mv "$tmp_map" "$MAP_PATH"
      echo "CLI_SKILLS_MAP: --fix updated declared count $declared_count -> $generated_count in ${MAP_PATH#"$REPO_ROOT"/}"
      declared_count="$generated_count"
    else
      fail "declared generated CLI command headings=$declared_count, cli/docs/COMMANDS.md has $generated_count"
    fi
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

  if ! awk -v hook="session-start.sh" '
    /^## Hooks → Commands/ { in_hooks = 1; next }
    in_hooks && /^---$/ { exit }
    in_hooks && index($0, hook) && index($0, "SessionStart") { found = 1 }
    END { exit found ? 0 : 1 }
  ' "$MAP_PATH"; then
    fail "SessionStart hook table must include session-start.sh"
  fi
fi

if [[ "$errors" -gt 0 ]]; then
  exit 1
fi

echo "CLI_SKILLS_MAP: PASS (generated CLI command headings: $generated_count)"
