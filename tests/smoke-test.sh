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
# Test 2b: Validate council/crank flag allowlists and contracts
# =============================================================================
log "Validating council/crank flag allowlists..."

if [[ -x "$REPO_ROOT/tests/skills/validate-skill.sh" ]]; then
    if bash "$REPO_ROOT/tests/skills/validate-skill.sh" council "$REPO_ROOT/skills" > /tmp/validate-council.log 2>&1; then
        pass "council validate-skill checks passed"
    else
        fail "council validate-skill checks failed"
        tail -n 30 /tmp/validate-council.log | sed 's/^/    /'
    fi

    if bash "$REPO_ROOT/tests/skills/validate-skill.sh" crank "$REPO_ROOT/skills" > /tmp/validate-crank.log 2>&1; then
        pass "crank validate-skill checks passed"
    else
        fail "crank validate-skill checks failed"
        tail -n 30 /tmp/validate-crank.log | sed 's/^/    /'
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
# Test 5: Validate hook preflight checklist
# =============================================================================
log "Validating hook preflight checklist..."

if [[ -x "scripts/validate-hook-preflight.sh" ]]; then
    preflight_output=""
    if preflight_output=$(./scripts/validate-hook-preflight.sh 2>&1); then
        pass "Hook preflight validation passed"
    else
        fail "Hook preflight validation failed"
        [[ "$VERBOSE" == "--verbose" ]] && echo "$preflight_output" | sed 's/^/    /'
    fi
else
    fail "scripts/validate-hook-preflight.sh missing or not executable"
fi

# =============================================================================
# Test 5b: Runtime smoke surfaces
# =============================================================================
log "Validating runtime smoke surfaces..."

for runtime_test in \
    "$REPO_ROOT/tests/skills/test-runtime-claude-code-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-codex-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-cursor-smoke.sh" \
    "$REPO_ROOT/tests/skills/test-runtime-opencode-smoke.sh" \
    "$REPO_ROOT/tests/scripts/test-headless-runtime-skills.sh"; do
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
# Test 7: Claude CLI load test
# =============================================================================
log "Testing Claude CLI plugin load..."

if command -v claude &>/dev/null; then
    load_output=$(timeout 10 claude --plugin-dir . --help 2>&1) || true
    if echo "$load_output" | grep -qiE "invalid manifest|validation error|failed to load"; then
        fail "Claude CLI load failed"
        echo "$load_output" | grep -iE "invalid|failed|error" | head -3 | sed 's/^/    /'
    else
        pass "Claude CLI loads plugin"
    fi
else
    warn "Claude CLI not available for load test"
fi

# =============================================================================
# Test 8: Flywheel loop (next-work round-trip)
# =============================================================================
log "Testing flywheel loop (next-work round-trip)..."

NEXTWORK_DIR="$REPO_ROOT/.agents/rpi"
NEXTWORK_FILE="$NEXTWORK_DIR/next-work.jsonl"
NEXTWORK_SCHEMA="$REPO_ROOT/docs/contracts/next-work.schema.md"

# Check schema contract exists
if [[ -f "$NEXTWORK_SCHEMA" ]]; then
    pass "next-work.schema.md exists"
else
    fail "next-work.schema.md missing"
fi

if [[ -x "$REPO_ROOT/scripts/validate-next-work-contract-parity.sh" ]]; then
    if "$REPO_ROOT/scripts/validate-next-work-contract-parity.sh" >/dev/null 2>&1; then
        pass "next-work contract parity validator passed"
    else
        fail "next-work contract parity validator failed"
    fi
else
    fail "scripts/validate-next-work-contract-parity.sh missing or not executable"
fi

