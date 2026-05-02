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
    jq -e '.status == "pass" and .targets[0].name == "loop" and .targets[0].status == "pass"' "$out"
}

@test "local HIL target failure fails the evidence lane" {
    out="$TMP_DIR/hil-evidence.json"

    run bash "$SCRIPT" --target "local:loop:false" --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and .targets[0].status == "fail"' "$out"
}
