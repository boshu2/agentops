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
#   bash scripts/check-cmdao-surface-parity.sh --write-surface
#
# Exit codes:
#   0 = all leaf commands are covered or allowlisted
#   1 = one or more leaf commands are uncovered and not allowlisted
#
# Environment overrides (for testing):
#   CMDAO_COMMANDS_MD    Path to CLI reference (default: cli/docs/COMMANDS.md)
#   CMDAO_SMOKE_TEST     Path to smoke test (default: scripts/release-smoke-test.sh)
#   CMDAO_TEST_GLOB_DIR  Directory for unit test files (default: cli/cmd/ao)
#   CMDAO_ALLOWLIST      Path to allowlist (default: scripts/cmdao-surface-allowlist.txt)
#   CMDAO_SURFACE_MD     Human command-surface inventory (default: docs/cli-surface.md)
#   CMDAO_SURFACE_JSON   Machine command-surface inventory (default: docs/cli-surface.json)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WRITE_SURFACE=false
if [[ "${1:-}" == "--write-surface" ]]; then
  WRITE_SURFACE=true
elif [[ $# -gt 0 ]]; then
  echo "Unknown argument: $1" >&2
  echo "Usage: bash scripts/check-cmdao-surface-parity.sh [--write-surface]" >&2
  exit 2
fi

COMMANDS_MD="${CMDAO_COMMANDS_MD:-$REPO_ROOT/cli/docs/COMMANDS.md}"
SMOKE_TEST="${CMDAO_SMOKE_TEST:-$REPO_ROOT/scripts/release-smoke-test.sh}"
TEST_GLOB_DIR="${CMDAO_TEST_GLOB_DIR:-$REPO_ROOT/cli/cmd/ao}"
ALLOWLIST="${CMDAO_ALLOWLIST:-$REPO_ROOT/scripts/cmdao-surface-allowlist.txt}"
SURFACE_MD="${CMDAO_SURFACE_MD:-$REPO_ROOT/docs/cli-surface.md}"
SURFACE_JSON="${CMDAO_SURFACE_JSON:-$REPO_ROOT/docs/cli-surface.json}"

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
ALLOWLIST_META_FILE="$TMP_DIR/allowlist_meta.tsv"
SURFACE_TSV="$TMP_DIR/cli_surface.tsv"
GENERATED_SURFACE_MD="$TMP_DIR/cli-surface.md"
GENERATED_SURFACE_JSON="$TMP_DIR/cli-surface.json"

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
# Format: category|command|reason

VALID_CATEGORIES_RE='^(public-tested|public-stateful-fixture-needed|internal-hidden|deprecated|unsafe-live|manual-only)$'

awk -F'|' -v valid_re="$VALID_CATEGORIES_RE" '
  /^[[:space:]]*(#|$)/ { next }
  {
    if (NF != 3) {
      printf "CMDAO_SURFACE_PARITY: invalid allowlist row %d: expected category|command|reason\n", NR > "/dev/stderr"
      exit 1
    }
    category=$1
    command=$2
    reason=$3
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", category)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", command)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", reason)
    if (category !~ valid_re) {
      printf "CMDAO_SURFACE_PARITY: invalid category on row %d: %s\n", NR, category > "/dev/stderr"
      exit 1
    }
    if (command == "" || reason == "") {
      printf "CMDAO_SURFACE_PARITY: allowlist row %d needs non-empty command and reason\n", NR > "/dev/stderr"
      exit 1
    }
    printf "%s\t%s\t%s\n", command, category, reason
  }
' "$ALLOWLIST" | sort -u > "$ALLOWLIST_META_FILE"

cut -f1 "$ALLOWLIST_META_FILE" > "$ALLOWLIST_FILE" || true

printf 'command\tkind\tcategory\tcoverage_status\treason\n' > "$SURFACE_TSV"

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
    awk -F'\t' -v cmd="$cmd" '$1 == cmd { printf "%s\tleaf\t%s\tallowlisted\t%s\n", $1, $2, $3 }' "$ALLOWLIST_META_FILE" >> "$SURFACE_TSV"
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

    # Command catalog map heuristic: test files enumerate command trees as
    # `"<parent>": {"<leaf>", "<leaf>"}`. This pattern is used by command-
    # registration smoke tests in cobra_commands_test.go and similar.
    if ! $covered; then
      parent=$(printf '%s' "$cmd" | awk '{print $1}')
      leaf=$(printf '%s' "$cmd" | awk '{for(i=2;i<=NF;i++){printf "%s%s", $i, (i<NF?" ":"")}}')
      map_pattern="\"$parent\"[[:space:]]*:[[:space:]]*\\{[^}]*\"$leaf\""
      if grep -rlE "$map_pattern" "$TEST_GLOB_DIR"/*_test.go >/dev/null 2>/dev/null; then
        covered=true
      fi
    fi

    # Handler-function naming convention: `runXxxYyy` mirrors `ao xxx yyy`. The
    # Go codebase wires Cobra subcommands to handlers with this pattern, and
    # unit tests often call the handler directly rather than routing through
    # cobra.
    if ! $covered; then
      title_cased=$(printf '%s' "$cmd" | awk '{for(i=1;i<=NF;i++){s=$i; printf "%s%s", toupper(substr(s,1,1)), substr(s,2)}}')
      # Also fold hyphens: e.g., "config models" is fine but "rpi verify-final" would need hyphen→case.
      title_cased=$(printf '%s' "$title_cased" | awk 'BEGIN{FS="-"; OFS=""} {for(i=1;i<=NF;i++){s=$i; printf "%s%s", toupper(substr(s,1,1)), substr(s,2)}}')
      if grep -rlE "\\brun${title_cased}\\b" "$TEST_GLOB_DIR"/*_test.go >/dev/null 2>/dev/null; then
        covered=true
      fi
    fi
  fi

  if ! $covered; then
    missing+=("$cmd")
    errors=$((errors + 1))
    printf '%s\tleaf\tmanual-only\tmissing\tNo smoke, direct test, or allowlist coverage found.\n' "$cmd" >> "$SURFACE_TSV"
  else
    printf '%s\tleaf\tpublic-tested\tcovered\tCovered by release smoke tests, direct command tests, or command handler tests.\n' "$cmd" >> "$SURFACE_TSV"
  fi

done < "$LEAF_COMMANDS_FILE"

# ─── Step 5: Generate or check command-surface sidecars ───────────────────────

python3 - "$SURFACE_TSV" "$GENERATED_SURFACE_MD" "$GENERATED_SURFACE_JSON" <<'PY'
import csv
import json
import sys
from pathlib import Path

tsv_path, md_path, json_path = map(Path, sys.argv[1:4])
with tsv_path.open(newline="") as f:
    rows = list(csv.DictReader(f, delimiter="\t"))

payload = {
    "schema_version": 1,
    "generated_by": "scripts/check-cmdao-surface-parity.sh --write-surface",
    "categories": [
        "public-tested",
        "public-stateful-fixture-needed",
        "internal-hidden",
        "deprecated",
        "unsafe-live",
        "manual-only",
    ],
    "commands": rows,
}

json_path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")

lines = [
    "# ao CLI Command Surface",
    "",
    "> Generated by `scripts/check-cmdao-surface-parity.sh --write-surface`.",
    "> Do not edit command rows manually.",
    "",
    "| Command | Category | Coverage | Reason |",
    "|---------|----------|----------|--------|",
]
for row in rows:
    command = row["command"].replace("|", "\\|")
    category = row["category"].replace("|", "\\|")
    coverage = row["coverage_status"].replace("|", "\\|")
    reason = row["reason"].replace("|", "\\|")
    lines.append(f"| `ao {command}` | `{category}` | `{coverage}` | {reason} |")

md_path.write_text("\n".join(lines) + "\n")
PY

if $WRITE_SURFACE; then
  mkdir -p "$(dirname "$SURFACE_MD")" "$(dirname "$SURFACE_JSON")"
  cp "$GENERATED_SURFACE_MD" "$SURFACE_MD"
  cp "$GENERATED_SURFACE_JSON" "$SURFACE_JSON"
else
  if [[ ! -f "$SURFACE_MD" || ! -f "$SURFACE_JSON" ]]; then
    echo "CMDAO_SURFACE_PARITY: command-surface sidecars are missing."
    echo "Run: bash scripts/check-cmdao-surface-parity.sh --write-surface"
    exit 1
  fi
  if ! diff -q "$SURFACE_MD" "$GENERATED_SURFACE_MD" >/dev/null 2>&1; then
    echo "CMDAO_SURFACE_PARITY: $SURFACE_MD is out of date."
    echo "Run: bash scripts/check-cmdao-surface-parity.sh --write-surface"
    diff -u "$SURFACE_MD" "$GENERATED_SURFACE_MD" || true
    exit 1
  fi
  if ! diff -q "$SURFACE_JSON" "$GENERATED_SURFACE_JSON" >/dev/null 2>&1; then
    echo "CMDAO_SURFACE_PARITY: $SURFACE_JSON is out of date."
    echo "Run: bash scripts/check-cmdao-surface-parity.sh --write-surface"
    diff -u "$SURFACE_JSON" "$GENERATED_SURFACE_JSON" || true
    exit 1
  fi
fi

# ─── Step 6: Report ───────────────────────────────────────────────────────────

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
  echo "        or add a category|command|reason row to scripts/cmdao-surface-allowlist.txt."
  echo ""
  echo "CMDAO_SURFACE_PARITY: FAILED ($errors uncovered command(s))"
  exit 1
fi

echo "CMDAO_SURFACE_PARITY: PASS (all $LEAF_COUNT leaf commands covered or allowlisted)"
exit 0
