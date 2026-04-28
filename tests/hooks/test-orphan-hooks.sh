#!/usr/bin/env bash
# test-orphan-hooks.sh - Detect unregistered hook scripts emitting JSON
# Hooks that emit hookSpecificOutput but aren't registered in hooks.json
# will have their output silently dropped by Claude Code.
# Usage: ./tests/hooks/test-orphan-hooks.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$REPO_ROOT/hooks"
HOOKS_JSON="$HOOKS_DIR/hooks.json"
CODEX_HOOKS_JSON="$HOOKS_DIR/codex-hooks.json"

ERRORS=0
WARNINGS=0

pass()  { printf '\033[0;32m✓\033[0m %s\n' "$1"; }
fail()  { printf '\033[0;31m✗\033[0m %s\n' "$1"; ERRORS=$((ERRORS + 1)); }
warn()  { printf '\033[0;33m⚠\033[0m %s\n' "$1"; WARNINGS=$((WARNINGS + 1)); }
info()  { printf '\033[0;34mℹ\033[0m %s\n' "$1"; }

# --- Pre-flight ---
if ! command -v jq >/dev/null 2>&1; then
    fail "jq is required but not found"
    exit 1
fi

if [[ ! -f "$HOOKS_JSON" ]]; then
    fail "hooks.json not found at $HOOKS_JSON"
    exit 1
fi

if [[ ! -f "$CODEX_HOOKS_JSON" ]]; then
    fail "codex-hooks.json not found at $CODEX_HOOKS_JSON"
    exit 1
fi

# --- Parse registered hooks from Claude and Codex manifests ---
registered_scripts=$(
    {
        jq -r '.. | .command? // empty' "$HOOKS_JSON"
        jq -r '.. | .command? // empty' "$CODEX_HOOKS_JSON"
    } | grep -o '[^/]*\.sh$' | sort -u
)

# --- List all hook scripts ---
all_scripts=()
for f in "$HOOKS_DIR"/*.sh; do
    [[ -f "$f" ]] || continue
    all_scripts+=("$(basename "$f")")
done

total=${#all_scripts[@]}
registered_count=0
unregistered_count=0
json_emitting=0
non_json=0

json_emitting_list=()
non_json_list=()

for script in "${all_scripts[@]}"; do
    if echo "$registered_scripts" | grep -qxF "$script"; then
        registered_count=$((registered_count + 1))
    else
        unregistered_count=$((unregistered_count + 1))
        if grep -q 'hookSpecificOutput' "$HOOKS_DIR/$script" 2>/dev/null; then
            json_emitting=$((json_emitting + 1))
            json_emitting_list+=("$script")
            warn "Unregistered + JSON-emitting: $script (hookSpecificOutput will be silently dropped)"
        else
            non_json=$((non_json + 1))
            non_json_list+=("$script")
            info "Unregistered utility: $script"
        fi
    fi
done

# --- Summary ---
echo ""
echo "=== Hook Registration Audit ==="
echo "Registered: $registered_count of $total"
echo "Unregistered: $unregistered_count"
echo "  - JSON-emitting (WARNING): $json_emitting"
echo "  - Non-JSON (info): $non_json"
echo ""

if [[ $json_emitting -eq 0 ]]; then
    pass "No unregistered hooks emitting JSON output in Claude or Codex manifests"
else
    warn "$json_emitting unregistered hook(s) emit JSON that goes nowhere"
fi

if [[ $WARNINGS -gt 0 || $ERRORS -gt 0 ]]; then
    echo ""
    echo "Warnings: $WARNINGS  Errors: $ERRORS"
fi

# Advisory mode: exit 0 even with warnings
# To promote to hard gate: change to exit $((ERRORS > 0 ? 1 : 0))
exit 0
