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
source "${SCRIPT_DIR}/lib/colors.sh"

TIER="${1:-}"
total_passed=0
total_failed=0
total_skipped=0

# Override helpers to increment local counters
pass() { echo -e "${GREEN}  ✓${NC} $1"; ((total_passed++)) || true; }
fail() { echo -e "${RED}  ✗${NC} $1"; ((total_failed++)) || true; }
skip() { echo -e "${YELLOW}  ⊘${NC} $1 (skipped)"; ((total_skipped++)) || true; }

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
if bash "$REPO_ROOT/scripts/validate-manifests.sh" --repo-root "$REPO_ROOT" > /dev/null 2>&1; then
    pass "Manifest schema validation"
else
    fail "Manifest schema validation"
fi

# Validate JSON files
for jf in \
    "$REPO_ROOT/.claude-plugin/plugin.json" \
    "$REPO_ROOT/.codex-plugin/plugin.json" \
    "$REPO_ROOT/plugins/marketplace.json" \
    "$REPO_ROOT/hooks/hooks.json"
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
    if bash "$SCRIPT_DIR/goals/validate-goals.sh" > /dev/null 2>&1; then
        pass "GOALS.yaml validation"
    else
        fail "GOALS.yaml validation"
    fi
else
    skip "GOALS.yaml validation (script not found)"
fi

# Validate documentation
if [[ -f "$SCRIPT_DIR/docs/validate-links.sh" ]]; then
    if bash "$SCRIPT_DIR/docs/validate-links.sh" > /dev/null 2>&1; then
        pass "Doc link validation"
    else
        fail "Doc link validation"
    fi
else
    skip "Doc link validation (script not found)"
fi

# Skill count validation deferred to validate-doc-release.sh (CI-active, runs validate-skill-count.sh + sync-skill-counts.sh)

if [[ -f "$SCRIPT_DIR/docs/validate-goal-count.sh" ]]; then
    if bash "$SCRIPT_DIR/docs/validate-goal-count.sh" > /dev/null 2>&1; then
        pass "Doc goal count validation"
    else
        fail "Doc goal count validation"
    fi
else
    skip "Doc goal count validation (script not found)"
fi

# Validate token budgets (static, no CLI needed)
if [[ -f "$SCRIPT_DIR/skills/test-token-budgets.sh" ]]; then
    if bash "$SCRIPT_DIR/skills/test-token-budgets.sh" > /dev/null 2>&1; then
        pass "Token budget validation"
    else
        fail "Token budget validation"
    fi
else
    skip "Token budget validation (script not found)"
fi

# Validate artifact-consistency behavior (static, no CLI needed)
if [[ -f "$SCRIPT_DIR/skills/test-artifact-consistency.sh" ]]; then
    if bash "$SCRIPT_DIR/skills/test-artifact-consistency.sh" > /dev/null 2>&1; then
        pass "Artifact consistency behavior tests"
    else
        fail "Artifact consistency behavior tests"
    fi
else
    skip "Artifact consistency behavior tests (script not found)"
fi

# Validate OL integration fixture-only scripts (static, no real ol binary needed)
if [[ -d "$SCRIPT_DIR/ol-integration" ]]; then
    for ol_test in "$SCRIPT_DIR"/ol-integration/*-ol-test.sh; do
        [[ ! -f "$ol_test" ]] && continue
        ol_name="$(basename "$ol_test" .sh)"
        if bash "$ol_test" > "/tmp/${ol_name}.log" 2>&1; then
            pass "$ol_name"
        else
            fail "$ol_name"
            tail -20 "/tmp/${ol_name}.log" | sed 's/^/    /'
        fi
    done
else
    skip "OL integration tests (directory not found)"
fi

# Validate team-runner fixture scripts (static, no live runtime needed)
if [[ -d "$SCRIPT_DIR/team-runner" ]]; then
    if bash "$SCRIPT_DIR/team-runner/run-all.sh" > /tmp/team-runner-tests.log 2>&1; then
        pass "Team runner fixture tests"
    else
        fail "Team runner fixture tests"
        tail -20 /tmp/team-runner-tests.log | sed 's/^/    /'
    fi
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
        skip "Claude CLI not available"
    else
        # Test plugin loads
        load_output=$(timeout 10 claude --plugin-dir "$REPO_ROOT" --help 2>&1) || true
        if echo "$load_output" | grep -qiE "invalid manifest|validation error|failed to load"; then
            fail "Claude CLI failed to load plugin"
            echo "$load_output" | grep -iE "invalid|failed|error" | head -3 | sed 's/^/    /'
        else
            pass "Claude CLI loads plugin"
        fi
    fi

    # Run smoke-test.sh if exists
    if [[ -f "$SCRIPT_DIR/smoke-test.sh" ]]; then
        if bash "$SCRIPT_DIR/smoke-test.sh" > /dev/null 2>&1; then
            pass "smoke-test.sh passed"
        else
            fail "smoke-test.sh failed"
        fi
    fi

    # Run Codex integration tests (requires Codex CLI)
    if [[ -f "$SCRIPT_DIR/codex/integration/run-all.sh" ]]; then
        if command -v codex &>/dev/null; then
            log "  Running Codex integration tests..."
            if bash "$SCRIPT_DIR/codex/integration/run-all.sh" > /tmp/codex-tests.log 2>&1; then
                pass "Codex integration tests"
            else
                fail "Codex integration tests"
                tail -20 /tmp/codex-tests.log | sed 's/^/    /'
            fi
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
            if bash "$SCRIPT_DIR/explicit-skill-requests/run-all.sh" > /tmp/explicit-tests.log 2>&1; then
                pass "Explicit skill request tests"
            else
                fail "Explicit skill request tests"
                tail -20 /tmp/explicit-tests.log | sed 's/^/    /'
            fi
        fi

        # Run natural language triggering tests
        if [[ -d "$SCRIPT_DIR/skill-triggering" ]]; then
            log "  Running skill triggering tests..."
            if bash "$SCRIPT_DIR/skill-triggering/run-all.sh" > /tmp/triggering-tests.log 2>&1; then
                pass "Skill triggering tests"
            else
                fail "Skill triggering tests"
                tail -20 /tmp/triggering-tests.log | sed 's/^/    /'
            fi
        fi

        # Run claude-code unit tests
        if [[ -d "$SCRIPT_DIR/claude-code" ]]; then
            log "  Running Claude Code unit tests..."
            if bash "$SCRIPT_DIR/claude-code/run-all.sh" > /tmp/unit-tests.log 2>&1; then
                pass "Claude Code unit tests"
            else
                fail "Claude Code unit tests"
                tail -20 /tmp/unit-tests.log | sed 's/^/    /'
            fi
        fi

        # Run release smoke tests (agents + skills)
        if [[ -f "$SCRIPT_DIR/release-smoke-test.sh" ]]; then
            log "  Running release smoke tests..."
            if bash "$SCRIPT_DIR/release-smoke-test.sh" > /tmp/release-tests.log 2>&1; then
                pass "Release smoke tests"
            else
                fail "Release smoke tests"
                tail -30 /tmp/release-tests.log | sed 's/^/    /'
            fi
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

                if bash "$test_script" > "/tmp/${test_name}.log" 2>&1; then
                    pass "$test_name"
                else
                    fail "$test_name"
                    tail -20 "/tmp/${test_name}.log" | sed 's/^/    /'
                fi
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
