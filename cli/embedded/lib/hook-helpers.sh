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
#
#   check_hook_failure_budget HOOK_NAME
#     Tracks consecutive failures per hook. Returns 1 (skip hook) when
#     MAX_CONSECUTIVE_HOOK_FAILURES (default 3, override via
#     AGENTOPS_HOOK_FAILURE_CAP) is exceeded. Call record_hook_success
#     on success to reset the counter.

# --- Hook failure budget ---
# Caps consecutive failures per hook to prevent runaway loops.
# Inspired by Claude Code's MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES pattern.
MAX_CONSECUTIVE_HOOK_FAILURES="${AGENTOPS_HOOK_FAILURE_CAP:-3}"

# check_hook_failure_budget HOOK_NAME
# Returns 0 if hook may run, 1 if budget exhausted (caller should skip).
check_hook_failure_budget() {
    local hook_name="$1"
    # Don't count when hooks are globally disabled
    [[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && return 0

    local state_dir="${ROOT:-.}/.agents/ao"
    local counter_file="$state_dir/.hook-failures-${hook_name}"
    mkdir -p "$state_dir" 2>/dev/null || return 0

    local count=0
    [ -f "$counter_file" ] && count=$(cat "$counter_file" 2>/dev/null || echo 0)

    if [ "$count" -ge "$MAX_CONSECUTIVE_HOOK_FAILURES" ]; then
        echo "HOOK DISABLED: $hook_name failed $count consecutive times (cap=$MAX_CONSECUTIVE_HOOK_FAILURES). Set AGENTOPS_HOOK_FAILURE_CAP to adjust." >&2
        return 1
    fi
    return 0
}

# record_hook_failure HOOK_NAME
# Increments the consecutive failure counter for a hook.
record_hook_failure() {
    local hook_name="$1"
    [[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && return 0

    local state_dir="${ROOT:-.}/.agents/ao"
    local counter_file="$state_dir/.hook-failures-${hook_name}"
    mkdir -p "$state_dir" 2>/dev/null || return 0

    local count=0
    [ -f "$counter_file" ] && count=$(cat "$counter_file" 2>/dev/null || echo 0)
    count=$((count + 1))
    echo "$count" > "$counter_file" 2>/dev/null || true

    if [ "$count" -ge "$MAX_CONSECUTIVE_HOOK_FAILURES" ]; then
        echo "WARNING: $hook_name has failed $count consecutive times. Hook will be disabled on next invocation." >&2
    fi
}

# record_hook_success HOOK_NAME
# Resets the consecutive failure counter on success.
record_hook_success() {
    local hook_name="$1"
    local state_dir="${ROOT:-.}/.agents/ao"
    local counter_file="$state_dir/.hook-failures-${hook_name}"
    rm -f "$counter_file" 2>/dev/null || true
}

# Guard: ROOT must be set
if [ -z "${ROOT:-}" ]; then
  ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
fi

_HOOK_HELPERS_ERROR_LOG_DIR="${ROOT}/.agents/ao"
_HOOK_PACKET_ROOT="${ROOT}/.agents/ao/packets"
_HOOK_PACKET_PENDING_DIR="${_HOOK_PACKET_ROOT}/pending"
_EVIDENCE_ONLY_CLOSURE_DIR="${ROOT}/.agents/council/evidence-only-closures"
_EVIDENCE_ONLY_CLOSURE_RELEASE_DIR="${ROOT}/.agents/releases/evidence-only-closures"

to_repo_relative_path() {
    local abs="$1"
    local repo="${ROOT%/}"
    case "$abs" in
        "$repo"/*) printf '.%s\n' "${abs#"$repo"}" ;;
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
        escaped_command=$(json_escape_value "$command")
        escaped_subject=$(json_escape_value "$task_subject")
        escaped_details=$(json_escape_value "$details")

        printf '{"schema_version":1,"ts":"%s","type":"%s","command":"%s","exit_code":%d,"task_subject":"%s","details":"%s"}\n' \
            "$ts" "$type" "$escaped_command" "$exit_code" "$escaped_subject" "$escaped_details" \
            > "$_HOOK_HELPERS_ERROR_LOG_DIR/last-failure.json" 2>/dev/null
    fi
}

# validate_restricted_cmd CMD [CONTEXT]
# Shared command validation: metachar block + bare-name guard + strict allowlist.
# Returns 0 if safe, 1 if blocked (message on stderr).
# Callers run the command themselves after validation passes.
validate_restricted_cmd() {
    local cmd="$1"
    local context="${2:-command}"

    # Block shell metacharacters and control chars
    if [[ "$cmd" == *$'\n'* ]] || [[ "$cmd" =~ [\;\|\&\`\$\(\)\<\>\'\"\\\] ]]; then
        echo "VALIDATION BLOCKED: shell metacharacters not allowed in $context" >&2
        return 1
    fi

    local -a _vrc_parts
    read -ra _vrc_parts <<< "$cmd"
    local binary="${_vrc_parts[0]}"

    # Binary must be a bare name (no path separators)
    if [[ "$binary" == */* ]]; then
        echo "VALIDATION BLOCKED: binary must be a bare name, not a path" >&2
        return 1
    fi

    # Strict allowlist
    local allowed="go pytest npm make"
    local found=0
    for a in $allowed; do
        [ "$binary" = "$a" ] && { found=1; break; }
    done
    if [ "$found" -ne 1 ]; then
        echo "VALIDATION BLOCKED: command '$binary' not in allowlist ($allowed)" >&2
        return 1
    fi

    return 0
}

# json_escape_value — Escape a string for safe use as a JSON string value.
# Handles: backslashes, double quotes, newlines, tabs, carriage returns.
# Usage: ESCAPED=$(json_escape_value "$RAW_VALUE")
json_escape_value() {
    local value="${1:-}"
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//$'\n'/\\n}
    value=${value//$'\r'/\\r}
    value=${value//$'\t'/\\t}
    printf '%s' "$value"
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

# validate_evidence_only_closure_packet_file — shallow schema check for
# evidence-only-closure v1 artifacts.
validate_evidence_only_closure_packet_file() {
    local packet_file="$1"
    [ -f "$packet_file" ] || return 1

    if command -v jq >/dev/null 2>&1; then
        jq -e '
            .schema_version == 1 and
            (.artifact_id | type == "string" and length > 0) and
            (.target_id | type == "string" and length > 0) and
            (.target_type | type == "string" and length > 0) and
            (.created_at | type == "string" and length > 0) and
            (.producer | type == "string" and length > 0) and
            (.evidence_mode | IN("commit", "staged", "worktree")) and
            (.validation_commands | type == "array" and length > 0) and
            all(.validation_commands[]; type == "string" and length > 0) and
            (.repo_state | type == "object") and
            (.repo_state.repo_root | type == "string" and length > 0) and
            (.repo_state.git_branch | type == "string") and
            (.repo_state.git_dirty | type == "boolean") and
            (.repo_state.head_sha | type == "string") and
            (.repo_state.modified_files | type == "array") and
            all(.repo_state.modified_files[]; type == "string") and
            (.repo_state.staged_files | type == "array") and
            all(.repo_state.staged_files[]; type == "string") and
            (.repo_state.unstaged_files | type == "array") and
            all(.repo_state.unstaged_files[]; type == "string") and
            (.repo_state.untracked_files | type == "array") and
            all(.repo_state.untracked_files[]; type == "string") and
            (.evidence | type == "object") and
            (.evidence.summary | type == "string" and length > 0) and
            (.evidence.artifacts | type == "array" and length > 0) and
            all(.evidence.artifacts[]; type == "string") and
            (.evidence.notes | type == "array") and
            all(.evidence.notes[]; type == "string")
        ' "$packet_file" >/dev/null 2>&1
        return $?
    fi

    grep -q '"schema_version"' "$packet_file" &&
        grep -q '"artifact_id"' "$packet_file" &&
        grep -q '"target_id"' "$packet_file" &&
        grep -q '"target_type"' "$packet_file" &&
        grep -q '"created_at"' "$packet_file" &&
        grep -q '"producer"' "$packet_file" &&
        grep -q '"evidence_mode"' "$packet_file" &&
        grep -q '"validation_commands"' "$packet_file" &&
        grep -q '"repo_state"' "$packet_file" &&
        grep -q '"evidence"' "$packet_file"
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

# write_evidence_only_closure_packet_to_dir TARGET_ID TARGET_TYPE PRODUCER
#   EVIDENCE_MODE VALIDATION_COMMANDS_JSON REPO_STATE_JSON EVIDENCE_JSON OUTPUT_DIR
# Emits a v1 evidence-only closure packet into the requested directory and
# prints the artifact path.
write_evidence_only_closure_packet_to_dir() {
    local target_id="$1"
    local target_type="$2"
    local producer="$3"
    local evidence_mode="$4"
    local validation_commands_json="$5"
    local repo_state_json="$6"
    local evidence_json="$7"
    local output_dir="$8"

    command -v jq >/dev/null 2>&1 || return 1
    mkdir -p "$output_dir" 2>/dev/null || return 1

    [ -n "$target_id" ] || return 1
    [ -n "$target_type" ] || return 1
    [ -n "$producer" ] || return 1
    [ -n "$output_dir" ] || return 1
    case "$evidence_mode" in
        commit|staged|worktree) ;;
        *) return 1 ;;
    esac

    echo "$validation_commands_json" | jq -e 'type == "array" and length > 0 and all(.[]; type == "string" and length > 0)' >/dev/null 2>&1 || return 1
    echo "$repo_state_json" | jq -e '
        type == "object" and
        (.repo_root | type == "string" and length > 0) and
        (.git_branch | type == "string") and
        (.git_dirty | type == "boolean") and
        (.head_sha | type == "string") and
        (.modified_files | type == "array") and
        all(.modified_files[]; type == "string") and
        (.staged_files | type == "array") and
        all(.staged_files[]; type == "string") and
        (.unstaged_files | type == "array") and
        all(.unstaged_files[]; type == "string") and
        (.untracked_files | type == "array") and
        all(.untracked_files[]; type == "string")
    ' >/dev/null 2>&1 || return 1
    echo "$evidence_json" | jq -e '
        type == "object" and
        (.summary | type == "string" and length > 0) and
        (.artifacts | type == "array") and
        all(.artifacts[]; type == "string") and
        (.notes | type == "array") and
        all(.notes[]; type == "string")
    ' >/dev/null 2>&1 || return 1

    local created_at safe_target artifact_id artifact_file
    created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
    safe_target="${target_id//\//_}"
    artifact_id="evidence-only-closure-${safe_target}"
    artifact_file="${output_dir}/${safe_target}.json"

    jq -n \
        --arg schema "../../../schemas/evidence-only-closure.v1.schema.json" \
        --argjson schema_version 1 \
        --arg artifact_id "$artifact_id" \
        --arg target_id "$target_id" \
        --arg target_type "$target_type" \
        --arg created_at "$created_at" \
        --arg producer "$producer" \
        --arg evidence_mode "$evidence_mode" \
        --argjson validation_commands "$validation_commands_json" \
        --argjson repo_state "$repo_state_json" \
        --argjson evidence "$evidence_json" \
        '{
            "$schema": $schema,
            schema_version: $schema_version,
            artifact_id: $artifact_id,
            target_id: $target_id,
            target_type: $target_type,
            created_at: $created_at,
            producer: $producer,
            evidence_mode: $evidence_mode,
            validation_commands: $validation_commands,
            repo_state: $repo_state,
            evidence: $evidence
        }' > "$artifact_file" 2>/dev/null || return 1

    if ! validate_evidence_only_closure_packet_file "$artifact_file"; then
        rm -f "$artifact_file" 2>/dev/null || true
        return 1
    fi

    printf '%s\n' "$artifact_file"
    return 0
}

# write_evidence_only_closure_packet TARGET_ID TARGET_TYPE PRODUCER
#   EVIDENCE_MODE VALIDATION_COMMANDS_JSON REPO_STATE_JSON EVIDENCE_JSON
# Emits a v1 evidence-only closure packet under
# .agents/council/evidence-only-closures and a durable tracked copy under
# .agents/releases/evidence-only-closures. Prints the council artifact path.
write_evidence_only_closure_packet() {
    local target_id="$1"
    local target_type="$2"
    local producer="$3"
    local evidence_mode="$4"
    local validation_commands_json="$5"
    local repo_state_json="$6"
    local evidence_json="$7"
    local local_artifact release_artifact

    local_artifact="$(
        write_evidence_only_closure_packet_to_dir \
            "$target_id" \
            "$target_type" \
            "$producer" \
            "$evidence_mode" \
            "$validation_commands_json" \
            "$repo_state_json" \
            "$evidence_json" \
            "$_EVIDENCE_ONLY_CLOSURE_DIR"
    )" || return 1

    release_artifact="$(
        write_evidence_only_closure_packet_to_dir \
            "$target_id" \
            "$target_type" \
            "$producer" \
            "$evidence_mode" \
            "$validation_commands_json" \
            "$repo_state_json" \
            "$evidence_json" \
            "$_EVIDENCE_ONLY_CLOSURE_RELEASE_DIR"
    )" || {
        rm -f "$local_artifact" 2>/dev/null || true
        return 1
    }

    [ -n "$release_artifact" ] || {
        rm -f "$local_artifact" 2>/dev/null || true
        return 1
    }

    printf '%s\n' "$local_artifact"
    return 0
}

# _validate_restricted_cmd CMD ALLOWED...
# Validate a command is in an allowlist before executing.
# Usage: _validate_restricted_cmd "command_string" allowed_array
# Returns 1 if the command binary is not in the allowlist.
_validate_restricted_cmd() {
    local cmd="$1"
    shift
    local -a allowlist=("$@")
    local binary
    binary=$(echo "$cmd" | awk '{print $1}')
    for allowed in "${allowlist[@]}"; do
        if [ "$binary" = "$allowed" ]; then
            return 0
        fi
    done
    echo "BLOCKED: '$binary' not in allowlist: ${allowlist[*]}" >&2
    return 1
}
