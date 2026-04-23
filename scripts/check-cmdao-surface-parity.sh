#!/usr/bin/env bash
# check-cmdao-surface-parity.sh
#
# Command-surface parity gate for the ao CLI.
#
# Extracts leaf commands from cli/docs/COMMANDS.md and checks whether each is
# referenced in scripts/release-smoke-test.sh or cli/cmd/ao/*_test.go files.
# Commands intentionally excluded from coverage are listed in the allowlist.
#
# Usage:
#   bash scripts/check-cmdao-surface-parity.sh
#
# Exit codes:
#   0 = all leaf commands are covered or allowlisted
#   1 = one or more leaf commands are uncovered and not allowlisted
#
# Environment overrides (for testing):
#   CMDAO_COMMANDS_MD    Path to CLI reference (default: cli/docs/COMMANDS.md)
#   CMDAO_SMOKE_TEST     Path to smoke test (default: scripts/release-smoke-test.sh)
#   CMDAO_TEST_GLOB      Glob for unit test files (default: cli/cmd/ao/*_test.go)
#   CMDAO_ALLOWLIST      Path to allowlist (default: scripts/cmdao-surface-allowlist.txt)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

COMMANDS_MD="${CMDAO_COMMANDS_MD:-$REPO_ROOT/cli/docs/COMMANDS.md}"
SMOKE_TEST="${CMDAO_SMOKE_TEST:-$REPO_ROOT/scripts/release-smoke-test.sh}"
TEST_GLOB_DIR="${CMDAO_TEST_GLOB_DIR:-$REPO_ROOT/cli/cmd/ao}"
ALLOWLIST="${CMDAO_ALLOWLIST:-$REPO_ROOT/scripts/cmdao-surface-allowlist.txt}"

# ─── Validate inputs ───────────────────────────────────────────────────────────

if [[ ! -f "$COMMANDS_MD" ]]; then
  echo "CMDAO_SURFACE_PARITY: COMMANDS_MD not found: $COMMANDS_MD"
  exit 1
fi

if [[ ! -f "$SMOKE_TEST" ]]; then
  echo "CMDAO_SURFACE_PARITY: smoke test not found: $SMOKE_TEST"
  exit 1
fi

if [[ ! -d "$TEST_GLOB_DIR" ]]; then
  echo "CMDAO_SURFACE_PARITY: test directory not found: $TEST_GLOB_DIR"
  exit 1
fi

if [[ ! -f "$ALLOWLIST" ]]; then
  echo "CMDAO_SURFACE_PARITY: allowlist not found: $ALLOWLIST"
  exit 1
fi

# ─── Temp workspace ────────────────────────────────────────────────────────────

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ALL_COMMANDS_FILE="$TMP_DIR/all_commands.txt"
LEAF_COMMANDS_FILE="$TMP_DIR/leaf_commands.txt"
PARENT_COMMANDS_FILE="$TMP_DIR/parent_commands.txt"
ALLOWLIST_FILE="$TMP_DIR/allowlist.txt"
UNCOVERED_FILE="$TMP_DIR/uncovered.txt"

# ─── Step 1: Extract all commands from COMMANDS.md ────────────────────────────
#
# COMMANDS.md uses heading levels to indicate command hierarchy:
#   ### `ao <cmd>`          — top-level command
#   #### `ao <cmd> <sub>`   — subcommand (one level deep)
#   ##### `ao <cmd> <sub> <leaf>` — nested subcommand (two levels deep)
#
# We extract the full command string (without backticks and "ao" prefix) from
# every heading line that starts with ### / #### / #####.

extract_commands() {
  local file="$1"
  # Match heading lines with backtick-quoted `ao ...` commands, strip punctuation
  grep -E '^#{3,5} `ao [^`]+`' "$file" \
    | sed -E 's/^#{3,5} `ao ([^`]+)`.*$/\1/' \
    | sed -E 's/[[:space:]]+$//' \
    | grep -v '^$' \
    | sort -u
}

extract_commands "$COMMANDS_MD" > "$ALL_COMMANDS_FILE"

if [[ ! -s "$ALL_COMMANDS_FILE" ]]; then
  echo "CMDAO_SURFACE_PARITY: no commands parsed from $COMMANDS_MD"
  echo "Expected heading style: ### \`ao <cmd>\` or #### \`ao <cmd> <sub>\`"
  exit 1
fi

# ─── Step 2: Identify leaf vs. group-parent commands ─────────────────────────
#
# A command is a PARENT (group, not a leaf) if any other command starts with
# it followed by a space. All others are leaves.

while IFS= read -r cmd; do
  is_parent=false
  while IFS= read -r other; do
    if [[ "$other" == "$cmd "* ]]; then
      is_parent=true
      break
    fi
  done < "$ALL_COMMANDS_FILE"
  if $is_parent; then
    echo "$cmd"
  fi
done < "$ALL_COMMANDS_FILE" | sort -u > "$PARENT_COMMANDS_FILE"

comm -23 \
  <(sort "$ALL_COMMANDS_FILE") \
  <(sort "$PARENT_COMMANDS_FILE") \
  > "$LEAF_COMMANDS_FILE"

LEAF_COUNT=$(wc -l < "$LEAF_COMMANDS_FILE" | tr -d ' ')

