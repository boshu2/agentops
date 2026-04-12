#!/usr/bin/env bash
# team-runner.sh — Headless Codex/Claude team orchestrator
#
# Usage: team-runner.sh <team-spec.json>
#   Reads a team spec, spawns parallel headless workers for the selected
#   runtime, watches JSONL streams for completion/timeout, validates outputs,
#   retries failures, and produces a team report.
#
# Environment:
#   CODEX_MODEL           — Codex model (default: gpt-5.3-codex)
#   CODEX_IDLE_TIMEOUT    — Per-agent idle timeout in seconds (default: 60)
#   CLAUDE_MODEL          — Claude model (default: sonnet)
#   CLAUDE_IDLE_TIMEOUT   — Per-agent idle timeout in seconds (default: 60)
#   CLAUDE_MAX_TURNS      — Max turns per Claude worker (default: 6)
#   CLAUDE_MAX_BUDGET_USD — Max budget per Claude worker (default: 5)
#   TEAM_RUNNER_MAX_AGENTS — Max concurrent agents (default: 6)
#   TEAM_RUNNER_DRY_RUN   — If set, print commands without executing
#   BEADS_NO_DAEMON       — Set automatically to prevent beads conflicts

set -uo pipefail

SPEC_FILE="${1:?Usage: team-runner.sh <team-spec.json>}"
CODEX_MODEL="${CODEX_MODEL:-gpt-5.3-codex}"
CODEX_IDLE_TIMEOUT="${CODEX_IDLE_TIMEOUT:-60}"
CLAUDE_MODEL="${CLAUDE_MODEL:-sonnet}"
CLAUDE_IDLE_TIMEOUT="${CLAUDE_IDLE_TIMEOUT:-60}"
CLAUDE_MAX_TURNS="${CLAUDE_MAX_TURNS:-6}"
CLAUDE_MAX_BUDGET_USD="${CLAUDE_MAX_BUDGET_USD:-5}"
MAX_AGENTS="${TEAM_RUNNER_MAX_AGENTS:-6}"
MAX_RETRIES=3
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEAM_RUNTIME=""
WATCHER=""
PROCESS_LABEL=""
MODEL_LABEL=""

# Cleanup on exit: kill any orphaned background jobs.
# shellcheck disable=SC2329
cleanup() {
    local pids
    pids=$(jobs -p 2>/dev/null)
    if [[ -n "$pids" ]]; then
        # Intentionally expand the PID list into multiple arguments.
        # shellcheck disable=SC2086
        kill $pids 2>/dev/null || true
        # shellcheck disable=SC2086
        wait $pids 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

# --- Pre-flight checks ---

preflight() {
    local fail=0
    for cmd in jq git; do
        if ! command -v "$cmd" &>/dev/null; then
            echo "ERROR: $cmd not found on PATH" >&2
            fail=1
        fi
    done
    if [[ ! -f "$SPEC_FILE" ]]; then
        echo "ERROR: Team spec not found: $SPEC_FILE" >&2
        fail=1
    fi
    if ! jq empty "$SPEC_FILE" 2>/dev/null; then
        echo "ERROR: Invalid JSON in $SPEC_FILE" >&2
        fail=1
    fi
    [[ $fail -ne 0 ]] && exit 1

    TEAM_RUNTIME=$(jq -r '.runtime // "codex"' "$SPEC_FILE")
    case "$TEAM_RUNTIME" in
        codex)
            WATCHER="${SCRIPT_DIR}/watch-codex-stream.sh"
            PROCESS_LABEL="codex"
            MODEL_LABEL="${CODEX_MODEL}"
            if ! command -v codex &>/dev/null; then
                echo "ERROR: codex not found on PATH" >&2
                fail=1
            fi
            ;;
        claude)
            WATCHER="${SCRIPT_DIR}/watch-claude-stream.sh"
            PROCESS_LABEL="claude"
            MODEL_LABEL="${CLAUDE_MODEL}"
            if ! command -v claude &>/dev/null; then
                echo "ERROR: claude not found on PATH" >&2
                fail=1
            fi
            ;;
        *)
            echo "ERROR: Unsupported runtime in spec: $TEAM_RUNTIME" >&2
            fail=1
            ;;
    esac

    if [[ ! -f "$WATCHER" ]]; then
        echo "ERROR: Watcher not found at $WATCHER" >&2
        fail=1
    fi

    [[ $fail -ne 0 ]] && exit 1

    # Prevent beads daemon conflicts
    export BEADS_NO_DAEMON=1
}

