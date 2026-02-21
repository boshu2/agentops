#!/bin/bash
# hook-helpers.sh — Shared utilities for AgentOps hooks
# Source this from any hook that needs structured failure output.
#
# Required before sourcing:
#   ROOT must be set (git rev-parse --show-toplevel or pwd fallback)
#
# Provides:
#   write_failure TYPE COMMAND EXIT_CODE DETAILS
#     Writes structured JSON to $ROOT/.agents/ao/last-failure.json
#     Callers should also echo human-readable message to stderr.

# Guard: ROOT must be set
if [ -z "${ROOT:-}" ]; then
  ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
fi

_HOOK_HELPERS_ERROR_LOG_DIR="${ROOT}/.agents/ao"

write_failure() {
    local type="$1"
    local command="$2"
    local exit_code="$3"
    local details="$4"

    mkdir -p "$_HOOK_HELPERS_ERROR_LOG_DIR" 2>/dev/null

    local task_subject="unknown"
    if [ -n "${INPUT:-}" ] && command -v jq >/dev/null 2>&1; then
        task_subject=$(echo "$INPUT" | jq -r '.subject // "unknown"' 2>/dev/null) || true
        [ -z "$task_subject" ] || [ "$task_subject" = "null" ] && task_subject="unknown"
    fi

    local ts
    ts=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

    if command -v jq >/dev/null 2>&1; then
        jq -n \
            --argjson schema_version 1 \
            --arg ts "$ts" \
            --arg type "$type" \
            --arg command "$command" \
            --argjson exit_code "$exit_code" \
            --arg task_subject "$task_subject" \
            --arg details "$details" \
            '{schema_version:$schema_version,ts:$ts,type:$type,command:$command,exit_code:$exit_code,task_subject:$task_subject,details:$details}' \
            > "$_HOOK_HELPERS_ERROR_LOG_DIR/last-failure.json" 2>/dev/null
    else
        local escaped_command escaped_subject escaped_details
        escaped_command=$(printf '%s' "$command" | sed 's/["\\]/\\&/g')
        escaped_subject=$(printf '%s' "$task_subject" | sed 's/["\\]/\\&/g')
        escaped_details=$(printf '%s' "$details" | sed 's/["\\]/\\&/g')

        printf '{"schema_version":1,"ts":"%s","type":"%s","command":"%s","exit_code":%d,"task_subject":"%s","details":"%s"}\n' \
            "$ts" "$type" "$escaped_command" "$exit_code" "$escaped_subject" "$escaped_details" \
            > "$_HOOK_HELPERS_ERROR_LOG_DIR/last-failure.json" 2>/dev/null
    fi
}

# read_hook_input — Read stdin and extract last_assistant_message.
# Sets global variables: INPUT, LAST_ASSISTANT_MSG
# Usage: call at top of hook script, then use $LAST_ASSISTANT_MSG
read_hook_input() {
    INPUT=$(cat)
    LAST_ASSISTANT_MSG=""
    if [ -n "$INPUT" ]; then
        if command -v jq >/dev/null 2>&1; then
            LAST_ASSISTANT_MSG=$(echo "$INPUT" | jq -r '.last_assistant_message // ""' 2>/dev/null) || true
        fi
        # Fallback without jq
        if [ -z "$LAST_ASSISTANT_MSG" ] && [ -n "$INPUT" ]; then
            LAST_ASSISTANT_MSG=$(echo "$INPUT" | grep -o '"last_assistant_message"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
                | sed 's/.*"last_assistant_message"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
        fi
    fi
}
