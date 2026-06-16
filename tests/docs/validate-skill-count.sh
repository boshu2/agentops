#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

errors=0
missing_patterns=0
MISSING_PATTERN_MODE="${SKILL_COUNT_MISSING_MODE:-fail}"

case "$MISSING_PATTERN_MODE" in
  fail|warn)
    ;;
  *)
    echo "ERROR: SKILL_COUNT_MISSING_MODE must be 'fail' or 'warn' (got '$MISSING_PATTERN_MODE')"
    exit 2
    ;;
esac

# Helper: extract a number from a pattern in a file.
# Usage: extract_number "sed-substitution-with-capture-group" file "label"
# Returns the first captured group (number) or NOT_FOUND and records missing patterns.
extract_number() {
  local pattern="$1"
  local file="$2"
  local label="$3"
  local result

  result=$(sed -n "${pattern}p" "$file" | head -1)
  if [[ -z "$result" ]]; then
    echo "MISSING_PATTERN: $label ($file)" >&2
    missing_patterns=$((missing_patterns + 1))
    if [[ "$MISSING_PATTERN_MODE" == "fail" ]]; then
      errors=$((errors + 1))
    fi
    echo "NOT_FOUND"
    return
  fi

  echo "$result"
}

check_numeric_match() {
  local label="$1"
  local claim="$2"
  local expected="$3"

  if [[ "$claim" == "NOT_FOUND" ]]; then
    echo "MISSING_REQUIRED: $label"
    missing_patterns=$((missing_patterns + 1))
    if [[ "$MISSING_PATTERN_MODE" == "fail" ]]; then
      errors=$((errors + 1))
    fi
    return
  fi

  if [[ "$claim" -ne "$expected" ]]; then
    echo "MISMATCH: $label says $claim, expected $expected"
    errors=$((errors + 1))
  fi
}

# --- Actual counts from disk ---

# `-not -name '_*'` excludes non-skill scaffolding (e.g. skills/_fixtures/,
# planted test fixtures) — they are not real skills and must not be counted.
actual_total=$(find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d -not -name '.*' -not -name '_*' | wc -l | tr -d ' ')
actual_codex_total=$(find "$REPO_ROOT/skills-codex" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | wc -l | tr -d ' ')
actual_codex_overrides=$(find "$REPO_ROOT/skills-codex-overrides" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | wc -l | tr -d ' ')

# Count skills listed in SKILL-TIERS.md user-facing table.
actual_user_facing=$(sed -n '/^### User-Facing Skills/,/^### Internal Skills/p' "$REPO_ROOT/skills/SKILL-TIERS.md" \
  | grep -c '^| \*\*')

# Count skills listed in SKILL-TIERS.md internal table.
actual_internal=$(sed -n '/^### Internal Skills/,/^---$/p' "$REPO_ROOT/skills/SKILL-TIERS.md" \
  | grep -c '^| ')
actual_internal=$((actual_internal - 1))

echo "=== Actual counts from disk ==="
echo "  Skill directories: $actual_total"
echo "  Codex skill directories: $actual_codex_total"
echo "  Codex override directories: $actual_codex_overrides"
echo "  SKILL-TIERS.md user-facing table rows: $actual_user_facing"
echo "  SKILL-TIERS.md internal table rows: $actual_internal"
echo "  Table total: $((actual_user_facing + actual_internal))"
echo ""

# --- Consistency: table rows vs directory count ---

table_total=$((actual_user_facing + actual_internal))
if [[ "$table_total" -ne "$actual_total" ]]; then
  echo "MISMATCH: SKILL-TIERS.md tables list $table_total skills, actual directories: $actual_total"
  errors=$((errors + 1))
fi

# --- Extract claimed counts from SKILL-TIERS.md headers ---

tiers_user_claim=$(extract_number 's/.*### User-Facing Skills (\([0-9][0-9]*\)).*/\1/' "$REPO_ROOT/skills/SKILL-TIERS.md" "SKILL-TIERS user-facing header")
tiers_internal_claim=$(extract_number 's/.*### Internal Skills (\([0-9][0-9]*\)).*/\1/' "$REPO_ROOT/skills/SKILL-TIERS.md" "SKILL-TIERS internal header")

echo "=== SKILL-TIERS.md header claims ==="
echo "  User-facing claim: $tiers_user_claim"
echo "  Internal claim: $tiers_internal_claim"
echo ""

check_numeric_match "SKILL-TIERS.md user-facing header" "$tiers_user_claim" "$actual_user_facing"
check_numeric_match "SKILL-TIERS.md internal header" "$tiers_internal_claim" "$actual_internal"

# --- Extract counts from PRODUCT.md ---

