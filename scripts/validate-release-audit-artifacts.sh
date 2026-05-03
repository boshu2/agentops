#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MODE="all"
TARGET_RELEASE=""

usage() {
    cat <<'EOF'
Usage: scripts/validate-release-audit-artifacts.sh [--mode all|latest|changed|target] [--target-release V]

Modes:
  all      Validate all audits; missing non-latest local artifact dirs are skipped.
  latest   Validate only the latest audit and require its artifacts.
  changed  Validate only audit files listed in RELEASE_AUDIT_CHANGED_PATHS.
  target   Validate the audit for --target-release and require its artifacts.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --mode)
            MODE="${2:-}"
            shift 2
            ;;
        --target-release)
            TARGET_RELEASE="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

case "$MODE" in
    all|latest|changed|target) ;;
    *)
        echo "Invalid --mode: $MODE" >&2
        usage >&2
        exit 2
        ;;
esac

if [[ "$MODE" == "target" ]]; then
    if [[ -z "$TARGET_RELEASE" ]]; then
        echo "--target-release is required with --mode target" >&2
        exit 2
    fi
    TARGET_RELEASE="${TARGET_RELEASE#v}"
fi

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

audit_selected() {
    local audit="$1"
    local version="$2"
    local rel_path

    rel_path="${audit#"$REPO_ROOT"/}"
    case "$MODE" in
        all)
            return 0
            ;;
        latest)
            [[ "$audit" == "$latest_audit" ]]
            ;;
        target)
            [[ "$version" == "$TARGET_RELEASE" ]]
            ;;
        changed)
            printf '%s\n' "${RELEASE_AUDIT_CHANGED_PATHS:-}" | grep -Fxq "$rel_path"
            ;;
    esac
}

failures=()
selected_count=0
for audit in "${audit_files[@]}"; do
    version="$(extract_version "$audit")"
    [[ -n "$version" ]] || continue

    if ! audit_selected "$audit" "$version"; then
        continue
    fi
    selected_count=$((selected_count + 1))

    artifact_dir="$(extract_artifact_dir "$audit")"
    [[ -n "$artifact_dir" ]] || continue
    artifact_dir="${artifact_dir%/}"

    manifest="$REPO_ROOT/$artifact_dir/release-artifacts.json"
    if [[ "$MODE" == "all" && ! -f "$manifest" && ! -d "$REPO_ROOT/$artifact_dir" && "$audit" != "$latest_audit" ]]; then
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

if [[ "$MODE" == "changed" && "$selected_count" -eq 0 ]]; then
    echo "release audit artifact validation passed: no changed release audits to validate."
    exit 0
fi

if [[ "$MODE" == "target" && "$selected_count" -eq 0 ]]; then
    printf 'release audit artifact validation failed:\n' >&2
    printf '  - no release audit found for target release %s\n' "$TARGET_RELEASE" >&2
    exit 1
fi

if (( ${#failures[@]} > 0 )); then
    printf 'release audit artifact validation failed:\n' >&2
    printf '  - %s\n' "${failures[@]}" >&2
    exit 1
fi

echo "release audit artifact validation passed."
