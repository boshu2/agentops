#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0
check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "name is security-suite" "grep -q '^name: security-suite' '$SKILL_DIR/SKILL.md'"
check "references policy exists" "[ -f '$SKILL_DIR/references/policy-example.json' ]"
check "security_suite.py exists" "[ -x '$SKILL_DIR/scripts/security_suite.py' ]"
check "security_suite.py compiles" "python3 -m py_compile '$SKILL_DIR/scripts/security_suite.py'"
check "policy JSON valid" "python3 -c \"import json, pathlib; json.loads(pathlib.Path('$SKILL_DIR/references/policy-example.json').read_text()); print('ok')\""

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
