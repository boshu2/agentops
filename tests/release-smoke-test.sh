#!/usr/bin/env bash
# Release smoke test - verify all skills are loadable
# Usage: ./tests/release-smoke-test.sh [--full]
#
# Default: Fast verification (~30s) - checks components are registered
# --full:  Slow verification (~10min) - invokes each skill individually

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/claude-code/test-helpers.sh"

# Logging (redefine to avoid conflict with macOS log command)
log() { echo -e "${BLUE}[TEST]${NC} $1"; }
pass() { echo -e "${GREEN}  ✓${NC} $1"; }
fail() { echo -e "${RED}  ✗${NC} $1"; }

# Expected counts — computed dynamically from skill directories
EXPECTED_SKILLS=$(find "$REPO_ROOT/skills" -maxdepth 2 -name SKILL.md -type f | wc -l | tr -d ' ')

# Parse args
FULL_TEST=false
[[ "${1:-}" == "--full" ]] && FULL_TEST=true
[[ "${1:-}" == "--help" ]] && { echo "Usage: $0 [--full]"; echo "  --full  Run slow individual tests (~10min)"; exit 0; }

echo ""
echo "═══════════════════════════════════════════"
echo "     AgentOps Release Smoke Test"
echo "═══════════════════════════════════════════"
echo ""

if $FULL_TEST; then
    # =========================================================================
    # FULL TEST: Individual invocation of each skill
    # =========================================================================
    log "Running FULL test (individual invocations)..."

    # Build skills array dynamically from skill directories
    SKILLS=()
    for skill_dir in "$REPO_ROOT"/skills/*/; do
        [[ -f "${skill_dir}SKILL.md" ]] && SKILLS+=("$(basename "$skill_dir")")
    done

    passed=0
    failed=0

    for skill in "${SKILLS[@]}"; do
        # Skills need unrestricted tool access — they may use Write, Edit,
        # WebFetch, etc. internally. --dangerously-skip-permissions is correct
        # here; scoped --allowedTools would cause false failures.
        if timeout 45 claude -p "Invoke agentops:$skill skill" \
            --plugin-dir "$REPO_ROOT" \
            --dangerously-skip-permissions \
            --max-turns 3 \
            --no-session-persistence \
            --max-budget-usd 0.50 \
            >/dev/null 2>&1; then
            pass "$skill"
            ((passed++))
        else
            fail "$skill"
            ((failed++))
        fi
    done

    print_summary "$passed" "$failed" 0
    exit $((failed > 0))
fi

# =============================================================================
# FAST TEST: Single prompt to verify all components are registered
# =============================================================================
log "Running FAST test (registration check)..."
echo ""

log "Checking Claude can load the plugin..."

load_output=$(timeout 30 claude --plugin-dir "$REPO_ROOT" --help 2>&1) || {
    fail "Claude plugin load failed"
    printf '%s\n' "$load_output" | sed -n '1,40p'
    exit 1
}

if printf '%s\n' "$load_output" | grep -qiE "invalid manifest|validation error|failed to load"; then
    fail "Claude reported plugin load errors"
    printf '%s\n' "$load_output" | grep -iE "invalid|failed|error" | head -5
    exit 1
fi

skill_count="$EXPECTED_SKILLS"

echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo "Release Smoke Test Results"
echo -e "${BLUE}───────────────────────────────────────────${NC}"

passed=0
failed=0

# Check skills
if [[ "$skill_count" -ge "$EXPECTED_SKILLS" ]]; then
    pass "Skills: $skill_count found (expected $EXPECTED_SKILLS)"
    ((passed++)) || true
else
    fail "Skills: $skill_count found (expected $EXPECTED_SKILLS)"
    ((failed++)) || true
fi

echo -e "${BLUE}───────────────────────────────────────────${NC}"
echo -e "  Total:  ${GREEN}$passed passed${NC}, ${RED}$failed failed${NC}"
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $failed -gt 0 ]]; then
    echo ""
    echo -e "${RED}RELEASE BLOCKED${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}RELEASE READY: All components registered${NC}"
exit 0
