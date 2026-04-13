#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/resolve-release-artifacts.sh"
    AUDIT_SCRIPT="$REPO_ROOT/scripts/validate-release-audit-artifacts.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/.agents/releases/local-ci"
    cp "$SCRIPT" "$FAKE_REPO/scripts/resolve-release-artifacts.sh"
    cp "$AUDIT_SCRIPT" "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
    chmod +x "$FAKE_REPO/scripts/resolve-release-artifacts.sh"
    chmod +x "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
}

write_audit() {
    local version="$1"
    local run_id="$2"
    local dir="$FAKE_REPO/docs/releases"
    mkdir -p "$dir"
    cat > "$dir/2026-03-22-v${version}-audit.md" <<EOF
# Release v${version} - Audit

**Local CI Artifacts:** \`.agents/releases/local-ci/$run_id/\`
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_manifest() {
    local run_id="$1"
    local version="$2"
    local fast_mode="$3"
    local dir="$FAKE_REPO/.agents/releases/local-ci/$run_id"
    mkdir -p "$dir"

    cat > "$dir/release-artifacts.json" <<EOF
{
  "schema_version": 1,
  "run_id": "$run_id",
  "generated_at": "2026-03-22T21:22:22Z",
  "artifact_dir": ".agents/releases/local-ci/$run_id",
  "release_version": "$version",
  "repo_version": "$version",
  "fast_mode": $fast_mode,
  "security_mode": "full",
  "sbom_cyclonedx": "sbom-v$version.cyclonedx.json",
  "sbom_spdx": "sbom-v$version.spdx.json",
  "security_report": "security-gate-full.json"
}
EOF
}

write_artifact_files() {
    local run_id="$1"
    local version="$2"
    local dir="$FAKE_REPO/.agents/releases/local-ci/$run_id"
    printf '{"bomFormat":"CycloneDX"}\n' > "$dir/sbom-v$version.cyclonedx.json"
    printf '{"spdxVersion":"SPDX-2.3"}\n' > "$dir/sbom-v$version.spdx.json"
    printf '{"gate_status":"pass"}\n' > "$dir/security-gate-full.json"
}

@test "resolve-release-artifacts picks the newest complete manifest for a version" {
    write_manifest "20260322T204431Z" "2.29.0" false
    write_artifact_files "20260322T204431Z" "2.29.0"
    write_manifest "20260322T212222Z" "2.29.0" false
    write_artifact_files "20260322T212222Z" "2.29.0"

    run "$FAKE_REPO/scripts/resolve-release-artifacts.sh" "v2.29.0"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"run_id": "20260322T212222Z"'* ]]
    [[ "$output" == *'"artifact_dir": ".agents/releases/local-ci/20260322T212222Z"'* ]]
}

@test "resolve-release-artifacts ignores fast-mode or incomplete manifests" {
    write_manifest "20260322T204431Z" "2.29.0" false
    write_artifact_files "20260322T204431Z" "2.29.0"
    write_manifest "20260322T212222Z" "2.29.0" true
    write_artifact_files "20260322T212222Z" "2.29.0"

    run "$FAKE_REPO/scripts/resolve-release-artifacts.sh" "2.29.0"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"run_id": "20260322T204431Z"'* ]]
}

@test "resolve-release-artifacts fails when no full artifact set exists for the version" {
    write_manifest "20260322T212222Z" "2.29.0" false

    run "$FAKE_REPO/scripts/resolve-release-artifacts.sh" "2.29.0"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no full local CI artifacts found for release version 2.29.0"* ]]
}

@test "validate-release-audit-artifacts accepts matching manifest versions" {
    write_manifest "20260322T212222Z" "2.29.0" false
    write_artifact_files "20260322T212222Z" "2.29.0"
    write_audit "2.29.0" "20260322T212222Z"

    run "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"release audit artifact validation passed"* ]]
}

@test "validate-release-audit-artifacts rejects mismatched manifest versions" {
    write_manifest "20260322T212222Z" "2.29.0" false
    write_artifact_files "20260322T212222Z" "2.29.0"
    write_audit "2.30.0" "20260322T212222Z"

    run "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"manifest release_version=2.29.0, expected 2.30.0"* ]]
}

@test "validate-release-audit-artifacts accepts legacy versioned artifact dirs" {
    mkdir -p "$FAKE_REPO/.agents/releases/local-ci/20260322T212222Z"
    write_artifact_files "20260322T212222Z" "2.29.0"
    write_audit "2.29.0" "20260322T212222Z"

    run "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"release audit artifact validation passed"* ]]
}