if [[ "$LEAF_COUNT" -eq 0 ]]; then
  echo "CMDAO_SURFACE_PARITY: no leaf commands found — check COMMANDS.md structure"
  exit 1
fi

# ─── Step 3: Load the allowlist ───────────────────────────────────────────────
#
# Strip comment lines (# ...) and blank lines.

grep -Ev '^[[:space:]]*(#|$)' "$ALLOWLIST" \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
  | sort -u \
  > "$ALLOWLIST_FILE" || true

# ─── Step 4: Check each leaf command for coverage ─────────────────────────────
#
# A leaf command is "covered" if:
#   (a) Its canonical form appears in scripts/release-smoke-test.sh, OR
#   (b) Its canonical form appears in any cli/cmd/ao/*_test.go file.
#
# The canonical form is "ao <subcommand>" (e.g., "ao rpi status").
# We search for the quoted form `ao <cmd>` and the unquoted form as well.

errors=0
missing=()

while IFS= read -r cmd; do
  # Check allowlist first
  if grep -qxF "$cmd" "$ALLOWLIST_FILE" 2>/dev/null; then
    continue
  fi

  # Build search patterns: the leaf subcommand portion (last word(s) after first token)
  # e.g., "rpi status" → search for "ao rpi status", "rpi status", '"rpi status"'
  full_cmd="ao $cmd"

  covered=false

  # Search smoke test
  if grep -qF "$full_cmd" "$SMOKE_TEST" 2>/dev/null; then
    covered=true
  fi

  # Search *_test.go files (only if not already found)
  if ! $covered; then
    if grep -rlF "$full_cmd" "$TEST_GLOB_DIR/"*_test.go >/dev/null 2>/dev/null; then
      covered=true
    fi
  fi

  # Also search for the subcommand path without "ao " prefix (e.g., "rpi status")
  # Some tests reference subcommand names without the binary prefix
  if ! $covered; then
    # Only search test files for bare subcommand if it's a multi-word command
    # (single-word commands have too many false positives)
    if [[ "$cmd" == *" "* ]]; then
      if grep -rlF "$cmd" "$TEST_GLOB_DIR/"*_test.go >/dev/null 2>/dev/null; then
        covered=true
      fi
    fi
  fi

  # Dedicated test file heuristic: a file named after the command (with spaces
  # and hyphens normalized to underscores) implies coverage even if the
  # "ao <cmd>" string literal never appears. Examples:
  #   retrieval-bench → retrieval_bench_test.go / retrieval_bench_*_test.go
  #   handoff         → handoff_test.go
  #   rpi serve       → rpi_serve_test.go
  if ! $covered; then
    basename_pat=$(printf '%s' "$cmd" | tr ' -' '__')
    # Use `find` so missing patterns don't fail under `set -euo pipefail`.
    if [[ -f "$TEST_GLOB_DIR/${basename_pat}_test.go" ]] \
       || find "$TEST_GLOB_DIR" -maxdepth 1 -name "${basename_pat}_*_test.go" -print -quit 2>/dev/null | grep -q .; then
      covered=true
    fi
  fi

  # executeCommand / Cobra SetArgs heuristic: Go tests often invoke subcommands
  # as comma-separated string args (e.g., executeCommand("scenario", "list"))
  # rather than as the literal "ao scenario list" token the earlier checks look
  # for. Detect this pattern for multi-word commands.
  if ! $covered && [[ "$cmd" == *" "* ]]; then
    # Build a regex like: "scenario"[[:space:]]*,[[:space:]]*"list"
    quoted_tokens=$(printf '%s' "$cmd" | awk '{for(i=1;i<=NF;i++){printf "\"%s\"", $i; if(i<NF) printf "[[:space:]]*,[[:space:]]*"}}')
    if grep -rlE "$quoted_tokens" "$TEST_GLOB_DIR"/*_test.go >/dev/null 2>/dev/null; then
      covered=true
    fi
  fi

  if ! $covered; then
    missing+=("$cmd")
    errors=$((errors + 1))
  fi

done < "$LEAF_COMMANDS_FILE"

# ─── Step 5: Report ───────────────────────────────────────────────────────────

ALLOWLIST_COUNT=$(wc -l < "$ALLOWLIST_FILE" | tr -d ' ')

echo "CMDAO_SURFACE_PARITY: $LEAF_COUNT leaf commands, $ALLOWLIST_COUNT allowlisted"

if [[ "${#missing[@]}" -gt 0 ]]; then
  echo ""
  echo "CMDAO_SURFACE_PARITY: UNCOVERED commands (not in smoke test, not in *_test.go, not allowlisted):"
  for cmd in "${missing[@]}"; do
    echo "  - $cmd"
  done
  echo ""
  echo "Action: add coverage to scripts/release-smoke-test.sh or cli/cmd/ao/*_test.go,"
  echo "        or add to scripts/cmdao-surface-allowlist.txt with a reason comment."
  echo ""
  echo "CMDAO_SURFACE_PARITY: FAILED ($errors uncovered command(s))"
  exit 1
fi

echo "CMDAO_SURFACE_PARITY: PASS (all $LEAF_COUNT leaf commands covered or allowlisted)"
exit 0
