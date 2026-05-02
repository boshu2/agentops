#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-readiness.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "official readiness passes with complete SIL/VIL/HIL evidence" {
    out="$TMP_DIR/release-readiness.json"

    run bash "$SCRIPT" \
        --mode official \
        --out "$out" \
        --sil pass \
        --vil pass \
        --hil-status pass \
        --artifacts pass \
        --security pass \
        --eval pass

    [ "$status" -eq 0 ]
    jq -e '.release_status == "pass" and .release_readiness_score == 10 and .dimensions.hil.status == "pass"' "$out"
}

@test "official readiness fails when HIL is missing" {
    out="$TMP_DIR/release-readiness.json"

    run bash "$SCRIPT" \
        --mode official \
        --out "$out" \
        --sil pass \
        --vil pass \
        --hil-status skipped \
        --artifacts pass \
        --security pass \
        --eval pass

    [ "$status" -eq 1 ]
    jq -e '.release_status == "fail" and .release_readiness_score == 8 and .dimensions.hil.status == "skipped"' "$out"
}

@test "official readiness accepts an explicit HIL waiver above threshold" {
    out="$TMP_DIR/release-readiness.json"

    run bash "$SCRIPT" \
        --mode official \
        --out "$out" \
        --sil pass \
        --vil pass \
        --hil-status waived \
        --hil-waiver "bench unavailable" \
        --artifacts pass \
        --security pass \
        --eval pass

    [ "$status" -eq 0 ]
    jq -e '.release_status == "pass" and .release_readiness_score == 9 and .hil_evidence.waiver == "bench unavailable"' "$out"
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

@test "readiness reads HIL status from a HIL evidence file" {
    hil="$TMP_DIR/hil-evidence.json"
    out="$TMP_DIR/release-readiness.json"
    jq -n '{schema_version:1,status:"pass",waiver:null,targets:[{name:"loop",status:"pass"}]}' > "$hil"

    run bash "$SCRIPT" \
        --mode official \
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