# --- Parse team spec ---

parse_spec() {
    TEAM_ID=$(jq -r '.team_id' "$SPEC_FILE")
    REPO_PATH=$(jq -r '.repo_path' "$SPEC_FILE")
    AGENT_COUNT=$(jq '.agents | length' "$SPEC_FILE")

    if [[ -z "$TEAM_ID" || "$TEAM_ID" == "null" ]]; then
        echo "ERROR: team_id is required in spec" >&2
        exit 1
    fi
    if ! echo "$TEAM_ID" | grep -qE '^[a-zA-Z0-9_-]+$'; then
        echo "ERROR: team_id must match [a-zA-Z0-9_-]+" >&2
        exit 1
    fi
    if [[ ! -d "$REPO_PATH" ]]; then
        echo "ERROR: repo_path does not exist: $REPO_PATH" >&2
        exit 1
    fi
    if [[ "$AGENT_COUNT" -eq 0 ]]; then
        echo "ERROR: No agents defined in spec" >&2
        exit 1
    fi
    if [[ "$AGENT_COUNT" -gt "$MAX_AGENTS" ]]; then
        echo "WARN: Agent count ($AGENT_COUNT) exceeds max ($MAX_AGENTS). Capping." >&2
        AGENT_COUNT=$MAX_AGENTS
    fi

    TEAM_DIR=".agents/teams/${TEAM_ID}"
    mkdir -p "$TEAM_DIR"
}

# --- Spawn a single agent ---

spawn_agent() {
    local idx=$1
    local attempt=$2
    local extra_context="${3:-}"

    local name
    name=$(jq -r ".agents[$idx].name" "$SPEC_FILE")
    local prompt
    prompt=$(jq -r ".agents[$idx].prompt" "$SPEC_FILE")
    local sandbox
    sandbox=$(jq -r ".agents[$idx].sandbox_level" "$SPEC_FILE")
    local timeout_ms
    timeout_ms=$(jq -r ".agents[$idx].timeout_ms" "$SPEC_FILE")

    local agent_dir="${TEAM_DIR}/${name}"
    mkdir -p "$agent_dir"

    # Build sandbox flags as array for safe expansion
    local -a sandbox_args
    if [[ "$sandbox" == "read-only" ]]; then
        sandbox_args=(-s read-only)
    elif [[ "$sandbox" == "danger-full-access" ]]; then
        sandbox_args=(-s danger-full-access)
    else
        sandbox_args=(--full-auto)
    fi

    # Inject retry context if this is a retry (sanitize to prevent prompt injection)
    if [[ -n "$extra_context" ]]; then
        # Strip control characters and limit length to prevent prompt manipulation
        local sanitized_context
        sanitized_context=$(printf '%s' "$extra_context" | tr -d '\000-\011\013-\037' | head -c 4096)
        prompt="${prompt}

RETRY CONTEXT (attempt ${attempt}/${MAX_RETRIES}):
${sanitized_context}"
    fi

    local output_file="${agent_dir}/output.json"
    local status_file="${agent_dir}/status.json"
    local exit_file="${agent_dir}/process-exit.txt"

    # Resolve schema path (absolute so it works regardless of -C)
    local schema_path
    schema_path="$(cd "$REPO_PATH" && pwd)/lib/schemas/worker-output.json"
    local schema_json
    schema_json=$(jq -c . "$schema_path")

    if [[ -n "${TEAM_RUNNER_DRY_RUN:-}" ]]; then
        if [[ "$TEAM_RUNTIME" == "claude" ]]; then
            echo "[DRY RUN] (cd ${REPO_PATH} && claude -p --model ${CLAUDE_MODEL} --plugin-dir ${REPO_PATH} --dangerously-skip-permissions --max-turns ${CLAUDE_MAX_TURNS} --no-session-persistence --max-budget-usd ${CLAUDE_MAX_BUDGET_USD} --output-format stream-json --verbose --json-schema '${schema_json}' \"${prompt:0:80}...\")" >&2
        else
            echo "[DRY RUN] codex exec ${sandbox_args[*]} --json -m ${CODEX_MODEL} -C ${REPO_PATH} --output-schema ${schema_path} -o ${output_file} \"${prompt:0:80}...\"" >&2
        fi
        echo 0 > "$exit_file"
        echo '{"status":"completed","token_usage":{"input":0,"output":0},"duration_ms":0,"events_count":0}' > "$status_file"
        echo '{"status":"done","summary":"dry run","artifacts":[],"errors":[],"token_usage":{"input":0,"output":0},"duration_ms":0}' > "$output_file"
        return 0
    fi

    local timeout_s=$(( timeout_ms / 1000 ))
    [[ $timeout_s -lt 10 ]] && timeout_s=120

    # Spawn the selected runtime with JSONL piped to its watcher.
    # Sidecar pattern: capture process exit code separately.
    if [[ "$TEAM_RUNTIME" == "claude" ]]; then
        (
            (
                cd "$REPO_PATH" && timeout "$timeout_s" claude -p "$prompt" \
                    --model "$CLAUDE_MODEL" \
                    --plugin-dir "$REPO_PATH" \
                    --dangerously-skip-permissions \
                    --max-turns "$CLAUDE_MAX_TURNS" \
                    --no-session-persistence \
                    --max-budget-usd "$CLAUDE_MAX_BUDGET_USD" \
                    --output-format stream-json \
                    --verbose \
                    --json-schema "$schema_json" 2>/dev/null
            )
            echo $? > "$exit_file"
        ) | CLAUDE_IDLE_TIMEOUT="$CLAUDE_IDLE_TIMEOUT" bash "$WATCHER" "$status_file" "$output_file" &
    else
        (
            timeout "$timeout_s" codex exec "${sandbox_args[@]}" --json \
                -m "$CODEX_MODEL" \
                -C "$REPO_PATH" \
                --output-schema "$schema_path" \
                -o "$output_file" \
                "$prompt" 2>/dev/null
            echo $? > "$exit_file"
        ) | CODEX_IDLE_TIMEOUT="$CODEX_IDLE_TIMEOUT" bash "$WATCHER" "$status_file" &
    fi

    echo $!
}

