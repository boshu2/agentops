#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/nightly-knowledge-cycle.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_AO="$TMP_DIR/ao"
    export NIGHTLY_KNOWLEDGE_CYCLE_DIR="$TMP_DIR/knowledge-cycle"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_fake_ao() {
    local payload="$1"
    cat >"$FAKE_AO" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "metrics" && "\$2" == "report" && "\$3" == "--json" ]]; then
    cat <<'JSON'
$payload
JSON
    exit 0
fi
exit 1
EOF
    chmod +x "$FAKE_AO"
}

@test "empty corpus skips nightly knowledge cycle as corpus-empty" {
    write_fake_ao '{"total_artifacts":0,"citations_this_period":0}'
    run env AO="$FAKE_AO" bash "$SCRIPT" precondition
    [ "$status" -eq 0 ]
    [[ "$output" == *"decision=SKIP"* ]]
    [[ "$output" == *"reason=corpus-empty:"* ]]
    [[ "$output" == *"total_artifacts=0"* ]]
    jq -e '.decision == "SKIP" and (.reason | startswith("corpus-empty:"))' \
        "$NIGHTLY_KNOWLEDGE_CYCLE_DIR/triage.json"
}

@test "dormant non-empty corpus skips as corpus-dormant" {
    write_fake_ao '{"total_artifacts":7,"citations_this_period":0}'
    run env AO="$FAKE_AO" bash "$SCRIPT" precondition
    [ "$status" -eq 0 ]
    [[ "$output" == *"decision=SKIP"* ]]
    [[ "$output" == *"reason=corpus-dormant:"* ]]
    [[ "$output" == *"total_artifacts=7"* ]]
}

@test "active corpus runs nightly knowledge cycle" {
    write_fake_ao '{"total_artifacts":7,"citations_this_period":1}'
    run env AO="$FAKE_AO" bash "$SCRIPT" precondition
    [ "$status" -eq 0 ]
    [[ "$output" == *"decision=RUN"* ]]
    [[ "$output" == *"reason=corpus-active:"* ]]
    [[ "$output" == *"citations_in_window=1"* ]]
}

@test "force override runs even when corpus is empty" {
    write_fake_ao '{"total_artifacts":0,"citations_this_period":0}'
    run env AO="$FAKE_AO" NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1 bash "$SCRIPT" precondition
    [ "$status" -eq 0 ]
    [[ "$output" == *"decision=RUN"* ]]
    [[ "$output" == *"reason=forced via NIGHTLY_KNOWLEDGE_CYCLE_FORCE=1"* ]]
}

@test "metrics report failure runs for diagnostic signal" {
    cat >"$FAKE_AO" <<'EOF'
#!/usr/bin/env bash
exit 2
EOF
    chmod +x "$FAKE_AO"

    run env AO="$FAKE_AO" bash "$SCRIPT" precondition
    [ "$status" -eq 0 ]
    [[ "$output" == *"decision=RUN"* ]]
    [[ "$output" == *"reason=metrics-report-unavailable:"* ]]
    jq -e '.precondition.metrics_status == "unavailable"' \
        "$NIGHTLY_KNOWLEDGE_CYCLE_DIR/triage.json"
}
