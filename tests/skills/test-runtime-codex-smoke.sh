#!/usr/bin/env bash
# Test: Codex runtime smoke — validates AgentOps installs and loads under the
# native Codex plugin model (.codex-plugin/, hookless — skills + ao CLI only).
# Standalone: does NOT require a live Codex session or network access.
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
PUBLIC_INSTALL="$REPO_ROOT/scripts/install-codex.sh"
PLUGIN_INSTALL="$REPO_ROOT/scripts/install-codex-plugin.sh"
# ── 1. Codex plugin manifest + marketplace wiring ────────────────────────────
echo "Stage 1: Codex plugin manifest"

if [[ -f "$PLUGIN_JSON" ]]; then
    python3 -m json.tool "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass ".codex-plugin/plugin.json is valid JSON" || fail ".codex-plugin/plugin.json is invalid JSON"
    jq -e '.name == "agentops"' "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass "plugin.json targets the agentops plugin" || fail "plugin.json missing agentops name"
    jq -e '.skills == "./skills-codex"' "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass "plugin.json points at ./skills-codex" || fail "plugin.json missing ./skills-codex entry"
else
    fail ".codex-plugin/plugin.json not found"
fi

if [[ -f "$MARKETPLACE_JSON" ]]; then
    python3 -m json.tool "$MARKETPLACE_JSON" >/dev/null 2>&1 \
        && pass "plugins/marketplace.json is valid JSON" || fail "plugins/marketplace.json is invalid JSON"
    jq -e '.plugins[] | select(.name == "agentops")' "$MARKETPLACE_JSON" >/dev/null 2>&1 \
        && pass "marketplace.json includes agentops" || fail "marketplace.json missing agentops entry"
else
    fail "plugins/marketplace.json not found"
fi

echo ""

# ── 2. Codex installer scripts are runtime-native ─────────────────────────────
echo "Stage 2: Codex installer scripts"

if [[ -f "$PUBLIC_INSTALL" ]]; then
    bash -n "$PUBLIC_INSTALL" && pass "install-codex.sh syntax valid" || fail "install-codex.sh syntax invalid"
    head -1 "$PUBLIC_INSTALL" | grep -qE '^#!/usr/bin/env bash|^#!/bin/bash' \
        && pass "install-codex.sh has valid shebang" || fail "install-codex.sh missing shebang"
else
    fail "scripts/install-codex.sh not found"
fi

if [[ -f "$PLUGIN_INSTALL" ]]; then
    bash -n "$PLUGIN_INSTALL" && pass "install-codex-plugin.sh syntax valid" || fail "install-codex-plugin.sh syntax invalid"
    ! grep -q 'codex-hooks.json' "$PLUGIN_INSTALL" \
        && pass "install-codex-plugin.sh ships hookless (no codex-hooks.json install flow)" || fail "install-codex-plugin.sh still references codex-hooks.json"
else
    fail "scripts/install-codex-plugin.sh not found"
fi

echo ""

# ── 3. Public installer smoke into temp HOME ─────────────────────────────────
echo "Stage 3: Codex native install smoke"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

HOME_ROOT="$TMP_ROOT/home"
CODEX_HOME="$HOME_ROOT/.codex"
PLUGIN_ROOT="$CODEX_HOME/plugins/cache/agentops-marketplace/agentops/local"
PLUGIN_SKILLS="$PLUGIN_ROOT/skills-codex"

if HOME="$HOME_ROOT" AGENTOPS_BUNDLE_ROOT="$REPO_ROOT" AGENTOPS_INSTALL_REF="test-local" \
    bash "$PUBLIC_INSTALL" >/dev/null 2>&1; then
    pass "install-codex.sh succeeds into temp HOME"
else
    fail "install-codex.sh failed in temp HOME"
fi

if [[ -d "$PLUGIN_SKILLS" ]]; then
    pass "native plugin cache created under ~/.codex/plugins/cache"
else
    fail "native plugin cache missing under ~/.codex/plugins/cache"
fi

if [[ -f "$CODEX_HOME/config.toml" ]]; then
    pass "config.toml created in ~/.codex"
    if grep -q '^hooks = true$' "$CODEX_HOME/config.toml"; then
        fail "default config.toml should not enable hooks"
    else
        pass "default config.toml leaves hooks disabled"
    fi
    if grep -q '^codex_hooks[[:space:]]*=' "$CODEX_HOME/config.toml"; then
        fail "config.toml still contains deprecated codex_hooks"
    else
        pass "config.toml omits deprecated codex_hooks"
    fi
    grep -q '^\[plugins\."agentops@agentops-marketplace"\]$' "$CODEX_HOME/config.toml" \
        && pass "config.toml enables the AgentOps plugin" || fail "config.toml missing AgentOps plugin block"
else
    fail "config.toml missing from ~/.codex"
fi

if [[ -f "$CODEX_HOME/hooks.json" ]]; then
    fail "$CODEX_HOME/hooks.json should not be created by default install"
else
    pass "default install does not create hooks.json"
fi

if [[ -f "$CODEX_HOME/.agentops-codex-install.json" ]]; then
    jq -e '.install_mode == "native-plugin"' "$CODEX_HOME/.agentops-codex-install.json" >/dev/null 2>&1 \
        && pass "install metadata records native-plugin mode" || fail "install metadata missing native-plugin mode"
    jq -e '.hook_runtime == "hookless-default" and .hooks_installed == false' "$CODEX_HOME/.agentops-codex-install.json" >/dev/null 2>&1 \
        && pass "install metadata records hookless default" || fail "install metadata missing hookless default"
else
    fail "Codex install metadata missing"
fi

if [[ ! -e "$HOME_ROOT/.agents/skills" ]] && [[ ! -e "$CODEX_HOME/skills" ]]; then
    pass "install leaves no raw ~/.agents/skills or ~/.codex/skills mirror"
else
    fail "install recreated a raw skill mirror under ~/.agents/skills or ~/.codex/skills"
fi

expected_count="$(find "$REPO_ROOT/skills-codex" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
installed_count="$(find "$PLUGIN_SKILLS" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
if [[ "$expected_count" == "$installed_count" ]]; then
    pass "installed native Codex bundle has the expected skill count ($installed_count)"
else
    fail "installed native Codex bundle count mismatch (expected $expected_count, got $installed_count)"
fi

while IFS= read -r -d '' entrypoint_file; do
    if grep -qE '[~]/\.codex/skills|\$HOME/\.codex/skills' "$entrypoint_file"; then
        fail "installed Codex entrypoint still references raw .codex/skills paths: $entrypoint_file"
        break
    fi
done < <(find "$PLUGIN_SKILLS" -type f \( -name 'SKILL.md' -o -name 'prompt.md' \) -print0)
if [[ $FAIL -eq 0 ]]; then
    pass "installed Codex entrypoints avoid stale raw .codex/skills references"
fi

echo ""

# ── Summary ───────────────────────────────────────────────────────────────────
echo "================================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
