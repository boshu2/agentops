#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: retro" "grep -q '^name: retro' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions .agents/learnings/ output" "grep -q '\.agents/learnings/' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions learning categories" "grep -qi 'category\|debugging\|architecture\|process' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions post-mortem delegation" "grep -qi 'post-mortem' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
