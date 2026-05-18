#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-readiness.sh"
    TMP_DIR="$(mktemp -d)"
    VERSION="2.29.0"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_official_evidence() {
    local generated_at="${1:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
    local version="${2:-$VERSION}"

    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{schema_version:1,evidence_kind:"software_in_loop",generated_at:$generated_at,release_version:$version,status:"pass",errors_before_readiness:0}' \
        > "$TMP_DIR/sil-evidence.json"
    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{schema_version:1,evidence_kind:"digital_twin",generated_at:$generated_at,release_version:$version,status:"pass",workflow_strength:"full",ao_bin_sha256:"0123456789abcdef",runtime:{os:"Linux",arch:"x86_64"},dimensions:{vil:{status:"pass"},release_smoke:{status:"pass"},hook_install_smoke:{status:"pass"},rpi_smoke:{status:"pass"}},checks:[range(0;6) | {name:("check-" + tostring),dimension:"release_smoke",status:"pass",exit_code:0,duration_seconds:0,output_preview:"ok"}]}' \
        > "$TMP_DIR/digital-twin-evidence.json"
    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{schema_version:1,generated_at:$generated_at,status:"pass",expected_version:$version,waiver:null,targets:[{name:"loop",kind:"local",host:null,status:"pass",workflow_strength:"strong",workflow_checks:["ao-version","ao-init","ao-hooks","ao-rpi"],command_sha256:"abcdef",runtime:{os:"Linux",arch:"x86_64"},version_verified:true}]}' \
        > "$TMP_DIR/hil-evidence.json"
    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{generated_at:$generated_at,release_version:$version,gate_status:"PASS"}' \
        > "$TMP_DIR/security-gate-full.json"
    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{schema_version:1,evidence_kind:"agentops_eval_fast",generated_at:$generated_at,release_version:$version,status:"pass",baseline_audit:"eval-baseline-audit.json"}' \
        > "$TMP_DIR/eval-agentops-fast.json"
    jq -n \
        --arg generated_at "$generated_at" \
        --arg version "$version" \
        '{generated_at:$generated_at,release_version:$version,suite_count:1,baseline_count:0,policy_mismatch_count:0,stale_suite_hashes:[]}' \
        > "$TMP_DIR/eval-baseline-audit.json"
}

run_official_readiness() {
    local out="$1"

    bash "$SCRIPT" \
        --mode official \
        --out "$out" \
        --artifact-dir "$TMP_DIR" \
        --evidence-dir "$TMP_DIR" \
        --release-version "$VERSION"
}

@test "official readiness passes with complete evidence artifacts" {
    out="$TMP_DIR/release-readiness.json"
    write_official_evidence

    run run_official_readiness "$out"

    [ "$status" -eq 0 ]
    jq -e '
        .release_status == "pass" and
        .release_readiness_score == 10 and
        .dimensions.sil.evidence_artifact == "sil-evidence.json" and
        .dimensions.vil.evidence_artifact == "digital-twin-evidence.json" and
        .dimensions.hil.evidence_artifact == "hil-evidence.json" and
        .dimensions.security.evidence_artifact == "security-gate-full.json" and
        .dimensions.evals.evidence_artifact == "eval-agentops-fast.json" and
        .evidence.policy.status_flags_trusted == false and
        .evidence.policy.pre_publish_blocking == true and
        .evidence.lanes.digital_twin.artifact == "digital-twin-evidence.json" and
        .evidence.lanes.digital_twin.workflow_strength == "full" and
        .evidence.lanes.digital_twin.binary_digest == "0123456789abcdef" and
        .evidence.lanes.hil.target_identity[0].name == "loop" and
        .evidence.lanes.artifacts.artifact == "release-artifacts.json"
    ' "$out"
}

@test "official readiness fails when required evidence is missing" {
    out="$TMP_DIR/release-readiness.json"
    write_official_evidence
    rm "$TMP_DIR/hil-evidence.json"

    run run_official_readiness "$out"

    [ "$status" -eq 1 ]
    jq -e '.release_status == "fail" and .dimensions.hil.status == "fail" and (.evidence_errors | index("missing HIL evidence: hil-evidence.json"))' "$out"
}

@test "official readiness accepts evidence-backed HIL waiver above threshold" {
    out="$TMP_DIR/release-readiness.json"
    write_official_evidence
    jq --arg waiver "bench unavailable" '.status = "waived" | .waiver = $waiver | .targets = []' \
        "$TMP_DIR/hil-evidence.json" > "$TMP_DIR/hil-evidence.json.tmp"
    mv "$TMP_DIR/hil-evidence.json.tmp" "$TMP_DIR/hil-evidence.json"

    run run_official_readiness "$out"

    [ "$status" -eq 0 ]
    jq -e '.release_status == "pass" and .release_readiness_score == 9 and .hil_evidence.waiver == "bench unavailable"' "$out"
}

@test "official readiness rejects caller status strings without evidence artifacts" {
    out="$TMP_DIR/release-readiness.json"

    run bash "$SCRIPT" \
        --mode official \
        --out "$out" \
        --artifact-dir "$TMP_DIR" \
        --sil pass \
        --vil pass \
        --hil-status pass \
        --artifacts pass \
        --security pass \
        --eval pass

    [ "$status" -eq 1 ]
    jq -e '.release_status == "fail" and .dimensions.sil.status == "fail"' "$out"
}

@test "official readiness rejects stale evidence" {
    out="$TMP_DIR/release-readiness.json"
    write_official_evidence "2000-01-01T00:00:00Z"

    run run_official_readiness "$out"

    [ "$status" -eq 1 ]
    jq -e '.release_status == "fail" and any(.evidence_errors[]; contains("stale"))' "$out"
}

@test "official readiness rejects mismatched release version evidence" {
    out="$TMP_DIR/release-readiness.json"
    write_official_evidence "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "2.30.0"

    run run_official_readiness "$out"

    [ "$status" -eq 1 ]
    jq -e '.release_status == "fail" and any(.evidence_errors[]; contains("version mismatch"))' "$out"
}

@test "advisory readiness records warning without failing the command" {
    out="$TMP_DIR/release-readiness.json"

    run bash "$SCRIPT" \
        --mode advisory \
        --out "$out" \
        --sil pass \
        --vil skipped \
        --hil-status skipped \
        --artifacts skipped \
        --security skipped \
        --eval pass

    [ "$status" -eq 0 ]
    jq -e '.release_status == "warn" and .release_readiness_score == 3' "$out"
}

@test "advisory readiness reads HIL status from a HIL evidence file" {
    hil="$TMP_DIR/hil-evidence.json"
    out="$TMP_DIR/release-readiness.json"
    jq -n '{schema_version:1,status:"pass",waiver:null,targets:[{name:"loop",status:"pass"}]}' > "$hil"

    run bash "$SCRIPT" \
        --mode advisory \
        --out "$out" \
        --hil-file "$hil" \
        --sil pass \
        --vil pass \
        --artifacts pass \
        --security pass \
        --eval pass

    [ "$status" -eq 0 ]
    jq -e '.release_status == "pass" and .hil_evidence.artifact == "hil-evidence.json"' "$out"
}
