#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0
check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

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

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
