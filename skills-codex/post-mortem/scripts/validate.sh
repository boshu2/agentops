#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: post-mortem" "grep -q '^name: post-mortem' '$SKILL_DIR/SKILL.md'"
check "references/ has at least 2 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 2 ]"
check "SKILL.md mentions harvest" "grep -qi 'harvest' '$SKILL_DIR/SKILL.md'"
check "SKILL.md has Step 2.6 (deep audit sweep)" "grep -q 'Step 2.6' '$SKILL_DIR/SKILL.md'"
check "SKILL.md references --skip-sweep" "grep -q '\-\-skip-sweep' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
