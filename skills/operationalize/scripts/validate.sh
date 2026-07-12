#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0
check() { if /bin/bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }
check_function() { if "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "name is operationalize" "grep -q '^name: operationalize' '$SKILL_DIR/SKILL.md'"
check "mentions automation-shape-routing" "grep -q 'automation-shape-routing' '$SKILL_DIR/SKILL.md'"

# Folded from inject (which had folded knowledge-activation, cp-auc): assert
# the activation capability survives each consolidation hop. This guard caught
# the same fold-regression class twice before — keep it with the content.
check "mentions ao knowledge activate" "grep -q 'ao knowledge activate' '$SKILL_DIR/SKILL.md'"
check "mentions ao knowledge gaps" "grep -q 'ao knowledge gaps' '$SKILL_DIR/SKILL.md'"
check "mentions absorbed inject trigger" "grep -qi 'absorbed from /inject' '$SKILL_DIR/SKILL.md'"
check "mentions citation recording" "grep -q 'ao metrics cite' '$SKILL_DIR/SKILL.md'"
check "activation references exist" "[ -f '$SKILL_DIR/references/knowledge-activation-dag.md' ] && [ -f '$SKILL_DIR/references/knowledge-activation-output-surfaces.md' ] && [ -f '$SKILL_DIR/references/knowledge-activation-script-contracts.md' ]"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Exact Markdown contract literals: backticks are data, never substitutions.
# shellcheck disable=SC2016
OUTPUT_MARKERS=(
  '## Output Specification'
  '**Format:** Markdown rule packet with sources-in-place, anchored "When X, do Y because Z" rules, DISPUTED entries, route table, validation evidence, and handoff stubs.'
  '**Path:** `.agents/operationalize/YYYY-MM-DD-<slug>.md` with one `## Handoffs` block per routed rule.'
  '**Filename:** `YYYY-MM-DD-<slug>.md`, where the slug names the source topic or operationalization goal.'
  '**Validation command:** run `skills/operationalize/scripts/validate.sh`, then obtain a successful [/validate](../validate/SKILL.md) verdict on the emitted packet.'
  '**Downstream handoff:** consumed only after validation by exactly one selected builder; each stub names the rule, anchors, route, target invocation, and next action.'
)

validate_output_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${OUTPUT_MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
  [[ "$(wc -l <"$file")" -le 250 ]]
}

validate_source_output_contract() {
  validate_output_contract "$SKILL_DIR/SKILL.md"
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${OUTPUT_MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL_DIR/SKILL.md" >"$variant"
    validate_output_contract "$variant" && return 1
  done
  return 0
}

check_function "output contract is all-of and kernel is within 250 lines" validate_source_output_contract
check_function "every missing output marker is rejected" delete_one_negative_fixture

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
