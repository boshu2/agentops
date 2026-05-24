#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: evolve" "grep -q '^name: evolve' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 1 file" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 1 ]"
check "SKILL.md mentions kill switch" "grep -qi 'kill switch' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions fitness" "grep -qi 'fitness' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions GOALS.yaml" "grep -q 'GOALS.yaml' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions cycle" "grep -qi 'cycle' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions /rpi" "grep -q '/rpi' '$SKILL_DIR/SKILL.md'"
# Behavioral contracts from retro learnings (2026-02-12)
check "SKILL.md has KILL file path" "grep -q 'KILL' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents regression detection" "grep -qi 'regression' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents snapshot enforcement" "grep -qi 'snapshot' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents session_start_sha" "grep -qi 'session.start.sha\|cycle_start_sha' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents continuous values" "grep -qi 'continuous\|value.*threshold' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents regression gate" "grep -qi 'regression gate' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents post-cycle snapshot" "grep -q 'fitness-.*-post' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents oscillation detection" "grep -qi 'oscillat' '$SKILL_DIR/SKILL.md'"
# Design-level checks (2026-03-01)
check "Step 0 has oscillation sweep (always-on)" "grep -q 'Pre-populate quarantine list' '$SKILL_DIR/SKILL.md'"
check "Step 5 has wiring script pre-flight check" "grep -q 'if.*check-wiring-closure.sh' '$SKILL_DIR/SKILL.md'"
check "No ambiguous YAML fallback in Step 2" "! grep -q 'run each goal.*check command manually' '$SKILL_DIR/SKILL.md'"
check "CLI required for fitness measurement" "grep -q 'CLI is required for fitness measurement' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents harvested-first selection order" "grep -Fq 'Harvested \`.agents/rpi/next-work.jsonl\` work' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents generator ladder" "grep -q 'Testing improvements' '$SKILL_DIR/SKILL.md' && grep -q 'Validation tightening and bug-hunt passes' '$SKILL_DIR/SKILL.md' && grep -q 'Concrete feature suggestions' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents dormancy as last resort" "grep -q 'Dormancy is last resort' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents queue claim before consume" "grep -q 'claim it first' '$SKILL_DIR/SKILL.md' && grep -Fq 'keep \`consumed: false\`' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents session-state persistence" "grep -q 'session-state.json' '$SKILL_DIR/SKILL.md'"
check "SKILL.md documents immediate queue reread after /rpi" "grep -Fq 'immediately re-read \`.agents/rpi/next-work.jsonl\`' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions repo execution profile" "grep -qi 'repo execution profile' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions startup_reads" "grep -q 'startup_reads' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions validation_commands" "grep -q 'validation_commands' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions definition_of_done" "grep -q 'definition_of_done' '$SKILL_DIR/SKILL.md'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
