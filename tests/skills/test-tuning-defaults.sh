#!/usr/bin/env bash
# Validate speed/cost tuning defaults across skills
# Tests the invariants from the 2026-02-18 tuning commits:
#   1. Council judges default to sonnet (not opus)
#   2. Pre-mortem default = 2 judges
#   3. Vibe default = 2 judges
#   4. PRODUCT.md adds exactly 1 consolidated judge (not 3)
#   5. RPI complexity scaling: low+medium use --quick for all gates
#   6. RPI high complexity uses full council (not --quick)
#   7. Codex review capped at 2000 chars
#   8. Vibe --quick skips heavy pre-steps (2a-2e)
#   9. Pre-mortem --quick skips knowledge search + product context
#  10. Swarm workers should use sonnet
#  11. Wave-level judges use haiku (cost efficiency)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

passed=0
failed=0

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    passed=$((passed + 1))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    failed=$((failed + 1))
}

skip() {
    echo -e "  ${YELLOW}-${NC} $1 (skipped: council extracted to mt-olympus)"
}

# council was extracted to mt-olympus (skill-prune Lane A, 2026-06-10); the
# in-repo SKILL.md is a routing stub. Council tuning assertions follow the
# canonical copy in ~/dev/mt-olympus and are skipped here when the stub marker
# is present.
COUNCIL_EXTRACTED=""
if grep -q 'moved to Mount Olympus' "$REPO_ROOT/skills/council/SKILL.md" 2>/dev/null; then
    COUNCIL_EXTRACTED=1
fi

echo "=== Tuning Defaults Validation ==="
echo ""

# --- Council Model Defaults ---
echo "--- Council Model Defaults ---"

# T1: Council judges default to sonnet
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T1: Council judges default to sonnet"
elif grep -q 'COUNCIL_CLAUDE_MODEL.*|.*sonnet' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T1: Council judges default to sonnet"
else
    fail "T1: Council judges should default to sonnet (not opus)"
fi

# T2: cli-spawning shows sonnet as Claude default
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T2: cli-spawning.md shows sonnet as Claude default"
elif grep -q '| Claude | sonnet |' "$REPO_ROOT/skills/council/references/cli-spawning.md"; then
    pass "T2: cli-spawning.md shows sonnet as Claude default"
else
    fail "T2: cli-spawning.md should show sonnet as Claude default"
fi

# T3: Explorer model still sonnet
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T3: Explorer model remains sonnet"
elif grep -q 'COUNCIL_EXPLORER_MODEL.*|.*sonnet' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T3: Explorer model remains sonnet"
else
    fail "T3: Explorer model should remain sonnet"
fi

# T4: Model profiles: balanced = sonnet, thorough = opus, fast = haiku
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T4: Model profiles correct (thorough=opus, balanced=sonnet, fast=haiku)"
elif grep -q '| `thorough` | opus |' "$REPO_ROOT/skills/council/references/model-profiles.md" && \
   grep -q '| `balanced` | sonnet |' "$REPO_ROOT/skills/council/references/model-profiles.md" && \
   grep -q '| `fast` | haiku |' "$REPO_ROOT/skills/council/references/model-profiles.md"; then
    pass "T4: Model profiles correct (thorough=opus, balanced=sonnet, fast=haiku)"
else
    fail "T4: Model profiles should be thorough=opus, balanced=sonnet, fast=haiku"
fi

echo ""

# --- Pre-mortem Judge Count ---
echo "--- Pre-mortem Judge Count ---"

# T5: Pre-mortem default = 2 judges (missing-requirements + feasibility)
if grep -q 'Default (2 judges' "$REPO_ROOT/skills/pre-mortem/SKILL.md" || \
   grep -q '2 judges with plan-review' "$REPO_ROOT/skills/pre-mortem/SKILL.md"; then
    pass "T5: Pre-mortem default = 2 judges"
else
    fail "T5: Pre-mortem should default to 2 judges"
