#!/usr/bin/env bash
# Release Smoke Test — validates ALL ao CLI commands before release
#
# Proves every registered command runs without panicking, produces expected
# output, and exits cleanly. Any broken registration, flag parsing error,
# or runtime panic is caught before tagging.
#
# Usage:
#   bash scripts/release-smoke-test.sh              # build + test
#   bash scripts/release-smoke-test.sh --skip-build # use existing cli/bin/ao
#
# Exit codes:
#   0 = all tests passed
#   1 = one or more tests failed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# ═══════════════════════════════════════════════════════
#  Options
# ═══════════════════════════════════════════════════════

SKIP_BUILD=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        -h|--help)
            echo "Usage: bash scripts/release-smoke-test.sh [--skip-build]"
            echo ""
            echo "Options:"
            echo "  --skip-build   Use existing cli/bin/ao instead of rebuilding"
            echo "  -h, --help     Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# ═══════════════════════════════════════════════════════
#  Colors & Counters
# ═══════════════════════════════════════════════════════

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0
TOTAL=0

pass() {
    echo -e "${GREEN}  PASS${NC}  $1"
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
}

fail() {
    echo -e "${RED}  FAIL${NC}  $1"
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
}

skip() {
    echo -e "${YELLOW}  SKIP${NC}  $1"
    SKIP=$((SKIP + 1))
    TOTAL=$((TOTAL + 1))
}

output_matches() {
    local output="$1"
    local pattern="$2"
    grep -qEi "$pattern" <<<"$output"
}

output_word_matches() {
    local output="$1"
    local word="$2"
    grep -qw -- "$word" <<<"$output"
}

print_output_head() {
    local output="$1"
    local lines="${2:-3}"
    sed -n "1,${lines}p" <<<"$output" | sed 's/^/    /'
}

print_panic_lines() {
    local output="$1"
    awk '/panic:|runtime error:|goroutine/ { print; count++; if (count >= 5) exit }' <<<"$output" | sed 's/^/    /'
}

section() {
    echo ""
    echo -e "${BLUE}── $1 ──${NC}"
}

AO="$REPO_ROOT/cli/bin/ao"

# ═══════════════════════════════════════════════════════
#  Panic / Crash Detection
# ═══════════════════════════════════════════════════════

# Check output for Go panic/crash markers. Returns 0 if clean, 1 if crash detected.
check_no_panic() {
    local output="$1"
    local label="$2"
    if grep -qE '(^panic:|runtime error:|^goroutine [0-9]+ \[)' <<<"$output"; then
        fail "$label — PANIC/CRASH DETECTED"
        print_panic_lines "$output"
        return 1
    fi
    return 0
}

# ═══════════════════════════════════════════════════════
#  Test Helpers
# ═══════════════════════════════════════════════════════

# test_exec: Run a command, assert exit 0, check for panics.
# Usage: test_exec "label" cmd [args...]
test_exec() {
    local label="$1"
    shift
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0  # already counted as FAIL
    fi

    if [[ "$rc" -eq 0 ]]; then
        pass "$label"
    else
        fail "$label (exit $rc)"
        print_output_head "$output" 3
    fi
}

# test_exec_output: Run a command, assert exit 0, assert output matches pattern.
# Usage: test_exec_output "label" "pattern" cmd [args...]
test_exec_output() {
    local label="$1"
    local pattern="$2"
    shift 2
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0
    fi

    if [[ "$rc" -ne 0 ]]; then
        fail "$label (exit $rc)"
        print_output_head "$output" 3
        return 0
    fi

    if output_matches "$output" "$pattern"; then
        pass "$label"
    else
        fail "$label — output missing pattern: $pattern"
        print_output_head "$output" 5
    fi
}

# test_exec_exact: Run a command, assert output exactly equals expected string.
# Usage: test_exec_exact "label" "expected" cmd [args...]
test_exec_exact() {
    local label="$1"
    local expected="$2"
    shift 2
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0
    fi

    if [[ "$rc" -ne 0 ]]; then
        fail "$label (exit $rc)"
        print_output_head "$output" 3
        return 0
    fi

    if [[ "$output" == "$expected" ]]; then
        pass "$label"
    else
        fail "$label — expected exactly '$expected', got '$(sed -n '1p' <<<"$output")'"
    fi
}

