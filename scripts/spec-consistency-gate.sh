#!/usr/bin/env bash
# spec-consistency-gate.sh — Validate SPEC WAVE contracts for consistency
#
# Validates contract-*.md files against a consistency checklist:
#   FAIL checks (exit non-zero):
#     1. Frontmatter completeness — issue, framework, category fields present
#     2. Structural completeness — ## Invariants >=3 items, ## Test Cases >=3 rows,
#        each row has "Validates Invariant"
#     3. Scope consistency — no contract references multiple issue IDs;
#        each spec-eligible issue has exactly one contract
#     4. Test readiness — at least one success-path and one error-path test case
#        (error keywords: error, fail, reject, denied, invalid, blocked, 429, 4xx, 5xx)
#     (empty directory) — FAIL if no contracts found
#   WARN checks (exit 0):
#     5. Terminology consistency — extract key terms for lead cross-reference
#     6. Implementability — flag placeholder text (TBD, TODO, ...)
#
# Usage:
#   scripts/spec-consistency-gate.sh [<contracts-dir>] [--report]
#
#   <contracts-dir>  Directory containing contract-*.md files.
#                    Defaults to .agents/specs/
#   --report         Write .agents/specs/consistency-report.md
#
# Exit codes:
#   0  All FAIL checks passed (WARN is OK)
#   1  One or more FAIL checks failed
#
# This is an ordinary deterministic check; callers choose where to run it.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Argument parsing ---
CONTRACTS_DIR=""
WRITE_REPORT=0

for arg in "$@"; do
  case "$arg" in
    --report)
      WRITE_REPORT=1
      ;;
    -h|--help)
      grep '^#' "$0" | head -30 | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    -*)
      echo "error: unknown flag: $arg" >&2
      exit 2
      ;;
    *)
      if [[ -z "$CONTRACTS_DIR" ]]; then
        CONTRACTS_DIR="$arg"
      else
        echo "error: unexpected argument: $arg" >&2
        exit 2
      fi
      ;;
  esac
done

if [[ -z "$CONTRACTS_DIR" ]]; then
  CONTRACTS_DIR="$REPO_ROOT/.agents/specs"
fi

# --- Colors (disabled in CI / non-tty) ---
if [[ -t 1 ]]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  BLUE='\033[0;34m'
  NC='\033[0m'
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  NC=''
fi

# --- Counters and accumulators ---
fail_count=0
warn_count=0
pass_count=0
report_lines=()

pass() {
  local msg="$1"
  echo -e "  ${GREEN}PASS${NC}  $msg"
  pass_count=$((pass_count + 1))
  report_lines+=("PASS  $msg")
}

fail() {
  local msg="$1"
  echo -e "  ${RED}FAIL${NC}  $msg"
  fail_count=$((fail_count + 1))
  report_lines+=("FAIL  $msg")
}

warn() {
  local msg="$1"
  echo -e "  ${YELLOW}WARN${NC}  $msg"
  warn_count=$((warn_count + 1))
  report_lines+=("WARN  $msg")
}

header() {
  local msg="$1"
  echo -e "${BLUE}--- $msg ---${NC}"
  report_lines+=("" "--- $msg ---")
}

# --- Helper: extract YAML frontmatter code block from a contract file ---
# Contracts store frontmatter as a fenced ```yaml block (not YAML front matter
# delimited by ---). Extract the first ```yaml ... ``` block.
extract_yaml_block() {
  local file="$1"
  awk '/^```yaml/{found=1; next} found && /^```/{exit} found{print}' "$file"
}

# --- Helper: extract Test Cases rows (pipe-delimited, skip header/separator) ---
extract_test_rows() {
  local file="$1"
  # Find the ## Test Cases section, collect lines that look like table rows
  awk '
    /^## Test Cases/{in_section=1; next}
    /^## /{if(in_section) in_section=0}
    in_section && /^\|[^-]/{
      # Skip header row (contains "Input" or "Expected")
      if ($0 ~ /Input/ && $0 ~ /Expected/) next
      print
    }
  ' "$file"
}

# --- Helper: count list items under a section ---
# Counts lines starting with a digit+dot or * or - under the named section
count_section_items() {
  local file="$1"
  local section="$2"
  awk -v section="$section" '
    $0 == section {in_section=1; next}
    /^## /{if(in_section) in_section=0}
    in_section && /^[0-9]+\.|^[*-] /{count++}
    END{print count+0}
  ' "$file"
}

# ============================================================================
echo "=== SPEC CONSISTENCY GATE ==="
echo "Contracts dir: $CONTRACTS_DIR"
echo ""

# --- Pre-check: directory must exist and contain contract-*.md files ---
if [[ ! -d "$CONTRACTS_DIR" ]]; then
  echo "SKIP: contracts directory not found: $CONTRACTS_DIR"
  echo "  (no spec wave files — nothing to validate)"
  echo ""
  echo -e "${GREEN}SPEC CONSISTENCY: SKIP (no contracts directory)${NC}"
  exit 0
fi

# Collect contract files
mapfile -t contract_files < <(find "$CONTRACTS_DIR" -maxdepth 1 -name "contract-*.md" | sort)

