#!/usr/bin/env bash
# Test: standards skill
# Verifies the standards library skill works correctly
# Tests language detection and standards loading
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Standards skill queries need extra turns to explore reference files
export MAX_TURNS="${MAX_TURNS:-5}"

source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: standards skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the standards skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "standards" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "language\|coding\|reference\|library" "Describes standards purpose"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify language detection
echo "Test 2: Language detection..."

output=$(run_claude "What programming languages does the standards skill provide standards for?" 45)

if assert_contains "$output" "python\|Python" "Python mentioned"; then
    :
else
    exit 1
fi

if assert_contains "$output" "go\|Go\|golang" "Go mentioned"; then
    :
else
    exit 1
fi

if assert_contains "$output" "rust\|Rust" "Rust mentioned"; then
    :
else
    exit 1
fi

echo ""

# Test 3: Verify library skill nature
echo "Test 3: Library skill nature..."

output=$(run_claude "Is the standards skill standalone or a library that other skills use?" 60)

if assert_contains "$output" "library\|depend\|other\|vibe\|implement" "Describes library nature"; then
    :
else
    exit 1
fi

echo ""

echo "=== All standards skill tests passed ==="
