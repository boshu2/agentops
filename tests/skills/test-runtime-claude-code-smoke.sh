#!/usr/bin/env bash
# Test: Claude Code runtime smoke — validates AgentOps skill files load correctly
# under the Claude Code plugin model (.claude-plugin/ manifest + skills/).
# Standalone: does NOT require a live Claude Code session.
# Promoted from: tests/_quarantine/claude-code/ (structural checks only)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP: $1"; SKIP=$((SKIP + 1)); }

echo "=== Claude Code Runtime Smoke Tests ==="
echo "Proof tier: Tier S structural/install smoke"
echo ""

# ── 1. Plugin manifest validity ───────────────────────────────────────────────
echo "Stage 1: Claude Code plugin manifest"

PLUGIN_JSON="$REPO_ROOT/.claude-plugin/plugin.json"
MARKETPLACE_JSON="$REPO_ROOT/.claude-plugin/marketplace.json"

if [[ -f "$PLUGIN_JSON" ]]; then
    python3 -m json.tool "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass "plugin.json is valid JSON" || fail "plugin.json is invalid JSON"
    jq -e '.name' "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass "plugin.json has .name field" || fail "plugin.json missing .name field"
    jq -e '.version' "$PLUGIN_JSON" >/dev/null 2>&1 \
        && pass "plugin.json has .version field" || fail "plugin.json missing .version field"
else
    fail ".claude-plugin/plugin.json not found"
fi

if [[ -f "$MARKETPLACE_JSON" ]]; then
    python3 -m json.tool "$MARKETPLACE_JSON" >/dev/null 2>&1 \
        && pass "marketplace.json is valid JSON" || fail "marketplace.json is invalid JSON"
else
    fail ".claude-plugin/marketplace.json not found"
fi

# Plugin and marketplace versions must match (also checked by manifest-versions-match gate)
if [[ -f "$PLUGIN_JSON" && -f "$MARKETPLACE_JSON" ]]; then
    plugin_ver=$(jq -r '.version' "$PLUGIN_JSON")
    market_ver=$(jq -r '.metadata.version' "$MARKETPLACE_JSON")
    if [[ "$plugin_ver" == "$market_ver" ]]; then
        pass "plugin.json and marketplace.json versions match ($plugin_ver)"
    else
        fail "version mismatch: plugin.json=$plugin_ver, marketplace.json=$market_ver"
    fi
fi

echo ""

# ── 2. Claude Code hooks configuration ───────────────────────────────────────
echo "Stage 2: Claude Code hooks"

HOOKS_JSON="$REPO_ROOT/hooks/hooks.json"
if [[ -f "$HOOKS_JSON" ]]; then
    python3 -m json.tool "$HOOKS_JSON" >/dev/null 2>&1 \
        && pass "hooks/hooks.json is valid JSON" || fail "hooks/hooks.json is invalid JSON"

    # Hooks must define at least one hook entry
    hook_count=$(jq 'length' "$HOOKS_JSON" 2>/dev/null || echo 0)
    if [[ "$hook_count" -gt 0 ]]; then
        pass "hooks/hooks.json defines $hook_count hook(s)"
    else
        fail "hooks/hooks.json is empty"
    fi
else
    fail "hooks/hooks.json not found"
fi

echo ""

# ── 3. Skill frontmatter name field matches directory ─────────────────────────
echo "Stage 3: Skill name/directory consistency (Claude Code loads by name)"

mismatch=0
for skill_md in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    dir_name="$(basename "$(dirname "$skill_md")")"
    # Extract name: field from frontmatter (strip surrounding quotes if present)
    skill_name=$(awk '/^---/{if(++c==1) next; exit} /^name:/{gsub(/^name:[[:space:]]*/, ""); gsub(/^["'"'"']|["'"'"']$/, ""); print}' "$skill_md")
    if [[ -n "$skill_name" && "$skill_name" != "$dir_name" ]]; then
        fail "$dir_name: SKILL.md name='$skill_name' does not match directory"
        mismatch=$((mismatch + 1))
    fi
done
if [[ $mismatch -eq 0 ]]; then
    pass "All skill name fields match their directory names"
fi

echo ""

# ── 4. No symlinks in skill directories (Claude Code plugin-load-test rule) ───
echo "Stage 4: No symlinks in skills/"

symlink_count=0
while IFS= read -r -d '' symlink; do
    fail "Symlink found: $symlink"
    symlink_count=$((symlink_count + 1))
done < <(find "$REPO_ROOT/skills" -type l -print0 2>/dev/null)

if [[ $symlink_count -eq 0 ]]; then
    pass "No symlinks in skills/ (CI rule satisfied)"
fi

echo ""

# ── 5. Skills have required frontmatter fields for Claude Code ────────────────
echo "Stage 5: Skill required frontmatter fields"

missing_fields=0
for skill_md in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    dir_name="$(basename "$(dirname "$skill_md")")"
    for field in name description; do
        if ! grep -q "^${field}:" "$skill_md"; then
            fail "$dir_name/SKILL.md missing frontmatter field: $field"
            missing_fields=$((missing_fields + 1))
        fi
    done
done
if [[ $missing_fields -eq 0 ]]; then
    pass "All skills have required frontmatter fields (name, description)"
fi

echo ""

# ── Summary ───────────────────────────────────────────────────────────────────
echo "================================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