# test_help: Run --help, assert exit 0 and output contains Usage.
# Usage: test_help "label" cmd [args...] --help
test_help() {
    local label="$1"
    shift
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0
    fi

    if [[ "$rc" -ne 0 ]]; then
        fail "$label (exit $rc)"
        print_output_head "$output" 3
        return 0
    fi

    if output_matches "$output" '(Usage|usage|Available Commands|Flags)'; then
        pass "$label"
    else
        fail "$label — help output missing Usage/Commands/Flags"
        print_output_head "$output" 5
    fi
}

# test_json: Run a command, assert exit 0 and output is valid JSON.
# Usage: test_json "label" cmd [args...]
test_json() {
    local label="$1"
    shift
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0
    fi

    if [[ "$rc" -ne 0 ]]; then
        fail "$label (exit $rc)"
        print_output_head "$output" 3
        return 0
    fi

    if jq . >/dev/null 2>&1 <<<"$output"; then
        pass "$label"
    else
        fail "$label — output is not valid JSON"
        print_output_head "$output" 3
    fi
}

# test_exec_tolerant: Run a command, accept exit 0 or 1 (some commands exit 1
# for "no data" which is acceptable), but fail on panic or exit >= 2.
# Usage: test_exec_tolerant "label" cmd [args...]
test_exec_tolerant() {
    local label="$1"
    shift
    local output
    local rc=0
    output=$("$@" 2>&1) || rc=$?

    if ! check_no_panic "$output" "$label"; then
        return 0
    fi

    if [[ "$rc" -le 1 ]]; then
        pass "$label"
    else
        fail "$label (exit $rc)"
        print_output_head "$output" 3
    fi
}

# ═══════════════════════════════════════════════════════
#  Build
# ═══════════════════════════════════════════════════════

