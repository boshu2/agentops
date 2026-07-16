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

# ── 1. OpenCode install tombstone + canonical docs ───────────────────────────
echo "Stage 1: OpenCode install surface"

OPENCODE_INSTALL="$REPO_ROOT/scripts/install-opencode.sh"
OPENCODE_DOCS="$REPO_ROOT/.opencode/INSTALL.md"
if [[ -f "$OPENCODE_INSTALL" ]]; then
    bash -n "$OPENCODE_INSTALL" && pass "install-opencode.sh syntax valid" || fail "install-opencode.sh syntax invalid"
    grep -q 'ao skills link' "$OPENCODE_INSTALL" \
        && pass "install-opencode.sh tombstone points at ao skills link" || fail "install-opencode.sh missing ao skills link"
    if bash "$OPENCODE_INSTALL" >/dev/null 2>&1; then
        fail "install-opencode.sh tombstone exited 0"
    else
        pass "install-opencode.sh tombstone exits nonzero"
    fi
else
    fail "install-opencode.sh not found at $OPENCODE_INSTALL"
fi

if [[ -f "$OPENCODE_DOCS" ]] && grep -q 'ao skills link' "$OPENCODE_DOCS"; then
    pass ".opencode/INSTALL.md documents ao skills link"
else
    fail ".opencode/INSTALL.md missing ao skills link guidance"
fi

echo ""

# ── 2. Skill SKILL.md files have no OpenCode-breaking characters ──────────────
echo "Stage 2: Skill frontmatter OpenCode compatibility"

skill_count=0
broken=0
for skill_md in "$REPO_ROOT/skills"/*/SKILL.md; do
    [[ -f "$skill_md" ]] || continue
    # Skip leading-underscore scaffolding (e.g. skills/_fixtures/) — planted
    # test fixtures, not real skills.
    case "$(basename "$(dirname "$skill_md")")" in _*) continue ;; esac
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
    case "$(basename "$skill_dir")" in _*) continue ;; esac
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
    case "$(basename "$skill_dir")" in _*) continue ;; esac
    if [[ -f "$skill_dir/README.md" ]]; then
        fail "$(basename "$skill_dir") has README.md (should be SKILL.md only)"
        readme_found=$((readme_found + 1))
    fi
done
if [[ $readme_found -eq 0 ]]; then
    pass "No skill directories have README.md (SKILL.md is entry point)"
fi

echo ""

# ── 4. OpenCode install docs ──────────────────────────────────────────────────
echo "Stage 4: OpenCode config path compatibility"

if [[ -f "$OPENCODE_DOCS" ]]; then
    if grep -qE 'opencode|\.opencode|ao skills link' "$OPENCODE_DOCS"; then
        pass ".opencode/INSTALL.md references opencode / skills link path"
    else
        skip ".opencode/INSTALL.md missing opencode path reference"
    fi
fi

echo ""

# ── Summary ───────────────────────────────────────────────────────────────────
echo "================================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "================================="

[[ $FAIL -eq 0 ]] || exit 1
