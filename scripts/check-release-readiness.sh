#!/usr/bin/env bash
set -euo pipefail

MODE="${AGENTOPS_RELEASE_READINESS_MODE:-official}"
THRESHOLD="${AGENTOPS_RELEASE_READINESS_THRESHOLD:-8}"
OUT=""
ARTIFACT_DIR=""
EVIDENCE_DIR="${AGENTOPS_RELEASE_EVIDENCE_DIR:-}"
RELEASE_VERSION="${AGENTOPS_RELEASE_VERSION:-}"
MAX_EVIDENCE_AGE_SECONDS="${AGENTOPS_RELEASE_MAX_EVIDENCE_AGE_SECONDS:-86400}"
SIL_EVIDENCE_FILE="${AGENTOPS_RELEASE_SIL_EVIDENCE_FILE:-}"
VIL_EVIDENCE_FILE="${AGENTOPS_RELEASE_VIL_EVIDENCE_FILE:-}"
DIGITAL_TWIN_EVIDENCE_FILE="${AGENTOPS_RELEASE_DIGITAL_TWIN_EVIDENCE_FILE:-}"
SECURITY_EVIDENCE_FILE="${AGENTOPS_RELEASE_SECURITY_EVIDENCE_FILE:-}"
EVAL_EVIDENCE_FILE="${AGENTOPS_RELEASE_EVAL_EVIDENCE_FILE:-}"
EVAL_BASELINE_FILE="${AGENTOPS_RELEASE_EVAL_BASELINE_FILE:-}"
SIL_STATUS="${AGENTOPS_RELEASE_SIL_STATUS:-pass}"
VIL_STATUS="${AGENTOPS_RELEASE_VIL_STATUS:-pass}"
HIL_STATUS="${AGENTOPS_RELEASE_HIL_STATUS:-}"
HIL_FILE="${AGENTOPS_RELEASE_HIL_FILE:-}"
HIL_WAIVER="${AGENTOPS_RELEASE_HIL_WAIVER:-}"
ARTIFACT_STATUS="${AGENTOPS_RELEASE_ARTIFACT_STATUS:-pass}"
SECURITY_STATUS="${AGENTOPS_RELEASE_SECURITY_STATUS:-pass}"
EVAL_STATUS="${AGENTOPS_RELEASE_EVAL_STATUS:-pass}"

