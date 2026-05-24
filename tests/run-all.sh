#!/usr/bin/env bash
# Master test runner for AgentOps plugin
# Runs all test tiers based on flag
#
# Usage:
#   ./tests/run-all.sh           # Run tier 1 (fast) only
#   ./tests/run-all.sh --tier=2  # Run tier 1 + tier 2 (smoke tests)
#   ./tests/run-all.sh --tier=3  # Run tier 1 + 2 + 3 (functional tests)
#   ./tests/run-all.sh --all     # Run all tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source shared colors and helpers
# shellcheck source=tests/lib/colors.sh
source "${SCRIPT_DIR}/lib/colors.sh"
# shellcheck source=tests/lib/run-with-timeout.sh
source "${SCRIPT_DIR}/lib/run-with-timeout.sh"

TIER="${1:-}"
total_passed=0
total_failed=0
total_skipped=0

RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS="${RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS:-120}"
RUN_ALL_CLAUDE_HELP_TIMEOUT_SECONDS="${RUN_ALL_CLAUDE_HELP_TIMEOUT_SECONDS:-10}"
RUN_ALL_SMOKE_TIMEOUT_SECONDS="${RUN_ALL_SMOKE_TIMEOUT_SECONDS:-300}"
RUN_ALL_CODEX_INTEGRATION_TIMEOUT_SECONDS="${RUN_ALL_CODEX_INTEGRATION_TIMEOUT_SECONDS:-600}"
RUN_ALL_FUNCTIONAL_LANE_TIMEOUT_SECONDS="${RUN_ALL_FUNCTIONAL_LANE_TIMEOUT_SECONDS:-600}"
RUN_ALL_RELEASE_SMOKE_TIMEOUT_SECONDS="${RUN_ALL_RELEASE_SMOKE_TIMEOUT_SECONDS:-900}"
RUN_ALL_INTEGRATION_SCRIPT_TIMEOUT_SECONDS="${RUN_ALL_INTEGRATION_SCRIPT_TIMEOUT_SECONDS:-300}"

# Override helpers to increment local counters
pass() { echo -e "${GREEN}  ✓${NC} $1"; ((total_passed++)) || true; }
fail() { echo -e "${RED}  ✗${NC} $1"; ((total_failed++)) || true; }
skip() { echo -e "${YELLOW}  ⊘${NC} $1 (skipped)"; ((total_skipped++)) || true; }

lane_log_file() {
    local name="${1//[^A-Za-z0-9_.-]/-}"
    echo "/tmp/agentops-run-all-${name}.log"
}

report_lane_status() {
    local label="${1:?label required}"
    local status="${2:?status required}"
    local log_file="${3:?log file required}"
    local timeout_seconds="${4:?timeout required}"

    if [[ "$status" -eq 124 ]]; then
        fail "$label timed out after ${timeout_seconds}s"
    else
        fail "$label"
    fi

    if [[ -s "$log_file" ]]; then
        tail -20 "$log_file" | sed 's/^/    /'
    fi
}

run_lane() {
    local label="${1:?label required}"
    local timeout_seconds="${2:?timeout required}"
    local log_file="${3:?log file required}"
    shift 3

    local status
    if run_with_timeout "$timeout_seconds" "$label" "$log_file" "$@"; then
        pass "$label"
    else
        status=$?
        report_lane_status "$label" "$status" "$log_file" "$timeout_seconds"
    fi
}

echo ""
echo "═══════════════════════════════════════════"
echo "AgentOps Plugin Test Suite"
echo "═══════════════════════════════════════════"
echo ""

# =============================================================================
# Tier 1: Static Validation (fast, no Claude CLI needed)
# =============================================================================
log "Tier 1: Static Validation"

# Validate manifests against canonical schemas
run_lane "Manifest schema validation" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file manifests)" \
    bash "$REPO_ROOT/scripts/validate-manifests.sh" --repo-root "$REPO_ROOT"

# Validate JSON files
for jf in \
    "$REPO_ROOT/.claude-plugin/plugin.json" \
    "$REPO_ROOT/.codex-plugin/plugin.json" \
    "$REPO_ROOT/plugins/marketplace.json"
do
    if [[ ! -f "$jf" ]]; then
        fail "$(basename "$jf") - not found"
        continue
    fi
    if python3 -m json.tool "$jf" > /dev/null 2>&1; then
        pass "$(basename "$jf") valid"
    else
        fail "$(basename "$jf") - invalid JSON"
    fi
done

# Skill structure validation deferred to smoke-test.sh and validate-doc-release.sh (CI-active)

