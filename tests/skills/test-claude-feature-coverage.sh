#!/bin/bash
# Validate Claude Code feature coverage across skill docs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SKILLS_DIR="$REPO_ROOT/skills"

FAILED=0

fail() {
    local msg="$1"
    echo "  ✗ $msg"
    FAILED=$((FAILED + 1))
}

pass() {
    local msg="$1"
    echo "  ✓ $msg"
}

echo "Claude feature coverage checks"

CONTRACT="$SKILLS_DIR/shared/references/claude-code-latest-features.md"
if [ ! -f "$CONTRACT" ]; then
    fail "missing feature contract: skills/shared/references/claude-code-latest-features.md"
else
    pass "feature contract exists"
fi

if [ -f "$CONTRACT" ]; then
    required_tokens=(
        "/agents"
        "/hooks"
        "/permissions"
        "/memory"
        "/mcp"
        "/output-style"
        "isolation: worktree"
        "background: true"
        "WorktreeCreate"
        "WorktreeRemove"
        "ConfigChange"
        "claude agents"
        "--worktree"
    )

    for token in "${required_tokens[@]}"; do
        if grep -Fq -- "$token" "$CONTRACT"; then
            pass "contract includes '$token'"
        else
            fail "contract missing '$token'"
        fi
    done
fi

# Core multi-agent skills should reference the shared contract.
core_multi_agent_skills=(
    "skills/shared/SKILL.md"
    "skills/council/SKILL.md"
    "skills/swarm/SKILL.md"
    "skills/research/SKILL.md"
    "skills/crank/SKILL.md"
    "skills/codex-team/SKILL.md"
)

for rel in "${core_multi_agent_skills[@]}"; do
    path="$REPO_ROOT/$rel"
    if [ ! -f "$path" ]; then
        fail "missing expected skill file: $rel"
        continue
    fi
    if grep -Fq -- "skills/shared/references/claude-code-latest-features.md" "$path"; then
        pass "$rel references shared Claude feature contract"
    else
        fail "$rel does not reference shared Claude feature contract"
    fi
done

# Prevent regressions to deprecated command names.
deprecated_out="$(mktemp)"
if rg -n --glob 'SKILL.md' '/approved-tools|/allowed-tools' "$SKILLS_DIR" >"$deprecated_out" 2>/dev/null; then
    fail "deprecated permission command names found in SKILL.md files"
    sed 's/^/    /' "$deprecated_out"
else
    pass "no deprecated permission command names"
fi
rm -f "$deprecated_out"

if [ "$FAILED" -gt 0 ]; then
    echo ""
    echo "Claude feature coverage: FAIL ($FAILED issue(s))"
    exit 1
fi

echo ""
echo "Claude feature coverage: PASS"
exit 0
