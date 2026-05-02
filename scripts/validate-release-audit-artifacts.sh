#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

extract_version() {
    local audit="$1"
    basename "$audit" | sed -n 's/.*-v\([0-9][0-9.]*\)-audit\.md/\1/p'
}

extract_artifact_dir() {
    local audit="$1"
    local artifact

    artifact="$(sed -n 's/^\*\*Retag Local CI Artifacts:\*\* `\([^`]*\)`.*/\1/p' "$audit" | tail -1)"
    if [[ -z "$artifact" ]]; then
        artifact="$(sed -n 's/^\*\*Local CI Artifacts:\*\* `\([^`]*\)`.*/\1/p' "$audit" | head -1)"
    fi
    if [[ -z "$artifact" ]]; then
        artifact="$(sed -n 's/^\*\*Original Local CI Artifacts:\*\* `\([^`]*\)`.*/\1/p' "$audit" | head -1)"
    fi

    printf '%s\n' "$artifact"
}

release_readiness_required_for_audit() {
    local audit="$1"
    local audit_date

    audit_date="$(basename "$audit" | sed -n 's/^\([0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]\)-v.*/\1/p')"
    [[ -n "$audit_date" && "$audit_date" > "2026-05-01" ]]
}

validate_manifest_artifacts() {
    local audit="$1"
    local version="$2"
    local artifact_dir="$3"
    local manifest="$4"
    local strict_readiness="$5"
    local schema_version
    local release_version
    local manifest_artifact_dir
    local fast_mode
    local sbom_cyclonedx
    local sbom_spdx
    local security_report
    local release_readiness
    local hil_evidence

    schema_version="$(jq -r '.schema_version // empty' "$manifest")"
    release_version="$(jq -r '.release_version // empty' "$manifest")"
    manifest_artifact_dir="$(jq -r '.artifact_dir // empty' "$manifest")"
    fast_mode="$(jq -r 'if has("fast_mode") then .fast_mode else empty end' "$manifest")"
    sbom_cyclonedx="$(jq -r '.sbom_cyclonedx // empty' "$manifest")"
    sbom_spdx="$(jq -r '.sbom_spdx // empty' "$manifest")"
    security_report="$(jq -r '.security_report // empty' "$manifest")"
    release_readiness="$(jq -r '.release_readiness // empty' "$manifest")"
    hil_evidence="$(jq -r '.hil_evidence // empty' "$manifest")"
    manifest_artifact_dir="${manifest_artifact_dir%/}"

    if [[ "$schema_version" != "1" ]]; then
        printf '%s: manifest schema_version=%s, expected 1\n' "$audit" "${schema_version:-<blank>}"
        return 1
    fi

    if [[ "$release_version" != "$version" ]]; then
        printf '%s: manifest release_version=%s, expected %s\n' "$audit" "$release_version" "$version"
        return 1
    fi

    if [[ "$manifest_artifact_dir" != "$artifact_dir" ]]; then
        printf '%s: manifest artifact_dir=%s, expected %s\n' "$audit" "${manifest_artifact_dir:-<blank>}" "$artifact_dir"
        return 1
    fi

    if [[ "$fast_mode" != "false" ]]; then
        printf '%s: manifest fast_mode=%s, expected false\n' "$audit" "${fast_mode:-<blank>}"
        return 1
    fi

    for artifact_file in "$sbom_cyclonedx" "$sbom_spdx" "$security_report"; do
        if [[ -z "$artifact_file" || ! -f "$REPO_ROOT/$artifact_dir/$artifact_file" ]]; then
            printf '%s: missing manifest artifact %s under %s\n' "$audit" "${artifact_file:-<blank>}" "$artifact_dir"
            return 1
        fi
    done

    if [[ "$strict_readiness" == "true" || -n "$release_readiness" || -n "$hil_evidence" ]]; then
        if [[ -z "$release_readiness" || ! -f "$REPO_ROOT/$artifact_dir/$release_readiness" ]]; then
            printf '%s: missing release readiness artifact %s under %s\n' "$audit" "${release_readiness:-<blank>}" "$artifact_dir"
            return 1
        fi
        if [[ -z "$hil_evidence" || ! -f "$REPO_ROOT/$artifact_dir/$hil_evidence" ]]; then
            printf '%s: missing HIL evidence artifact %s under %s\n' "$audit" "${hil_evidence:-<blank>}" "$artifact_dir"
            return 1
        fi
        if ! jq -e '
            .schema_version == 1 and
            (.release_readiness_score | type == "number") and
            (.release_readiness_score >= 8) and
            .release_status == "pass" and
            .dimensions.sil.status == "pass" and
            .dimensions.vil.status == "pass" and
            (.dimensions.hil.status == "pass" or .dimensions.hil.status == "waived")
        ' "$REPO_ROOT/$artifact_dir/$release_readiness" >/dev/null; then
            printf '%s: release readiness artifact did not pass >=8 SIL/VIL/HIL gate: %s/%s\n' "$audit" "$artifact_dir" "$release_readiness"
            return 1
        fi
    fi

    return 0
}

validate_legacy_artifacts() {
    local audit="$1"
    local version="$2"
    local artifact_dir="$3"
    local dir="$REPO_ROOT/$artifact_dir"

    if [[ -f "$dir/sbom-v${version}.cyclonedx.json" && \
          -f "$dir/sbom-v${version}.spdx.json" && \
          -f "$dir/security-gate-full.json" ]]; then
        return 0
    fi

    printf '%s: no release-artifacts.json and no complete versioned artifact fallback under %s\n' "$audit" "$artifact_dir"
    return 1
}

audit_files=()
while IFS= read -r audit_file; do
    audit_files+=("$audit_file")
done < <(find "$REPO_ROOT/docs/releases" -maxdepth 1 -type f -name '*-audit.md' | sort)
latest_audit=""
if (( ${#audit_files[@]} > 0 )); then
    latest_audit="${audit_files[$((${#audit_files[@]} - 1))]}"
fi

failures=()
for audit in "${audit_files[@]}"; do
    version="$(extract_version "$audit")"
    [[ -n "$version" ]] || continue

    artifact_dir="$(extract_artifact_dir "$audit")"
    [[ -n "$artifact_dir" ]] || continue
    artifact_dir="${artifact_dir%/}"

    manifest="$REPO_ROOT/$artifact_dir/release-artifacts.json"
    if [[ ! -f "$manifest" && ! -d "$REPO_ROOT/$artifact_dir" && "$audit" != "$latest_audit" ]]; then
        continue
    fi

    if [[ -f "$manifest" ]]; then
        strict_readiness=false
        if release_readiness_required_for_audit "$audit"; then
            strict_readiness=true
        fi
        if ! output="$(validate_manifest_artifacts "$audit" "$version" "$artifact_dir" "$manifest" "$strict_readiness")"; then
            failures+=("$output")
        fi
    elif ! output="$(validate_legacy_artifacts "$audit" "$version" "$artifact_dir")"; then
        failures+=("$output")
    fi
done

if (( ${#failures[@]} > 0 )); then
    printf 'release audit artifact validation failed:\n' >&2
    printf '  - %s\n' "${failures[@]}" >&2
    exit 1
fi

echo "release audit artifact validation passed."
