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
_HOOK_PACKET_ROOT="${ROOT}/.agents/ao/packets"
_HOOK_PACKET_PENDING_DIR="${_HOOK_PACKET_ROOT}/pending"

to_repo_relative_path() {
    local abs="$1"
    local repo="${ROOT%/}"
    case "$abs" in
        "$repo"/*) printf '.%s\n' "${abs#$repo}" ;;
        *) printf '%s\n' "$abs" ;;
    esac
}

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

# json_escape_value — Escape a string for safe use as a JSON string value.
# Handles: backslashes, double quotes, newlines, tabs, carriage returns.
# Usage: ESCAPED=$(json_escape_value "$RAW_VALUE")
json_escape_value() {
    printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/	/\\t/g' | tr '\n' ' ' | tr '\r' ' '
}

# timeout_run SECONDS COMMAND [ARGS...]
# Uses GNU timeout if available, falls back to gtimeout (macOS coreutils),
# and finally runs without timeout to preserve fail-open hook behavior.
timeout_run() {
    local seconds="$1"
    shift
    if command -v timeout >/dev/null 2>&1; then
        timeout "$seconds" "$@"
    elif command -v gtimeout >/dev/null 2>&1; then
        gtimeout "$seconds" "$@"
    else
        "$@"
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

# validate_memory_packet_file — shallow schema check for memory-packet v1.
# Returns 0 if valid, non-zero otherwise.
validate_memory_packet_file() {
    local packet_file="$1"
    [ -f "$packet_file" ] || return 1

    if command -v jq >/dev/null 2>&1; then
        jq -e '
            .schema_version == 1 and
            (.packet_id | type == "string" and length > 0) and
            (.packet_type | type == "string" and length > 0) and
            (.created_at | type == "string" and length > 0) and
            (.source_hook | type == "string" and length > 0) and
            (.session_id | type == "string" and length > 0) and
            (.payload | type == "object")
        ' "$packet_file" >/dev/null 2>&1
        return $?
    fi

    # Fallback (no jq): coarse key-presence checks.
    grep -q '"schema_version"' "$packet_file" &&
        grep -q '"packet_id"' "$packet_file" &&
        grep -q '"packet_type"' "$packet_file" &&
        grep -q '"created_at"' "$packet_file" &&
        grep -q '"source_hook"' "$packet_file" &&
        grep -q '"session_id"' "$packet_file" &&
        grep -q '"payload"' "$packet_file"
}

# write_memory_packet TYPE SOURCE PAYLOAD_JSON [HANDOFF_FILE]
# Emits a v1 memory packet under .agents/ao/packets/pending and prints packet path.
write_memory_packet() {
    local packet_type="$1"
    local source_hook="$2"
    local payload_json="$3"
    local handoff_file="${4:-}"

    mkdir -p "$_HOOK_PACKET_PENDING_DIR" 2>/dev/null || return 1

    local created_at safe_ts packet_id packet_file session_id
    created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
    safe_ts=$(date -u +%Y%m%dT%H%M%SZ 2>/dev/null || echo "unknown")
    session_id="${CLAUDE_SESSION_ID:-unknown}"
    packet_id="${safe_ts}-${packet_type}-$$"
    packet_file="${_HOOK_PACKET_PENDING_DIR}/${packet_id}.json"

    if command -v jq >/dev/null 2>&1; then
        if [ -z "$payload_json" ] || ! echo "$payload_json" | jq -e . >/dev/null 2>&1; then
            payload_json='{}'
        fi

        if [ -n "$handoff_file" ]; then
            jq -n \
                --argjson schema_version 1 \
                --arg packet_id "$packet_id" \
                --arg packet_type "$packet_type" \
                --arg created_at "$created_at" \
                --arg source_hook "$source_hook" \
                --arg session_id "$session_id" \
                --arg handoff_file "$handoff_file" \
                --argjson payload "$payload_json" \
                '{
                    schema_version: $schema_version,
                    packet_id: $packet_id,
                    packet_type: $packet_type,
                    created_at: $created_at,
                    source_hook: $source_hook,
                    session_id: $session_id,
                    handoff_file: $handoff_file,
                    payload: $payload
                }' > "$packet_file" 2>/dev/null || return 1
        else
            jq -n \
                --argjson schema_version 1 \
                --arg packet_id "$packet_id" \
                --arg packet_type "$packet_type" \
                --arg created_at "$created_at" \
                --arg source_hook "$source_hook" \
                --arg session_id "$session_id" \
                --argjson payload "$payload_json" \
                '{
                    schema_version: $schema_version,
                    packet_id: $packet_id,
                    packet_type: $packet_type,
                    created_at: $created_at,
                    source_hook: $source_hook,
                    session_id: $session_id,
                    payload: $payload
                }' > "$packet_file" 2>/dev/null || return 1
        fi
    else
        local esc_payload esc_handoff
        esc_payload=$(json_escape_value "$payload_json")
        esc_handoff=$(json_escape_value "$handoff_file")
        if [ -n "$handoff_file" ]; then
            printf '{"schema_version":1,"packet_id":"%s","packet_type":"%s","created_at":"%s","source_hook":"%s","session_id":"%s","handoff_file":"%s","payload":{"raw":"%s"}}\n' \
                "$packet_id" "$packet_type" "$created_at" "$source_hook" "$session_id" "$esc_handoff" "$esc_payload" \
                > "$packet_file" 2>/dev/null || return 1
        else
            printf '{"schema_version":1,"packet_id":"%s","packet_type":"%s","created_at":"%s","source_hook":"%s","session_id":"%s","payload":{"raw":"%s"}}\n' \
                "$packet_id" "$packet_type" "$created_at" "$source_hook" "$session_id" "$esc_payload" \
                > "$packet_file" 2>/dev/null || return 1
        fi
    fi

    if ! validate_memory_packet_file "$packet_file"; then
        rm -f "$packet_file" 2>/dev/null || true
        return 1
    fi

    printf '%s\n' "$packet_file"
    return 0
}