fi

# T6: Pre-mortem --deep = 4 judges
if grep -q 'deep.*4 judges' "$REPO_ROOT/skills/pre-mortem/SKILL.md"; then
    pass "T6: Pre-mortem --deep = 4 judges"
else
    fail "T6: Pre-mortem --deep should be 4 judges"
fi

# T7: Pre-mortem PRODUCT.md adds exactly 1 consolidated judge
if grep -q '3 judges total (2 plan-review + 1 product)' "$REPO_ROOT/skills/pre-mortem/SKILL.md"; then
    pass "T7: Pre-mortem PRODUCT.md adds 1 consolidated product judge (3 total)"
else
    fail "T7: Pre-mortem PRODUCT.md should add 1 product judge (3 total)"
fi

# T8: Pre-mortem --quick fast path skips 1a and 1b
if grep -q 'Step 1.5: Fast Path' "$REPO_ROOT/skills/pre-mortem/SKILL.md" && \
   grep -q 'skip Steps 1a and 1b' "$REPO_ROOT/skills/pre-mortem/SKILL.md"; then
    pass "T8: Pre-mortem --quick fast path documented (skip 1a, 1b)"
else
    fail "T8: Pre-mortem should document --quick fast path skipping 1a and 1b"
fi

echo ""

# --- Vibe Judge Count ---
echo "--- Vibe Judge Count ---"

# T9: Vibe default = 2 independent judges (not 3)
if grep -q '2 independent judges' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"; then
    pass "T9: Vibe default = 2 independent judges"
else
    fail "T9: Vibe should default to 2 independent judges"
fi

# T10: Vibe PRODUCT.md adds 1 consolidated DX judge
if grep -q '3 judges: 2 .* + 1 DX' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md" || \
   grep -q '2 independent + 1 DX' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"; then
    pass "T10: Vibe PRODUCT.md adds 1 consolidated DX judge"
else
    fail "T10: Vibe PRODUCT.md should add 1 DX judge (3 total)"
fi

# T11: Codex review capped at 2000 chars
if grep -q '2000 chars' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md" && \
   grep -q 'first 2000 chars' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"; then
    pass "T11: Codex review capped at 2000 chars"
else
    fail "T11: Codex review should be capped at 2000 chars"
fi

# T12: Vibe --quick fast path skips Steps 2a-2e
if grep -q 'Step 1.5: Fast Path' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md" && \
   grep -q 'skip Steps 2a.*2e' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"; then
    pass "T12: Vibe --quick fast path documented (skip 2a-2e)"
else
    fail "T12: Vibe should document --quick fast path skipping 2a-2e"
fi

# T13: Individual steps 2a, 2b, 2c, 2.5, 2d, 2e, 3 marked "Skip if --quick"
quick_skip_count=0
for step in "Step 2a" "Step 2b" "Step 2c" "Step 2.5" "Step 2d" "Step 2e" "Step 3:"; do
    # Check that within 3 lines after the step header, "Skip if" appears
    if grep -A3 "### $step" "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md" | grep -q "Skip if"; then
        quick_skip_count=$((quick_skip_count + 1))
    fi
done
if [ "$quick_skip_count" -ge 6 ]; then
    pass "T13: ${quick_skip_count}/7 heavy steps marked 'Skip if --quick'"
else
    fail "T13: Only ${quick_skip_count}/7 heavy steps marked 'Skip if --quick' (need >=6)"
fi

echo ""

# --- RPI Complexity Scaling ---
echo "--- RPI Complexity Scaling ---"

RPI_SKILL="$REPO_ROOT/skills/rpi/SKILL.md"

# T14: Pre-mortem: low = --quick, medium = --quick, high = full council
if grep -A3 'Pre-mortem' "$RPI_SKILL" | grep -q 'complexity == "low".*inline\|"low".*no spawning'; then
    pass "T14a: RPI pre-mortem low = --quick"
