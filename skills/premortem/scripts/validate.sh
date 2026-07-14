#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
SCHEMA="$SKILL_DIR/schemas/plan-verdict.schema.json"
VALIDATOR="$SKILL_DIR/scripts/validate-output.sh"
PASS=0
FAIL=0

check() {
  if bash -c "$2"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
  fi
}

check "SKILL.md exists" "test -f '$SKILL'"
check "frontmatter name" "grep -q '^name: premortem' '$SKILL'"
check "exact-plan output contract" "grep -q '^output_contract: skills/premortem/schemas/plan-verdict.schema.json' '$SKILL' || grep -Fq '[plan-verdict.schema.json](schemas/plan-verdict.schema.json)' '$SKILL'"
check "fresh author-distinct judge" "grep -Fq 'author_id != judge_id' '$SKILL'"
check "binary complete verdict" "grep -Fq 'Emit exactly' '$SKILL' && grep -q 'complete nonempty blocker set' '$SKILL'"
check "family is optional metadata" "grep -q 'Model and family' '$SKILL' && grep -q 'metadata are optional' '$SKILL' && grep -q 'no risk class requires different model families' '$SKILL'"
check "no local controller ownership" "grep -q 'Do not own retries, attempt maps, budgets, helper state' '$SKILL'"
check "schema and validator exist" "test -f '$SCHEMA' && test -x '$VALIDATOR'"
check "schema is strict" "jq -e '.additionalProperties == false and (.properties.verdict.enum == [\"PASS\",\"FAIL\"]) and (.properties.blockers_complete.const == true)' '$SCHEMA' >/dev/null"
check "plan-pawl authority removed" "! rg -q 'plan-pawl|ApprovalEdge|Fable|WARN.*Ready|PASS/WARN/FAIL|cross-family rule for one-way doors' '$SKILL' '$SKILL_DIR/references/mandatory-checks.md' '$SKILL_DIR/references/premortem.feature' '$SKILL_DIR/references/write-premortem-output.md'"
check "kernel stays within 250 lines" "test \$(wc -l < '$SKILL') -le 250"
check "focused direct-cut acceptance exists" "test -f '$REPO_ROOT/tests/scripts/premortem-plan-verdict-direct-cut.bats'"

echo
echo "Results: $PASS passed, $FAIL failed"
test "$FAIL" -eq 0
