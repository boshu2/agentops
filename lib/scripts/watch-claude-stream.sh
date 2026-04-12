#!/usr/bin/env bash
# watch-claude-stream.sh — JSONL event stream watcher for claude -p
#
# Usage: watch-claude-stream.sh <status-file> <output-file>
#   Reads Claude stream-json events from stdin, writes structured status JSON
#   to <status-file>, and writes final structured output to <output-file>.
#
# Exit codes:
#   0 = completed (result success with structured_output received)
#   1 = error (stream error, EOF before result, missing structured_output, or no events received)
#   2 = timeout (no events for CLAUDE_IDLE_TIMEOUT seconds)
#
# Environment:
#   CLAUDE_IDLE_TIMEOUT — idle timeout in seconds (default: 60)

set -uo pipefail

STATUS_FILE="${1:?Usage: watch-claude-stream.sh <status-file> <output-file>}"
OUTPUT_FILE="${2:?Usage: watch-claude-stream.sh <status-file> <output-file>}"
IDLE_TIMEOUT="${CLAUDE_IDLE_TIMEOUT:-60}"

events_count=0
input_tokens=0
output_tokens=0
start_time=$(date +%s)
completed=false
timeout_triggered=""
eof_triggered=""

write_status() {
    local end_time
    end_time=$(date +%s)
    local duration_ms=$(( (end_time - start_time) * 1000 ))

    local status="error"
    local exit_code=1
    if [[ "$completed" == "true" ]]; then
        status="completed"
        exit_code=0
    elif [[ -n "$timeout_triggered" ]]; then
        status="timeout"
        exit_code=2
    elif [[ -n "$eof_triggered" ]]; then
        status="eof"
    fi

    cat > "$STATUS_FILE" <<STATUSEOF
{"status":"${status}","token_usage":{"input":${input_tokens},"output":${output_tokens}},"duration_ms":${duration_ms},"events_count":${events_count}}
STATUSEOF

    exit "$exit_code"
}

read_status=0
while true; do
    if IFS= read -r -t "$IDLE_TIMEOUT" line; then
        :
    else
        read_status=$?
        break
    fi

    [[ -z "$line" ]] && continue
    events_count=$((events_count + 1))

    event_type=$(printf '%s' "$line" | jq -r '.type // empty' 2>/dev/null) || continue
    [[ -z "$event_type" ]] && continue

    if [[ "$event_type" == "result" ]]; then
        event_subtype=$(printf '%s' "$line" | jq -r '.subtype // empty' 2>/dev/null) || event_subtype=""
        event_error=$(printf '%s' "$line" | jq -r '.is_error // false' 2>/dev/null) || event_error="false"

        if [[ "$event_subtype" == "success" && "$event_error" != "true" ]]; then
            local_input=$(printf '%s' "$line" | jq -r '.usage.input_tokens // 0' 2>/dev/null) || local_input=0
            local_output=$(printf '%s' "$line" | jq -r '.usage.output_tokens // 0' 2>/dev/null) || local_output=0
            [[ -z "$local_input" || "$local_input" == "null" ]] && local_input=0
            [[ -z "$local_output" || "$local_output" == "null" ]] && local_output=0
            input_tokens=$((input_tokens + local_input))
            output_tokens=$((output_tokens + local_output))

            structured_output=$(printf '%s' "$line" | jq -c '.structured_output // empty' 2>/dev/null) || structured_output=""
            if [[ -z "$structured_output" || "$structured_output" == "null" ]]; then
                write_status
            fi

            printf '%s\n' "$structured_output" > "$OUTPUT_FILE"
            completed=true
            break
        fi

        if [[ "$event_error" == "true" ]]; then
            write_status
        fi
    fi
done

if [[ "$completed" != "true" && $events_count -eq 0 ]]; then
    write_status
elif [[ "$completed" != "true" && $read_status -gt 128 ]]; then
    timeout_triggered="true"
    write_status
elif [[ "$completed" != "true" ]]; then
    eof_triggered="true"
    write_status
fi

write_status
