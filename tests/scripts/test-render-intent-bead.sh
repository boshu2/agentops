#!/usr/bin/env bash
# practices: [bdd-gherkin, tdd]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

TEXT_OUT="$TMP_DIR/intent.txt"
JSON_OBJ="$TMP_DIR/intent.json"
JSON_ARRAY="$TMP_DIR/intent-array.json"

"$ROOT/scripts/render-intent-bead.sh" \
  --title "Loop shape helper smoke" \
  --context bc-loop \
  --scenario "Generated bead passes loop shape" \
  --given "an operator has a non-trivial loop change" \
  --when "the intent-bead helper renders a dry-run body" \
  --then "the body contains BDD, bounded context, slice, and proof evidence" \
  --slice "Render one compliant intent bead body" \
  --proof "bash scripts/check-loop-shape.sh --self-test" \
  --write-scope "scripts/render-intent-bead.sh tests/scripts/test-render-intent-bead.sh" \
  >"$TEXT_OUT"

grep -q "Labels: non-trivial,bc-loop,operating-loop,bdd,ddd,xp" "$TEXT_OUT"
grep -q "Given an operator has a non-trivial loop change" "$TEXT_OUT"
grep -q "First failing proof: bash scripts/check-loop-shape.sh --self-test" "$TEXT_OUT"

"$ROOT/scripts/render-intent-bead.sh" \
  --json \
  --title "Loop shape helper smoke" \
  --context bc-loop \
  --scenario "Generated bead passes loop shape" \
  --given "an operator has a non-trivial loop change" \
  --when "the intent-bead helper renders a dry-run body" \
  --then "the body contains BDD, bounded context, slice, and proof evidence" \
  --slice "Render one compliant intent bead body" \
  --proof "bash scripts/check-loop-shape.sh --self-test" \
  --write-scope "scripts/render-intent-bead.sh tests/scripts/test-render-intent-bead.sh" \
  >"$JSON_OBJ"

jq -e '.labels | index("non-trivial") and index("bc-loop")' "$JSON_OBJ" >/dev/null
jq -s '.' "$JSON_OBJ" >"$JSON_ARRAY"
bash "$ROOT/scripts/check-loop-shape.sh" --strict --json "$JSON_ARRAY" >/dev/null

echo "test-render-intent-bead: PASS"
