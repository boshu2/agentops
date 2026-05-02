#!/usr/bin/env bash
set -euo pipefail

OUT=""
REQUIRED=false
WAIVER="${AGENTOPS_RELEASE_HIL_WAIVER:-}"
TIMEOUT_SECONDS="${AGENTOPS_RELEASE_HIL_TIMEOUT:-30}"
ENV_TARGETS="${AGENTOPS_RELEASE_HIL_TARGETS:-}"
TARGETS=()

usage() {
    cat <<'USAGE'
Usage: scripts/check-release-hil.sh [options]

Capture release HIL evidence from real targets.

Options:
  --out PATH          Write JSON evidence to PATH
  --target SPEC       Add a target; repeatable
                      local:<name>:<command>
                      ssh:<name>:<host>:<command>
  --targets TEXT      Newline-separated target specs
  --required          Fail when no target is available unless --waiver is set
  --waiver TEXT       Record an explicit HIL waiver
  --timeout SECONDS   Per-target timeout when timeout(1) is available (default: 30)
  -h, --help          Show this help

AGENTOPS_RELEASE_HIL_TARGETS may also provide newline-separated target specs.
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --out)
            OUT="${2:-}"
            shift 2
            ;;
        --target)
            TARGETS+=("${2:-}")
            shift 2
            ;;
        --targets)
            ENV_TARGETS="${2:-}"
            shift 2
            ;;
        --required)
            REQUIRED=true
            shift
            ;;
        --waiver)
            WAIVER="${2:-}"
            shift 2
            ;;
        --timeout)
            TIMEOUT_SECONDS="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required for release HIL evidence" >&2
    exit 1
fi

if [[ ! "$TIMEOUT_SECONDS" =~ ^[0-9]+$ || "$TIMEOUT_SECONDS" -eq 0 ]]; then
    echo "--timeout must be a positive integer" >&2
    exit 1
fi

if [[ -n "$ENV_TARGETS" ]]; then
    while IFS= read -r target_spec; do
        [[ -n "$target_spec" ]] && TARGETS+=("$target_spec")
    done <<<"$ENV_TARGETS"
fi

timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

run_local_target() {
    local cmd_string="$1"
    if command -v timeout >/dev/null 2>&1; then
        timeout "$TIMEOUT_SECONDS" bash -lc "$cmd_string"
    else
        bash -lc "$cmd_string"
    fi
}

run_ssh_target() {
    local host="$1"
    local cmd_string="$2"
    if command -v timeout >/dev/null 2>&1; then
        timeout "$TIMEOUT_SECONDS" ssh -o BatchMode=yes -o ConnectTimeout=10 "$host" "$cmd_string"
    else
        ssh -o BatchMode=yes -o ConnectTimeout=10 "$host" "$cmd_string"
    fi
}

append_target_result() {
    local current_json="$1"
    local name="$2"
    local kind="$3"
    local host="$4"
    local status="$5"
    local exit_code="$6"
    local duration_seconds="$7"
    local output_preview="$8"

    jq -c \
        --arg name "$name" \
        --arg kind "$kind" \
        --arg host "$host" \
        --arg status "$status" \
        --arg output_preview "$output_preview" \
        --argjson exit_code "$exit_code" \
        --argjson duration_seconds "$duration_seconds" \
        '. + [{
          name: $name,
          kind: $kind,
          host: (if $host == "" then null else $host end),
          status: $status,
          exit_code: $exit_code,
          duration_seconds: $duration_seconds,
          output_preview: $output_preview
        }]' <<<"$current_json"
}

TARGET_RESULTS='[]'
FAILURES=0

if [[ "${#TARGETS[@]}" -eq 0 ]]; then
    if [[ "$REQUIRED" == "true" && -z "$WAIVER" ]]; then
        STATUS="fail"
        EXIT_CODE=1
    elif [[ "$REQUIRED" == "true" ]]; then
        STATUS="waived"
        EXIT_CODE=0
    else
        STATUS="skipped"
        EXIT_CODE=0
    fi
else
    STATUS="pass"
    EXIT_CODE=0
    for target_spec in "${TARGETS[@]}"; do
        started_epoch="$(date +%s)"
        target_name=""
        target_kind=""
        target_host=""
        target_command=""
        tmp_output="$(mktemp)"
        target_rc=0

        if [[ "$target_spec" == local:*:* ]]; then
            target_kind="local"
            rest="${target_spec#local:}"
            target_name="${rest%%:*}"
            target_command="${rest#*:}"
            set +e
            run_local_target "$target_command" >"$tmp_output" 2>&1
            target_rc=$?
            set -e
        elif [[ "$target_spec" == ssh:*:*:* ]]; then
            target_kind="ssh"
            rest="${target_spec#ssh:}"
            target_name="${rest%%:*}"
            rest="${rest#*:}"
            target_host="${rest%%:*}"
            target_command="${rest#*:}"
            set +e
            run_ssh_target "$target_host" "$target_command" >"$tmp_output" 2>&1
            target_rc=$?
            set -e
        else
            target_kind="invalid"
            target_name="${target_spec%%:*}"
            printf 'invalid target spec: %s\n' "$target_spec" >"$tmp_output"
            target_rc=2
        fi

        duration_seconds=$(( $(date +%s) - started_epoch ))
        output_preview="$(tail -20 "$tmp_output")"
        rm -f "$tmp_output"

        target_status="pass"
        if [[ "$target_rc" -ne 0 ]]; then
            target_status="fail"
            FAILURES=$((FAILURES + 1))
            STATUS="fail"
            EXIT_CODE=1
        fi

        TARGET_RESULTS="$(append_target_result "$TARGET_RESULTS" "$target_name" "$target_kind" "$target_host" "$target_status" "$target_rc" "$duration_seconds" "$output_preview")"
    done
fi

REQUIRED_JSON=false
[[ "$REQUIRED" == "true" ]] && REQUIRED_JSON=true

DOCUMENT="$(jq -n \
    --arg generated_at "$(timestamp)" \
    --arg status "$STATUS" \
    --arg waiver "$WAIVER" \
    --argjson required "$REQUIRED_JSON" \
    --argjson timeout_seconds "$TIMEOUT_SECONDS" \
    --argjson targets "$TARGET_RESULTS" \
    '{
      schema_version: 1,
      generated_at: $generated_at,
      status: $status,
      required: $required,
      waiver: (if $waiver == "" then null else $waiver end),
      timeout_seconds: $timeout_seconds,
      targets: $targets
    }')"

if [[ -n "$OUT" ]]; then
    mkdir -p "$(dirname "$OUT")"
    printf '%s\n' "$DOCUMENT" >"$OUT"
fi

printf '%s\n' "$DOCUMENT"

exit "$EXIT_CODE"
