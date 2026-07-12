#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: implement" "grep -q '^name: implement' '$SKILL_DIR/SKILL.md'"
check "constraints are front-loaded" "awk 'BEGIN{n=0;i=0;found=0} /^---$/{n++;next} n==2{i++; if (/^## Constraints$/){found=1;exit} if (i>80) exit} END{exit !found}' '$SKILL_DIR/SKILL.md'"
check "constraints pin scope test and pawl" "grep -Fq 'Freeze the claimed issue' '$SKILL_DIR/SKILL.md' && grep -Fq 'right-reason failing test' '$SKILL_DIR/SKILL.md' && grep -Fq 'validation result back through automatic repair' '$SKILL_DIR/SKILL.md'"
check "output specification is complete" "grep -q '^## Output Specification$' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Path:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Filename:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Format:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Validation command:\*\*' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Downstream handoff:\*\*' '$SKILL_DIR/SKILL.md'"
check "quality checklist has three rules" "awk '/^## Quality Checklist$/{f=1;next} f&&/^## /{exit} f&&/^- /{n++} END{exit !(n>=3)}' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 2 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 2 ]"
check "SKILL.md mentions br for issue tracking" "grep -q 'br ' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions beads" "grep -qi 'beads' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions \$validate for closeout" "grep -q '[$]validate' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions Explore agent" "grep -qi 'explore' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions verification gate" "grep -qi 'verification\|verify' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions ratchet record" "grep -q 'ratchet record' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions GREEN mode" "grep -q 'GREEN' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions DONE/BLOCKED/PARTIAL markers" "grep -q 'DONE\|BLOCKED\|PARTIAL' '$SKILL_DIR/SKILL.md'"
check "receipt schema is valid JSON" "jq empty '$SKILL_DIR/schemas/implementation-receipt.schema.json'"
check "workflow contract validator is executable" "[ -x '$SKILL_DIR/scripts/validate-workflow-contract.sh' ]"
check "receipt verifier close wrapper and forged-dimension fixtures are executable" "[ -x '$SKILL_DIR/scripts/verify-implementation-receipt.sh' ] && [ -x '$SKILL_DIR/scripts/close-with-implementation-receipt.sh' ] && [ -x '$SKILL_DIR/scripts/test-implementation-receipt.sh' ]"
check "receipt entrypoints use non-executable internal Bash implementations" "[ -f '$SKILL_DIR/scripts/verify-implementation-receipt.bash' ] && [ ! -x '$SKILL_DIR/scripts/verify-implementation-receipt.bash' ] && [ -f '$SKILL_DIR/scripts/close-with-implementation-receipt.bash' ] && [ ! -x '$SKILL_DIR/scripts/close-with-implementation-receipt.bash' ]"
check "workflow contract enforces ordering receipts immutability and pawl routing" "'$SKILL_DIR/scripts/validate-workflow-contract.sh' codex"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
