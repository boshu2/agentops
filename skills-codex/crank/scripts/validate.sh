#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: crank" "grep -q '^name: crank' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "SKILL.md mentions wave concept" "grep -qi 'wave' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions worker concept" "grep -qi 'worker' '$SKILL_DIR/SKILL.md'"
check "SKILL.md requires metadata.issue_type" "grep -q 'metadata.issue_type' '$SKILL_DIR/SKILL.md'"
check "Lead-only commit pattern documented" "grep -rqi 'lead.*commit\|lead-only' '$SKILL_DIR/'"
check "FIRE loop documented" "grep -q 'FIRE' '$SKILL_DIR/SKILL.md'"
check "wave checkpoint validator exists" "[ -x '$SKILL_DIR/scripts/validate-wave-checkpoint.sh' ]"
check "SKILL.md runs wave checkpoint validator" "grep -q 'validate-wave-checkpoint.sh' '$SKILL_DIR/SKILL.md'"
check "No phantom bd cook refs" "! grep -q 'bd cook' '$SKILL_DIR/SKILL.md'"
check "No phantom gt convoy refs" "! grep -q 'gt convoy' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