# Validate agents
agent_count=0
[[ -d "$REPO_ROOT/agents" ]] && agent_count=$(find "$REPO_ROOT/agents" -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
if [[ $agent_count -gt 0 ]]; then
    pass "Found $agent_count agents"
else
    skip "No agents found (optional)"
fi

# Validate GOALS.yaml
if [[ -f "$SCRIPT_DIR/goals/validate-goals.sh" ]]; then
    run_lane "GOALS.yaml validation" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file goals)" \
        bash "$SCRIPT_DIR/goals/validate-goals.sh"
else
    skip "GOALS.yaml validation (script not found)"
fi

# Validate documentation
if [[ -f "$SCRIPT_DIR/docs/validate-links.sh" ]]; then
    run_lane "Doc link validation" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file doc-links)" \
        bash "$SCRIPT_DIR/docs/validate-links.sh"
else
    skip "Doc link validation (script not found)"
fi

# Skill count validation deferred to validate-doc-release.sh (CI-active, runs validate-skill-count.sh + sync-skill-counts.sh)

if [[ -f "$SCRIPT_DIR/docs/validate-goal-count.sh" ]]; then
    run_lane "Doc goal count validation" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file doc-goal-count)" \
        bash "$SCRIPT_DIR/docs/validate-goal-count.sh"
else
    skip "Doc goal count validation (script not found)"
fi

# Validate token budgets (static, no CLI needed)
if [[ -f "$SCRIPT_DIR/skills/test-token-budgets.sh" ]]; then
    run_lane "Token budget validation" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file token-budgets)" \
        bash "$SCRIPT_DIR/skills/test-token-budgets.sh"
else
    skip "Token budget validation (script not found)"
fi

# Validate artifact-consistency behavior (static, no CLI needed)
if [[ -f "$SCRIPT_DIR/skills/test-artifact-consistency.sh" ]]; then
    run_lane "Artifact consistency behavior tests" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "$(lane_log_file artifact-consistency)" \
        bash "$SCRIPT_DIR/skills/test-artifact-consistency.sh"
else
    skip "Artifact consistency behavior tests (script not found)"
fi

