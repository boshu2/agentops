#!/usr/bin/env bash
# Test: Codex runtime smoke — validates AgentOps installs and loads under the
# native Codex plugin model (.codex-plugin/ + hooks/codex-hooks.json).
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
echo ""

PLUGIN_JSON="$REPO_ROOT/.codex-plugin/plugin.json"
MARKETPLACE_JSON="$REPO_ROOT/plugins/marketplace.json"
CODEX_HOOKS_JSON="$REPO_ROOT/hooks/codex-hooks.json"
PUBLIC_INSTALL="$REPO_ROOT/scripts/install-codex.sh"
PLUGIN_INSTALL="$REPO_ROOT/scripts/install-codex-plugin.sh"
EXPECTED_CODEX_HOOK_SCRIPTS=(
    "session-start.sh"
    "ao-flywheel-close.sh"
    "prompt-nudge.sh"
    "quality-signals.sh"
    "go-test-precommit.sh"
    "commit-review-gate.sh"
    "ratchet-advance.sh"
)

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

# ── 2. Native Codex hook bundle ───────────────────────────────────────────────
echo "Stage 2: Codex native hooks"

if [[ -f "$CODEX_HOOKS_JSON" ]]; then
    python3 -m json.tool "$CODEX_HOOKS_JSON" >/dev/null 2>&1 \
        && pass "hooks/codex-hooks.json is valid JSON" || fail "hooks/codex-hooks.json is invalid JSON"
    jq -e '.hooks | type == "object" and length == 5' "$CODEX_HOOKS_JSON" >/dev/null 2>&1 \
        && pass "codex hook bundle defines 5 native hook events" || fail "codex hook bundle event map is unexpectedly small"
    jq -e '[.hooks | to_entries[] | .value[] | .hooks[]] | length == 7' "$CODEX_HOOKS_JSON" >/dev/null 2>&1 \
        && pass "codex hook bundle defines 7 native hook handlers" || fail "codex hook bundle handler count drifted"
    for hook_script in "${EXPECTED_CODEX_HOOK_SCRIPTS[@]}"; do
        if jq -e --arg script "$hook_script" \
            '[.hooks | to_entries[] | .value[] | .hooks[] | select(.command | contains("/hooks/" + $script))] | length == 1' \
            "$CODEX_HOOKS_JSON" >/dev/null 2>&1; then
            pass "codex hook bundle includes exactly one $hook_script handler"
        else
            fail "codex hook bundle missing or duplicates $hook_script"
        fi
    done
    if jq -e '.hooks.SessionStart[]?.hooks[] | select(.command | test("session-start\\.sh$"))' "$CODEX_HOOKS_JSON" >/dev/null 2>&1; then
        pass "codex hook bundle includes session-start.sh"
    else
        fail "codex hook bundle missing session-start.sh"
    fi
    if jq -e '.hooks.SessionStart[]?.hooks[] | select(.command | test("ao-inject\\.sh$"))' "$CODEX_HOOKS_JSON" >/dev/null 2>&1; then
        fail "codex SessionStart must not include ao-inject.sh"
    else
        pass "codex SessionStart omits noisy ao-inject.sh"
    fi
    if jq -e '.hooks.Stop[]?.hooks[] | select(.command | test("ao-flywheel-close\\.sh$"))' "$CODEX_HOOKS_JSON" >/dev/null 2>&1; then
        pass "codex hook bundle includes ao-flywheel-close.sh"
    else
        fail "codex hook bundle missing ao-flywheel-close.sh"
    fi
    if rg --pcre2 -n '"[^"]*/(plan|pre-mortem|vibe|post-mortem)([^A-Za-z0-9-]|")' \
        "$REPO_ROOT/hooks/prompt-nudge.sh" "$REPO_ROOT/hooks/ratchet-advance.sh" >/dev/null 2>&1; then
        fail "codex prompt hooks contain slash-command suggestions"
    else
        pass "codex prompt hooks use runtime-neutral suggestions"
    fi
else
    fail "hooks/codex-hooks.json not found"
