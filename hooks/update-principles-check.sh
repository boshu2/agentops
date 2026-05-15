#!/bin/bash
# update-principles-check.sh — PreToolUse hook: warn when a git commit
# message is missing the fitness-delta signal required by
# docs/contracts/update-principles.md (Principle 4).
#
# Scope (v1): warn-only, non-blocking. Fires on Bash tool calls that
# match `git commit -m …` or `git commit -F …`. Surfaces a warning via
# additionalContext when the proposed commit message body lacks a
# numerical-pair fitness delta (`N → M`, `N/X → M/X`, `N%`).
#
# This is the harness automation that retroactively wires cycle 45's
# hypothesis H45.3 (gate-output structural check). Principle 4 is the
# safest to mechanically enforce: false positives are rare because the
# numerical-pair pattern is concrete; false negatives are tolerable
# because the warn-only posture keeps the commit flowing.
#
# Future principles (TODO, separate cycles):
#  - Principle 2 (drift-blocking test included): warn when staged diff
#    modifies .go/.sh but adds no _test.go/.bats
#  - Principle 1 (single concern): too subjective for mechanical lint;
#    relies on commit-message-body bullet count or file-count heuristic
#
# practices: [continuous-integration, design-by-contract, code-complete]
set -uo pipefail

# Kill switches — both global and hook-specific
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_UPDATE_PRINCIPLES_DISABLED:-}" = "1" ] && exit 0

# Only fire on Bash tool calls
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [ -z "$TOOL_NAME" ]; then
    INPUT=$(cat)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[ "$TOOL_NAME" != "Bash" ] && exit 0

# Extract the command from the tool input
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$COMMAND" ]; then
    [ -z "${INPUT:-}" ] && INPUT=$(cat)
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || exit 0
fi

# Match only `git commit -m …` or `git commit -F …` patterns
# (Skip `git commit --amend` without -m; we can't extract the message)
case "$COMMAND" in
    *"git commit "*-m\ *|*"git commit "*--message\ *|*"git commit "*-F\ *|*"git commit "*--file\ *)
        ;;
    *)
        exit 0
        ;;
esac

# Extract the commit message body. The bash command is shell, so the
# message is a quoted string after -m / --message. Best-effort parse:
# look for content between the first '-m' (or --message) flag and the
# next flag-or-EOL.
MSG=""
# Heredoc patterns are the most common in this repo (see CLAUDE.md
# guidance). They contain the literal commit message between EOF
# markers. Honor those.
if echo "$COMMAND" | grep -qE "cat <<['\"]?EOF['\"]?"; then
    # Extract between EOF markers
    MSG=$(echo "$COMMAND" | awk '/cat <<.?EOF.?/{flag=1; next} /^EOF/{flag=0} flag')
elif echo "$COMMAND" | grep -qE -- "-m[[:space:]]+['\"]"; then
    # -m "message" form
    MSG=$(echo "$COMMAND" | sed -nE "s/.*-m[[:space:]]+['\"]([^'\"]*)['\"].*/\1/p")
fi

# If we couldn't parse a message, exit silently — better to under-warn
# than to over-warn on parse failures.
[ -z "$MSG" ] && exit 0

# Principle 4 check: fitness delta pattern. Acceptable forms:
#   - N → M    (e.g. "39 → 40 contracts catalogued")
#   - N/X → M/X (e.g. "134/139 → 139/139")
#   - N% → M%  (e.g. "75.8% → 80%")
#   - N → N+M (count-only delta written explicitly)
FITNESS_DELTA_REGEX='[0-9]+(\.[0-9]+)?(/[0-9]+)?(%)?[[:space:]]+(→|->|to)[[:space:]]+[0-9]+(\.[0-9]+)?(/[0-9]+)?(%)?'

if echo "$MSG" | grep -qE "$FITNESS_DELTA_REGEX"; then
    # Fitness delta present — pass silently
    exit 0
fi

# Missing fitness delta — emit additionalContext warning (non-blocking)
cat <<WARN_EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"WARN: Commit message body lacks a fitness-delta signal (docs/contracts/update-principles.md Principle 4). Acceptable forms include 'N → M' / 'N/X → M/X' / 'N% → M%'. This is a warning only; commit will proceed. To make the signal mechanical, include a numerical pair somewhere in the body."}}
WARN_EOF

# Always exit 0 — this is warn-only in v1
exit 0