# Validate OL integration fixture-only scripts (static, no real ol binary needed)
if [[ -d "$SCRIPT_DIR/ol-integration" ]]; then
    for ol_test in "$SCRIPT_DIR"/ol-integration/*-ol-test.sh; do
        [[ ! -f "$ol_test" ]] && continue
        ol_name="$(basename "$ol_test" .sh)"
        run_lane "$ol_name" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" "/tmp/${ol_name}.log" bash "$ol_test"
    done
else
    skip "OL integration tests (directory not found)"
fi

# Validate team-runner fixture scripts (static, no live runtime needed)
if [[ -d "$SCRIPT_DIR/team-runner" ]]; then
    run_lane "Team runner fixture tests" "$RUN_ALL_STATIC_LANE_TIMEOUT_SECONDS" /tmp/team-runner-tests.log \
        bash "$SCRIPT_DIR/team-runner/run-all.sh"
else
    skip "Team runner tests (directory not found)"
fi

echo ""

# =============================================================================
# Tier 2: Smoke Tests (needs Claude CLI, fast)
# =============================================================================
if [[ "$TIER" == "--tier=2" ]] || [[ "$TIER" == "--tier=3" ]] || [[ "$TIER" == "--all" ]]; then
    log "Tier 2: Smoke Tests"

    if ! command -v claude &>/dev/null; then
        skip "Claude CLI not available - skipping Claude load smoke"
    else
        # Test plugin loads
        claude_load_log="$(lane_log_file claude-load)"
        if run_with_timeout "$RUN_ALL_CLAUDE_HELP_TIMEOUT_SECONDS" "Claude CLI loads plugin" "$claude_load_log" \
            claude --plugin-dir "$REPO_ROOT" --help; then
            claude_load_status=0
        else
            claude_load_status=$?
        fi

        if [[ "$claude_load_status" -eq 124 ]]; then
            report_lane_status "Claude CLI load smoke" "$claude_load_status" "$claude_load_log" "$RUN_ALL_CLAUDE_HELP_TIMEOUT_SECONDS"
        elif grep -qiE "invalid manifest|validation error|failed to load" "$claude_load_log"; then
            fail "Claude CLI failed to load plugin"
            grep -iE "invalid|failed|error" "$claude_load_log" | head -3 | sed 's/^/    /'
        elif [[ "$claude_load_status" -ne 0 ]]; then
            report_lane_status "Claude CLI load smoke failed" "$claude_load_status" "$claude_load_log" "$RUN_ALL_CLAUDE_HELP_TIMEOUT_SECONDS"
        else
            pass "Claude CLI loads plugin"
        fi
    fi

    # Run smoke-test.sh if exists
    if [[ -f "$SCRIPT_DIR/smoke-test.sh" ]]; then
        run_lane "smoke-test.sh" "$RUN_ALL_SMOKE_TIMEOUT_SECONDS" "$(lane_log_file smoke-test)" \
            bash "$SCRIPT_DIR/smoke-test.sh"
    fi

    # Run Codex integration tests (requires Codex CLI)
    if [[ -f "$SCRIPT_DIR/codex/integration/run-all.sh" ]]; then
        if command -v codex &>/dev/null; then
            log "  Running Codex integration tests..."
            run_lane "Codex integration tests" "$RUN_ALL_CODEX_INTEGRATION_TIMEOUT_SECONDS" /tmp/codex-tests.log \
                bash "$SCRIPT_DIR/codex/integration/run-all.sh"
        else
            skip "Codex CLI not available - skipping Codex integration tests"
        fi
    fi

    echo ""
fi

# =============================================================================
# Tier 3: Functional Tests (needs Claude CLI, slower)
# =============================================================================
if [[ "$TIER" == "--tier=3" ]] || [[ "$TIER" == "--all" ]]; then
    log "Tier 3: Functional Tests"

    if ! command -v claude &>/dev/null; then
        skip "Claude CLI not available - skipping functional tests"
    else
        # Run explicit skill request tests
        if [[ -d "$SCRIPT_DIR/explicit-skill-requests" ]]; then
            log "  Running explicit skill request tests..."
            run_lane "Explicit skill request tests" "$RUN_ALL_FUNCTIONAL_LANE_TIMEOUT_SECONDS" /tmp/explicit-tests.log \
                bash "$SCRIPT_DIR/explicit-skill-requests/run-all.sh"
        fi

        # Run natural language triggering tests
        if [[ -d "$SCRIPT_DIR/skill-triggering" ]]; then
            log "  Running skill triggering tests..."
            run_lane "Skill triggering tests" "$RUN_ALL_FUNCTIONAL_LANE_TIMEOUT_SECONDS" /tmp/triggering-tests.log \
                bash "$SCRIPT_DIR/skill-triggering/run-all.sh"
        fi

        # Run claude-code unit tests
        if [[ -d "$SCRIPT_DIR/claude-code" ]]; then
            log "  Running Claude Code unit tests..."
            run_lane "Claude Code unit tests" "$RUN_ALL_FUNCTIONAL_LANE_TIMEOUT_SECONDS" /tmp/unit-tests.log \
                bash "$SCRIPT_DIR/claude-code/run-all.sh"
        fi

        # Run release smoke tests (agents + skills)
        if [[ -f "$SCRIPT_DIR/release-smoke-test.sh" ]]; then
            log "  Running release smoke tests..."
            run_lane "Release smoke tests" "$RUN_ALL_RELEASE_SMOKE_TIMEOUT_SECONDS" /tmp/release-tests.log \
                bash "$SCRIPT_DIR/release-smoke-test.sh"
        fi

        # Run integration tests (CLI commands, skill invocation, hook chain)
        if [[ -d "$SCRIPT_DIR/integration" ]]; then
            log "  Running integration tests..."
            for test_script in "$SCRIPT_DIR"/integration/test-*.sh; do
                [[ ! -f "$test_script" ]] && continue
                test_name="$(basename "$test_script" .sh)"

                # Skip skill invocation tests if Claude CLI not available
                if [[ "$test_name" == "test-skill-invocation" ]] && ! command -v claude &>/dev/null; then
                    skip "$test_name (claude CLI not available)"
                    continue
                fi

                # Skip CLI tests if Go not available
                if [[ "$test_name" == "test-cli-commands" ]] && ! command -v go &>/dev/null; then
                    skip "$test_name (go not available)"
                    continue
                fi

                run_lane "$test_name" "$RUN_ALL_INTEGRATION_SCRIPT_TIMEOUT_SECONDS" "/tmp/${test_name}.log" \
                    bash "$test_script"
            done
        fi
    fi

    echo ""
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
total=$((total_passed + total_failed + total_skipped))
echo -e "Total: $total tests"
echo -e "  ${GREEN}Passed:${NC}  $total_passed"
echo -e "  ${RED}Failed:${NC}  $total_failed"
echo -e "  ${YELLOW}Skipped:${NC} $total_skipped"
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $total_failed -gt 0 ]]; then
    exit 1
fi
exit 0
