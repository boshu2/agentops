#!/bin/bash
# check-sibling-citation-on-commit.sh — PreToolUse hook: warn when a
# git commit message body lacks a sibling-pattern citation, as required
# by docs/contracts/update-principles.md (Principle 3: every fix names
# the precedent it follows).
#
# Scope (v1): warn-only, non-blocking. Fires on Bash tool calls that
# match `git commit -m …` or `git commit -F …`. Parses the commit
# message body and emits a warning when no recognized sibling-citation
# phrasing is present.
#
# This is the third mechanical enforcer of update-principles.md
# (cycles 54 and 55 landed P4 and P2). Together they cover principles
# 2, 3, and 4. Principles 1 (single concern) and 5 (clean branch point)
# remain TODO under epic soc-5yuy.
#
# Recognized sibling-citation patterns (case-insensitive):
#   - "matching … pattern" / "matching … shape" / "matching … SKIP"
#   - "follows the … pattern" / "follows the … shape"
#   - "follows … pattern" / "follows … shape"
#   - "sibling …"
#   - "sibling pattern:"
#   - "shape from commit <sha>"
#   - "<word>-pattern" / "<word>-shape" (compound nouns)
#
# Exemptions:
#   - commits with no body (trivial/typo/dep-bump) are silent
#   - commits with explicit `[no-sibling]` tag in body skip the check
#
# practices: [continuous-integration, design-by-contract, code-complete]
set -uo pipefail

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SIBLING_CITATION_DISABLED:-}" = "1" ] && exit 0

# Only fire on Bash tool calls
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [ -z "$TOOL_NAME" ]; then
    INPUT=$(cat)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[ "$TOOL_NAME" != "Bash" ] && exit 0

# Extract command
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$COMMAND" ]; then
    [ -z "${INPUT:-}" ] && INPUT=$(cat)
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || exit 0
fi

# Match only git commit -m / -F patterns
case "$COMMAND" in
    *"git commit "*-m\ *|*"git commit "*--message\ *|*"git commit "*-F\ *|*"git commit "*--file\ *)
        ;;
    *)
        exit 0
        ;;
esac

# Extract commit message body
MSG=""
if echo "$COMMAND" | grep -qE "cat <<['\"]?EOF['\"]?"; then
    MSG=$(echo "$COMMAND" | awk '/cat <<.?EOF.?/{flag=1; next} /^EOF/{flag=0} flag')
elif echo "$COMMAND" | grep -qE -- "-m[[:space:]]+['\"]"; then
    MSG=$(echo "$COMMAND" | sed -nE "s/.*-m[[:space:]]+['\"]([^'\"]*)['\"].*/\1/p")
fi
[ -z "$MSG" ] && exit 0

# Explicit opt-out: [no-sibling] tag in body
if echo "$MSG" | grep -qF '[no-sibling]'; then
    exit 0
fi

# Trivial-body exemption: under 80 chars total → likely typo/dep-bump
MSG_LEN=$(printf '%s' "$MSG" | wc -c)
[ "$MSG_LEN" -lt 80 ] && exit 0

# Pattern: case-insensitive search for sibling-citation phrasing.
# The disjunction covers the documented forms in update-principles.md
# and the operator-exemplar commit (1b9d139c).
SIBLING_REGEX='([Mm]atch(es|ing)?[[:space:]]+(the[[:space:]]+)?.+[[:space:]](pattern|shape|SKIP|invocation)|[Ff]ollow(s|ing)?[[:space:]]+(the[[:space:]]+)?.+[[:space:]](pattern|shape)|[Ss]ibling([[:space:]]+pattern)?:|[Ss]hape[[:space:]]+from[[:space:]]+commit[[:space:]]+[0-9a-f]+|[Mm]irrors([[:space:]]+the)?[[:space:]]+.+[[:space:]](pattern|shape)|[Cc]opies[[:space:]]+from)'

if echo "$MSG" | grep -qE "$SIBLING_REGEX"; then
    exit 0
fi

# No sibling citation found — warn
MSG_OUT="WARN: Commit message body lacks a sibling-pattern citation (docs/contracts/update-principles.md Principle 3). Acceptable forms: 'matching the X pattern', 'follows the X shape', 'sibling pattern: …', 'shape from commit <sha>', 'mirrors the X shape', 'copies from …'. To suppress this warn for genuine first-of-kind work, add [no-sibling] to the body with rationale."

JSON_OUT=$(jq -nc --arg msg "$MSG_OUT" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: $msg}}' 2>/dev/null) || exit 0
echo "$JSON_OUT"

exit 0
