#!/usr/bin/env bash
set -euo pipefail

OUT=""
MODE="${AGENTOPS_RELEASE_DIGITAL_TWIN_MODE:-official}"
AO_BIN="${AGENTOPS_RELEASE_AO_BIN:-}"
EXPECTED_VERSION="${AGENTOPS_RELEASE_VERSION:-}"
WAIVER="${AGENTOPS_RELEASE_DIGITAL_TWIN_WAIVER:-}"
TIMEOUT_SECONDS="${AGENTOPS_RELEASE_DIGITAL_TWIN_TIMEOUT:-30}"

usage() {
    cat <<'USAGE'
Usage: scripts/check-release-digital-twin.sh [options]

Capture release digital-twin/VIL evidence in a disposable local environment.

Options:
  --out PATH              Write JSON evidence to PATH
  --mode MODE             official|advisory|fast (default: official)
  --ao-bin PATH           ao binary to exercise (default: cli/bin/ao, then PATH)
  --expected-version V    Require ao version output to mention this version
  --waiver TEXT           Record an explicit digital-twin waiver
  --timeout SECONDS       Per-check timeout when timeout(1) is available (default: 30)
  -h, --help              Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --out)
            OUT="${2:-}"
            shift 2
            ;;
        --mode)
            MODE="${2:-}"
            shift 2
            ;;
        --ao-bin)
            AO_BIN="${2:-}"
            shift 2
            ;;
        --expected-version)
            EXPECTED_VERSION="${2:-}"
            shift 2
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
    echo "jq is required for digital-twin evidence" >&2
    exit 1
fi

if [[ "$MODE" != "official" && "$MODE" != "advisory" && "$MODE" != "fast" ]]; then
    echo "--mode must be official, advisory, or fast" >&2
    exit 1
fi

if [[ ! "$TIMEOUT_SECONDS" =~ ^[0-9]+$ || "$TIMEOUT_SECONDS" -eq 0 ]]; then
    echo "--timeout must be a positive integer" >&2
    exit 1
fi

timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

normalize_version() {
    local version="$1"
    printf '%s\n' "${version#v}"
}

json_array_add() {
    local current_json="$1"
    local value="$2"

    jq -c --arg value "$value" '. + [$value]' <<<"$current_json"
}

runtime_identity() {
    jq -n \
        --arg os "$(uname -s 2>/dev/null || true)" \
        --arg arch "$(uname -m 2>/dev/null || true)" \
        --arg kernel "$(uname -r 2>/dev/null || true)" \
        --arg hostname "$(hostname 2>/dev/null || true)" \
        '{os: $os, arch: $arch, kernel: $kernel, hostname: $hostname}'
}

if [[ -z "$AO_BIN" ]]; then
    if [[ -x "cli/bin/ao" ]]; then
        AO_BIN="cli/bin/ao"
    elif command -v ao >/dev/null 2>&1; then
        AO_BIN="$(command -v ao)"
    fi
fi

CHECKS='[]'
FAILURE_REASONS='[]'
DIM_VIL="pass"
DIM_RELEASE_SMOKE="pass"
DIM_INSTALL_UPGRADE="pass"
DIM_HOOK_INSTALL="pass"
DIM_RPI="pass"
DIM_PLUGIN_RUNTIME="pass"
DIM_VERSION_PARITY="pass"
STATUS="pass"
EXIT_CODE=0
WORKFLOW_STRENGTH="full"
TMP_ROOT=""

set_dimension_fail() {
    local dimension="$1"

    case "$dimension" in
        vil) DIM_VIL="fail" ;;
        release_smoke) DIM_RELEASE_SMOKE="fail" ;;
        install_upgrade) DIM_INSTALL_UPGRADE="fail" ;;
        hook_install_smoke) DIM_HOOK_INSTALL="fail" ;;
        rpi_smoke) DIM_RPI="fail" ;;
        plugin_runtime) DIM_PLUGIN_RUNTIME="fail" ;;
        version_parity) DIM_VERSION_PARITY="fail"; DIM_VIL="fail" ;;
    esac
}

