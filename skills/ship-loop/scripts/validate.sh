#!/usr/bin/env bash
# validate.sh — self-validation for the ship-loop skill
#
# Structural checks: SKILL.md present, frontmatter shape, references resolvable.
# Invoked by the skill-auditor on PR; can also be run manually.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"

errors=0
fail() { echo "FAIL: $1" >&2; errors=$((errors + 1)); }
pass() { echo "ok:   $1"; }

[ -f "$SKILL_MD" ] || { fail "SKILL.md missing at $SKILL_MD"; exit 1; }
pass "SKILL.md exists"

# Required frontmatter fields
for field in name description practices hexagonal_role skill_api_version metadata output_contract; do
    if grep -q "^${field}:" "$SKILL_MD"; then
        pass "frontmatter field present: $field"
    else
        fail "frontmatter field missing: $field"
    fi
done

# Triggers — at least one keyword the user might type
if grep -qE "^  triggers:" "$SKILL_MD"; then
    pass "triggers declared"
else
    fail "no triggers list in metadata"
fi

# References resolvable: extract only the references/<name> path, not the markdown link wrapper
for ref in $(grep -oE 'references/[A-Za-z0-9._/-]+\.md' "$SKILL_MD" | sort -u); do
    if [ -f "$SKILL_DIR/$ref" ]; then
        pass "ref resolves: $ref"
    else
        fail "ref missing: $ref"
    fi
done

# Line budget (judgment tier cap is high, but ship-loop should stay tight)
lines=$(wc -l < "$SKILL_MD")
if [ "$lines" -le 250 ]; then
    pass "line count ${lines} <= 250 (template ceiling)"
elif [ "$lines" -le 600 ]; then
    pass "line count ${lines} <= 600 (execution-tier cap)"
else
    fail "line count ${lines} exceeds execution-tier cap (600)"
fi

# Codex twin parity
CODEX_SKILL="$SKILL_DIR/../../skills-codex/ship-loop/SKILL.md"
CODEX_PROMPT="$SKILL_DIR/../../skills-codex/ship-loop/prompt.md"
[ -f "$CODEX_SKILL" ] && pass "codex twin SKILL.md exists" || fail "codex twin missing: $CODEX_SKILL"
[ -f "$CODEX_PROMPT" ] && pass "codex twin prompt.md exists" || fail "codex twin missing: $CODEX_PROMPT"

# Codex SKILL.md must NOT have skill_api_version (slim frontmatter)
if [ -f "$CODEX_SKILL" ]; then
    if grep -q "^skill_api_version:" "$CODEX_SKILL"; then
        fail "codex SKILL.md has skill_api_version (should be slim)"
    else
        pass "codex SKILL.md uses slim frontmatter"
    fi
fi

if [ "$errors" -eq 0 ]; then
    echo ""
    echo "ship-loop: all checks PASSED"
    exit 0
else
    echo ""
    echo "ship-loop: ${errors} check(s) FAILED" >&2
    exit 1
fi
