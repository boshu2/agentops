#!/usr/bin/env bash
# Smoke test for pre-mortem-gate.sh hook
# Validates: script exists, syntax, kill switches, and basic flow

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared colors and helpers
source "${SCRIPT_DIR}/../lib/colors.sh"

errors=0
fail() { echo -e "${RED}  ✗${NC} $1"; ((errors++)) || true; }

HOOK="$REPO_ROOT/hooks/pre-mortem-gate.sh"

# =============================================================================
# Structural checks
# =============================================================================

log "Testing pre-mortem-gate.sh structure..."

# Test 1: Script exists
if [[ -f "$HOOK" ]]; then
    pass "Script exists"
else
    fail "Script not found at hooks/pre-mortem-gate.sh"
    echo -e "${RED}FAILED${NC} - Cannot continue without script"
    exit 1
fi

# Test 2: Script is executable or has valid bash syntax
if bash -n "$HOOK" 2>/dev/null; then
    pass "Valid bash syntax"
else
    fail "Bash syntax error in pre-mortem-gate.sh"
fi

# Test 3: Has kill switch (AGENTOPS_HOOKS_DISABLED)
if grep -q 'AGENTOPS_HOOKS_DISABLED' "$HOOK"; then
    pass "Kill switch: AGENTOPS_HOOKS_DISABLED"
else
    fail "Missing kill switch: AGENTOPS_HOOKS_DISABLED"
fi

# Test 4: Has specific gate bypass (AGENTOPS_SKIP_PRE_MORTEM_GATE)
if grep -q 'AGENTOPS_SKIP_PRE_MORTEM_GATE' "$HOOK"; then
    pass "Gate bypass: AGENTOPS_SKIP_PRE_MORTEM_GATE"
else
    fail "Missing gate bypass: AGENTOPS_SKIP_PRE_MORTEM_GATE"
fi

# Test 5: Worker exemption
if grep -q 'AGENTOPS_WORKER' "$HOOK"; then
    pass "Worker exemption: AGENTOPS_WORKER"
else
    fail "Missing worker exemption"
fi

# Test 6: Checks for jq availability (graceful degradation)
if grep -q 'command -v jq' "$HOOK"; then
    pass "jq availability check (graceful degradation)"
else
    fail "Missing jq availability check"
fi

# Test 7: Only gates Skill tool calls for crank
if grep -q 'TOOL_NAME.*Skill' "$HOOK" || grep -q '"Skill"' "$HOOK"; then
    pass "Gates Skill tool calls only"
else
    fail "Missing Skill tool name check"
fi

# Test 8: Checks for --skip-pre-mortem bypass
if grep -q 'skip-pre-mortem' "$HOOK"; then
    pass "--skip-pre-mortem bypass in args"
else
    fail "Missing --skip-pre-mortem bypass check"
fi

# =============================================================================
# Behavioral checks
# =============================================================================

log "Testing pre-mortem-gate.sh behavior..."

# Test 9: Kill switch disables gate (exit 0)
EXIT_CODE=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-test"}}' | \
    AGENTOPS_HOOKS_DISABLED=1 bash "$HOOK" >/dev/null 2>&1 || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "AGENTOPS_HOOKS_DISABLED=1 exits 0"
else
    fail "AGENTOPS_HOOKS_DISABLED=1 exits $EXIT_CODE (expected 0)"
fi

# Test 10: Worker exemption (exit 0)
EXIT_CODE=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-test"}}' | \
    AGENTOPS_WORKER=1 bash "$HOOK" >/dev/null 2>&1 || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "AGENTOPS_WORKER=1 exits 0"
else
    fail "AGENTOPS_WORKER=1 exits $EXIT_CODE (expected 0)"
fi

# Test 11: Non-Skill tool calls pass through (exit 0)
EXIT_CODE=0
echo '{"tool_name":"Bash","tool_input":{"command":"ls"}}' | \
    bash "$HOOK" >/dev/null 2>&1 || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "Non-Skill tool passes through"
else
    fail "Non-Skill tool exits $EXIT_CODE (expected 0)"
fi

# Test 12: Non-crank skill calls pass through (exit 0)
EXIT_CODE=0
echo '{"tool_name":"Skill","tool_input":{"skill":"vibe","args":"recent"}}' | \
    bash "$HOOK" >/dev/null 2>&1 || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "Non-crank skill passes through"
else
    fail "Non-crank skill exits $EXIT_CODE (expected 0)"
fi

# =============================================================================
# Lock-on-error fallback semantics
# =============================================================================
log "Testing lock-on-error fallback semantics..."

# Sandbox: a fake repo root with .agents/rpi/phased-state.json under our control,
# plus a fake `bd` binary on PATH that pretends every epic has 5 children. This
# lets us drive the gate past the early-exit checks without touching the real repo.
SANDBOX="$(mktemp -d 2>/dev/null || mktemp -d -t pmgate)"
trap 'rm -rf "$SANDBOX"' EXIT

mkdir -p "$SANDBOX/.agents/rpi" "$SANDBOX/bin"
( cd "$SANDBOX" && git init -q 2>/dev/null && git config user.email t@t && git config user.name t && git commit --allow-empty -q -m init ) || true

