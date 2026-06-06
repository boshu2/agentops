#!/usr/bin/env bash
set -euo pipefail

# Synchronize the hard-coded CLI heading counts in the smoke script and
# canary matrix JSON with the actual counts in cli/docs/COMMANDS.md.
#
# Usage:
#   scripts/update-cli-surface-counts.sh          # Check; exit 1 if drifted
#   scripts/update-cli-surface-counts.sh --fix    # Rewrite counts in-place

FIX=false
if [[ "${1:-}" == "--fix" ]]; then
  FIX=true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMMANDS_PATH="$REPO_ROOT/cli/docs/COMMANDS.md"
SMOKE_PATH="$REPO_ROOT/evals/agentops-core/fixtures/cli-command-surface-smoke.sh"
MATRIX_PATH="$REPO_ROOT/evals/agentops-core/cli-command-surface-matrix.json"

errors=0

fail() {
  printf 'CLI_SURFACE_COUNTS: FAIL %s\n' "$*" >&2
  errors=$((errors + 1))
}

if [[ ! -f "$COMMANDS_PATH" ]]; then
  fail "COMMANDS.md not found: $COMMANDS_PATH"
  exit 1
fi

actual_top="$(grep -Ec '^### `ao ' "$COMMANDS_PATH" || true)"
actual_sub="$(grep -Ec '^#### `ao ' "$COMMANDS_PATH" || true)"
actual_all="$(grep -Ec '^#{3,4} `ao ' "$COMMANDS_PATH" || true)"

smoke_line="$(grep -E '^\s*if \[\[ "\$top_count"' "$SMOKE_PATH" 2>/dev/null || true)"
smoke_top="$(echo "$smoke_line" | sed -nE 's/.*"\$top_count" != "([0-9]+)".*/\1/p')"
smoke_sub="$(echo "$smoke_line" | sed -nE 's/.*"\$sub_count" != "([0-9]+)".*/\1/p')"
smoke_all="$(echo "$smoke_line" | sed -nE 's/.*"\$all_count" != "([0-9]+)".*/\1/p')"

smoke_array_line="$(grep -E '^\s*if \[\[ "\$\{#commands' "$SMOKE_PATH" 2>/dev/null || true)"
smoke_array_count="$(echo "$smoke_array_line" | sed -nE 's/.*-ne ([0-9]+).*/\1/p')"

matrix_line="$(grep -o 'cli-command-headings: top=[0-9]* sub=[0-9]* all=[0-9]*' "$MATRIX_PATH" 2>/dev/null || true)"
matrix_top="$(echo "$matrix_line" | sed -nE 's/.*top=([0-9]+).*/\1/p')"
matrix_sub="$(echo "$matrix_line" | sed -nE 's/.*sub=([0-9]+).*/\1/p')"
matrix_all="$(echo "$matrix_line" | sed -nE 's/.*all=([0-9]+).*/\1/p')"

smoke_drifted=false
if [[ "$smoke_top" != "$actual_top" || "$smoke_sub" != "$actual_sub" || "$smoke_all" != "$actual_all" ]]; then
  smoke_drifted=true
fi
if [[ -n "$smoke_array_count" && "$smoke_array_count" != "$actual_all" ]]; then
  smoke_drifted=true
fi

if $smoke_drifted; then
  if $FIX; then
    new_assert="if [[ \"\$top_count\" != \"$actual_top\" || \"\$sub_count\" != \"$actual_sub\" || \"\$all_count\" != \"$actual_all\" ]]; then"
    old_assert="if [[ \"\$top_count\" != \"$smoke_top\" || \"\$sub_count\" != \"$smoke_sub\" || \"\$all_count\" != \"${smoke_all:-$actual_all}\" ]]; then"
    tmp="$(mktemp)"
    awk -v old="$old_assert" -v new="$new_assert" \
        -v old_ne="${smoke_array_count:-$actual_all}" -v new_ne="$actual_all" \
        '{
          if ($0 ~ /top_count.*!=.*sub_count.*!=.*all_count/) { print new }
          else if ($0 ~ /-ne [0-9]+/) { gsub("-ne " old_ne, "-ne " new_ne); print }
          else { print }
        }' "$SMOKE_PATH" > "$tmp"
    mv "$tmp" "$SMOKE_PATH"
    chmod +x "$SMOKE_PATH"
    printf 'CLI_SURFACE_COUNTS: --fix updated smoke script -> top=%s sub=%s all=%s\n' \
      "$actual_top" "$actual_sub" "$actual_all"
  else
    fail "smoke script counts drifted: expected top=$actual_top sub=$actual_sub all=$actual_all, got top=$smoke_top sub=$smoke_sub all=${smoke_all:-?}"
  fi
fi

if [[ "$matrix_top" != "$actual_top" || "$matrix_sub" != "$actual_sub" || "$matrix_all" != "$actual_all" ]]; then
  if $FIX; then
    tmp="$(mktemp)"
    sed "s/cli-command-headings: top=$matrix_top sub=$matrix_sub all=$matrix_all/cli-command-headings: top=$actual_top sub=$actual_sub all=$actual_all/" \
      "$MATRIX_PATH" > "$tmp"
    mv "$tmp" "$MATRIX_PATH"
    printf 'CLI_SURFACE_COUNTS: --fix updated matrix JSON -> top=%s sub=%s all=%s\n' \
      "$actual_top" "$actual_sub" "$actual_all"
  else
    fail "matrix JSON counts drifted: expected top=$actual_top sub=$actual_sub all=$actual_all, got top=$matrix_top sub=$matrix_sub all=$matrix_all"
  fi
fi

if [[ "$errors" -gt 0 ]]; then
  printf 'CLI_SURFACE_COUNTS: %d drift(s) detected. Run with --fix to update.\n' "$errors" >&2
  exit 1
fi

printf 'CLI_SURFACE_COUNTS: PASS (top=%s sub=%s all=%s)\n' "$actual_top" "$actual_sub" "$actual_all"
