#!/usr/bin/env bash
set -euo pipefail
# new-user-welcome.sh - UserPromptSubmit hook: show a one-time startup nudge for
# truly fresh repos without changing runtime mode.
# practices: [lean-startup, agile-manifesto]

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_NEW_USER_WELCOME_DISABLED:-}" = "1" ] && exit 0

trim_text() {
    printf '%s' "${1:-}" \
        | tr '\n' ' ' \
        | tr -s '[:space:]' ' ' \
        | sed 's/^ //; s/ $//'
}

extract_prompt() {
    local input="$1"
    if command -v jq >/dev/null 2>&1; then
        printf '%s' "$input" | jq -r '.prompt // ""' 2>/dev/null || true
        return 0
    fi
    printf '%s' "$input" \
        | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' \
        | head -1 \
        | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//' || true
}

is_substantive_prompt() {
    local prompt lowered
    prompt="$(trim_text "$1")"
    [ -n "$prompt" ] || return 1

    case "$prompt" in
        /session-start*|/session-end*|/quickstart*|/status*|/help*|/research*|/implement*|/council*|/*)
            return 1
            ;;
    esac

    lowered="$(printf '%s' "$prompt" | tr '[:upper:]' '[:lower:]')"
    case "$lowered" in
        ""|"help"|"status"|"where was i"|"where was i?"|"what should i work on"|"what should i work on?"|"continue"|"resume")
            return 1
            ;;
    esac

    [ "${#prompt}" -ge 12 ]
}

INPUT=$(cat)
PROMPT="$(extract_prompt "$INPUT")"
[ -n "$PROMPT" ] || exit 0
[ "$PROMPT" != "null" ] || exit 0

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
STATE_DIR="$ROOT/.agents/ao"
MARKER="$STATE_DIR/.new-user-welcome-needed"

[ -f "$MARKER" ] || exit 0
is_substantive_prompt "$PROMPT" || exit 0

trap 'rm -f "$MARKER" 2>/dev/null || true' EXIT

WELCOME_MSG=$(cat <<'EOF'
NEW TO AGENTOPS? Start with one of these:
- `/research "how does auth work"` to understand the repo before changing it
- `/implement "fix the login bug"` to run one scoped task end to end
- `/council validate this plan` to pressure-test a plan, PR, or direction before shipping
EOF
)

if command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$WELCOME_MSG" '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":$ctx}}'
else
    safe_msg=${WELCOME_MSG//\\/\\\\}
    safe_msg=${safe_msg//\"/\\\"}
    safe_msg=$(printf '%s' "$safe_msg" | tr '\n' ' ')
    printf '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"%s"}}\n' "$safe_msg"
fi

exit 0
