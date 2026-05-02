#!/usr/bin/env bash
# Overnight evolve preflight — verify environment before kicking off
# scripts/overnight-evolve.sh.
#
# Plan: ~/dev/agentops/.agents/plans/2026-05-01-overnight-evolve-loop.md
#
# Exits 0 if all checks pass; non-zero on any FAIL.

set -uo pipefail

cd "$HOME/dev/agentops" || { echo "FAIL: no agentops repo at ~/dev/agentops"; exit 1; }

PASS=0
FAIL=0
pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

# 1. Working tree clean
if [ -z "$(git status --porcelain)" ]; then
    pass "git working tree clean"
else
    fail "git working tree dirty (stash or commit before launch)"
    git status --short | head -5
fi

# 2. ao on PATH + responds
if command -v ao >/dev/null 2>&1; then
    if ao --help >/dev/null 2>&1; then
        pass "ao CLI available"
    else
        fail "ao CLI present but --help failed"
    fi
else
    fail "ao not on PATH"
fi

# 3. claude on PATH + responds
if command -v claude >/dev/null 2>&1; then
    if claude --version >/dev/null 2>&1; then
        pass "claude CLI available"
    else
        fail "claude CLI present but --version failed"
    fi
else
    fail "claude not on PATH"
fi

# 4. gtimeout (or timeout) available
if command -v gtimeout >/dev/null 2>&1; then
    pass "gtimeout available"
elif command -v timeout >/dev/null 2>&1; then
    pass "timeout available (Linux fallback)"
else
    fail "neither gtimeout nor timeout on PATH (brew install coreutils)"
fi

# 5. ao goals measure works (GOALS.md well-formed)
if ao goals measure --json >/dev/null 2>&1; then
    pass "ao goals measure runs cleanly"
else
    fail "ao goals measure failed (GOALS.md may be malformed)"
fi

# 6. bd ready returns something
if bd ready >/dev/null 2>&1; then
    pass "bd ready works"
else
    fail "bd ready failed (beads workspace issue)"
fi

# 7. ~/.agents/overnight/ writable
mkdir -p "$HOME/.agents/overnight" 2>/dev/null
if [ -w "$HOME/.agents/overnight" ]; then
    pass "$HOME/.agents/overnight writable"
else
    fail "$HOME/.agents/overnight not writable"
fi

# 8. Seed file present + valid JSONL
SEED="$HOME/dev/agentops/.agents/rpi/next-work-overnight-seed.jsonl"
if [ -r "$SEED" ]; then
    if jq -c . < "$SEED" >/dev/null 2>&1; then
        N="$(wc -l < "$SEED" | tr -d ' ')"
        pass "seed JSONL valid ($N lines)"
    else
        fail "seed JSONL malformed at $SEED"
    fi
else
    fail "seed JSONL missing at $SEED"
fi

# 9. Kickoff script present + executable
KICKOFF="$HOME/dev/agentops/scripts/overnight-evolve.sh"
if [ -x "$KICKOFF" ]; then
    pass "scripts/overnight-evolve.sh executable"
else
    fail "scripts/overnight-evolve.sh missing or not +x"
fi

# 10. shellcheck the kickoff script
if command -v shellcheck >/dev/null 2>&1; then
    if shellcheck --severity=error "$KICKOFF" "$0" >/dev/null 2>&1; then
        pass "shellcheck clean on kickoff + this preflight"
    else
        fail "shellcheck flagged errors"
        shellcheck --severity=error "$KICKOFF" "$0" 2>&1 | head -10
    fi
else
    echo "INFO: shellcheck not installed (skipping syntax check)"
fi

echo
echo "===================================================================="
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ] && echo "ready to launch: bash scripts/overnight-evolve.sh" \
    || echo "fix the FAILs above before launching"
echo "===================================================================="
[ "$FAIL" -eq 0 ]