append_check() {
    local name="$1"
    local dimension="$2"
    local status="$3"
    local exit_code="$4"
    local duration_seconds="$5"
    local output_preview="$6"
    local command_label="$7"

    CHECKS="$(jq -c \
        --arg name "$name" \
        --arg dimension "$dimension" \
        --arg status "$status" \
        --arg output_preview "$output_preview" \
        --arg command "$command_label" \
        --argjson exit_code "$exit_code" \
        --argjson duration_seconds "$duration_seconds" \
        '. + [{
          name: $name,
          dimension: $dimension,
          status: $status,
          exit_code: $exit_code,
          duration_seconds: $duration_seconds,
          command: $command,
          output_preview: $output_preview
        }]' <<<"$CHECKS")"
}

write_document() {
    local generated_at
    local artifact_dir

    generated_at="$(timestamp)"
    artifact_dir=""
    if [[ -n "$OUT" ]]; then
        artifact_dir="$(dirname "$OUT")"
    fi

    DOCUMENT="$(jq -n \
        --arg generated_at "$generated_at" \
        --arg artifact_dir "$artifact_dir" \
        --arg mode "$MODE" \
        --arg status "$STATUS" \
        --arg workflow_strength "$WORKFLOW_STRENGTH" \
        --arg release_version "$(normalize_version "$EXPECTED_VERSION")" \
        --arg ao_bin "$AO_BIN" \
        --arg waiver "$WAIVER" \
        --arg vil_status "$DIM_VIL" \
        --arg release_smoke_status "$DIM_RELEASE_SMOKE" \
        --arg install_upgrade_status "$DIM_INSTALL_UPGRADE" \
        --arg hook_install_status "$DIM_HOOK_INSTALL" \
        --arg rpi_status "$DIM_RPI" \
        --arg plugin_runtime_status "$DIM_PLUGIN_RUNTIME" \
        --arg version_parity_status "$DIM_VERSION_PARITY" \
        --argjson timeout_seconds "$TIMEOUT_SECONDS" \
        --argjson checks "$CHECKS" \
        --argjson runtime "$(runtime_identity)" \
        --argjson failure_reasons "$FAILURE_REASONS" \
        '{
          schema_version: 1,
          evidence_kind: "digital_twin",
          generated_at: $generated_at,
          artifact_dir: (if $artifact_dir == "" then null else $artifact_dir end),
          mode: $mode,
          status: $status,
          workflow_strength: $workflow_strength,
          release_version: (if $release_version == "" then null else $release_version end),
          ao_bin: (if $ao_bin == "" then null else $ao_bin end),
          waiver: (if $waiver == "" then null else $waiver end),
          timeout_seconds: $timeout_seconds,
          runtime: $runtime,
          dimensions: {
            vil: {status: $vil_status},
            release_smoke: {status: $release_smoke_status},
            install_upgrade: {status: $install_upgrade_status},
            hook_install_smoke: {status: $hook_install_status},
            rpi_smoke: {status: $rpi_status},
            plugin_runtime: {status: $plugin_runtime_status},
            version_parity: {status: $version_parity_status}
          },
          checks: $checks,
          failure_reasons: $failure_reasons
        }')"

    if [[ -n "$OUT" ]]; then
        mkdir -p "$(dirname "$OUT")"
        printf '%s\n' "$DOCUMENT" >"$OUT"
    fi
    printf '%s\n' "$DOCUMENT"
}

finish_unavailable() {
    local reason="$1"

    FAILURE_REASONS="$(json_array_add "$FAILURE_REASONS" "$reason")"
    DIM_VIL="skipped"
    DIM_RELEASE_SMOKE="skipped"
    DIM_INSTALL_UPGRADE="skipped"
    DIM_HOOK_INSTALL="skipped"
    DIM_RPI="skipped"
    DIM_PLUGIN_RUNTIME="skipped"
    DIM_VERSION_PARITY="skipped"

    if [[ -n "$WAIVER" ]]; then
        STATUS="waived"
        EXIT_CODE=0
    elif [[ "$MODE" == "official" ]]; then
        STATUS="fail"
        DIM_VIL="fail"
        EXIT_CODE=1
    else
        STATUS="skipped"
        EXIT_CODE=0
    fi

    write_document
    exit "$EXIT_CODE"
}

