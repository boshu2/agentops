#!/usr/bin/env bash
set -euo pipefail

MODE="${AGENTOPS_RELEASE_READINESS_MODE:-official}"
THRESHOLD="${AGENTOPS_RELEASE_READINESS_THRESHOLD:-8}"
OUT=""
ARTIFACT_DIR=""
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

if [[ -n "$HIL_FILE" ]]; then
    if [[ -f "$HIL_FILE" ]]; then
        HIL_STATUS="$(jq -r '.status // "fail"' "$HIL_FILE")"
        if [[ -z "$HIL_WAIVER" ]]; then
            HIL_WAIVER="$(jq -r '.waiver // empty' "$HIL_FILE")"
        fi
    else
        HIL_STATUS="fail"
    fi
fi

if [[ -z "$HIL_STATUS" ]]; then
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

timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

HIL_ARTIFACT=""
if [[ -n "$HIL_FILE" ]]; then
    HIL_ARTIFACT="$(basename "$HIL_FILE")"
fi

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
    --arg hil_artifact "$HIL_ARTIFACT" \
    --arg hil_waiver "$HIL_WAIVER" \
    --argjson threshold "$THRESHOLD" \
    --argjson release_readiness_score "$SCORE" \
    --argjson sil_points "$SIL_POINTS" \
    --argjson vil_points "$VIL_POINTS" \
    --argjson hil_points "$HIL_POINTS" \
    --argjson artifact_points "$ARTIFACT_POINTS" \
    --argjson security_points "$SECURITY_POINTS" \
    --argjson eval_points "$EVAL_POINTS" \
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
        sil: {status: $sil_status, weight: 2, points: $sil_points},
        vil: {status: $vil_status, weight: 2, points: $vil_points},
        hil: {status: $hil_status, weight: 2, points: $hil_points},
        artifacts: {status: $artifact_status, weight: 1.5, points: $artifact_points},
        security: {status: $security_status, weight: 1.5, points: $security_points},
        evals: {status: $eval_status, weight: 1, points: $eval_points}
      },
      hil_evidence: {
        status: $hil_status,
        artifact: (if $hil_artifact == "" then null else $hil_artifact end),
        waiver: (if $hil_waiver == "" then null else $hil_waiver end)
      },
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
