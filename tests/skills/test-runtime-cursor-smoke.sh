#!/usr/bin/env bash
# Test: Cursor runtime smoke - validates AgentOps can export skills as Cursor rules.
# Proof tier: Tier S structural/export smoke. No live Cursor runtime is required.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "=== Cursor Runtime Smoke Tests ==="
echo "Proof tier: Tier S structural/export smoke"
echo ""

CONVERTER="$REPO_ROOT/skills/converter/scripts/convert.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ -f "$CONVERTER" ]]; then
    bash -n "$CONVERTER" && pass "convert.sh syntax valid" || fail "convert.sh syntax invalid"
else
    fail "converter script not found at $CONVERTER"
fi

if bash "$CONVERTER" "$REPO_ROOT/skills/converter" cursor "$TMP_DIR" >/dev/null 2>&1; then
    pass "converter exports a skill to Cursor format"
else
    fail "converter failed to export Cursor format"
fi

CURSOR_RULE="$TMP_DIR/converter.mdc"
if [[ -f "$CURSOR_RULE" ]]; then
    pass "Cursor .mdc rule was written"
    head -1 "$CURSOR_RULE" | grep -q '^---$' \
        && pass "Cursor rule has YAML frontmatter" || fail "Cursor rule missing YAML frontmatter"
    grep -q '^alwaysApply: false$' "$CURSOR_RULE" \
        && pass "Cursor rule marks alwaysApply false" || fail "Cursor rule missing alwaysApply false"
    grep -q '^# /converter' "$CURSOR_RULE" \
        && pass "Cursor rule contains skill body" || fail "Cursor rule missing skill body"
else
    fail "Cursor .mdc rule missing at $CURSOR_RULE"
fi

echo ""
echo "================================="
echo "Results: $PASS passed, $FAIL failed"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
