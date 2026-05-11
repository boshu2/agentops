#!/usr/bin/env bash
# factory-router.sh - UserPromptSubmit hook: capture first substantive prompt as
# factory state when SessionStart had no goal. This hook stays silent.
# practices: [wiki-knowledge-surface, dora-metrics]

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_FACTORY_ROUTER_DISABLED:-}" = "1" ] && exit 0

AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

run_with_timeout() {
    local seconds="$1"
    shift
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "$seconds" "$@" 2>/dev/null
        return $?
    fi
    "$@" 2>/dev/null
}

trim_text() {
    printf '%s' "${1:-}" \
        | tr '\n' ' ' \
        | tr -s '[:space:]' ' ' \
        | sed 's/^ //; s/ $//'
}

resolve_startup_context_mode() {
    local mode
    if [ "${AGENTOPS_STARTUP_LEGACY_INJECT:-}" = "1" ]; then
        printf 'manual'
        return 0
    fi

    mode=$(printf '%s' "${AGENTOPS_STARTUP_CONTEXT_MODE:-factory}" | tr '[:upper:]' '[:lower:]')
    case "$mode" in
        ""|factory)
            printf 'factory'
            ;;
        manual|lean|legacy)
            printf 'manual'
            ;;
        *)
            printf 'factory'
            ;;
    esac
}

build_factory_briefing() {
    local goal="$1"
    local output path

    [ -n "$goal" ] || return 0
    command -v ao >/dev/null 2>&1 || return 0
    command -v jq >/dev/null 2>&1 || return 0

    output=$(run_with_timeout 8 ao knowledge brief --json --goal "$goal") || return 0
    [ -n "$output" ] || return 0

    path=$(printf '%s' "$output" | jq -r '.output_path // empty' 2>/dev/null)
    path=$(trim_text "$path")
    [ -n "$path" ] || return 0
    [ -f "$path" ] || return 0
    printf '%s' "$path"
}

extract_prompt() {
    local input="$1"
    if command -v jq >/dev/null 2>&1; then
        printf '%s' "$input" | jq -r '.prompt // ""' 2>/dev/null
        return 0
    fi
    printf '%s' "$input" \
        | grep -o '"prompt"[[:space:]]*:[[:space:]]*"[^"]*"' \
        | head -1 \
        | sed 's/.*"prompt"[[:space:]]*:[[:space:]]*"//;s/"$//'
}

normalize_factory_goal() {
    local prompt goal
    prompt="$(trim_text "$1")"
    [ -n "$prompt" ] || return 0

    case "$prompt" in
        /session-start*|/session-end*|/quickstart*|/status*|/help*)
            return 0
            ;;
        /rpi*)
            goal="$(trim_text "${prompt#/rpi}")"
            goal="${goal#\"}"
            goal="${goal%\"}"
            prompt="$(trim_text "$goal")"
            ;;
        /*)
            return 0
            ;;
    esac

    case "$(printf '%s' "$prompt" | tr '[:upper:]' '[:lower:]')" in
        ""|"help"|"status"|"where was i"|"where was i?"|"what should i work on"|"what should i work on?"|"continue"|"resume")
            return 0
            ;;
    esac

    if [ "${#prompt}" -lt 12 ]; then
        return 0
    fi

    printf '%s' "$prompt"
}

INPUT=$(cat)
PROMPT="$(extract_prompt "$INPUT")"
[ -n "$PROMPT" ] || exit 0
[ "$(resolve_startup_context_mode)" = "factory" ] || exit 0

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
STATE_DIR="$ROOT/.agents/ao"
INTAKE_FLAG="$STATE_DIR/.factory-intake-needed"
ROUTER_FLAG="$STATE_DIR/.factory-router-fired"
GOAL_FILE="$STATE_DIR/factory-goal.txt"
BRIEFING_FILE="$STATE_DIR/factory-briefing.txt"
mkdir -p "$STATE_DIR" 2>/dev/null

[ -f "$INTAKE_FLAG" ] || exit 0
[ ! -s "$GOAL_FILE" ] || exit 0

if [ -f "$ROUTER_FLAG" ] && find "$ROUTER_FLAG" -mmin -30 2>/dev/null | grep -q .; then
    exit 0
fi

GOAL="$(normalize_factory_goal "$PROMPT")"
[ -n "$GOAL" ] || exit 0

BRIEFING_PATH="$(build_factory_briefing "$GOAL")"
printf '%s' "$GOAL" > "$GOAL_FILE" 2>/dev/null || true
rm -f "$INTAKE_FLAG" 2>/dev/null || true
printf '%s\n' "${CLAUDE_SESSION_ID:-unknown}" > "$ROUTER_FLAG" 2>/dev/null || true

if [ -n "$BRIEFING_PATH" ]; then
    printf '%s' "$BRIEFING_PATH" > "$BRIEFING_FILE" 2>/dev/null || true
else
    rm -f "$BRIEFING_FILE" 2>/dev/null || true
fi

exit 0