product_distribution_shared=$(extract_number 's|.*Distribution/runtime reach: \([0-9][0-9]*\) shared skills, [0-9][0-9]* checked-in Codex artifacts, and [0-9][0-9]* Codex overrides.*|\1|' "$REPO_ROOT/PRODUCT.md" "PRODUCT.md distribution shared skill count")
product_distribution_codex=$(extract_number 's|.*Distribution/runtime reach: [0-9][0-9]* shared skills, \([0-9][0-9]*\) checked-in Codex artifacts, and [0-9][0-9]* Codex overrides.*|\1|' "$REPO_ROOT/PRODUCT.md" "PRODUCT.md distribution Codex artifact count")
product_distribution_overrides=$(extract_number 's|.*Distribution/runtime reach: [0-9][0-9]* shared skills, [0-9][0-9]* checked-in Codex artifacts, and \([0-9][0-9]*\) Codex overrides.*|\1|' "$REPO_ROOT/PRODUCT.md" "PRODUCT.md distribution Codex override count")

echo "=== PRODUCT.md claims ==="
echo "  Distribution shared skills: $product_distribution_shared"
echo "  Distribution Codex artifacts: $product_distribution_codex"
echo "  Distribution Codex overrides: $product_distribution_overrides"
echo ""

check_numeric_match "PRODUCT.md distribution shared skill count" "$product_distribution_shared" "$actual_total"
check_numeric_match "PRODUCT.md distribution Codex artifact count" "$product_distribution_codex" "$actual_codex_total"
check_numeric_match "PRODUCT.md distribution Codex override count" "$product_distribution_overrides" "$actual_codex_overrides"

# --- Cross-file consistency ---

echo "=== Cross-file consistency ==="

totals=()
users=()
internals=()

[[ "$tiers_user_claim" != "NOT_FOUND" && "$tiers_internal_claim" != "NOT_FOUND" ]] && totals+=("SKILL-TIERS-headers:$((tiers_user_claim + tiers_internal_claim))")
[[ "$product_distribution_shared" != "NOT_FOUND" ]] && totals+=("PRODUCT-distribution-shared:$product_distribution_shared")

[[ "$tiers_user_claim" != "NOT_FOUND" ]] && users+=("SKILL-TIERS:$tiers_user_claim")

[[ "$tiers_internal_claim" != "NOT_FOUND" ]] && internals+=("SKILL-TIERS:$tiers_internal_claim")

if [[ ${#totals[@]} -gt 1 ]]; then
  first_val="${totals[0]#*:}"
  for entry in "${totals[@]:1}"; do
    val="${entry#*:}"
    src="${entry%%:*}"
    if [[ "$val" -ne "$first_val" ]]; then
      echo "MISMATCH: Cross-file total disagreement: ${totals[0]} vs $src:$val"
      errors=$((errors + 1))
    fi
  done
fi

if [[ ${#users[@]} -gt 1 ]]; then
  first_val="${users[0]#*:}"
  for entry in "${users[@]:1}"; do
    val="${entry#*:}"
    src="${entry%%:*}"
    if [[ "$val" -ne "$first_val" ]]; then
      echo "MISMATCH: Cross-file user-facing disagreement: ${users[0]} vs $src:$val"
      errors=$((errors + 1))
    fi
  done
fi

if [[ ${#internals[@]} -gt 1 ]]; then
  first_val="${internals[0]#*:}"
  for entry in "${internals[@]:1}"; do
    val="${entry#*:}"
    src="${entry%%:*}"
    if [[ "$val" -ne "$first_val" ]]; then
      echo "MISMATCH: Cross-file internal disagreement: ${internals[0]} vs $src:$val"
      errors=$((errors + 1))
    fi
  done
fi

echo ""

# --- Summary ---

if [[ "$missing_patterns" -gt 0 ]]; then
  if [[ "$MISSING_PATTERN_MODE" == "fail" ]]; then
    echo "FAIL-CLOSED: $missing_patterns required extraction pattern(s) are missing."
    echo "Migration note: temporarily set SKILL_COUNT_MISSING_MODE=warn while updating patterns."
  else
    echo "WARN: $missing_patterns extraction pattern(s) are missing."
    echo "Migration note: set SKILL_COUNT_MISSING_MODE=fail to enforce fail-closed behavior."
  fi
  echo ""
fi

if [[ "$errors" -gt 0 ]]; then
  echo "FAIL: $errors mismatch(es) found"
  exit 1
else
  if [[ "$missing_patterns" -gt 0 ]]; then
    echo "PASS (WARN): Skill counts consistent but missing patterns were tolerated"
  else
    echo "PASS: All skill counts consistent (total=$actual_total, user-facing=$actual_user_facing, internal=$actual_internal)"
  fi
  exit 0
fi