START_TIME=$(date +%s)

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Release Smoke Test — ao CLI Command Coverage${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"

section "Build"

if [[ "$SKIP_BUILD" == "true" ]]; then
    if [[ -x "$AO" ]]; then
        pass "Using existing binary: $AO"
    else
        fail "Binary not found at $AO — run without --skip-build"
        echo ""
        echo -e "${RED}ABORT: No binary to test${NC}"
        exit 1
    fi
else
    if (cd "$REPO_ROOT/cli" && make build >/dev/null 2>&1); then
        pass "Built ao binary"
    else
        fail "Build failed"
        echo ""
        echo -e "${RED}ABORT: Build failed${NC}"
        exit 1
    fi
fi

# Verify binary is executable
if "$AO" version >/dev/null 2>&1; then
    pass "Binary executes successfully"
else
    fail "Binary fails to execute"
    echo ""
    echo -e "${RED}ABORT: Binary broken${NC}"
    exit 1
fi

# Codex bundle parity removed — skills-codex/ is manually maintained
# test_exec "Codex release bundle parity" bash "$REPO_ROOT/scripts/validate-codex-install-bundle.sh"

# ═══════════════════════════════════════════════════════
#  Exhaustive Leaf Help Smoke
# ═══════════════════════════════════════════════════════

section "All Leaf Commands (--help execution)"

leaf_smoke_log="$(mktemp "${TMPDIR:-/tmp}/ao-leaf-help-smoke.XXXXXX")"
if bash "$REPO_ROOT/tests/cli/test-all-leaf-help-smoke.sh" --skip-build --binary "$AO" >"$leaf_smoke_log" 2>&1; then
    leaf_count=$(grep -E '^Leaf commands:' "$leaf_smoke_log" | awk '{print $3}' || true)
    pass "All generated leaf commands run --help${leaf_count:+ ($leaf_count commands)}"
else
    fail "All generated leaf commands run --help"
    tail -40 "$leaf_smoke_log" | sed 's/^/    /'
fi
rm -f "$leaf_smoke_log"

# ═══════════════════════════════════════════════════════
#  Safe Commands (read-only / reporting — actually execute)
# ═══════════════════════════════════════════════════════

section "Safe Commands (read-only execution)"

# Core info
test_exec_output "ao version" "version|Version" "$AO" version
test_exec_output "ao status" "Status|AgentOps|Initialized" "$AO" status
test_exec_tolerant "ao doctor" "$AO" doctor

# Search
test_exec "ao search 'test'" "$AO" search "test"
test_exec_exact "ao search --json --local nonexistent => []" "[]" "$AO" search --json --local "nonexistent-xyz-12345"

# Knowledge injection
test_exec_tolerant "ao inject" "$AO" inject
test_exec_output "ao inject --index-only" "Knowledge Index|ID|Title" "$AO" inject --index-only

# Lookup
test_exec_tolerant "ao lookup --query 'test'" "$AO" lookup --query "test"
test_exec_tolerant "ao findings list" "$AO" findings list
test_exec_output "ao findings stats" "Total findings|By status|Most cited|Total hits" "$AO" findings stats

# Metrics
test_exec_output "ao metrics health" "sigma|rho|delta|retrieval|citation|decay" "$AO" metrics health
test_exec_output "ao metrics report" "Flywheel|Metrics|Period|decay|retrieval|citation" "$AO" metrics report

# Flywheel
test_exec_output "ao flywheel status" "Flywheel|status|COMPOUNDING|decay|retrieval" "$AO" flywheel status
test_exec_output "ao flywheel close-loop" "Close-Loop|Summary|Pool|promote|Citation" "$AO" flywheel close-loop

# Pool
test_exec_tolerant "ao pool list" "$AO" pool list

# Maturity
test_exec_output "ao maturity --scan" "Maturity|Distribution|Provisional|Candidate|No learnings" "$AO" maturity --scan

# Anti-patterns, constraints, contradict, dedup
test_exec_tolerant "ao anti-patterns" "$AO" anti-patterns
test_exec_tolerant "ao constraint list" "$AO" constraint list
test_exec_tolerant "ao contradict" "$AO" contradict
test_exec_output "ao dedup" "Dedup|Scan|Total|Duplicate|No learnings" "$AO" dedup

# Curate
test_exec_tolerant "ao curate status" "$AO" curate status

# Notebook and memory (quiet mode to avoid state changes)
test_exec "ao notebook update --quiet" "$AO" notebook update --quiet
test_exec_tolerant "ao memory sync --quiet" "$AO" memory sync --quiet

# Trace (help only — requires artifact path arg)
test_help "ao trace --help" "$AO" trace --help
test_help "ao findings --help" "$AO" findings --help

# Goals
test_exec_output "ao goals validate" "VALID|goals|version" "$AO" goals validate
test_exec_output "ao goals measure" "GOAL|RESULT|pass|fail" "$AO" goals measure --timeout 1

# Ratchet
test_exec_output "ao ratchet status" "Ratchet|Chain|Status|STEP" "$AO" ratchet status

# RPI
test_exec_tolerant "ao rpi status" "$AO" rpi status

# Badge
test_exec_output "ao badge" "AGENTOPS|KNOWLEDGE|Sessions|Learnings|Citations" "$AO" badge

# Context
test_exec_tolerant "ao context status" "$AO" context status

# Vibe-check (help only — actual execution takes a while)
test_help "ao vibe-check --help" "$AO" vibe-check --help

# ═══════════════════════════════════════════════════════
#  Help-Only Commands (would modify state — test --help)
# ═══════════════════════════════════════════════════════

section "Help-Only Commands (state-modifying — --help only)"

test_help "ao init --help" "$AO" init --help
test_help "ao seed --help" "$AO" seed --help
test_help "ao demo --help" "$AO" demo --help
test_help "ao quick-start --help" "$AO" quick-start --help
test_help "ao forge --help" "$AO" forge --help
test_help "ao session --help" "$AO" session --help
test_help "ao hooks --help" "$AO" hooks --help
test_help "ao config --help" "$AO" config --help
test_help "ao completion --help" "$AO" completion --help
test_help "ao plans --help" "$AO" plans --help
test_help "ao gate --help" "$AO" gate --help
test_help "ao curate verify --help" "$AO" curate verify --help

# ═══════════════════════════════════════════════════════
#  Subcommand Help Coverage (verify subcommands exist)
# ═══════════════════════════════════════════════════════

section "Subcommand Help Coverage (verify command groups list subcommands)"

test_help "ao goals --help" "$AO" goals --help
test_help "ao ratchet --help" "$AO" ratchet --help
test_help "ao metrics --help" "$AO" metrics --help
test_help "ao pool --help" "$AO" pool --help
test_help "ao constraint --help" "$AO" constraint --help
test_help "ao curate --help" "$AO" curate --help
test_help "ao session --help" "$AO" session --help
test_help "ao rpi --help" "$AO" rpi --help
test_help "ao flywheel --help" "$AO" flywheel --help
test_help "ao maturity --help" "$AO" maturity --help
test_help "ao memory --help" "$AO" memory --help
test_help "ao notebook --help" "$AO" notebook --help
test_help "ao trace --help" "$AO" trace --help
test_help "ao findings --help" "$AO" findings --help

# ═══════════════════════════════════════════════════════
#  Flag Testing (verify key flags produce valid output)
# ═══════════════════════════════════════════════════════

section "Flag Testing (JSON output validation)"

test_json "ao search --json 'test'" "$AO" search --json "test"
test_json "ao search --json nonexistent => valid JSON" "$AO" search --json "nonexistent-xyz-12345"

# Some commands support --json; test where available
if "$AO" doctor --json >/dev/null 2>&1; then
    test_json "ao doctor --json" "$AO" doctor --json
else
    skip "ao doctor --json (flag not supported)"
fi

if "$AO" pool list --json >/dev/null 2>&1; then
    test_json "ao pool list --json" "$AO" pool list --json
else
    skip "ao pool list --json (flag not supported)"
fi

if "$AO" ratchet status --json >/dev/null 2>&1; then
    test_json "ao ratchet status --json" "$AO" ratchet status --json
else
    skip "ao ratchet status --json (flag not supported)"
fi

if "$AO" flywheel status --json >/dev/null 2>&1; then
    test_json "ao flywheel status --json" "$AO" flywheel status --json
else
    skip "ao flywheel status --json (flag not supported)"
fi

if "$AO" metrics health --json >/dev/null 2>&1; then
    test_json "ao metrics health --json" "$AO" metrics health --json
else
    skip "ao metrics health --json (flag not supported)"
fi

if "$AO" constraint list --json >/dev/null 2>&1; then
    test_json "ao constraint list --json" "$AO" constraint list --json
else
    skip "ao constraint list --json (flag not supported)"
fi

if "$AO" findings list --json >/dev/null 2>&1; then
    test_json "ao findings list --json" "$AO" findings list --json
else
    skip "ao findings list --json (flag not supported)"
fi

if "$AO" findings stats --json >/dev/null 2>&1; then
    test_json "ao findings stats --json" "$AO" findings stats --json
else
    skip "ao findings stats --json (flag not supported)"
fi

# ═══════════════════════════════════════════════════════
#  Top-Level Help (catch unregistered commands)
# ═══════════════════════════════════════════════════════

section "Top-Level Help (catch registration issues)"

test_help "ao --help" "$AO" --help

# Verify expected command groups appear in top-level help
TOP_HELP=$("$AO" --help 2>&1)
EXPECTED_COMMANDS=(
    doctor status version completion
    search inject lookup forge trace findings
    rpi ratchet goals session
    flywheel pool metrics gate maturity
    config plans hooks memory notebook
    demo init seed quick-start
    badge constraint contradict dedup curate
    anti-patterns vibe-check extract
)

for cmd in "${EXPECTED_COMMANDS[@]}"; do
    if output_word_matches "$TOP_HELP" "$cmd"; then
        pass "ao --help lists '$cmd'"
    else
        fail "ao --help missing '$cmd'"
    fi
done

# ═══════════════════════════════════════════════════════
#  Summary
# ═══════════════════════════════════════════════════════

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Release Smoke Test Results${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Total:   ${TOTAL}"
echo -e "  ${GREEN}Passed:  ${PASS}${NC}"
echo -e "  ${RED}Failed:  ${FAIL}${NC}"
echo -e "  ${YELLOW}Skipped: ${SKIP}${NC}"
echo -e "  Time:    ${ELAPSED}s"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
    echo -e "${RED}  RELEASE SMOKE TEST FAILED ($FAIL failure(s))${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    exit 1
fi

echo -e "${GREEN}  RELEASE SMOKE TEST PASSED${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
exit 0
