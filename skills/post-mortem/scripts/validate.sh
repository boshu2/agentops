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
check "skill has Step 2.6 (deep audit sweep)" "grep -rqs 'Step 2.6' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "SKILL.md references --skip-sweep" "grep -q '\-\-skip-sweep' '$SKILL_DIR/SKILL.md'"
check "skill mentions compiled prevention inputs" "grep -rqs '\.agents/pre-mortem-checks/\|\.agents/planning-rules/' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "skill mentions finding registry" "grep -rqs 'registry.jsonl' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "skill mentions dedup_key" "grep -rqs 'dedup_key' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "skill refreshes finding compiler after writes" "grep -rqs 'finding-compiler.sh' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "closure-integrity audit script exists" "[ -f '$SKILL_DIR/scripts/closure-integrity-audit.sh' ]"
check "reference preflight passes in strict mode" "bash '$SKILL_DIR/scripts/preflight-refs.sh' --strict >/dev/null"
check "evidence-only closure writer exists" "[ -f '$SKILL_DIR/scripts/write-evidence-only-closure.sh' ]"
check "evidence-only closure schema exists" "[ -f '$SKILL_DIR/../../schemas/evidence-only-closure.v1.schema.json' ]"
check "skill documents durable closure proof path" "grep -rqs '\.agents/releases/evidence-only-closures/' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "closure-integrity reference documents durable closure proof path" "grep -q '\.agents/releases/evidence-only-closures/' '$SKILL_DIR/references/closure-integrity-audit.md'"
check "evidence-only closure schema requires supporting artifacts" "grep -q '\"minItems\": 1' '$SKILL_DIR/../../schemas/evidence-only-closure.v1.schema.json'"
check "harvest-next-work documents claim lifecycle" "grep -q 'Queue Lifecycle' '$SKILL_DIR/references/harvest-next-work.md' && grep -q 'Never mark an item consumed at pick-time' '$SKILL_DIR/references/harvest-next-work.md'"
check "harvest-next-work documents evolve-generated sources" "grep -q 'feature-suggestion' '$SKILL_DIR/references/harvest-next-work.md'"
check "SKILL.md points to claim/finalize lifecycle for next-work" "grep -q 'claim/finalize lifecycle' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
