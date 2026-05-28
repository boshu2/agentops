#!/usr/bin/env bash
set -euo pipefail

OUT=""
REQUIRED=false
WAIVER="${AGENTOPS_RELEASE_HIL_WAIVER:-}"
TIMEOUT_SECONDS="${AGENTOPS_RELEASE_HIL_TIMEOUT:-30}"
ENV_TARGETS="${AGENTOPS_RELEASE_HIL_TARGETS:-}"
EXPECTED_VERSION="${AGENTOPS_RELEASE_VERSION:-}"
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
  --expected-version V
                      Require target output to mention this release version
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
        --expected-version)
            EXPECTED_VERSION="${2:-}"
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

json_array_add() {
    local current_json="$1"
    local value="$2"

    jq -c --arg value "$value" '. + [$value]' <<<"$current_json"
}

command_sha256() {
    local command_string="$1"

    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s' "$command_string" | sha256sum | awk '{print $1}'
    else
        printf '%s' "$command_string" | shasum -a 256 | awk '{print $1}'
    fi
}

detect_workflow_checks() {
    local command_string="$1"
    local checks='[]'

    [[ "$command_string" == *"ao version"* ]] && checks="$(json_array_add "$checks" "ao-version")"
    [[ "$command_string" == *"ao init"* ]] && checks="$(json_array_add "$checks" "ao-init")"
    [[ "$command_string" == *"ao hooks"* ]] && checks="$(json_array_add "$checks" "ao-hooks")"
    [[ "$command_string" == *"ao rpi"* ]] && checks="$(json_array_add "$checks" "ao-rpi")"
    [[ "$command_string" == *" install"* || "$command_string" == install* ]] && checks="$(json_array_add "$checks" "install")"
    [[ "$command_string" == *" upgrade"* || "$command_string" == upgrade* ]] && checks="$(json_array_add "$checks" "upgrade")"

    printf '%s\n' "$checks"
}

local_runtime_identity() {
    jq -n \
        --arg os "$(uname -s 2>/dev/null || true)" \
        --arg arch "$(uname -m 2>/dev/null || true)" \
        --arg kernel "$(uname -r 2>/dev/null || true)" \
        --arg hostname "$(hostname 2>/dev/null || true)" \
        '{os: $os, arch: $arch, kernel: $kernel, hostname: $hostname}'
}

remote_runtime_identity() {
    local host="$1"
    jq -n --arg host "$host" '{host: $host, runtime_identity: "remote-command"}'
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
    local command_hash="$9"
    local workflow_checks="${10}"
    local workflow_strength="${11}"
    local expected_version="${12}"
    local version_verified="${13}"
    local runtime_identity="${14}"
    local failure_reasons="${15}"

    jq -c \
        --arg name "$name" \
        --arg kind "$kind" \
        --arg host "$host" \
        --arg status "$status" \
        --arg output_preview "$output_preview" \
        --arg command_sha256 "$command_hash" \
        --arg workflow_strength "$workflow_strength" \
        --arg expected_version "$expected_version" \
        --arg version_verified "$version_verified" \
        --argjson exit_code "$exit_code" \
        --argjson duration_seconds "$duration_seconds" \
        --argjson workflow_checks "$workflow_checks" \
        --argjson runtime_identity "$runtime_identity" \
        --argjson failure_reasons "$failure_reasons" \
        '. + [{
          name: $name,
          kind: $kind,
          host: (if $host == "" then null else $host end),
          status: $status,
          exit_code: $exit_code,
          duration_seconds: $duration_seconds,
          command_sha256: $command_sha256,
          workflow_strength: $workflow_strength,
          workflow_checks: $workflow_checks,
          expected_version: (if $expected_version == "" then null else $expected_version end),
          version_verified: (if $expected_version == "" then null else ($version_verified == "true") end),
          runtime: $runtime_identity,
          failure_reasons: $failure_reasons,
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
        failure_reasons='[]'
        runtime_identity='{}'

        if [[ "$target_spec" == local:*:* ]]; then
            target_kind="local"
            rest="${target_spec#local:}"
            target_name="${rest%%:*}"
            target_command="${rest#*:}"
            runtime_identity="$(local_runtime_identity)"
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
            runtime_identity="$(remote_runtime_identity "$target_host")"
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

        workflow_checks="$(detect_workflow_checks "$target_command")"
        workflow_count="$(jq -r 'length' <<<"$workflow_checks")"
        workflow_strength="strong"
        if [[ "$workflow_count" -lt 2 ]]; then
            workflow_strength="weak"
        fi

        version_verified=false
        if [[ -n "$EXPECTED_VERSION" ]]; then
            if grep -Fq "$EXPECTED_VERSION" <<<"$output_preview"; then
                version_verified=true
            else
                failure_reasons="$(json_array_add "$failure_reasons" "version_mismatch")"
            fi
        fi

        target_status="pass"
        if [[ "$target_rc" -ne 0 ]]; then
            failure_reasons="$(json_array_add "$failure_reasons" "command_failed")"
        fi
        if [[ "$REQUIRED" == "true" && "$workflow_strength" == "weak" ]]; then
            failure_reasons="$(json_array_add "$failure_reasons" "weak_workflow")"
        fi
        if [[ "$(jq -r 'length' <<<"$failure_reasons")" -gt 0 ]]; then
            target_status="fail"
            FAILURES=$((FAILURES + 1))
            STATUS="fail"
            EXIT_CODE=1
        fi

        TARGET_RESULTS="$(append_target_result \
            "$TARGET_RESULTS" "$target_name" "$target_kind" "$target_host" \
            "$target_status" "$target_rc" "$duration_seconds" "$output_preview" \
            "$(command_sha256 "$target_command")" "$workflow_checks" "$workflow_strength" \
            "$EXPECTED_VERSION" "$version_verified" "$runtime_identity" "$failure_reasons")"
    done
fi

REQUIRED_JSON=false
[[ "$REQUIRED" == "true" ]] && REQUIRED_JSON=true

DOCUMENT="$(jq -n \
    --arg generated_at "$(timestamp)" \
    --arg status "$STATUS" \
    --arg waiver "$WAIVER" \
    --arg expected_version "$EXPECTED_VERSION" \
    --argjson required "$REQUIRED_JSON" \
    --argjson timeout_seconds "$TIMEOUT_SECONDS" \
    --argjson targets "$TARGET_RESULTS" \
    '{
      schema_version: 1,
      generated_at: $generated_at,
      status: $status,
      required: $required,
      waiver: (if $waiver == "" then null else $waiver end),
      expected_version: (if $expected_version == "" then null else $expected_version end),
      timeout_seconds: $timeout_seconds,
      targets: $targets
    }')"

if [[ -n "$OUT" ]]; then
    mkdir -p "$(dirname "$OUT")"
    printf '%s\n' "$DOCUMENT" >"$OUT"
fi

printf '%s\n' "$DOCUMENT"

exit "$EXIT_CODE"
