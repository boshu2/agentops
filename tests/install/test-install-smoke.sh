#!/usr/bin/env bash
set -euo pipefail

# Smoke tests for install surfaces — validates syntax and the canonical
# AgentOps 3.3 install path (ao skills link). Legacy 3.x plugin installers are
# retained only as tombstones that exit nonzero with migration guidance.

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

has_shebang() {
    local file="$1"
    head -1 "$file" | grep -qE '^#!/usr/bin/env bash|^#!/bin/bash'
}

has_strict_mode() {
    local file="$1"
    grep -qE 'set -e(uo pipefail)?$' "$file"
}

echo "=== Install Surface Smoke Tests ==="
echo ""

# ── Keepers ──
KEEP_SCRIPTS=(
    "scripts/install-bd.sh"
    "scripts/install-workflows.sh"
    "scripts/install-ms-reindex-hook.sh"
    "scripts/install-installed-skill-edit-guard.sh"
)

for script in "${KEEP_SCRIPTS[@]}"; do
    check "$script syntax valid" bash -n "$REPO_ROOT/$script"
    check "$script has valid shebang" has_shebang "$REPO_ROOT/$script"
    check "$script has strict error handling" has_strict_mode "$REPO_ROOT/$script"
done

check "installer-common.sh exists" test -f "$REPO_ROOT/scripts/lib/installer-common.sh"
check "installer-bootstrap.sh exists" test -f "$REPO_ROOT/scripts/lib/installer-bootstrap.sh"
check "installer-common.sh syntax valid" bash -n "$REPO_ROOT/scripts/lib/installer-common.sh"
check "install-bd.sh loads installer-common" \
    grep -q 'installer-common\|installer-bootstrap\|_load_installer_common' "$REPO_ROOT/scripts/install-bd.sh"

echo ""

# ── Tombstones: public curl entrypoints must refuse and point at ao skills link ──
TOMBSTONES=(
    "scripts/install.sh"
    "scripts/install-claude.sh"
    "scripts/install-codex.sh"
    "scripts/install-agy.sh"
    "scripts/install-opencode.sh"
)

for script in "${TOMBSTONES[@]}"; do
    check "$script syntax valid" bash -n "$REPO_ROOT/$script"
    check "$script is a removed-installer tombstone" \
        grep -q 'removed in 3.3' "$REPO_ROOT/$script"
    check "$script points at ao skills link" \
        grep -q 'ao skills link' "$REPO_ROOT/$script"
    # Must exit nonzero (migration refusal).
    if bash "$REPO_ROOT/$script" >/dev/null 2>&1; then
        echo "FAIL: $script tombstone exited 0"
        FAIL=$((FAIL + 1))
    else
        echo "PASS: $script tombstone exits nonzero"
        PASS=$((PASS + 1))
    fi
done

echo ""

# ── Canonical product path ──
check "README documents ao skills link" \
    grep -q 'ao skills link' "$REPO_ROOT/README.md"
check "MIGRATION documents ao skills link" \
    grep -q 'ao skills link' "$REPO_ROOT/docs/MIGRATION.md"
check "refresh-codex-local.sh wraps ao skills link" \
    grep -q 'skills link' "$REPO_ROOT/scripts/refresh-codex-local.sh"

# Deleted internals must stay gone.
for gone in \
    scripts/install-codex-plugin.sh \
    scripts/install-codex-native-skills.sh \
    scripts/select-spine-skills.sh
do
    if [[ -e "$REPO_ROOT/$gone" ]]; then
        echo "FAIL: $gone still exists (should be deleted)"
        FAIL=$((FAIL + 1))
    else
        echo "PASS: $gone deleted"
        PASS=$((PASS + 1))
    fi
done

echo ""

# ── Runtime: ao skills link --help ──
AO_BIN=""
if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
elif command -v ao >/dev/null 2>&1; then
    AO_BIN="$(command -v ao)"
fi

if [[ -n "$AO_BIN" ]]; then
    check "ao skills link --help works" bash -c "'$AO_BIN' skills link --help 2>&1 | grep -q 'Track main'"
    check "ao version runs" "$AO_BIN" version
else
    echo "SKIP: ao binary not found — build with 'cd cli && make build'"
fi

echo ""
echo "================================="
echo "Results: $PASS passed, $FAIL failed"
echo "================================="
[[ $FAIL -eq 0 ]] || exit 1
