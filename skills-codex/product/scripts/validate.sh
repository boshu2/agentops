#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
ARTIFACT_PATH=""

if [[ "${1:-}" == "--artifact" ]]; then
  [[ -n "${2:-}" && $# -eq 2 ]] || { echo "usage: $0 [--artifact <target-dir>/PRODUCT.md]" >&2; exit 2; }
  ARTIFACT_PATH="$2"
elif [[ $# -ne 0 ]]; then
  echo "usage: $0 [--artifact <target-dir>/PRODUCT.md]" >&2
  exit 2
fi

PASS=0; FAIL=0
check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

# Backticks are exact Markdown state markers, never shell substitutions.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- **Consult the pawl before raising the andon.** A plain WARN, FAIL, or REFUTED result repairs and reruns automatically; only a breaker may enter HOLD or consume the one-helper lane.'
  '## Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the product artifact and rerun the pawl; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** pause automation in HOLD and route exactly one bounded helper consultation.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, resume automatic repair, and re-earn an independent verdict before proceeding.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and surface the helper'\''s escalation to the human operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** stop automation and route directly to the human operator with the helper skipped.'
  '**Checkpoint:** before writing, confirm the user-approved mission/personas, evidence-vs-aspiration labels, honest gaps/alternatives, PMF wedge, 10-star journey, and rollback choice for an existing file. After writing, validate the resolved `<target-dir>/PRODUCT.md` path from the Output Specification.'
  '## Output Specification'
  '- **Path:** `<target-dir>/PRODUCT.md` in the user-selected target directory.'
  '- **Filename convention:** `PRODUCT.md`.'
  '- **Serialization/schema format:** Markdown with a closed leading YAML frontmatter block containing exactly one valid `last_reviewed`, followed by the canonical body section order in [references/product-frameworks.md](references/product-frameworks.md#canonical-productmd-template).'
  '- **Validator command:** resolve the target once, then run `bash skills/product/scripts/validate.sh --artifact "<target-dir>/PRODUCT.md"`; never fall back to `./PRODUCT.md`.'
  '## Quality Checklist'
  '- Evidence is dated and attributable; aspirations and pre-traction hypotheses are labeled instead of presented as measured fact.'
  '- Competitive positioning says where alternatives win, and Known Product Gaps names at least two concrete adoption or product risks.'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "product contract validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

ARTIFACT_HEADINGS=(
  '## Mission'
  '## Vision'
  '## Target Personas'
  '## PMF Wedge'
  '## 10-Star Experience'
  '## What the Product Actually Is'
  '## Core Value Propositions'
  '## Product Strategy'
  '## Design Principles'
  '## Competitive Positioning'
  '## Product Sense Review'
  '## Strategic Bet'
  '## Evidence'
  '## Known Product Gaps'
  '## Usage'
)

validate_product_artifact() {
  local file="$1" heading close_line date_line year month day max_day
  [[ "$file" == */PRODUCT.md || "$file" == PRODUCT.md ]] || return 1
  [[ -s "$file" ]] || return 1
  close_line="$(awk '
    NR == 1 { if ($0 != "---") exit 2; next }
    $0 == "---" { found=1; print NR; exit }
    END { if (!found) exit 3 }
  ' "$file")" || return 1
  [[ "$close_line" =~ ^[0-9]+$ && "$close_line" -gt 2 ]] || return 1
  [[ "$(awk -v endline="$close_line" 'NR > 1 && NR < endline && /^last_reviewed:/ { n++ } END { print n+0 }' "$file")" -eq 1 ]] || return 1
  date_line="$(awk -v endline="$close_line" 'NR > 1 && NR < endline && /^last_reviewed:/ { print }' "$file")"
  [[ "$date_line" =~ ^last_reviewed:\ ([0-9]{4})-([0-9]{2})-([0-9]{2})$ ]] || return 1
  year=$((10#${BASH_REMATCH[1]}))
  month=$((10#${BASH_REMATCH[2]}))
  day=$((10#${BASH_REMATCH[3]}))
  case "$month" in
    1|3|5|7|8|10|12) max_day=31 ;;
    4|6|9|11) max_day=30 ;;
    2)
      max_day=28
      if (( year % 400 == 0 || (year % 4 == 0 && year % 100 != 0) )); then max_day=29; fi
      ;;
    *) return 1 ;;
  esac
  (( day >= 1 && day <= max_day )) || return 1
  for heading in "${ARTIFACT_HEADINGS[@]}"; do
    awk -v endline="$close_line" 'NR > endline' "$file" | grep -Fqx -- "$heading" || return 1
  done
}

write_artifact_fixture() {
  local file="$1" mode="$2" heading
  {
    printf '%s\n' '---'
    case "$mode" in
      body-only-date) ;;
      duplicate-date) printf '%s\n' 'last_reviewed: 2026-07-12' 'last_reviewed: 2026-07-13' ;;
      invalid-date) printf '%s\n' 'last_reviewed: 2026-02-31' ;;
      *) printf '%s\n' 'last_reviewed: 2026-07-12' ;;
    esac
    if [[ "$mode" == "heading-in-frontmatter" ]]; then printf '%s\n' '## Mission'; fi
    if [[ "$mode" != "missing-close" ]]; then printf '%s\n' '---'; fi
    printf '%s\n' '# PRODUCT.md'
    if [[ "$mode" == "body-only-date" ]]; then printf '%s\n' 'last_reviewed: 2026-07-12'; fi
    for heading in "${ARTIFACT_HEADINGS[@]}"; do
      if [[ "$mode" == "heading-in-frontmatter" && "$heading" == "## Mission" ]]; then continue; fi
      printf '\n%s\nfixture\n' "$heading"
    done
  } >"$file"
}

expect_invalid_artifact() {
  local file="$1" label="$2"
  if validate_product_artifact "$file"; then
    echo "product artifact validator accepted invalid fixture: $label" >&2
    return 1
  fi
}

non_default_target_fixture() {
  local root="$TMP/repo" target="$TMP/repo/nested/PRODUCT.md"
  mkdir -p "$root/nested"
  write_artifact_fixture "$root/PRODUCT.md" valid
  printf '%s\n' '---' 'last_reviewed: 2026-07-12' '---' '# wrong nested artifact' >"$target"
  expect_invalid_artifact "$target" "nested target masked by valid repo-root PRODUCT.md"
  write_artifact_fixture "$target" valid
  validate_product_artifact "$target"
}

frontmatter_negative_fixtures() {
  local mode file
  for mode in missing-close body-only-date duplicate-date invalid-date heading-in-frontmatter; do
    file="$TMP/$mode/PRODUCT.md"
    mkdir -p "${file%/*}"
    write_artifact_fixture "$file" "$mode"
    expect_invalid_artifact "$file" "$mode"
  done
}

validate_contract "$SKILL"
delete_one_negative_fixture
non_default_target_fixture
frontmatter_negative_fixtures
if [[ -n "$ARTIFACT_PATH" ]]; then
  validate_product_artifact "$ARTIFACT_PATH"
fi

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "name is product" "grep -q '^name: product' '$SKILL_DIR/SKILL.md'"
check "mentions PRODUCT.md" "grep -q 'PRODUCT.md' '$SKILL_DIR/SKILL.md'"
check "mentions personas" "grep -qi 'personas' '$SKILL_DIR/SKILL.md'"
check "mentions value propositions" "grep -qi 'value prop' '$SKILL_DIR/SKILL.md'"
check "mentions competitive landscape" "grep -qi 'competitive' '$SKILL_DIR/SKILL.md'"
check "mentions 10-star experience" "grep -qi '10-star experience' '$SKILL_DIR/SKILL.md'"
check "mentions PMF wedge" "grep -qi 'PMF Wedge' '$SKILL_DIR/SKILL.md'"
check "mentions product sense" "grep -qi 'Product Sense' '$SKILL_DIR/SKILL.md'"
check "mentions anti-personas" "grep -qi 'Anti-personas' '$SKILL_DIR/SKILL.md'"
check "kernel stays within 250 lines" "test \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250"
check "canonical template stays referenced" "grep -q '^## Canonical PRODUCT.md Template$' '$SKILL_DIR/references/product-frameworks.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
