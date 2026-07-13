#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

EVOLVE="$REPO_ROOT/skills/evolve/SKILL.md"
EVOLVE_EXAMPLES="$REPO_ROOT/skills/evolve/references/examples.md"
RPI_CONTRACT="$REPO_ROOT/skills/rpi/references/phase-data-contracts.md"
RPI_GATE4="$REPO_ROOT/skills/rpi/references/gate4-loop-and-spawn.md"
NEXT_WORK_CONTRACT="$REPO_ROOT/docs/contracts/next-work.schema.md"

check_contains() {
    local file="$1"
    local pattern="$2"
    local message="$3"
    if grep -qE "$pattern" "$file"; then
        pass "$message"
    else
        fail "$message"
    fi
}

check_contains "$EVOLVE" 'Harvested `.agents/rpi/next-work.jsonl` work' "/evolve prioritizes harvested work first"
check_contains "$EVOLVE" 'Open ready beads work' "/evolve prioritizes ready beads second"
check_contains "$EVOLVE" 'Failing goals and directive gaps' "/evolve keeps goals/directives in the ladder"
check_contains "$EVOLVE" 'Testing improvements' "/evolve has testing-improvement generator layer"
check_contains "$EVOLVE" 'Validation tightening and bug-hunt passes' "/evolve has validation and bug-hunt generator layer"
check_contains "$EVOLVE" 'Concrete feature suggestions' "/evolve has feature-suggestion fallback"
check_contains "$EVOLVE" 'Dormancy is last resort' "/evolve no longer treats empty queues as immediate success"
check_contains "$EVOLVE" 'immediately re-read `.agents/rpi/next-work.jsonl`' "/evolve re-reads harvested work after each /rpi cycle"
check_contains "$EVOLVE" 'claim it first' "/evolve claims queue items before consuming them"
check_contains "$EVOLVE" 'session-state.json' "/evolve persists resume state on disk"
check_contains "$EVOLVE_EXAMPLES" 'beads -> harvested work -> goals -> testing -> bug hunt -> feature suggestion' "worked example covers the full long-running ladder"
check_contains "$EVOLVE_EXAMPLES" 're-reads the queue and runs it immediately' "examples show post-RPI harvested work pickup"
check_contains "$EVOLVE_EXAMPLES" 're-queued instead of being lost' "examples show requeue behavior on failure"
check_contains "$RPI_CONTRACT" 'claim_status' "/rpi phase contract includes queue claim metadata"
check_contains "$RPI_GATE4" 'Never mark an item consumed at pick-time' "/rpi gate4 documents claim-before-consume semantics"
check_contains "$NEXT_WORK_CONTRACT" 'Lifecycle Rules' "next-work contract defines queue lifecycle"
check_contains "$NEXT_WORK_CONTRACT" 'Failed or regressed cycles release the claim' "next-work contract documents release on failure"

echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