usage() {
    cat <<'USAGE'
Usage: scripts/check-release-readiness.sh [options]

Write a scored release-readiness artifact.

Options:
  --out PATH              Write JSON readiness artifact to PATH
  --artifact-dir PATH     Artifact directory recorded in the JSON
  --evidence-dir PATH     Directory containing official evidence JSON files
  --release-version V     Expected release version for official evidence
  --max-evidence-age-seconds N
                          Maximum official evidence age (default: 86400)
  --sil-evidence-file PATH
                          SIL evidence JSON (default: evidence-dir/sil-evidence.json)
  --vil-evidence-file PATH
                          VIL evidence JSON (default: evidence-dir/digital-twin-evidence.json)
  --digital-twin-file PATH
                          Digital-twin evidence JSON (alias for --vil-evidence-file)
  --security-file PATH    Security gate JSON (default: evidence-dir/security-gate-full.json)
  --eval-file PATH        Eval fast JSON (default: evidence-dir/eval-agentops-fast.json)
  --eval-baseline-file PATH
                          Eval baseline audit JSON (default: evidence-dir/eval-baseline-audit.json)
  --mode MODE             official|advisory|fast (default: official)
  --threshold NUMBER      Minimum score for pass (default: 8)
  --sil STATUS            pass|fail|skipped (default: pass)
  --vil STATUS            pass|fail|skipped (default: pass)
  --hil-status STATUS     pass|fail|skipped|waived
  --hil-file PATH         Read HIL status from check-release-hil.sh JSON
  --hil-waiver TEXT       Record an explicit HIL waiver
  --artifacts STATUS      pass|fail|skipped (default: pass)
  --security STATUS       pass|fail|skipped (default: pass)
  --eval STATUS           pass|fail|skipped (default: pass)
  -h, --help              Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --out)
            OUT="${2:-}"
            shift 2
            ;;
        --artifact-dir)
            ARTIFACT_DIR="${2:-}"
            shift 2
            ;;
        --evidence-dir)
            EVIDENCE_DIR="${2:-}"
            shift 2
            ;;
        --release-version)
            RELEASE_VERSION="${2:-}"
            shift 2
            ;;
        --max-evidence-age-seconds)
            MAX_EVIDENCE_AGE_SECONDS="${2:-}"
            shift 2
            ;;
        --sil-evidence-file)
            SIL_EVIDENCE_FILE="${2:-}"
            shift 2
            ;;
        --vil-evidence-file)
            VIL_EVIDENCE_FILE="${2:-}"
            shift 2
            ;;
        --digital-twin-file)
            DIGITAL_TWIN_EVIDENCE_FILE="${2:-}"
            shift 2
            ;;
        --security-file)
            SECURITY_EVIDENCE_FILE="${2:-}"
            shift 2
            ;;
        --eval-file)
            EVAL_EVIDENCE_FILE="${2:-}"
            shift 2
            ;;
        --eval-baseline-file)
            EVAL_BASELINE_FILE="${2:-}"
            shift 2
            ;;
        --mode)
            MODE="${2:-}"
            shift 2
            ;;
        --threshold)
            THRESHOLD="${2:-}"
            shift 2
            ;;
        --sil)
            SIL_STATUS="${2:-}"
            shift 2
            ;;
        --vil)
            VIL_STATUS="${2:-}"
            shift 2
            ;;
        --hil-status)
            HIL_STATUS="${2:-}"
            shift 2
            ;;
        --hil-file)
            HIL_FILE="${2:-}"
            shift 2
            ;;
        --hil-waiver)
            HIL_WAIVER="${2:-}"
            shift 2
            ;;
        --artifacts)
            ARTIFACT_STATUS="${2:-}"
            shift 2
            ;;
        --security)
            SECURITY_STATUS="${2:-}"
            shift 2
            ;;
        --eval)
            EVAL_STATUS="${2:-}"
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
    echo "jq is required for release readiness scoring" >&2
    exit 1
fi

if [[ "$MODE" != "official" && "$MODE" != "advisory" && "$MODE" != "fast" ]]; then
    echo "--mode must be official, advisory, or fast" >&2
    exit 1
fi

if ! awk -v threshold="$THRESHOLD" 'BEGIN { exit !(threshold + 0 == threshold && threshold > 0) }'; then
    echo "--threshold must be a positive number" >&2
    exit 1
fi

if [[ ! "$MAX_EVIDENCE_AGE_SECONDS" =~ ^[0-9]+$ || "$MAX_EVIDENCE_AGE_SECONDS" -eq 0 ]]; then
    echo "--max-evidence-age-seconds must be a positive integer" >&2
    exit 1
fi

validate_status() {
    local label="$1"
    local status="$2"
    local allow_waived="${3:-false}"

    case "$status" in
        pass|fail|skipped)
            return 0
            ;;
        waived)
            [[ "$allow_waived" == "true" ]] && return 0
            ;;
    esac

    if [[ "$allow_waived" == "true" ]]; then
        echo "$label must be pass, fail, skipped, or waived" >&2
    else
        echo "$label must be pass, fail, or skipped" >&2
    fi
    exit 1
}

timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

EVIDENCE_ERRORS='[]'
OFFICIAL_ARTIFACTS_OK=true
SIL_ARTIFACT=""
VIL_ARTIFACT=""
HIL_ARTIFACT=""
ARTIFACTS_ARTIFACT=""
SECURITY_ARTIFACT=""
EVAL_ARTIFACT=""
DIGITAL_TWIN_WORKFLOW_STRENGTH=""
DIGITAL_TWIN_BINARY_DIGEST=""
DIGITAL_TWIN_TARGET_IDENTITY="null"
DIGITAL_TWIN_LOGS="null"
HIL_TARGET_IDENTITY="[]"
HIL_LOGS="null"

add_evidence_error() {
    local item="$1"
    EVIDENCE_ERRORS="$(jq -c --arg item "$item" '. + [$item]' <<<"$EVIDENCE_ERRORS")"
}

artifact_name() {
    local path="$1"

    if [[ -z "$path" ]]; then
        printf ''
    else
        basename "$path"
    fi
}

