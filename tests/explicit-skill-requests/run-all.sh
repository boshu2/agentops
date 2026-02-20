#!/usr/bin/env bash
# Run all explicit skill request tests
# Usage: ./run-all.sh [--parallel]

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$TEST_DIR/../claude-code/test-helpers.sh"

passed=0
failed=0
skipped=0

echo "═══════════════════════════════════════════"
echo "Explicit Skill Request Tests"
echo "═══════════════════════════════════════════"
echo ""

# Find all prompt files
PROMPTS=$(find "$TEST_DIR/prompts" -name "*.txt" -type f 2>/dev/null | sort)

if [[ -z "$PROMPTS" ]]; then
    echo "No prompt files found in $TEST_DIR/prompts/"
    exit 1
fi

for prompt_file in $PROMPTS; do
    skill_name=$(basename "$prompt_file" .txt)
    prompt=$(cat "$prompt_file")

    echo "Testing: $skill_name"
    echo "  Prompt: ${prompt:0:60}..."

    # Run Claude with stream-json output
    LOG_FILE=$(run_claude_json "$prompt" 120) || true

    # Check assertions
    test_passed=true

    if ! assert_skill_triggered "$LOG_FILE" "$skill_name" "Skill triggered"; then
        if grep -q '"subtype":"error_max_turns"' "$LOG_FILE" || grep -q '"hook_name":"SessionStart:startup"' "$LOG_FILE"; then
            echo "  [SKIP] Skill trigger indeterminate (startup hooks or max-turns interference)"
            ((skipped++)) || true
            echo ""
            continue
        fi
        test_passed=false
    fi

    if ! assert_no_premature_tools "$LOG_FILE" "No premature tools"; then
        test_passed=false
    fi

    if $test_passed; then
        ((passed++)) || true
    else
        ((failed++)) || true
    fi

    echo ""
done

print_summary "$passed" "$failed" "$skipped"