else
    fail "T14a: RPI pre-mortem low should use --quick"
fi

if grep -A3 'Pre-mortem' "$RPI_SKILL" | grep -q 'complexity == "medium".*inline\|"medium".*fast default'; then
    pass "T14b: RPI pre-mortem medium = --quick"
else
    fail "T14b: RPI pre-mortem medium should use --quick"
fi

if grep -A3 'Pre-mortem' "$RPI_SKILL" | grep -q 'complexity == "high".*full.*council\|"high".*2-judge'; then
    pass "T14c: RPI pre-mortem high = full council"
else
    fail "T14c: RPI pre-mortem high should use full council"
fi

# T15: Final Vibe: low = --quick, medium = --quick, high = full council
if grep -A3 'Final Vibe' "$RPI_SKILL" | grep -q 'complexity == "low".*inline\|"low".*no spawning'; then
    pass "T15a: RPI final vibe low = --quick"
else
    fail "T15a: RPI final vibe low should use --quick"
fi

if grep -A3 'Final Vibe' "$RPI_SKILL" | grep -q 'complexity == "medium".*inline\|"medium".*fast default'; then
    pass "T15b: RPI final vibe medium = --quick"
else
    fail "T15b: RPI final vibe medium should use --quick"
fi

if grep -A3 'Final Vibe' "$RPI_SKILL" | grep -q 'complexity == "high".*full.*council\|"high".*2-judge'; then
    pass "T15c: RPI final vibe high = full council"
else
    fail "T15c: RPI final vibe high should use full council"
fi

# T16: Post-mortem: low = --quick, medium = --quick, high = full council
if grep -A3 'Post-mortem .STEP 2' "$RPI_SKILL" | grep -q 'complexity == "low".*inline\|"low".*no spawning'; then
    pass "T16a: RPI post-mortem low = --quick"
else
    fail "T16a: RPI post-mortem low should use --quick"
fi

if grep -A3 'Post-mortem .STEP 2' "$RPI_SKILL" | grep -q 'complexity == "medium".*inline\|"medium".*fast default'; then
    pass "T16b: RPI post-mortem medium = --quick"
else
    fail "T16b: RPI post-mortem medium should use --quick"
fi

if grep -A3 'Post-mortem .STEP 2' "$RPI_SKILL" | grep -q 'complexity == "high".*full.*council\|"high".*2-judge'; then
    pass "T16c: RPI post-mortem high = full council"
else
    fail "T16c: RPI post-mortem high should use full council"
fi

echo ""

# --- Complexity Scaling Reference ---
echo "--- Complexity Scaling Reference ---"

SCALING_REF="$REPO_ROOT/skills/rpi/references/complexity-scaling.md"

# T17: Medium ceremony = lean/quick (not standard council)
if grep -q 'medium.*lean\|medium.*quick' "$SCALING_REF"; then
    pass "T17: complexity-scaling.md medium = lean (--quick)"
else
    fail "T17: complexity-scaling.md medium should say lean/quick, not standard council"
fi

# T18: Medium should NOT say "standard council"
if grep -q '| \*\*medium\*\*.*standard council' "$SCALING_REF"; then
    fail "T18: complexity-scaling.md medium should NOT say 'standard council'"
else
    pass "T18: complexity-scaling.md medium does not say 'standard council'"
fi

# T19: Design rationale present
if grep -q 'Design rationale' "$SCALING_REF"; then
    pass "T19: Design rationale present in complexity-scaling.md"
else
    fail "T19: complexity-scaling.md should have design rationale"
fi

echo ""

# --- Swarm Worker Model ---
echo "--- Swarm Worker Model ---"

LOCAL_MODE="$REPO_ROOT/skills/swarm/references/local-mode.md"

# T20: Swarm local-mode documents sonnet for workers
if grep -q 'Workers.*sonnet\|workers.*sonnet' "$LOCAL_MODE"; then
    pass "T20: Swarm local-mode specifies sonnet for workers"
