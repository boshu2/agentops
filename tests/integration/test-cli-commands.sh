#!/usr/bin/env bash
# CLI command smoke tests for ao
# Tests basic subcommands with minimal setup

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared colors and helpers
source "${SCRIPT_DIR}/../lib/colors.sh"

errors=0

# Override fail() to increment local error counter
fail() { echo -e "${RED}  ✗${NC} $1"; ((errors++)) || true; }

# Pre-flight: check for Go
if ! command -v go &>/dev/null; then
    fail "Go not available — cannot build ao CLI"
    echo -e "${RED}FAILED${NC} - Prerequisites not met"
    exit 1
fi

# Build ao from source
log "Building ao CLI from source..."

TMPDIR="${TMPDIR:-/tmp}"
TMPBIN="$TMPDIR/ao-test-$$"
TMPDIR_TEST="$TMPDIR/ao-test-dir-$$"
CACHE_BIN="${TMPDIR}/ao-test-cached"

# Trap cleanup (preserve cache, clean per-run artifacts)
cleanup() {
    [[ -f "$TMPBIN" ]] && rm -f "$TMPBIN"
    [[ -d "$TMPDIR_TEST" ]] && rm -rf "$TMPDIR_TEST"
}
trap cleanup EXIT

# Check cache: if binary exists and no .go files are newer, use it
if [[ -f "$CACHE_BIN" ]] && [[ -z "$(find "$REPO_ROOT/cli" -name '*.go' -newer "$CACHE_BIN" | head -1)" ]]; then
    /bin/cp "$CACHE_BIN" "$TMPBIN"
    pass "Using cached binary (source unchanged)"
else
    if (cd "$REPO_ROOT/cli" && go build -o "$TMPBIN" ./cmd/ao 2>/dev/null); then
        /bin/cp "$TMPBIN" "$CACHE_BIN"
        pass "Built ao CLI successfully (cache updated)"
    else
        fail "go build failed"
        echo -e "${RED}FAILED${NC} - Build failed"
        exit 1
    fi
fi

# Set up minimal .agents/ directory for commands that need it
log "Setting up test environment..."
mkdir -p "$TMPDIR_TEST/.agents/learnings"
mkdir -p "$TMPDIR_TEST/.agents/research"
mkdir -p "$TMPDIR_TEST/.agents/pool"
mkdir -p "$TMPDIR_TEST/.agents/rpi"
pass "Created test directory: $TMPDIR_TEST"

# Change to test dir for commands
cd "$TMPDIR_TEST"

# =============================================================================
# Test subcommands
# =============================================================================

test_command() {
    local cmd="$1"
    local name="$2"
    local output
    local exit_code=0

    log "Testing: $name"

    if output=$($cmd 2>&1); then
        exit_code=0
    else
        exit_code=$?
    fi

    # Check exit code 0
    if [[ $exit_code -eq 0 ]]; then
        pass "Exit code 0"
    else
        fail "Exit code $exit_code (expected 0)"
        [[ -n "$output" ]] && echo "$output" | head -5 | sed 's/^/    /'
        return
    fi

    # Check non-empty output
    if [[ -n "$output" ]]; then
        pass "Non-empty output (${#output} chars)"
    else
        fail "Empty output"
    fi

    # Check output pattern (optional 3rd argument)
    local pattern="${3:-}"
    if [[ -n "$pattern" ]]; then
        if echo "$output" | grep -qE "$pattern"; then
            pass "Output matches pattern: $pattern"
        else
            fail "Output missing pattern: $pattern"
        fi
    fi
}

# Test 1: ao status (may show "Not initialized" in test env)
test_command "$TMPBIN status" "ao status" "Status:|AgentOps"

# Test 2: ao version
test_command "$TMPBIN version" "ao version" "version|Version"

# Test 3: ao search
test_command "$TMPBIN search test" "ao search \"test\""

# Test 4: ao ratchet status
test_command "$TMPBIN ratchet status" "ao ratchet status"

# Test 5: ao flywheel status
test_command "$TMPBIN flywheel status" "ao flywheel status" "velocity|health|status"

# Test 6: ao pool list
test_command "$TMPBIN pool list" "ao pool list"

# Test 7: ao doctor (may exit 1 if health checks fail in test env — test help instead)
test_command "$TMPBIN doctor --help" "ao doctor --help" "doctor|health|check|Usage"

# Test 8: ao forge (help — requires transcript path or --last-session)
test_command "$TMPBIN forge --help" "ao forge --help" "forge|transcript"

# Test 9: ao extract (help — may produce empty output without transcripts)
test_command "$TMPBIN extract --help" "ao extract --help" "extract|usage|Usage"

# Test 10: ao inject
test_command "$TMPBIN inject" "ao inject"

# Test 11: ao ratchet (help — subcommands require args)
test_command "$TMPBIN ratchet --help" "ao ratchet --help" "ratchet|status|record|Usage"

# Test 12: ao rpi status — REMOVED: the `ao rpi` command surface was removed
# (f61c5f0e7); the operating loop replaced it as the live navigation path.

# Test 13: ao pool promote (help — requires arg)
test_command "$TMPBIN pool promote --help" "ao pool promote --help" "promote|usage|Usage"

# Test 14: ao ratchet record (help — requires step name)
test_command "$TMPBIN ratchet record --help" "ao ratchet record --help" "record|usage|Usage"

# Test 15: ao rpi --help — REMOVED: `ao rpi` command surface removed (f61c5f0e7).

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $errors -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $errors errors"
    exit 1
else
    echo -e "${GREEN}PASSED${NC} - All CLI command tests passed"
    exit 0
fi