fi

echo ""

# ── 3. Codex installer scripts are runtime-native ─────────────────────────────
echo "Stage 3: Codex installer scripts"

if [[ -f "$PUBLIC_INSTALL" ]]; then
    bash -n "$PUBLIC_INSTALL" && pass "install-codex.sh syntax valid" || fail "install-codex.sh syntax invalid"
    head -1 "$PUBLIC_INSTALL" | grep -qE '^#!/usr/bin/env bash|^#!/bin/bash' \
        && pass "install-codex.sh has valid shebang" || fail "install-codex.sh missing shebang"
else
    fail "scripts/install-codex.sh not found"
fi

if [[ -f "$PLUGIN_INSTALL" ]]; then
    bash -n "$PLUGIN_INSTALL" && pass "install-codex-plugin.sh syntax valid" || fail "install-codex-plugin.sh syntax invalid"
    grep -q 'codex_hooks' "$PLUGIN_INSTALL" \
        && pass "install-codex-plugin.sh manages codex_hooks enablement" || fail "install-codex-plugin.sh missing codex_hooks handling"
    grep -q 'hooks.json' "$PLUGIN_INSTALL" \
        && pass "install-codex-plugin.sh installs ~/.codex/hooks.json" || fail "install-codex-plugin.sh missing hooks.json install flow"
else
    fail "scripts/install-codex-plugin.sh not found"
fi

echo ""

# ── 4. Public installer smoke into temp HOME ─────────────────────────────────
echo "Stage 4: Codex native install smoke"

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
    grep -q '^codex_hooks = true$' "$CODEX_HOME/config.toml" \
        && pass "config.toml enables codex_hooks" || fail "config.toml missing codex_hooks = true"
    grep -q '^\[plugins\."agentops@agentops-marketplace"\]$' "$CODEX_HOME/config.toml" \
        && pass "config.toml enables the AgentOps plugin" || fail "config.toml missing AgentOps plugin block"
else
    fail "config.toml missing from ~/.codex"
fi

if [[ -f "$CODEX_HOME/hooks.json" ]]; then
    pass "$CODEX_HOME/hooks.json created by installer"
    jq -e '.hooks | type == "object" and length == 5' "$CODEX_HOME/hooks.json" >/dev/null 2>&1 \
        && pass "$CODEX_HOME/hooks.json uses the native event-map schema" || fail "$CODEX_HOME/hooks.json did not install the native event-map schema"
    for hook_script in "${EXPECTED_CODEX_HOOK_SCRIPTS[@]}"; do
        if jq -e --arg script "$hook_script" \
            '[.hooks | to_entries[] | .value[] | .hooks[] | select(.command | contains("/hooks/" + $script))] | length == 1' \
            "$CODEX_HOME/hooks.json" >/dev/null 2>&1; then
            pass "$CODEX_HOME/hooks.json includes exactly one $hook_script handler"
        else
            fail "$CODEX_HOME/hooks.json missing or duplicates $hook_script"
        fi
    done
    jq -e '.hooks.SessionStart[]?.hooks[] | select(.command | test("session-start\\.sh$"))' "$CODEX_HOME/hooks.json" >/dev/null 2>&1 \
        && pass "$CODEX_HOME/hooks.json includes session-start.sh" || fail "$CODEX_HOME/hooks.json missing session-start.sh"
else
    fail "$CODEX_HOME/hooks.json missing after install"
fi

if [[ -f "$CODEX_HOME/.agentops-codex-install.json" ]]; then
    jq -e '.install_mode == "native-plugin"' "$CODEX_HOME/.agentops-codex-install.json" >/dev/null 2>&1 \
        && pass "install metadata records native-plugin mode" || fail "install metadata missing native-plugin mode"
    jq -e '.hook_runtime == "codex-native-hooks"' "$CODEX_HOME/.agentops-codex-install.json" >/dev/null 2>&1 \
        && pass "install metadata records codex-native-hooks runtime" || fail "install metadata missing codex-native-hooks runtime"
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