else
    fail "T20: Swarm local-mode should specify sonnet for workers"
fi

# T21: Swarm local-mode documents opus for lead
if grep -q 'Lead.*opus\|lead.*opus\|orchestrator.*opus' "$LOCAL_MODE"; then
    pass "T21: Swarm local-mode specifies opus for lead"
else
    fail "T21: Swarm local-mode should specify opus for lead"
fi

# T22: Model selection table exists in local-mode
if grep -q 'Model Selection' "$LOCAL_MODE"; then
    pass "T22: Model Selection section exists in swarm local-mode"
else
    fail "T22: Swarm local-mode should have Model Selection section"
fi

echo ""

# --- Wave-Level Judges ---
echo "--- Wave-Level Judges (Crank) ---"

WAVE_PATTERNS="$REPO_ROOT/skills/crank/references/wave-patterns.md"

# T23: Wave-level judges use haiku
if grep -q 'model: "haiku"' "$WAVE_PATTERNS"; then
    pass "T23: Wave-level judges use haiku"
else
    fail "T23: Wave-level judges should use haiku"
fi

echo ""

# --- Cross-Consistency Checks ---
echo "--- Cross-Consistency ---"

# T24: No skill file references "3 independent judges" (should be 2)
three_judge_files=$(grep -rl '3 independent judges' "$REPO_ROOT/skills/" 2>/dev/null || true)
if [ -z "$three_judge_files" ]; then
    pass "T24: No skill references '3 independent judges' (should be 2)"
else
    fail "T24: Found '3 independent judges' in: $(echo "$three_judge_files" | tr '\n' ' ')"
fi

# T25: Council SKILL.md and cli-spawning.md both say sonnet as default
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T25: Council SKILL.md and cli-spawning.md both default to sonnet"
else
    council_has_sonnet=$(grep 'COUNCIL_CLAUDE_MODEL' "$REPO_ROOT/skills/council/SKILL.md" | head -1 | grep -c 'sonnet' || true)
    cli_has_sonnet=$(grep '| Claude |' "$REPO_ROOT/skills/council/references/cli-spawning.md" | head -1 | grep -c 'sonnet' || true)
    if [ "$council_has_sonnet" -ge 1 ] && [ "$cli_has_sonnet" -ge 1 ]; then
        pass "T25: Council SKILL.md and cli-spawning.md both default to sonnet"
    else
        fail "T25: Council SKILL.md (sonnet: $council_has_sonnet) and cli-spawning.md (sonnet: $cli_has_sonnet) should both say sonnet"
    fi
fi

# T26: Pre-mortem Step 1a marked "Skip if --quick"
if grep -A2 "### Step 1a" "$REPO_ROOT/skills/pre-mortem/SKILL.md" | grep -q "Skip if"; then
    pass "T26: Pre-mortem Step 1a marked 'Skip if --quick'"
else
    fail "T26: Pre-mortem Step 1a should be marked 'Skip if --quick'"
fi

# T27: Pre-mortem Step 1b marked "Skip if --quick"
if grep -A2 "### Step 1b" "$REPO_ROOT/skills/pre-mortem/SKILL.md" | grep -q "Skip if"; then
    pass "T27: Pre-mortem Step 1b marked 'Skip if --quick'"
else
    fail "T27: Pre-mortem Step 1b should be marked 'Skip if --quick'"
fi

# ============================================================
# Section 7: Cross-cutting timeout defaults
# ============================================================
echo ""
echo "--- Section 7: Timeout Defaults ---"

# T28: Council default timeout is 120s
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T28: Council default timeout is 120s"
elif grep -q 'COUNCIL_TIMEOUT.*120' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T28: Council default timeout is 120s"
else
    fail "T28: Council SKILL.md should define COUNCIL_TIMEOUT default as 120"
fi

