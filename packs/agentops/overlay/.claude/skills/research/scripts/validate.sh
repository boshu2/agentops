#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: research" "grep -q '^name: research' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 3 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 3 ]"
check "SKILL.md mentions .agents/research/ output path" "grep -q '\.agents/research/' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions .agents/findings/registry.jsonl" "grep -q '\.agents/findings/registry.jsonl' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions reusable findings" "grep -qi 'reusable findings' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions dedup_key" "grep -q 'dedup_key' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions temp-file-plus-rename atomic write rule" "grep -q 'temp-file-plus-rename atomic write rule' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions finding-compiler.sh refresh" "grep -q 'finding-compiler.sh' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions Explore agent" "grep -qi 'explore' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions --auto flag" "grep -q '\-\-auto' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions ao lookup or ao search" "grep -q 'ao lookup\|ao search' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions knowledge flywheel" "grep -qi 'knowledge' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions backend detection" "grep -qi 'backend\|spawn' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions quality validation" "grep -qi 'coverage\|depth\|gap' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
