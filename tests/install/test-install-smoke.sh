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
    "scripts/install-claude.sh"
    "scripts/install-codex.sh"
    "scripts/install-agy.sh"
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

# install-claude.sh must use the Claude marketplace plugin path
check "install-claude.sh uses Claude marketplace plugin" \
    grep -q 'claude plugin marketplace' "$REPO_ROOT/scripts/install-claude.sh"

# install-agy.sh must use the Gemini/Antigravity image bundle path
check "install-agy.sh installs Gemini image bundle" \
    grep -q 'images/gemini' "$REPO_ROOT/scripts/install-agy.sh"

# All install scripts should have a usage comment
for script in "${INSTALL_SCRIPTS[@]}"; do
    check "$script has usage documentation" \
        grep -qi 'usage\|Usage' "$REPO_ROOT/$script"
done

echo ""

# Dry-run paths must not require vendor CLIs or mutate local state.
check "install-claude.sh dry-run exits cleanly" \
    bash "$REPO_ROOT/scripts/install-claude.sh" --dry-run --quiet
check "install-agy.sh dry-run exits cleanly" \
    bash "$REPO_ROOT/scripts/install-agy.sh" --dry-run --quiet

echo ""

# ── Runtime execution tests ──
# Verify that a locally-built ao binary is executable and responds to basic commands.
# These tests require the CLI to be built (cd cli && make build).

echo "=== Runtime Execution Tests ==="
echo ""

AO_BIN=""
if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
elif command -v ao >/dev/null 2>&1; then
    AO_BIN="$(command -v ao)"
fi

if [[ -n "$AO_BIN" ]]; then
    # ao binary responds to version subcommand
    check "ao binary runs (version)" "$AO_BIN" version

    # ao help exits cleanly
    check "ao help exits cleanly" "$AO_BIN" help

    # ao flywheel subcommand is registered
    check "ao flywheel subcommand registered" bash -c "'$AO_BIN' help 2>&1 | grep -q flywheel"

    # ao goals subcommand is registered
    check "ao goals subcommand registered" bash -c "'$AO_BIN' help 2>&1 | grep -q goals"

    # ao inject subcommand is registered
    check "ao inject subcommand registered" bash -c "'$AO_BIN' help 2>&1 | grep -q inject"
else
    echo "SKIP: ao binary not found — build with 'cd cli && make build' to enable runtime execution tests"
fi

echo ""

# ── Summary ──

echo "================================="
echo "Results: $PASS passed, $FAIL failed"
echo "================================="
[[ $FAIL -eq 0 ]] || exit 1
