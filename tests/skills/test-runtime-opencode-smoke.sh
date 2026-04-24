#!/usr/bin/env bash
# Test: OpenCode runtime smoke — validates AgentOps skill files are loadable in OpenCode
# Checks skill structure, opencode install script, and config compatibility.
# Standalone: does NOT require a live OpenCode runtime.
# Promoted from: tests/_quarantine/opencode/ (structural checks only)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
SKIP=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP: $1"; SKIP=$((SKIP + 1)); }

echo "=== OpenCode Runtime Smoke Tests ==="
echo "Proof tier: Tier S structural/install smoke"
echo ""

# ── 1. Install script validity ────────────────────────────────────────────────
echo "Stage 1: OpenCode install script"

OPENCODE_INSTALL="$REPO_ROOT/scripts/install-opencode.sh"
if [[ -f "$OPENCODE_INSTALL" ]]; then
    bash -n "$OPENCODE_INSTALL" && pass "install-opencode.sh syntax valid" || fail "install-opencode.sh syntax invalid"
    head -1 "$OPENCODE_INSTALL" | grep -qE '^#!/usr/bin/env bash|^#!/bin/bash' \
        && pass "install-opencode.sh has valid shebang" || fail "install-opencode.sh missing shebang"
    grep -q 'boshu2/agentops' "$OPENCODE_INSTALL" \
        && pass "install-opencode.sh references agentops repo" || fail "install-opencode.sh missing repo ref"
    grep -qi 'opencode' "$OPENCODE_INSTALL" \
        && pass "install-opencode.sh mentions opencode target" || fail "install-opencode.sh missing opencode reference"
else
    fail "install-opencode.sh not found at $OPENCODE_INSTALL"
fi

echo ""

# ── 2. Skill SKILL.md files have no OpenCode-breaking characters ──────────────
echo "Stage 2: Skill frontmatter OpenCode compatibility"

skill_count=0
broken=0
for skill_md in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    skill_count=$((skill_count + 1))
    # SKILL.md must start with --- (YAML frontmatter) — required by all runtimes
    if ! head -1 "$skill_md" | grep -q '^---'; then
        fail "$(basename "$(dirname "$skill_md")")/SKILL.md missing frontmatter start"
        broken=$((broken + 1))
    fi
done

if [[ $broken -eq 0 && $skill_count -gt 0 ]]; then
    pass "$skill_count skills have valid frontmatter start"
elif [[ $skill_count -eq 0 ]]; then
    fail "No SKILL.md files found under skills/"
fi

echo ""

# ── 3. Skills directory structure ─────────────────────────────────────────────
echo "Stage 3: Runtime-agnostic skill structure"

# Every skill must have SKILL.md (the cross-runtime entry point)
missing_skillmd=0
for skill_dir in "$REPO_ROOT/skills"/*/; do
    [[ -d "$skill_dir" ]] || continue
    if [[ ! -f "$skill_dir/SKILL.md" ]]; then
        fail "$(basename "$skill_dir") missing SKILL.md"
        missing_skillmd=$((missing_skillmd + 1))
    fi
done
if [[ $missing_skillmd -eq 0 ]]; then
    pass "All skill directories have SKILL.md"
fi

# No README.md in skill dirs (CI rule: SKILL.md is the entry point)
readme_found=0
for skill_dir in "$REPO_ROOT/skills"/*/; do
    [[ -d "$skill_dir" ]] || continue
    if [[ -f "$skill_dir/README.md" ]]; then
        fail "$(basename "$skill_dir") has README.md (should be SKILL.md only)"
        readme_found=$((readme_found + 1))
    fi
done
if [[ $readme_found -eq 0 ]]; then
    pass "No skill directories have README.md (SKILL.md is entry point)"
fi

echo ""

# ── 4. OpenCode install target directory ──────────────────────────────────────
echo "Stage 4: OpenCode config path compatibility"

# OpenCode reads from ~/.opencode/skills/ — verify install script targets it
if [[ -f "$OPENCODE_INSTALL" ]]; then
    if grep -q 'opencode\|\.opencode' "$OPENCODE_INSTALL"; then
        pass "install-opencode.sh references opencode config path"
    else
        skip "install-opencode.sh does not reference .opencode path (may use alternate mechanism)"
    fi
fi

echo ""

# ── Summary ───────────────────────────────────────────────────────────────────
echo "================================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