cat > "$SANDBOX/bin/bd" <<'BDFAKE'
#!/usr/bin/env bash
# Fake bd: `bd children <id>` prints 5 lines so child count >= 3.
if [ "${1:-}" = "children" ]; then
    printf 'a\nb\nc\nd\ne\n'
fi
exit 0
BDFAKE
chmod +x "$SANDBOX/bin/bd"

run_gate_in_sandbox() {
    # $1 = extra env var assignments (string), rest = stdin payload
    local env_pairs="$1"; shift
    local payload="$1"; shift
    (
        cd "$SANDBOX" || exit 99
        # Put fake bd first on PATH
        export PATH="$SANDBOX/bin:$PATH"
        # shellcheck disable=SC2086
        env $env_pairs bash "$HOOK" <<<"$payload" >/dev/null 2>&1
    )
}

CRANK_PAYLOAD='{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-test1"}}'

# Test 13: Missing phased-state.json → BLOCKED (strict default)
EXIT_CODE=0
run_gate_in_sandbox "" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 2 ]]; then
    pass "Missing state → BLOCKED (exit 2)"
else
    fail "Missing state expected exit 2, got $EXIT_CODE"
fi

# Test 14: Unparseable phased-state.json → BLOCKED
echo "{not valid json" > "$SANDBOX/.agents/rpi/phased-state.json"
EXIT_CODE=0
run_gate_in_sandbox "" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 2 ]]; then
    pass "Unparseable state → BLOCKED"
else
    fail "Unparseable state expected exit 2, got $EXIT_CODE"
fi

# Test 15: Valid 'passed' verdict → ALLOWED
printf '{"verdicts":{"pre_mortem":"passed"},"run_id":"r1"}' > "$SANDBOX/.agents/rpi/phased-state.json"
EXIT_CODE=0
run_gate_in_sandbox "" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "Valid 'passed' verdict → ALLOWED"
else
    fail "Passed verdict expected exit 0, got $EXIT_CODE"
fi

# Test 16: 'expired' verdict → BLOCKED
printf '{"verdicts":{"pre_mortem":"expired"},"run_id":"r1"}' > "$SANDBOX/.agents/rpi/phased-state.json"
EXIT_CODE=0
run_gate_in_sandbox "" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 2 ]]; then
    pass "'expired' verdict → BLOCKED"
else
    fail "Expired verdict expected exit 2, got $EXIT_CODE"
fi

# Test 17: Unknown verdict enum value → BLOCKED
printf '{"verdicts":{"pre_mortem":"maybe-sometime"},"run_id":"r1"}' > "$SANDBOX/.agents/rpi/phased-state.json"
EXIT_CODE=0
run_gate_in_sandbox "" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 2 ]]; then
    pass "Unknown verdict → BLOCKED"
else
    fail "Unknown verdict expected exit 2, got $EXIT_CODE"
fi

# Test 18: Bootstrap escape hatch → ALLOWED with stderr warning
rm -f "$SANDBOX/.agents/rpi/phased-state.json"
EXIT_CODE=0
STDERR_CAPTURE=$(
    cd "$SANDBOX" || exit 99
    export PATH="$SANDBOX/bin:$PATH"
    AGENTOPS_PREMORTEM_GATE_BOOTSTRAP=1 bash "$HOOK" <<<"$CRANK_PAYLOAD" 2>&1 >/dev/null
) || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]] && echo "$STDERR_CAPTURE" | grep -q "AGENTOPS_PREMORTEM_GATE_BOOTSTRAP"; then
    pass "Bootstrap env var → ALLOWED with warning"
else
    fail "Bootstrap expected exit 0 + warning, got exit $EXIT_CODE stderr='$STDERR_CAPTURE'"
fi

# Test 19: Kill switch → silent allow (no stderr)
EXIT_CODE=0
STDERR_CAPTURE=$(
    cd "$SANDBOX" || exit 99
    export PATH="$SANDBOX/bin:$PATH"
    AGENTOPS_HOOKS_DISABLED=1 bash "$HOOK" <<<"$CRANK_PAYLOAD" 2>&1 >/dev/null
) || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]] && [[ -z "$STDERR_CAPTURE" ]]; then
    pass "Kill switch → silent ALLOW"
else
    fail "Kill switch expected silent exit 0, got exit $EXIT_CODE stderr='$STDERR_CAPTURE'"
fi

# Test 20: AGENTOPS_PREMORTEM_FALLBACK=open + missing state → ALLOWED (legacy mode)
EXIT_CODE=0
run_gate_in_sandbox "AGENTOPS_PREMORTEM_FALLBACK=open" "$CRANK_PAYLOAD" || EXIT_CODE=$?
if [[ $EXIT_CODE -eq 0 ]]; then
    pass "FALLBACK=open + missing state → ALLOWED (legacy)"
else
    fail "Open mode missing state expected exit 0, got $EXIT_CODE"
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $errors -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $errors errors"
    exit 1
else
    echo -e "${GREEN}PASSED${NC} - All pre-mortem-gate tests passed"
    exit 0
fi