# T29: Explorer timeout is 60s
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T29: Explorer timeout is 60s"
elif grep -q 'COUNCIL_EXPLORER_TIMEOUT.*60' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T29: Explorer timeout is 60s"
else
    fail "T29: Council SKILL.md should define COUNCIL_EXPLORER_TIMEOUT default as 60"
fi

# T30: R2 debate timeout is 90s
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T30: R2 debate timeout is 90s"
elif grep -q 'COUNCIL_R2_TIMEOUT.*90' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T30: R2 debate timeout is 90s"
else
    fail "T30: Council SKILL.md should define COUNCIL_R2_TIMEOUT default as 90"
fi

# ============================================================
# Section 8: RPI retry limits
# ============================================================
echo ""
echo "--- Section 8: RPI Retry Limits ---"

# T31: Pre-mortem retry limit is 3
if grep -q 'max 3 total attempts' "$REPO_ROOT/skills/rpi/SKILL.md"; then
    pass "T31: RPI pre-mortem retry limit is 3"
else
    fail "T31: RPI SKILL.md should specify 'max 3 total attempts' for pre-mortem"
fi

# T32: All 3 retry gates use same limit (3)
retry_count=$(grep -c 'max 3 total attempts' "$REPO_ROOT/skills/rpi/SKILL.md")
if [ "$retry_count" -ge 3 ]; then
    pass "T32: All 3 RPI retry gates use 'max 3 total attempts' ($retry_count occurrences)"
else
    fail "T32: Expected 3+ retry gates with 'max 3 total attempts', found $retry_count"
fi

# T33: RPI --test-first default is on (strict-quality default)
if grep -q '| `--test-first` | on |' "$REPO_ROOT/skills/rpi/SKILL.md"; then
    pass "T33: RPI --test-first default is on (strict-quality default)"
else
    fail "T33: RPI SKILL.md should document --test-first default as on"
fi

# T34: RPI exposes explicit opt-out via --no-test-first
if grep -q '| `--no-test-first` | off |' "$REPO_ROOT/skills/rpi/SKILL.md"; then
    pass "T34: RPI exposes explicit opt-out via --no-test-first"
else
    fail "T34: RPI SKILL.md should document --no-test-first opt-out flag"
fi

# ============================================================
# Section 9: Codex model default
# ============================================================
echo ""
echo "--- Section 9: Codex Model ---"

# T35: Codex model is gpt-5.3-codex
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T35: Council Codex model default is gpt-5.3-codex"
elif grep -q 'gpt-5.3-codex' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T35: Council Codex model default is gpt-5.3-codex"
else
    fail "T35: Council SKILL.md should reference gpt-5.3-codex as Codex model"
fi

# ============================================================
# Section 10: Output path consistency
# ============================================================
echo ""
echo "--- Section 10: Output Paths ---"

# T36: Council outputs to .agents/council/
if [ -n "$COUNCIL_EXTRACTED" ]; then
    skip "T36: Council outputs to .agents/council/"
elif grep -q '\.agents/council/' "$REPO_ROOT/skills/council/SKILL.md"; then
    pass "T36: Council outputs to .agents/council/"
else
    fail "T36: Council SKILL.md should output to .agents/council/"
fi

# T37: Vibe outputs to .agents/council/ (same dir)
if grep -q '\.agents/council/' "$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"; then
    pass "T37: Vibe outputs to .agents/council/"
else
    fail "T37: Vibe SKILL.md should output to .agents/council/"
fi

echo ""

# --- Summary ---
echo "==========================="
total=$((passed + failed))
echo -e "  ${GREEN}✓ Passed:${NC} $passed"
echo -e "  ${RED}✗ Failed:${NC} $failed"
echo -e "  Total:  $total"
echo "==========================="

if [ "$failed" -gt 0 ]; then
    echo -e "${RED}OVERALL: FAIL${NC}"
    exit 1
else
    echo -e "${GREEN}OVERALL: PASS${NC}"
    exit 0
fi
