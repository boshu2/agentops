#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

phase_control_pattern='MAX_EPIC_WAVES|wave=0|wave=\$\(\(wave|\$wave -ge 50|global wave limit \(50\)|max budget per task: 2|retry once|max 2|max 3 total attempts|--max-cycles|3 validation failures|3\+ failures|after 3 failures|max 2 attempts|after 2 attempts|max 2 retries|after 2 retries|Retry \$RETRY_COUNT/2|Premortem failed 3x|retry limit|MAX_RETRIES|Attempts: 3/3|attempt: 1/3|Attempt counter: 2/3|--budget='

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: crank" "grep -q '^name: crank' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "SKILL.md mentions wave concept" "grep -qi 'wave' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions worker concept" "grep -qi 'worker' '$SKILL_DIR/SKILL.md'"
check "skill requires metadata.issue_type" "grep -rqs 'metadata.issue_type' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "Lead-only commit pattern documented" "grep -rqi 'lead.*commit\|lead-only' '$SKILL_DIR/'"
check "FIRE loop documented" "grep -q 'FIRE' '$SKILL_DIR/SKILL.md'"
check "wave checkpoint validator exists" "[ -x '$SKILL_DIR/scripts/validate-wave-checkpoint.sh' ]"
check "skill runs wave checkpoint validator" "grep -rqs 'validate-wave-checkpoint.sh' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/'"
check "No phantom bd cook refs" "! grep -q 'bd cook' '$SKILL_DIR/SKILL.md'"
check "No phantom gt convoy refs" "! grep -q 'gt convoy' '$SKILL_DIR/SKILL.md'"
check "Crank consumes persistent RPI governor" "grep -q 'pull-flow-governor.md' '$SKILL_DIR/SKILL.md' && grep -q 'run-governor.py admit' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank has no phase-local wave counter" "! grep -Eq 'MAX_EPIC_WAVES|wave=0|wave=\\$\\(\\(wave|RPI_MAX_WAVES' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/execution-preflight.md' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank has no private retry/helper multiplier" "! grep -Eq 'Budget: 2 per task|3 total attempts before' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/execution-preflight.md' '$SKILL_DIR/references/wave-dispatch.md'"
check "Crank authoritative references have no private phase controller" \
  "! rg -q -i '$phase_control_pattern' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references'"

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
