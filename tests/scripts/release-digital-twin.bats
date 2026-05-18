#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-digital-twin.sh"
    TMP_DIR="$(mktemp -d)"
    VERSION="2.29.0"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_fake_ao() {
    local path="$TMP_DIR/ao"
    cat > "$path" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
    version)
        echo "ao version ${AO_FAKE_VERSION:-2.29.0}"
        ;;
    init)
        if [[ "${2:-}" == "--hooks" ]]; then
            mkdir -p .agentops-init
            echo "hooks initialized"
        else
            echo "init help"
        fi
        ;;
    hooks)
        echo "hooks ok"
        ;;
    rpi)
        if [[ "${AO_FAKE_FAIL_RPI:-0}" == "1" ]]; then
            echo "rpi failed" >&2
            exit 12
        fi
        echo "rpi ok"
        ;;
    doctor)
        echo "doctor ok"
        ;;
    --help|-h)
        echo "ao help"
        ;;
    *)
        echo "unknown command: ${1:-}" >&2
        exit 2
        ;;
esac
SH
    chmod +x "$path"
    printf '%s\n' "$path"
}

@test "fast digital twin skips when ao binary is unavailable" {
    out="$TMP_DIR/digital-twin-evidence.json"

    run bash "$SCRIPT" --mode fast --ao-bin "$TMP_DIR/missing-ao" --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "skipped" and .dimensions.vil.status == "skipped"' "$out"
}

@test "official digital twin fails when ao binary is unavailable" {
    out="$TMP_DIR/digital-twin-evidence.json"

    run bash "$SCRIPT" --mode official --ao-bin "$TMP_DIR/missing-ao" --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and (.failure_reasons | index("ao_binary_unavailable"))' "$out"
}

@test "official digital twin can be explicitly waived" {
    out="$TMP_DIR/digital-twin-evidence.json"

    run bash "$SCRIPT" \
        --mode official \
        --ao-bin "$TMP_DIR/missing-ao" \
        --waiver "disposable runner unavailable" \
        --out "$out"

    [ "$status" -eq 0 ]
    jq -e '.status == "waived" and .waiver == "disposable runner unavailable"' "$out"
}

@test "official digital twin passes with full workflow evidence" {
    out="$TMP_DIR/digital-twin-evidence.json"
    ao_bin="$(write_fake_ao)"

    run bash "$SCRIPT" \
        --mode official \
        --ao-bin "$ao_bin" \
        --expected-version "$VERSION" \
        --out "$out"

    [ "$status" -eq 0 ]
    jq -e '
      .schema_version == 1 and
      .evidence_kind == "digital_twin" and
      .status == "pass" and
      .release_version == "2.29.0" and
      .workflow_strength == "full" and
      .dimensions.vil.status == "pass" and
      .dimensions.install_upgrade.status == "pass" and
      .dimensions.hook_install_smoke.status == "pass" and
      .dimensions.rpi_smoke.status == "pass" and
      (.checks | length) >= 6
    ' "$out"
}

@test "official digital twin fails on version mismatch" {
    out="$TMP_DIR/digital-twin-evidence.json"
    ao_bin="$(AO_FAKE_VERSION=2.30.0 write_fake_ao)"

    run env AO_FAKE_VERSION=2.30.0 bash "$SCRIPT" \
        --mode official \
        --ao-bin "$ao_bin" \
        --expected-version "$VERSION" \
        --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and (.failure_reasons | index("version_mismatch"))' "$out"
}

@test "official digital twin fails when a required workflow check fails" {
    out="$TMP_DIR/digital-twin-evidence.json"
    ao_bin="$(write_fake_ao)"

    run env AO_FAKE_FAIL_RPI=1 bash "$SCRIPT" \
        --mode official \
        --ao-bin "$ao_bin" \
        --expected-version "$VERSION" \
        --out "$out"

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and (.failure_reasons | index("workflow_failed")) and .dimensions.rpi_smoke.status == "fail"' "$out"
}