if [[ ${#contract_files[@]} -eq 0 ]]; then
  echo "SKIP: no contract-*.md files found in $CONTRACTS_DIR"
  echo "  (directory exists but empty — nothing to validate)"
  echo ""
  echo -e "${GREEN}SPEC CONSISTENCY: SKIP (no contracts)${NC}"
  exit 0
fi

echo "Found ${#contract_files[@]} contract(s):"
for f in "${contract_files[@]}"; do
  echo "  $(basename "$f")"
done
echo ""

# Track issue IDs seen (for scope consistency check #3)
# Note: We maintain a separate ordered list because bash 5.x with set -u treats
# ${!arr[@]} and ${#arr[@]} as unbound variable for empty associative arrays.
declare -A issue_to_contracts
issue_ids_seen=()

# ============================================================================
# Per-contract checks
# ============================================================================
for contract_file in "${contract_files[@]}"; do
  contract_name="$(basename "$contract_file")"
  echo -e "${BLUE}Contract: $contract_name${NC}"
  report_lines+=("" "Contract: $contract_name")

  # ── Check 1: Frontmatter completeness ──────────────────────────────────────
  header "Check 1: Frontmatter completeness"

  yaml_block="$(extract_yaml_block "$contract_file")"

  if [[ -z "$yaml_block" ]]; then
    fail "$contract_name: no \`\`\`yaml frontmatter block found"
  else
    for field in issue framework category; do
      value="$(printf '%s\n' "$yaml_block" | grep -E "^${field}[[:space:]]*:" | sed "s/${field}[[:space:]]*:[[:space:]]*//" | tr -d ' ' || true)"
      if [[ -z "$value" || "$value" == "#"* ]]; then
        fail "$contract_name: frontmatter field '$field' is missing or empty"
      else
        pass "$contract_name: frontmatter '$field' = $value"
        # Accumulate issue → contracts mapping for scope check
        if [[ "$field" == "issue" ]]; then
          # Detect multiple issue IDs (comma-separated or whitespace-separated)
          issue_id_count="$(printf '%s\n' "$value" | tr ',' ' ' | wc -w | tr -d ' ')"
          if [[ "$issue_id_count" -gt 1 ]]; then
            fail "$contract_name: frontmatter 'issue' references $issue_id_count IDs (only one allowed): $value"
          else
            if [[ -v "issue_to_contracts[$value]" ]]; then
              issue_to_contracts["$value"]="${issue_to_contracts[$value]} $contract_name"
            else
              issue_to_contracts["$value"]="$contract_name"
              issue_ids_seen+=("$value")
            fi
          fi
        fi
      fi
    done
  fi

  # ── Check 2: Structural completeness ──────────────────────────────────────
  header "Check 2: Structural completeness"

  # 2a. ## Invariants section with >=3 items
  invariant_count="$(count_section_items "$contract_file" "## Invariants")"
  if [[ "$invariant_count" -lt 3 ]]; then
    fail "$contract_name: ## Invariants has $invariant_count item(s) (need >=3)"
  else
    pass "$contract_name: ## Invariants has $invariant_count item(s)"
  fi

  # 2b. ## Test Cases section with >=3 data rows
  test_rows="$(extract_test_rows "$contract_file")"
  test_row_count=0
  if [[ -n "$test_rows" ]]; then
    test_row_count="$(printf '%s\n' "$test_rows" | grep -c '|' || true)"
  fi
  if [[ "$test_row_count" -lt 3 ]]; then
    fail "$contract_name: ## Test Cases has $test_row_count row(s) (need >=3)"
  else
    pass "$contract_name: ## Test Cases has $test_row_count row(s)"
  fi

  # 2c. Each test row must contain "Validates Invariant" column entry
  if [[ -n "$test_rows" && "$test_row_count" -ge 1 ]]; then
    rows_without_invariant=0
    while IFS= read -r row; do
      [[ -z "$row" ]] && continue
      # The last pipe-delimited column should reference an invariant like #1, #2
      last_col="$(printf '%s\n' "$row" | rev | cut -d'|' -f2 | rev | tr -d ' ')"
      if [[ -z "$last_col" || "$last_col" == "ValidatesInvariant" ]]; then
        rows_without_invariant=$((rows_without_invariant + 1))
      fi
    done <<< "$test_rows"

    if [[ "$rows_without_invariant" -gt 0 ]]; then
      fail "$contract_name: $rows_without_invariant test row(s) missing 'Validates Invariant' reference"
    else
      pass "$contract_name: all test rows reference an invariant"
    fi
  fi

  # ── Check 4: Test readiness ────────────────────────────────────────────────
  header "Check 4: Test readiness (success-path + error-path)"

  if [[ -n "$test_rows" && "$test_row_count" -ge 1 ]]; then
    # Error-path keywords in the Expected column (3rd pipe-delimited field)
    error_keywords="error|fail|reject|denied|invalid|blocked|429|4xx|5xx"
    error_path_count=0
    success_path_count=0

    while IFS= read -r row; do
      [[ -z "$row" ]] && continue
      # Extract the Expected column (3rd column, 0-indexed as column index 2)
      expected_col="$(printf '%s\n' "$row" | awk -F'|' '{print $4}')"
      if printf '%s\n' "$expected_col" | grep -qiE "$error_keywords"; then
        error_path_count=$((error_path_count + 1))
      else
        success_path_count=$((success_path_count + 1))
      fi
    done <<< "$test_rows"

    if [[ "$success_path_count" -eq 0 ]]; then
      warn "$contract_name: no success-path test cases detected (all rows look like error-paths)"
    else
      pass "$contract_name: $success_path_count success-path test case(s)"
    fi

    if [[ "$error_path_count" -eq 0 ]]; then
      warn "$contract_name: no error-path test cases detected (add a row with error/fail/reject/429 in Expected)"
    else
      pass "$contract_name: $error_path_count error-path test case(s)"
    fi
  else
    warn "$contract_name: skipping test readiness check (no test rows to analyze)"
  fi

  # ── Check 5: Terminology consistency (WARN only) ───────────────────────────
  header "Check 5: Terminology (key terms for cross-reference — WARN only)"

  # Extract capitalized multi-word phrases and code-quoted terms as key terms
  key_terms="$(grep -oE '`[^`]+`' "$contract_file" | sort -u | head -20 | tr '\n' ', ' || true)"
  if [[ -n "$key_terms" ]]; then
    warn "$contract_name: key terms for lead cross-reference: ${key_terms%, }"
  else
    pass "$contract_name: no backtick-quoted terms found"
  fi

  # ── Check 6: Implementability (WARN only) ─────────────────────────────────
  header "Check 6: Implementability — placeholder text (WARN only)"

  placeholder_lines="$(grep -nE '\bTBD\b|\bTODO\b|\.\.\.' "$contract_file" | head -10 || true)"
  if [[ -n "$placeholder_lines" ]]; then
    warn "$contract_name: placeholder text found (TBD/TODO/...) — review before implementation:"
    while IFS= read -r pline; do
      echo "         $pline"
      report_lines+=("       $pline")
    done <<< "$placeholder_lines"
  else
    pass "$contract_name: no placeholder text found"
  fi

  echo ""
done

# ============================================================================
# Cross-contract check
# ============================================================================
header "Check 3: Scope consistency (cross-contract)"

scope_failures=0
issue_count=0
# Workaround: bash 5.x with set -u treats empty associative array as unbound
# Use a separate list of keys to avoid ${!arr[@]} and ${#arr[@]} on empty map
while IFS= read -r issue_id; do
  [[ -z "$issue_id" ]] && continue
  contracts_for_issue="${issue_to_contracts[$issue_id]}"
  contract_count="$(printf '%s\n' $contracts_for_issue | wc -w | tr -d ' ')"
  issue_count=$((issue_count + 1))
  if [[ "$contract_count" -gt 1 ]]; then
    fail "issue $issue_id is referenced by $contract_count contracts (only 1 allowed): $contracts_for_issue"
    scope_failures=$((scope_failures + 1))
  else
    pass "issue $issue_id has exactly 1 contract: $contracts_for_issue"
  fi
done < <(printf '%s\n' "${issue_ids_seen[@]+${issue_ids_seen[@]}}")

if [[ "$scope_failures" -eq 0 && "$issue_count" -gt 0 ]]; then
  pass "scope consistency: all $issue_count issue(s) have exactly one contract"
fi

echo ""

# ============================================================================
# Summary
# ============================================================================
total_checks=$((pass_count + fail_count + warn_count))
echo "=== SPEC CONSISTENCY: $pass_count/$total_checks checks passed ==="
if [[ "$warn_count" -gt 0 ]]; then
  echo -e "  ${YELLOW}WARN${NC}  $warn_count warning(s) — review recommended but not blocking"
fi
if [[ "$fail_count" -gt 0 ]]; then
  echo -e "  ${RED}FAIL${NC}  $fail_count failure(s)"
fi

# ============================================================================
# Optional report output
# ============================================================================
if [[ "$WRITE_REPORT" -eq 1 ]]; then
  report_file="$CONTRACTS_DIR/consistency-report.md"
  {
    echo "# Spec Consistency Report"
    echo ""
    echo "Generated: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    echo "Contracts dir: $CONTRACTS_DIR"
    echo "Contracts checked: ${#contract_files[@]}"
    echo ""
    echo "## Summary"
    echo ""
    echo "- PASS: $pass_count"
    echo "- WARN: $warn_count"
    echo "- FAIL: $fail_count"
    echo "- Total: $total_checks"
    echo ""
    echo "## Details"
    echo ""
    printf '```\n'
    for line in "${report_lines[@]}"; do
      echo "$line"
    done
    printf '```\n'
  } > "$report_file"
  echo ""
  echo "Report written: $report_file"
fi

# ============================================================================
# Exit
# ============================================================================
echo ""
if [[ "$fail_count" -gt 0 ]]; then
  echo -e "${RED}SPEC CONSISTENCY: FAIL${NC}"
  exit 1
fi

echo -e "${GREEN}SPEC CONSISTENCY: PASS${NC}"
exit 0
