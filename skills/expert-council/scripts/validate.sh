#!/usr/bin/env bash
# validate.sh — self-validation for the expert-council skill
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

# --- Structure ---
check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: expert-council" "grep -q '^name: expert-council' '$SKILL_DIR/SKILL.md'"
check "SKILL.md is at most 250 lines" "[ \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250 ]"
check "references/dueling-route.md exists" "[ -f '$SKILL_DIR/references/dueling-route.md' ]"
check "references/dueling-route.md is linked from SKILL.md" "grep -q 'references/dueling-route.md' '$SKILL_DIR/SKILL.md'"

# --- Behavioral contracts: the dueling route must stay documented ---
check "metadata.tier is judgment" "grep -q 'tier: judgment' '$SKILL_DIR/SKILL.md'"
check "the seven-phase workflow is documented" "grep -q 'Phase 7: Synthesis' '$SKILL_DIR/SKILL.md'"
check "the duel (cross-scoring) phase is documented" "grep -qi 'cross-scoring' '$SKILL_DIR/SKILL.md'"
check "the reveal phase is documented" "grep -qi 'Reveal' '$SKILL_DIR/SKILL.md'"
check "0-1000 scoring scale is specified" "grep -q '0-1000' '$SKILL_DIR/SKILL.md'"
check "reveal-is-mandatory constraint is present" "grep -qi 'reveal is mandatory' '$SKILL_DIR/SKILL.md'"
check "briefing-on-disk constraint is present" "grep -qi 'argv' '$SKILL_DIR/SKILL.md'"
check "distinction from /council is documented" "grep -q 'vs ./council.' '$SKILL_DIR/SKILL.md'"
check "output path pattern documented" "grep -q '.agents/council/' '$SKILL_DIR/SKILL.md'"
check "Codex/NTM gotcha is documented" "grep -qi 'gpt-\*-codex\|ChatGPT-billed' '$SKILL_DIR/SKILL.md'"
check "prompt templates live in references" "grep -q 'Phase 5' '$SKILL_DIR/references/dueling-route.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