# --- Validate agent output ---

validate_agent() {
    local idx=$1
    local name
    name=$(jq -r ".agents[$idx].name" "$SPEC_FILE")
    local agent_dir="${TEAM_DIR}/${name}"

    local status_file="${agent_dir}/status.json"
    local output_file="${agent_dir}/output.json"
    local process_exit="${agent_dir}/process-exit.txt"

    # Check watcher status
    if [[ ! -f "$status_file" ]]; then
        echo "FAIL:no_status_file"
        return 1
    fi

    local watcher_status
    watcher_status=$(jq -r '.status' "$status_file" 2>/dev/null)

    # Check runtime exit code before generic watcher status so a subprocess
    # crash is not masked as EOF/timeout after partial JSONL output.
    if [[ -f "$process_exit" ]]; then
        local exit_code
        exit_code=$(cat "$process_exit")
        if [[ "$exit_code" != "0" ]]; then
            echo "FAIL:${PROCESS_LABEL}_exit_${exit_code}"
            return 1
        fi
    fi

    if [[ "$watcher_status" == "timeout" ]]; then
        echo "FAIL:timeout"
        return 1
    fi

    if [[ "$watcher_status" != "completed" ]]; then
        echo "FAIL:watcher_${watcher_status}"
        return 1
    fi

    # Check output file exists and is valid JSON
    if [[ ! -f "$output_file" ]]; then
        echo "FAIL:no_output"
        return 1
    fi

    if ! jq empty "$output_file" 2>/dev/null; then
        echo "FAIL:invalid_json"
        return 1
    fi

    # Check worker reported success
    local worker_status
    worker_status=$(jq -r '.status' "$output_file" 2>/dev/null)
    if [[ "$worker_status" != "done" ]]; then
        echo "FAIL:worker_${worker_status}"
        return 1
    fi

    echo "PASS"
    return 0
}

# --- Generate team report ---

