#!/usr/bin/env bash
# Test: Codex runtime smoke — validates AgentOps 4 source-link install under
# an isolated HOME (ao skills link → ~/.codex/skills). Standalone: no live
# Codex session or network required.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP: $1"; SKIP=$((SKIP + 1)); }

echo "=== Codex Runtime Smoke Tests ==="
echo "Proof tier: Tier S structural/install smoke"
echo ""

PLUGIN_JSON="$REPO_ROOT/.codex-plugin/plugin.json"
MARKETPLACE_JSON="$REPO_ROOT/plugins/marketplace.json"
TOMBSTONE="$REPO_ROOT/scripts/install-codex.sh"

# ── 1. Legacy plugin manifests still ship (compat artifacts) ─────────────────
echo "Stage 1: Codex plugin manifest artifacts"

if [[ -f "$PLUGIN_JSON" ]]; then
    python3 -m json.tool "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass ".codex-plugin/plugin.json is valid JSON" || fail ".codex-plugin/plugin.json is invalid JSON"
else
    fail ".codex-plugin/plugin.json not found"
fi

if [[ -f "$MARKETPLACE_JSON" ]]; then
    python3 -m json.tool "$MARKETPLACE_JSON" >/dev/null 2>&1 \
        && pass "plugins/marketplace.json is valid JSON" || fail "plugins/marketplace.json is invalid JSON"
else
    fail "plugins/marketplace.json not found"
fi

echo ""

# ── 2. Public installer is a tombstone pointing at ao skills link ────────────
echo "Stage 2: Codex installer tombstone"

if [[ -f "$TOMBSTONE" ]]; then
    bash -n "$TOMBSTONE" && pass "install-codex.sh syntax valid" || fail "install-codex.sh syntax invalid"
    grep -q 'ao skills link' "$TOMBSTONE" \
        && pass "install-codex.sh tombstone points at ao skills link" || fail "install-codex.sh missing ao skills link"
    if bash "$TOMBSTONE" >/dev/null 2>&1; then
        fail "install-codex.sh tombstone exited 0"
    else
        pass "install-codex.sh tombstone exits nonzero"
    fi
else
    fail "scripts/install-codex.sh not found"
fi

if [[ ! -e "$REPO_ROOT/scripts/install-codex-plugin.sh" ]]; then
    pass "install-codex-plugin.sh deleted"
else
    fail "install-codex-plugin.sh still exists"
fi

echo ""

# ── 3. ao skills link into temp HOME ─────────────────────────────────────────
echo "Stage 3: ao skills link smoke"

AO_BIN=""
if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
elif command -v ao >/dev/null 2>&1; then
    AO_BIN="$(command -v ao)"
else
    (cd "$REPO_ROOT/cli" && go build -o bin/ao ./cmd/ao)
    AO_BIN="$REPO_ROOT/cli/bin/ao"
fi

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
HOME_ROOT="$TMP_ROOT/home"
mkdir -p "$HOME_ROOT"

if (cd "$REPO_ROOT" && HOME="$HOME_ROOT" "$AO_BIN" skills link >/dev/null 2>&1); then
    pass "ao skills link succeeds into temp HOME"
else
    fail "ao skills link failed in temp HOME"
fi

if [[ -d "$HOME_ROOT/.agents/skills" ]]; then
    pass "linked skills under ~/.agents/skills"
else
    fail "missing ~/.agents/skills after ao skills link"
fi

# Codex skills root appears when ~/.codex exists or is created by link detection.
if [[ -d "$HOME_ROOT/.codex/skills" ]]; then
    pass "linked skills under ~/.codex/skills"
else
    # skills link only fans into detected runtimes; creating ~/.codex may be required.
    mkdir -p "$HOME_ROOT/.codex"
    (cd "$REPO_ROOT" && HOME="$HOME_ROOT" "$AO_BIN" skills link >/dev/null 2>&1) || true
    if [[ -d "$HOME_ROOT/.codex/skills" ]]; then
        pass "linked skills under ~/.codex/skills after creating ~/.codex"
    else
        skip "Codex skills root not linked (runtime root not detected)"
    fi
fi

echo ""
echo "================================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
