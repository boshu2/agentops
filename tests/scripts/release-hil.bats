#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-hil.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "optional HIL evidence skips cleanly when no target is configured" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "skipped" and .required == false and (.targets | length == 0)' "$out"
}

@test "required HIL evidence fails without target or waiver" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --required --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and .required == true' "$out"
}

@test "required HIL evidence can be explicitly waived" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --required --waiver "bench unavailable" --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "waived" and .waiver == "bench unavailable"' "$out"
}

@test "local HIL target pass is recorded" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --target "local:loop:true" --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "pass" and .targets[0].name == "loop" and .targets[0].status == "pass" and .targets[0].runtime.os != null and .targets[0].command_sha256 != null' "$out"
}

@test "local HIL target failure fails the evidence lane" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --target "local:loop:false" --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and .targets[0].status == "fail"' "$out"
}

@test "required HIL target rejects ao version only as weak evidence" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" \
        --required \
        --expected-version "2.42.0" \
        --target "local:loop:printf 'ao version 2.42.0\n'" \
        --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and .targets[0].workflow_strength == "weak" and (.targets[0].failure_reasons | index("weak_workflow")) != null' "$out"
}

@test "required HIL target records strong install hooks rpi workflow evidence" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" \
        --required \
        --expected-version "2.42.0" \
        --target "local:loop:printf 'ao version 2.42.0\nao init ok\nao hooks ok\nao rpi ok\n'" \
        --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "pass" and .expected_version == "2.42.0" and .targets[0].workflow_strength == "strong" and .targets[0].version_verified == true and (.targets[0].workflow_checks | index("ao-rpi")) != null' "$out"
}

@test "required HIL target rejects mismatched release version" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" \
        --required \
        --expected-version "2.42.0" \
        --target "local:loop:printf 'ao version 2.41.0\nao init ok\nao hooks ok\nao rpi ok\n'" \
        --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and .targets[0].version_verified == false and (.targets[0].failure_reasons | index("version_mismatch")) != null' "$out"
}
