#!/bin/bash
# Smoke test for AgentOps plugin
# Usage: ./tests/smoke-test.sh [--verbose]
# Updated for unified structure (skills/ at repo root)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERBOSE="${1:-}"

# Source shared colors and helpers
source "${SCRIPT_DIR}/lib/colors.sh"

errors=0
warnings=0

# Override fail() and warn() to increment local counters
fail() { echo -e "${RED}  ✗${NC} $1"; ((errors++)) || true; }
warn() { echo -e "${YELLOW}  ⚠${NC} $1"; ((warnings++)) || true; }

cd "$REPO_ROOT"

# =============================================================================
# Test 1: Validate manifests against versioned schemas
# =============================================================================
log "Validating manifests against schemas..."

if "$REPO_ROOT/scripts/validate-manifests.sh" --repo-root "$REPO_ROOT" >/dev/null 2>&1; then
    pass "Manifest schemas valid"
else
    fail "Manifest schema validation failed"
fi

# =============================================================================
# Test 2: Validate skill structure (unified - skills/ at root)
# =============================================================================
log "Validating skill structure..."

skill_count=0
skill_errors=0

for skill_dir in skills/*/; do
    [[ ! -d "$skill_dir" ]] && continue
    skill_name=$(basename "$skill_dir")
    # Skip leading-underscore scaffolding (e.g. skills/_fixtures/) — planted
    # test fixtures, not real skills.
    case "$skill_name" in _*) continue ;; esac
    skill_md="${skill_dir}SKILL.md"

    if [[ -f "$skill_md" ]]; then
        # Check for frontmatter
        if head -1 "$skill_md" | grep -q "^---$"; then
            if grep -q "^name:" "$skill_md"; then
                skill_count=$((skill_count + 1))
            else
                fail "$skill_name - missing 'name' in frontmatter"
                skill_errors=$((skill_errors + 1))
            fi
        else
            fail "$skill_name - no YAML frontmatter"
            skill_errors=$((skill_errors + 1))
        fi
    else
        fail "$skill_name/SKILL.md missing"
        skill_errors=$((skill_errors + 1))
    fi
done

if [[ $skill_errors -eq 0 ]] && [[ $skill_count -gt 0 ]]; then
    pass "All $skill_count skills have valid SKILL.md"
fi

# =============================================================================
# Test 2b: Validate the optional council strategy contract
# =============================================================================
log "Validating council strategy contract..."

if [[ -x "$REPO_ROOT/tests/skills/validate-skill.sh" ]]; then
    if bash "$REPO_ROOT/tests/skills/validate-skill.sh" council "$REPO_ROOT/skills" > /tmp/validate-council.log 2>&1; then
        pass "council validate-skill checks passed"
    else
        fail "council validate-skill checks failed"
        tail -n 30 /tmp/validate-council.log | sed 's/^/    /'
    fi
else
    warn "tests/skills/validate-skill.sh missing or not executable"
fi

# =============================================================================
# Test 3: Validate agents structure
# =============================================================================
log "Validating agents structure..."

agent_count=0
[[ -d "agents" ]] && agent_count=$(find agents -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

if [[ $agent_count -gt 0 ]]; then
    pass "Found $agent_count agents"
else
    warn "No agents found (optional)"
fi

# =============================================================================
# Test 4: Validate CLI builds (if Go available)
# =============================================================================
log "Validating ao CLI..."

if [[ -d "cli" ]]; then
    if command -v go &>/dev/null; then
        tmpdir="$(mktemp -d -t ao-test.XXXXXX)" || { fail "mktemp failed"; tmpdir=""; }
        trap '[ -n "$tmpdir" ] && [ "$tmpdir" != "/" ] && rm -rf "$tmpdir"' EXIT
        tmpbin="$tmpdir/ao"
        if (cd "$REPO_ROOT/cli" && go build -o "$tmpbin" ./cmd/ao 2>/dev/null); then
            pass "ao CLI builds successfully"
        else
            fail "ao CLI build failed"
        fi
    else
        warn "Go not available - skipping CLI build test"
    fi
else
    warn "No cli/ directory"
fi

# =============================================================================
# Test 5b: Runtime smoke surfaces
# =============================================================================
log "Validating runtime smoke surfaces..."

for runtime_test in \
    "$REPO_ROOT/tests/skills/test-runtime-claude-code-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-codex-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-cursor-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-opencode-smoke.sh"; do
    [[ -f "$runtime_test" ]] || continue
    test_name="$(basename "$runtime_test")"
    if bash "$runtime_test" >"/tmp/${test_name}.log" 2>&1; then
        pass "$test_name passed"
    else
        fail "$test_name failed"
        [[ "$VERBOSE" == "--verbose" ]] && tail -30 "/tmp/${test_name}.log" | sed 's/^/    /'
    fi
done

# =============================================================================
# Test 6: Check for placeholder patterns
# =============================================================================
log "Checking for issues..."

# Check for [your-email] placeholders
placeholder_files=$(grep -rl "\[your-email\]" --include="*.md" . 2>/dev/null | wc -l | tr -d ' ') || true
if [[ "${placeholder_files:-0}" -gt 0 ]]; then
    fail "$placeholder_files files with [your-email] placeholders"
    [[ "$VERBOSE" == "--verbose" ]] && grep -rn "\[your-email\]" --include="*.md" . 2>/dev/null | head -3 || true
else
    pass "No placeholder emails"
fi

# Check for TODO/FIXME in skills
todo_files=$(grep -lE "(TODO|FIXME):" skills/*/SKILL.md 2>/dev/null | wc -l | tr -d ' ') || true
if [[ "${todo_files:-0}" -gt 0 ]]; then
    warn "$todo_files skills with TODO/FIXME"
else
    pass "No TODO/FIXME in skills"
fi

# =============================================================================
# Test 7: Optional Claude CLI load test
# =============================================================================
if [[ "${AGENTOPS_EXERCISE_CLAUDE_RUNTIME:-0}" == "1" ]] && command -v claude &>/dev/null; then
    log "Testing explicitly requested Claude CLI plugin load..."
    load_output=$(timeout 10 claude --plugin-dir . --help 2>&1) || true
    if echo "$load_output" | grep -qiE "invalid manifest|validation error|failed to load"; then
        fail "Claude CLI load failed"
        echo "$load_output" | grep -iE "invalid|failed|error" | head -3 | sed 's/^/    /'
    else
        pass "Claude CLI loads plugin"
    fi
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"

if [[ $errors -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $errors errors, $warnings warnings"
    exit 1
elif [[ $warnings -gt 0 ]]; then
    echo -e "${YELLOW}PASSED WITH WARNINGS${NC} - $warnings warnings"
    exit 0
else
    echo -e "${GREEN}PASSED${NC} - All smoke tests passed"
    exit 0
fi