resolve_evidence_path() {
    local candidate="$1"
    local default_name="$2"
    local base_dir="$EVIDENCE_DIR"

    [[ -z "$base_dir" ]] && base_dir="$ARTIFACT_DIR"

    if [[ -n "$candidate" ]]; then
        if [[ "$candidate" == /* || -z "$base_dir" ]]; then
            printf '%s\n' "$candidate"
        else
            printf '%s/%s\n' "${base_dir%/}" "$candidate"
        fi
    elif [[ -n "$base_dir" ]]; then
        printf '%s/%s\n' "${base_dir%/}" "$default_name"
    else
        printf '%s\n' "$default_name"
    fi
}

first_existing_evidence_path() {
    local candidate

    for candidate in "$@"; do
        [[ -n "$candidate" && -f "$candidate" ]] && {
            printf '%s\n' "$candidate"
            return 0
        }
    done

    printf '%s\n' "${1:-}"
}

normalize_version() {
    local version="$1"
    printf '%s\n' "${version#v}"
}

json_timestamp_epoch() {
    local value="$1"

    date -u -d "$value" +%s 2>/dev/null || \
        date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$value" +%s 2>/dev/null || \
        return 1
}

validate_evidence_common() {
    local label="$1"
    local file="$2"
    local require_version="${3:-false}"
    local generated_at
    local generated_epoch
    local now_epoch
    local age_seconds
    local expected_version
    local evidence_version

    if [[ -z "$file" || ! -f "$file" ]]; then
        add_evidence_error "missing ${label} evidence: $(artifact_name "$file")"
        OFFICIAL_ARTIFACTS_OK=false
        return 1
    fi

    if ! jq empty "$file" >/dev/null 2>&1; then
        add_evidence_error "malformed ${label} evidence: $(artifact_name "$file")"
        OFFICIAL_ARTIFACTS_OK=false
        return 1
    fi

    generated_at="$(jq -r '.generated_at // empty' "$file")"
    if [[ -z "$generated_at" ]]; then
        add_evidence_error "missing ${label} generated_at: $(artifact_name "$file")"
        OFFICIAL_ARTIFACTS_OK=false
        return 1
    fi
    if ! generated_epoch="$(json_timestamp_epoch "$generated_at")"; then
        add_evidence_error "invalid ${label} generated_at: $(artifact_name "$file")"
        OFFICIAL_ARTIFACTS_OK=false
        return 1
    fi
    now_epoch="$(date -u +%s)"
    age_seconds=$((now_epoch - generated_epoch))
    if (( age_seconds < -300 || age_seconds > MAX_EVIDENCE_AGE_SECONDS )); then
        add_evidence_error "stale ${label} evidence: $(artifact_name "$file")"
        OFFICIAL_ARTIFACTS_OK=false
        return 1
    fi

    expected_version="$(normalize_version "$RELEASE_VERSION")"
    if [[ "$require_version" == "true" && -n "$expected_version" ]]; then
        evidence_version="$(jq -r '.release_version // .expected_version // empty' "$file")"
        evidence_version="$(normalize_version "$evidence_version")"
        if [[ -z "$evidence_version" ]]; then
            add_evidence_error "missing ${label} release version: $(artifact_name "$file")"
            OFFICIAL_ARTIFACTS_OK=false
            return 1
        fi
        if [[ "$evidence_version" != "$expected_version" ]]; then
            add_evidence_error "version mismatch in ${label} evidence: expected ${expected_version}, got ${evidence_version}"
            OFFICIAL_ARTIFACTS_OK=false
            return 1
        fi
    fi

    return 0
}

derive_official_evidence() {
    local sil_file
    local vil_file
    local security_file
    local eval_file
    local baseline_file
    local security_full
    local security_summary
    local security_quick

    [[ -z "$EVIDENCE_DIR" ]] && EVIDENCE_DIR="$ARTIFACT_DIR"
    [[ -n "$DIGITAL_TWIN_EVIDENCE_FILE" && -z "$VIL_EVIDENCE_FILE" ]] && \
        VIL_EVIDENCE_FILE="$DIGITAL_TWIN_EVIDENCE_FILE"

    sil_file="$(resolve_evidence_path "$SIL_EVIDENCE_FILE" "sil-evidence.json")"
    vil_file="$(resolve_evidence_path "$VIL_EVIDENCE_FILE" "digital-twin-evidence.json")"
    HIL_FILE="$(resolve_evidence_path "$HIL_FILE" "hil-evidence.json")"
    eval_file="$(resolve_evidence_path "$EVAL_EVIDENCE_FILE" "eval-agentops-fast.json")"
    baseline_file="$(resolve_evidence_path "$EVAL_BASELINE_FILE" "eval-baseline-audit.json")"

    security_full="$(resolve_evidence_path "$SECURITY_EVIDENCE_FILE" "security-gate-full.json")"
    security_summary="$(resolve_evidence_path "$SECURITY_EVIDENCE_FILE" "security-gate-summary.json")"
    security_quick="$(resolve_evidence_path "$SECURITY_EVIDENCE_FILE" "security-gate-quick.json")"
    security_file="$(first_existing_evidence_path "$security_full" "$security_summary" "$security_quick")"

    SIL_ARTIFACT="$(artifact_name "$sil_file")"
    VIL_ARTIFACT="$(artifact_name "$vil_file")"
    HIL_ARTIFACT="$(artifact_name "$HIL_FILE")"
    SECURITY_ARTIFACT="$(artifact_name "$security_file")"
    EVAL_ARTIFACT="$(artifact_name "$eval_file")"
    ARTIFACTS_ARTIFACT="release-artifacts.json"

    if [[ -f "$vil_file" ]] && jq empty "$vil_file" >/dev/null 2>&1; then
        DIGITAL_TWIN_WORKFLOW_STRENGTH="$(jq -r '.workflow_strength // empty' "$vil_file")"
        DIGITAL_TWIN_BINARY_DIGEST="$(jq -r '.ao_bin_sha256 // empty' "$vil_file")"
        DIGITAL_TWIN_TARGET_IDENTITY="$(jq -c '.runtime // null' "$vil_file")"
        DIGITAL_TWIN_LOGS="$(jq -c '{checks: (.checks // [] | map({name, dimension, status, exit_code, duration_seconds, output_preview})), failure_reasons: (.failure_reasons // [])}' "$vil_file")"
    fi

    if [[ -f "$HIL_FILE" ]] && jq empty "$HIL_FILE" >/dev/null 2>&1; then
        HIL_TARGET_IDENTITY="$(jq -c '[.targets[]? | {name, kind, host, status, command_sha256, workflow_strength, workflow_checks, runtime, version_verified}]' "$HIL_FILE")"
        HIL_LOGS="$(jq -c '{targets: [.targets[]? | {name, status, exit_code, duration_seconds, output_preview, failure_reasons}]}' "$HIL_FILE")"
    fi

    SIL_STATUS="fail"
    if validate_evidence_common "SIL" "$sil_file" true && \
        jq -e '.schema_version == 1 and .evidence_kind == "software_in_loop" and .status == "pass"' "$sil_file" >/dev/null; then
        SIL_STATUS="pass"
    else
        add_evidence_error "SIL evidence did not pass: $(artifact_name "$sil_file")"
    fi

    VIL_STATUS="fail"
    if validate_evidence_common "digital twin" "$vil_file" true && \
        jq -e '
          .schema_version == 1 and
          .evidence_kind == "digital_twin" and
          .status == "pass" and
          .workflow_strength == "full" and
          ((.ao_bin_sha256 // "") | length > 0) and
          (.runtime | type == "object") and
          (((.checks // []) | length) >= 6) and
          .dimensions.vil.status == "pass"
        ' "$vil_file" >/dev/null; then
        VIL_STATUS="pass"
    else
        add_evidence_error "digital twin/VIL evidence did not pass: $(artifact_name "$vil_file")"
    fi

    HIL_STATUS="fail"
    HIL_WAIVER=""
    if validate_evidence_common "HIL" "$HIL_FILE" true; then
        HIL_STATUS="$(jq -r '.status // "fail"' "$HIL_FILE")"
        HIL_WAIVER="$(jq -r '.waiver // empty' "$HIL_FILE")"
        if [[ "$HIL_STATUS" == "pass" ]]; then
            if ! jq -e '(.targets | type == "array") and ((.targets | length) > 0) and all(.targets[]; .status == "pass" and .workflow_strength == "strong" and (.runtime | type == "object"))' "$HIL_FILE" >/dev/null; then
                add_evidence_error "HIL evidence lacks strong target identity: $(artifact_name "$HIL_FILE")"
                HIL_STATUS="fail"
            fi
        elif [[ "$HIL_STATUS" == "waived" && -n "$HIL_WAIVER" ]]; then
            :
        else
            add_evidence_error "HIL evidence did not pass or carry a waiver: $(artifact_name "$HIL_FILE")"
            HIL_STATUS="fail"
        fi
    else
        add_evidence_error "HIL evidence did not pass: $(artifact_name "$HIL_FILE")"
    fi

    SECURITY_STATUS="fail"
    if validate_evidence_common "security" "$security_file" true && \
        jq -e '((.gate_status // "") | ascii_downcase) == "pass"' "$security_file" >/dev/null; then
        SECURITY_STATUS="pass"
    else
        add_evidence_error "security evidence did not pass: $(artifact_name "$security_file")"
    fi

    EVAL_STATUS="fail"
    if validate_evidence_common "eval" "$eval_file" true && \
        validate_evidence_common "eval baseline" "$baseline_file" true && \
        jq -e --arg baseline "$(artifact_name "$baseline_file")" \
            '.schema_version == 1 and .evidence_kind == "agentops_eval_fast" and .status == "pass" and .baseline_audit == $baseline' \
            "$eval_file" >/dev/null && \
        jq -e '(.policy_mismatch_count | type == "number") and (((.stale_suite_hashes // []) | length) == 0)' "$baseline_file" >/dev/null; then
        EVAL_STATUS="pass"
    else
        add_evidence_error "eval evidence did not pass: $(artifact_name "$eval_file")"
    fi

    ARTIFACT_STATUS="pass"
    if [[ "$OFFICIAL_ARTIFACTS_OK" != "true" ]]; then
        ARTIFACT_STATUS="fail"
    fi
}

if [[ "$MODE" == "official" ]]; then
    derive_official_evidence
elif [[ -n "$HIL_FILE" ]]; then
    if [[ -f "$HIL_FILE" ]]; then
        HIL_STATUS="$(jq -r '.status // "fail"' "$HIL_FILE")"
        if [[ -z "$HIL_WAIVER" ]]; then
            HIL_WAIVER="$(jq -r '.waiver // empty' "$HIL_FILE")"
        fi
    else
        HIL_STATUS="fail"
    fi
fi

if [[ "$MODE" != "official" && -z "$HIL_STATUS" ]]; then
    if [[ -n "$HIL_WAIVER" ]]; then
        HIL_STATUS="waived"
    else
        HIL_STATUS="skipped"
    fi
fi

validate_status "--sil" "$SIL_STATUS"
validate_status "--vil" "$VIL_STATUS"
validate_status "--hil-status" "$HIL_STATUS" true
validate_status "--artifacts" "$ARTIFACT_STATUS"
validate_status "--security" "$SECURITY_STATUS"
validate_status "--eval" "$EVAL_STATUS"

score_for_status() {
    local status="$1"
    local weight="$2"

    case "$status" in
        pass)
            printf '%s\n' "$weight"
            ;;
        waived)
            awk -v weight="$weight" 'BEGIN { printf "%.1f\n", weight * 0.5 }'
            ;;
        *)
            printf '0\n'
            ;;
    esac
}

SIL_POINTS="$(score_for_status "$SIL_STATUS" 2)"
VIL_POINTS="$(score_for_status "$VIL_STATUS" 2)"
HIL_POINTS="$(score_for_status "$HIL_STATUS" 2)"
ARTIFACT_POINTS="$(score_for_status "$ARTIFACT_STATUS" 1.5)"
SECURITY_POINTS="$(score_for_status "$SECURITY_STATUS" 1.5)"
EVAL_POINTS="$(score_for_status "$EVAL_STATUS" 1)"

SCORE="$(awk \
    -v sil="$SIL_POINTS" \
    -v vil="$VIL_POINTS" \
    -v hil="$HIL_POINTS" \
    -v artifacts="$ARTIFACT_POINTS" \
    -v security="$SECURITY_POINTS" \
    -v evals="$EVAL_POINTS" \
    'BEGIN { printf "%.1f\n", sil + vil + hil + artifacts + security + evals }')"

MEETS_THRESHOLD=false
if awk -v score="$SCORE" -v threshold="$THRESHOLD" 'BEGIN { exit !(score >= threshold) }'; then
    MEETS_THRESHOLD=true
fi

OFFICIAL_MANDATORY_OK=true
if [[ "$MODE" == "official" ]]; then
    for status in "$SIL_STATUS" "$VIL_STATUS" "$ARTIFACT_STATUS" "$SECURITY_STATUS" "$EVAL_STATUS"; do
        if [[ "$status" != "pass" ]]; then
            OFFICIAL_MANDATORY_OK=false
        fi
    done
    if [[ "$HIL_STATUS" != "pass" && "$HIL_STATUS" != "waived" ]]; then
        OFFICIAL_MANDATORY_OK=false
    fi
fi

RELEASE_STATUS="warn"
if [[ "$MEETS_THRESHOLD" == "true" && "$OFFICIAL_MANDATORY_OK" == "true" ]]; then
    RELEASE_STATUS="pass"
elif [[ "$MODE" == "official" ]]; then
    RELEASE_STATUS="fail"
fi

RECOMMENDATIONS='[]'
add_recommendation() {
    local item="$1"
    RECOMMENDATIONS="$(jq -c --arg item "$item" '. + [$item]' <<<"$RECOMMENDATIONS")"
}

[[ "$SIL_STATUS" == "pass" ]] || add_recommendation "Run the deterministic local release gate until SIL passes."
[[ "$VIL_STATUS" == "pass" ]] || add_recommendation "Confirm the validate/release workflow lane or remote parity evidence before tagging."
if [[ "$HIL_STATUS" != "pass" && "$HIL_STATUS" != "waived" ]]; then
    add_recommendation "Run check-release-hil.sh against a real target or record an explicit waiver."
fi
[[ "$ARTIFACT_STATUS" == "pass" ]] || add_recommendation "Regenerate release artifacts before resolving a release audit."
[[ "$SECURITY_STATUS" == "pass" ]] || add_recommendation "Run the full security gate and include its JSON report."
[[ "$EVAL_STATUS" == "pass" ]] || add_recommendation "Run release smoke/eval checks and attach the result."

[[ -z "$HIL_ARTIFACT" && -n "$HIL_FILE" ]] && HIL_ARTIFACT="$(artifact_name "$HIL_FILE")"

DOCUMENT="$(jq -n \
    --arg generated_at "$(timestamp)" \
    --arg mode "$MODE" \
    --arg artifact_dir "$ARTIFACT_DIR" \
    --arg release_status "$RELEASE_STATUS" \
    --arg sil_status "$SIL_STATUS" \
    --arg vil_status "$VIL_STATUS" \
    --arg hil_status "$HIL_STATUS" \
    --arg artifact_status "$ARTIFACT_STATUS" \
    --arg security_status "$SECURITY_STATUS" \
    --arg eval_status "$EVAL_STATUS" \
    --arg sil_artifact "$SIL_ARTIFACT" \
    --arg vil_artifact "$VIL_ARTIFACT" \
    --arg hil_artifact "$HIL_ARTIFACT" \
    --arg artifacts_artifact "$ARTIFACTS_ARTIFACT" \
    --arg security_artifact "$SECURITY_ARTIFACT" \
    --arg eval_artifact "$EVAL_ARTIFACT" \
    --arg hil_waiver "$HIL_WAIVER" \
    --arg digital_twin_workflow_strength "$DIGITAL_TWIN_WORKFLOW_STRENGTH" \
    --arg digital_twin_binary_digest "$DIGITAL_TWIN_BINARY_DIGEST" \
    --argjson threshold "$THRESHOLD" \
    --argjson release_readiness_score "$SCORE" \
    --argjson sil_points "$SIL_POINTS" \
    --argjson vil_points "$VIL_POINTS" \
    --argjson hil_points "$HIL_POINTS" \
    --argjson artifact_points "$ARTIFACT_POINTS" \
    --argjson security_points "$SECURITY_POINTS" \
    --argjson eval_points "$EVAL_POINTS" \
    --argjson digital_twin_target_identity "$DIGITAL_TWIN_TARGET_IDENTITY" \
    --argjson digital_twin_logs "$DIGITAL_TWIN_LOGS" \
    --argjson hil_target_identity "$HIL_TARGET_IDENTITY" \
    --argjson hil_logs "$HIL_LOGS" \
    --argjson evidence_errors "$EVIDENCE_ERRORS" \
    --argjson recommendations "$RECOMMENDATIONS" \
    '{
      schema_version: 1,
      generated_at: $generated_at,
      mode: $mode,
      threshold: $threshold,
      release_readiness_score: $release_readiness_score,
      release_status: $release_status,
      artifact_dir: (if $artifact_dir == "" then null else $artifact_dir end),
      dimensions: {
        sil: {status: $sil_status, weight: 2, points: $sil_points, evidence_artifact: (if $sil_artifact == "" then null else $sil_artifact end)},
        vil: {status: $vil_status, weight: 2, points: $vil_points, evidence_artifact: (if $vil_artifact == "" then null else $vil_artifact end)},
        hil: {status: $hil_status, weight: 2, points: $hil_points, evidence_artifact: (if $hil_artifact == "" then null else $hil_artifact end)},
        artifacts: {status: $artifact_status, weight: 1.5, points: $artifact_points, evidence_artifact: (if $artifacts_artifact == "" then null else $artifacts_artifact end)},
        security: {status: $security_status, weight: 1.5, points: $security_points, evidence_artifact: (if $security_artifact == "" then null else $security_artifact end)},
        evals: {status: $eval_status, weight: 1, points: $eval_points, evidence_artifact: (if $eval_artifact == "" then null else $eval_artifact end)}
      },
      hil_evidence: {
        status: $hil_status,
        artifact: (if $hil_artifact == "" then null else $hil_artifact end),
        waiver: (if $hil_waiver == "" then null else $hil_waiver end)
      },
      evidence: {
        policy: {
          name: "zero-trust-release-evidence",
          official_mode: ($mode == "official"),
          status_flags_trusted: ($mode != "official"),
          pre_publish_blocking: ($mode == "official"),
          threshold: $threshold
        },
        lanes: {
          sil: {
            status: $sil_status,
            artifact: (if $sil_artifact == "" then null else $sil_artifact end),
            evidence_kind: "software_in_loop",
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          },
          vil: {
            status: $vil_status,
            artifact: (if $vil_artifact == "" then null else $vil_artifact end),
            evidence_kind: "digital_twin",
            source: "digital_twin",
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          },
          digital_twin: {
            status: $vil_status,
            artifact: (if $vil_artifact == "" then null else $vil_artifact end),
            evidence_kind: "digital_twin",
            workflow_strength: (if $digital_twin_workflow_strength == "" then null else $digital_twin_workflow_strength end),
            binary_digest: (if $digital_twin_binary_digest == "" then null else $digital_twin_binary_digest end),
            target_identity: $digital_twin_target_identity,
            logs: $digital_twin_logs,
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          },
          hil: {
            status: $hil_status,
            artifact: (if $hil_artifact == "" then null else $hil_artifact end),
            evidence_kind: "hardware_in_loop",
            waiver: (if $hil_waiver == "" then null else $hil_waiver end),
            target_identity: $hil_target_identity,
            logs: $hil_logs,
            required: ($mode == "official"),
            blocking: ($mode == "official" and $hil_status != "waived"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          },
          artifacts: {
            status: $artifact_status,
            artifact: (if $artifacts_artifact == "" then null else $artifacts_artifact end),
            evidence_kind: "release_artifact_manifest",
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: false,
            release_version_required: false
          },
          security: {
            status: $security_status,
            artifact: (if $security_artifact == "" then null else $security_artifact end),
            evidence_kind: "security_gate",
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          },
          evals: {
            status: $eval_status,
            artifact: (if $eval_artifact == "" then null else $eval_artifact end),
            evidence_kind: "agentops_eval_fast",
            required: ($mode == "official"),
            blocking: ($mode == "official"),
            freshness_required: ($mode == "official"),
            release_version_required: ($mode == "official")
          }
        }
      },
      evidence_errors: $evidence_errors,
      recommendations: $recommendations
    }')"

if [[ -n "$OUT" ]]; then
    mkdir -p "$(dirname "$OUT")"
    printf '%s\n' "$DOCUMENT" >"$OUT"
fi

printf '%s\n' "$DOCUMENT"

if [[ "$MODE" == "official" && "$RELEASE_STATUS" != "pass" ]]; then
    exit 1
fi

exit 0