generate_report() {
    local report="${TEAM_DIR}/team-report.md"
    local total_input=0
    local total_output=0
    local total_duration=0
    local passed=0
    local failed=0

    {
        echo "# Team Report: ${TEAM_ID}"
        echo ""
        echo "**Date:** $(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -Iseconds)"
        echo "**Spec:** ${SPEC_FILE}"
        echo "**Runtime:** ${TEAM_RUNTIME}"
        echo "**Model:** ${MODEL_LABEL}"
        echo ""
        echo "| Agent | Status | Tokens (in/out) | Duration |"
        echo "|-------|--------|-----------------|----------|"

        for ((i=0; i<AGENT_COUNT; i++)); do
            local name
            name=$(jq -r ".agents[$i].name" "$SPEC_FILE")
            local agent_dir="${TEAM_DIR}/${name}"
            local status_file="${agent_dir}/status.json"

            local status="unknown"
            local in_tok=0
            local out_tok=0
            local dur=0

            if [[ -f "$status_file" ]]; then
                status=$(jq -r '.status' "$status_file" 2>/dev/null)
                in_tok=$(jq -r '.token_usage.input // 0' "$status_file" 2>/dev/null)
                out_tok=$(jq -r '.token_usage.output // 0' "$status_file" 2>/dev/null)
                dur=$(jq -r '.duration_ms // 0' "$status_file" 2>/dev/null)
            fi

            total_input=$((total_input + in_tok))
            total_output=$((total_output + out_tok))
            total_duration=$((total_duration + dur))

            if [[ "$status" == "completed" ]]; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi

            echo "| ${name} | ${status} | ${in_tok}/${out_tok} | ${dur}ms |"
        done

        echo ""
        echo "**Totals:** ${passed} passed, ${failed} failed"
        echo "**Tokens:** ${total_input} input, ${total_output} output"
        echo "**Duration:** ${total_duration}ms"
    } > "$report"

    echo "$report"
}

# --- Main ---

main() {
    preflight
    parse_spec

    echo "=== Team Runner: ${TEAM_ID} ==="
    echo "Agents: ${AGENT_COUNT}, Runtime: ${TEAM_RUNTIME}, Model: ${MODEL_LABEL}, Max retries: ${MAX_RETRIES}"
    echo ""

    # Track PIDs for waiting
    declare -A AGENT_PIDS

    # Spawn all agents
    for ((i=0; i<AGENT_COUNT; i++)); do
        local name
        name=$(jq -r ".agents[$i].name" "$SPEC_FILE")
        echo "Spawning agent: ${name}"
        pid=$(spawn_agent "$i" 1)
        AGENT_PIDS[$i]="$pid"
    done

    echo ""
    echo "Waiting for ${AGENT_COUNT} agents..."

    # Wait for all agents
    local any_failed=false
    for ((i=0; i<AGENT_COUNT; i++)); do
        local pid="${AGENT_PIDS[$i]}"
        if [[ -n "${TEAM_RUNNER_DRY_RUN:-}" ]]; then
            continue
        fi
        wait "$pid" 2>/dev/null || true
    done

    echo "All agents completed. Validating..."

    # Validate and retry
    for ((i=0; i<AGENT_COUNT; i++)); do
        local name
        name=$(jq -r ".agents[$i].name" "$SPEC_FILE")
        local result
        result=$(validate_agent "$i")

        if [[ "$result" != "PASS" ]]; then
            echo "  ${name}: ${result} — retrying..."

            for ((attempt=2; attempt<=MAX_RETRIES; attempt++)); do
                local context="Previous attempt failed: ${result}"
                pid=$(spawn_agent "$i" "$attempt" "$context")

                if [[ -z "${TEAM_RUNNER_DRY_RUN:-}" ]]; then
                    wait "$pid" 2>/dev/null || true
                fi

                result=$(validate_agent "$i")
                if [[ "$result" == "PASS" ]]; then
                    echo "  ${name}: PASS (attempt ${attempt})"
                    break
                fi
                echo "  ${name}: ${result} (attempt ${attempt}/${MAX_RETRIES})"
            done

            if [[ "$result" != "PASS" ]]; then
                echo "  ${name}: FAILED after ${MAX_RETRIES} attempts"
                any_failed=true
            fi
        else
            echo "  ${name}: PASS"
        fi
    done

    # Generate report
    local report
    report=$(generate_report)
    echo ""
    echo "Report: ${report}"

    if [[ "$any_failed" == "true" ]]; then
        echo "WARNING: Some agents failed. See report for details."
        exit 1
    fi

    echo "All agents passed."
    exit 0
}

main "$@"