if [[ -n "$WAIVER" ]]; then
    finish_unavailable "waived"
fi

if [[ -z "$AO_BIN" || ! -x "$AO_BIN" ]]; then
    finish_unavailable "ao_binary_unavailable"
fi

TMP_ROOT="$(mktemp -d)"
cleanup() {
    [[ -n "$TMP_ROOT" ]] && rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

TMP_HOME="$TMP_ROOT/home"
TMP_REPO="$TMP_ROOT/repo"
INSTALL_DIR="$TMP_ROOT/bin"
mkdir -p "$TMP_HOME" "$TMP_REPO" "$INSTALL_DIR"
if command -v git >/dev/null 2>&1; then
    git -C "$TMP_REPO" init -q
fi

INSTALLED_AO="$INSTALL_DIR/ao"
cp "$AO_BIN" "$INSTALLED_AO"
chmod +x "$INSTALLED_AO"

run_command() {
    local command_string="$1"

    if command -v timeout >/dev/null 2>&1; then
        HOME="$TMP_HOME" timeout "$TIMEOUT_SECONDS" bash -lc "$command_string"
    else
        HOME="$TMP_HOME" bash -lc "$command_string"
    fi
}

run_check() {
    local name="$1"
    local dimension="$2"
    local command_string="$3"
    local started_epoch
    local duration_seconds
    local tmp_output
    local rc=0
    local output_preview

    started_epoch="$(date +%s)"
    tmp_output="$(mktemp)"
    set +e
    (
        cd "$TMP_REPO"
        run_command "$command_string"
    ) >"$tmp_output" 2>&1
    rc=$?
    set -e
    duration_seconds=$(( $(date +%s) - started_epoch ))
    output_preview="$(tail -20 "$tmp_output")"
    rm -f "$tmp_output"

    if [[ "$rc" -eq 0 ]]; then
        append_check "$name" "$dimension" "pass" "$rc" "$duration_seconds" "$output_preview" "$name"
    else
        append_check "$name" "$dimension" "fail" "$rc" "$duration_seconds" "$output_preview" "$name"
        FAILURE_REASONS="$(json_array_add "$FAILURE_REASONS" "workflow_failed")"
        set_dimension_fail "$dimension"
        STATUS="fail"
        EXIT_CODE=1
    fi
}

if [[ "$MODE" == "fast" ]]; then
    WORKFLOW_STRENGTH="lightweight"
fi

run_check "install" "install_upgrade" "test -x '$INSTALLED_AO' && '$INSTALLED_AO' version"
run_check "version" "version_parity" "'$INSTALLED_AO' version"
run_check "core-help" "release_smoke" "'$INSTALLED_AO' --help"
run_check "rpi-help" "rpi_smoke" "'$INSTALLED_AO' rpi --help"

if [[ "$MODE" != "fast" ]]; then
    run_check "upgrade" "install_upgrade" "cp '$AO_BIN' '$INSTALL_DIR/ao.next' && chmod +x '$INSTALL_DIR/ao.next' && mv '$INSTALL_DIR/ao.next' '$INSTALLED_AO' && '$INSTALLED_AO' version"
    run_check "init-hooks" "hook_install_smoke" "'$INSTALLED_AO' init --hooks && '$INSTALLED_AO' hooks show"
    run_check "rpi-status" "rpi_smoke" "'$INSTALLED_AO' rpi status"
    run_check "plugin-runtime" "plugin_runtime" "'$INSTALLED_AO' hooks --help && '$INSTALLED_AO' doctor --help"
fi

if [[ -n "$EXPECTED_VERSION" ]]; then
    version_output="$(HOME="$TMP_HOME" "$INSTALLED_AO" version 2>/dev/null || true)"
    if ! grep -Fq "$(normalize_version "$EXPECTED_VERSION")" <<<"$version_output"; then
        FAILURE_REASONS="$(json_array_add "$FAILURE_REASONS" "version_mismatch")"
        set_dimension_fail "version_parity"
        STATUS="fail"
        EXIT_CODE=1
    fi
fi

if [[ "$STATUS" == "fail" && "$MODE" == "advisory" ]]; then
    EXIT_CODE=0
fi

write_document
exit "$EXIT_CODE"
