#!/usr/bin/env bash
set -euo pipefail

# Smoke tests for install scripts — validates syntax, headers, and safety invariants
# without actually running any installs (no side effects).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

check() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        echo "PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $desc"
        FAIL=$((FAIL + 1))
    fi
}

# Helper: verify file starts with a valid shebang
has_shebang() {
    local file="$1"
    head -1 "$file" | grep -qE '^#!/usr/bin/env bash|^#!/bin/bash'
}

# Helper: verify file has strict error handling
has_strict_mode() {
    local file="$1"
    grep -qE 'set -e(uo pipefail)?$' "$file"
}

# Helper: verify no hardcoded absolute paths to non-standard locations
no_hardcoded_user_paths() {
    local file="$1"
    # Flag hardcoded /home/<user> or /Users/<user> paths (except in comments)
    # Allow $HOME, ~, and standard paths like /usr, /tmp, /dev
    ! grep -nE '^\s*[^#]*(/home/[a-z][a-z0-9_]+/|/Users/[A-Za-z][A-Za-z0-9_]+/)' "$file"
}

# ── Core install scripts ──

INSTALL_SCRIPTS=(
    "scripts/install.sh"
    "scripts/install-codex.sh"
    "scripts/install-opencode.sh"
)

echo "=== Install Script Smoke Tests ==="
echo ""

# Syntax validation (bash -n)
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script syntax valid" bash -n "$REPO_ROOT/$script"
done

echo ""

# Shebang check
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script has valid shebang" has_shebang "$REPO_ROOT/$script"
done

echo ""

# Strict mode check
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script has strict error handling" has_strict_mode "$REPO_ROOT/$script"
done

echo ""

# No hardcoded user paths
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script no hardcoded user paths" no_hardcoded_user_paths "$REPO_ROOT/$script"
done

echo ""

# ── Supporting install scripts ──

SUPPORT_SCRIPTS=(
    "scripts/install-codex-plugin.sh"
    "scripts/install-codex-native-skills.sh"
    "scripts/install-dev-hooks.sh"
)

for script in "${SUPPORT_SCRIPTS[@]}"; do
    if [[ -f "$REPO_ROOT/$script" ]]; then
        check "$script syntax valid" bash -n "$REPO_ROOT/$script"
        check "$script has valid shebang" has_shebang "$REPO_ROOT/$script"
    fi
done

echo ""

# ── Structural invariants ──

# install.sh must reference install-codex.sh (it delegates to it)
check "install.sh delegates to install-codex.sh" \
    grep -q 'install-codex' "$REPO_ROOT/scripts/install.sh"

# install-codex.sh must reference install-codex-plugin.sh
check "install-codex.sh delegates to install-codex-plugin.sh" \
    grep -q 'install-codex-plugin.sh' "$REPO_ROOT/scripts/install-codex.sh"

# install-opencode.sh must reference the repo URL
check "install-opencode.sh references agentops repo" \
    grep -q 'boshu2/agentops' "$REPO_ROOT/scripts/install-opencode.sh"

# All install scripts should have a usage comment
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script has usage documentation" \
        grep -qi 'usage\|Usage' "$REPO_ROOT/$script"
done

echo ""

# ── Summary ──

echo "================================="
echo "Results: $PASS passed, $FAIL failed"
echo "================================="
[[ $FAIL -eq 0 ]] || exit 1