# Validate existing next-work.jsonl if present
if [[ -f "$NEXTWORK_FILE" ]]; then
    if command -v jq &>/dev/null; then
        # Validate each line is valid JSON with required fields
        line_num=0
        nw_errors=0
        while IFS= read -r line; do
            ((line_num++)) || true
            [[ -z "$line" ]] && continue
            if ! echo "$line" | jq -e '
                .source_epic and
                ((.timestamp | type == "string") or (.created_at | type == "string")) and
                (.consumed | type == "boolean") and
                (
                  ((.items | type) == "array") or
                  ((.title | type == "string") and (.type | type == "string") and (.severity | type == "string"))
                )
            ' >/dev/null 2>&1; then
                fail "next-work.jsonl line $line_num: missing required fields for batch or legacy flat schema"
                ((nw_errors++)) || true
            fi
            # Validate optional target_repo is a string when present
            if echo "$line" | jq -e 'has("target_repo")' >/dev/null 2>&1; then
                if ! echo "$line" | jq -e '(.target_repo | type) == "string"' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: target_repo must be a string"
                    ((nw_errors++)) || true
                fi
            fi
            if echo "$line" | jq -e 'has("items") and (.items | type == "array")' >/dev/null 2>&1; then
                if ! echo "$line" | jq -e 'all(.items[]?; (has("target_repo") | not) or (.target_repo | type == "string"))' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: item target_repo must be a string"
                    ((nw_errors++)) || true
                fi
                if ! echo "$line" | jq -e 'all(.items[]?; (has("claim_status") | not) or (.claim_status == "available" or .claim_status == "in_progress" or .claim_status == "consumed"))' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: item claim_status must be available|in_progress|consumed"
                    ((nw_errors++)) || true
                fi
                if ! echo "$line" | jq -e 'all(.items[]?; (has("claimed_by") | not) or (.claimed_by == null) or ((.claimed_by | type) == "string"))' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: item claimed_by must be null or string"
                    ((nw_errors++)) || true
                fi
                if ! echo "$line" | jq -e 'all(.items[]?; (has("claimed_at") | not) or (.claimed_at == null) or ((.claimed_at | type) == "string"))' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: item claimed_at must be null or string"
                    ((nw_errors++)) || true
                fi
            fi
            if echo "$line" | jq -e 'has("claim_status")' >/dev/null 2>&1; then
                if ! echo "$line" | jq -e '(.claim_status == "available" or .claim_status == "in_progress" or .claim_status == "consumed")' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: claim_status must be available|in_progress|consumed"
                    ((nw_errors++)) || true
                fi
            fi
            if echo "$line" | jq -e 'has("claimed_by")' >/dev/null 2>&1; then
                if ! echo "$line" | jq -e '(.claimed_by == null) or ((.claimed_by | type) == "string")' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: claimed_by must be null or string"
                    ((nw_errors++)) || true
                fi
            fi
            if echo "$line" | jq -e 'has("claimed_at")' >/dev/null 2>&1; then
                if ! echo "$line" | jq -e '(.claimed_at == null) or ((.claimed_at | type) == "string")' >/dev/null 2>&1; then
                    fail "next-work.jsonl line $line_num: claimed_at must be null or string"
                    ((nw_errors++)) || true
                fi
            fi
        done < "$NEXTWORK_FILE"
        if [[ $nw_errors -eq 0 ]]; then
            pass "next-work.jsonl: all $line_num entries have valid schema"
        fi
    else
        warn "jq not available — skipping next-work.jsonl schema validation"
    fi
else
    # Not an error — file only exists after first post-mortem with Step 8
    if [[ "$VERBOSE" == "--verbose" ]]; then
        log "  next-work.jsonl not present (expected before first flywheel cycle)"
    fi
    pass "next-work.jsonl absent (pre-flywheel state is valid)"
fi

# =============================================================================
# Test 9: RPI context-windowing contract (large-repo mode)
# =============================================================================
log "Testing RPI context-windowing contract..."

if [[ -x "scripts/rpi/context-window-contract.sh" ]]; then
    cw_output=""
    if cw_output=$(./scripts/rpi/context-window-contract.sh 2>&1); then
        pass "RPI context-window contract passed"
    else
        fail "RPI context-window contract failed"
        [[ "$VERBOSE" == "--verbose" ]] && echo "$cw_output" | sed 's/^/    /'
    fi
else
    fail "scripts/rpi/context-window-contract.sh missing or not executable"
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
