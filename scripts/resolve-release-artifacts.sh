#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ARTIFACT_ROOT="$REPO_ROOT/.agents/releases/local-ci"

usage() {
    cat <<'USAGE'
Usage: scripts/resolve-release-artifacts.sh <version>

Find the newest full local-CI artifact set for a release version and print its
manifest JSON. The version may be passed as X.Y.Z or vX.Y.Z.
USAGE
}

if [[ $# -ne 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    [[ $# -eq 1 ]] && exit 0
    exit 1
fi

VERSION="${1#v}"

if [[ ! -d "$ARTIFACT_ROOT" ]]; then
    echo "ERROR: local CI artifact root not found: $ARTIFACT_ROOT" >&2
    exit 1
fi

while IFS= read -r manifest; do
    if ! jq -e --arg version "$VERSION" '
        .schema_version == 1 and
        .release_version == $version and
        .fast_mode == false and
        (.artifact_dir | type == "string" and length > 0) and
        (.sbom_cyclonedx | type == "string" and length > 0) and
        (.sbom_spdx | type == "string" and length > 0) and
        (.security_report | type == "string" and length > 0) and
        (.release_readiness | type == "string" and length > 0) and
        (.hil_evidence | type == "string" and length > 0) and
        (.vil_evidence | type == "string" and length > 0) and
        (.digital_twin_evidence | type == "string" and length > 0) and
        (.eval_fast_report | type == "string" and length > 0) and
        (.eval_baseline_audit | type == "string" and length > 0)
    ' "$manifest" >/dev/null 2>&1; then
        continue
    fi

    artifact_dir="$(jq -r '.artifact_dir' "$manifest")"
    sbom_cyclonedx="$(jq -r '.sbom_cyclonedx' "$manifest")"
    sbom_spdx="$(jq -r '.sbom_spdx' "$manifest")"
    security_report="$(jq -r '.security_report' "$manifest")"
    release_readiness="$(jq -r '.release_readiness' "$manifest")"
    hil_evidence="$(jq -r '.hil_evidence' "$manifest")"
    vil_evidence="$(jq -r '.vil_evidence' "$manifest")"
    digital_twin_evidence="$(jq -r '.digital_twin_evidence' "$manifest")"
    eval_fast_report="$(jq -r '.eval_fast_report' "$manifest")"
    eval_baseline_audit="$(jq -r '.eval_baseline_audit' "$manifest")"

    if [[ -f "$REPO_ROOT/$artifact_dir/$sbom_cyclonedx" && \
          -f "$REPO_ROOT/$artifact_dir/$sbom_spdx" && \
          -f "$REPO_ROOT/$artifact_dir/$security_report" && \
          -f "$REPO_ROOT/$artifact_dir/$release_readiness" && \
          -f "$REPO_ROOT/$artifact_dir/$hil_evidence" && \
          -f "$REPO_ROOT/$artifact_dir/$vil_evidence" && \
          -f "$REPO_ROOT/$artifact_dir/$digital_twin_evidence" && \
          -f "$REPO_ROOT/$artifact_dir/$eval_fast_report" && \
          -f "$REPO_ROOT/$artifact_dir/$eval_baseline_audit" ]] && \
        jq -e '
          .schema_version == 1 and
          .release_status == "pass" and
          (.release_readiness_score >= 8) and
          .dimensions.sil.status == "pass" and
          .dimensions.vil.status == "pass" and
          (.dimensions.hil.status == "pass" or .dimensions.hil.status == "waived")
        ' "$REPO_ROOT/$artifact_dir/$release_readiness" >/dev/null 2>&1 && \
        jq -e '((.gate_status // "") | ascii_downcase) == "pass"' "$REPO_ROOT/$artifact_dir/$security_report" >/dev/null 2>&1 && \
        jq -e '.schema_version == 1 and .status == "pass"' "$REPO_ROOT/$artifact_dir/$eval_fast_report" >/dev/null 2>&1 && \
        jq -e '(.policy_mismatch_count | type == "number") and (((.stale_suite_hashes // []) | length) == 0)' "$REPO_ROOT/$artifact_dir/$eval_baseline_audit" >/dev/null 2>&1 && \
        jq -e '.schema_version == 1 and .evidence_kind == "digital_twin" and .status == "pass" and .dimensions.vil.status == "pass"' "$REPO_ROOT/$artifact_dir/$digital_twin_evidence" >/dev/null 2>&1; then
        cat "$manifest"
        exit 0
    fi
done < <(find "$ARTIFACT_ROOT" -mindepth 2 -maxdepth 2 -type f -name 'release-artifacts.json' | sort -r)

echo "ERROR: no full local CI artifacts found for release version $VERSION" >&2
echo "Run: ./scripts/ci-local-release.sh --release-version $VERSION" >&2
exit 1
