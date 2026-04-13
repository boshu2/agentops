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

validate_manifest_artifacts() {
    local audit="$1"
    local version="$2"
    local artifact_dir="$3"
    local manifest="$4"
    local release_version
    local sbom_cyclonedx
    local sbom_spdx
    local security_report

    release_version="$(jq -r '.release_version // empty' "$manifest")"
    sbom_cyclonedx="$(jq -r '.sbom_cyclonedx // empty' "$manifest")"
    sbom_spdx="$(jq -r '.sbom_spdx // empty' "$manifest")"
    security_report="$(jq -r '.security_report // empty' "$manifest")"

    if [[ "$release_version" != "$version" ]]; then
        printf '%s: manifest release_version=%s, expected %s\n' "$audit" "$release_version" "$version"
        return 1
    fi

    for artifact_file in "$sbom_cyclonedx" "$sbom_spdx" "$security_report"; do
        if [[ -z "$artifact_file" || ! -f "$REPO_ROOT/$artifact_dir/$artifact_file" ]]; then
            printf '%s: missing manifest artifact %s under %s\n' "$audit" "${artifact_file:-<blank>}" "$artifact_dir"
            return 1
        fi
    done
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

failures=()
while IFS= read -r audit; do
    version="$(extract_version "$audit")"
    [[ -n "$version" ]] || continue

    artifact_dir="$(extract_artifact_dir "$audit")"
    [[ -n "$artifact_dir" ]] || continue
    artifact_dir="${artifact_dir%/}"

    manifest="$REPO_ROOT/$artifact_dir/release-artifacts.json"
    if [[ -f "$manifest" ]]; then
        if ! output="$(validate_manifest_artifacts "$audit" "$version" "$artifact_dir" "$manifest")"; then
            failures+=("$output")
        fi
    elif ! output="$(validate_legacy_artifacts "$audit" "$version" "$artifact_dir")"; then
        failures+=("$output")
    fi
done < <(find "$REPO_ROOT/docs/releases" -maxdepth 1 -type f -name '*-audit.md' | sort)

if (( ${#failures[@]} > 0 )); then
    printf 'release audit artifact validation failed:\n' >&2
    printf '  - %s\n' "${failures[@]}" >&2
    exit 1
fi

echo "release audit artifact validation passed."
