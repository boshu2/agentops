#!/usr/bin/env bash
# watch-codex-stream.sh — JSONL event stream watcher for codex exec --json
#
# Usage: watch-codex-stream.sh <status-file>
#   Reads JSONL from stdin (piped from codex exec --json stdout).
#   Writes structured status JSON to <status-file> on exit.
#
# Exit codes:
#   0 = completed (turn.completed received)
#   1 = error (stream error or no events received)
#   2 = timeout (no events for CODEX_IDLE_TIMEOUT seconds)
#
# Environment:
#   CODEX_IDLE_TIMEOUT  — idle timeout in seconds (default: 60)

set -uo pipefail

STATUS_FILE="${1:?Usage: watch-codex-stream.sh <status-file>}"
IDLE_TIMEOUT="${CODEX_IDLE_TIMEOUT:-60}"

# State tracking
events_count=0
input_tokens=0
output_tokens=0
start_time=$(date +%s)
completed=false
timeout_triggered=""

# Write status file on exit
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
    fi

    cat > "$STATUS_FILE" <<STATUSEOF
{"status":"${status}","token_usage":{"input":${input_tokens},"output":${output_tokens}},"duration_ms":${duration_ms},"events_count":${events_count}}
STATUSEOF

    exit "$exit_code"
}

# Process JSONL line by line with idle timeout
while IFS= read -r -t "$IDLE_TIMEOUT" line; do
    # Skip empty lines
    [[ -z "$line" ]] && continue

    events_count=$((events_count + 1))

    # Parse event type — skip malformed JSON silently
    event_type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null) || continue
    [[ -z "$event_type" ]] && continue

    # Extract token usage from turn.completed events
    if [[ "$event_type" == "turn.completed" ]]; then
        local_input=$(echo "$line" | jq -r '.usage.input_tokens // .usage.input // 0' 2>/dev/null) || local_input=0
        local_output=$(echo "$line" | jq -r '.usage.output_tokens // .usage.output // 0' 2>/dev/null) || local_output=0
        [[ -z "$local_input" || "$local_input" == "null" ]] && local_input=0
        [[ -z "$local_output" || "$local_output" == "null" ]] && local_output=0
        input_tokens=$((input_tokens + local_input))
        output_tokens=$((output_tokens + local_output))
        completed=true
    fi
done

# If read -t timed out, the loop exits with read returning > 128
if [[ "$completed" != "true" && $events_count -eq 0 ]]; then
    # Empty stream — codex likely died immediately
    write_status
elif [[ "$completed" != "true" ]]; then
    # Timed out waiting for events
    timeout_triggered="true"
    write_status
fi

# Normal completion
write_status
