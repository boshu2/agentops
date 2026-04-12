#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-retrieval-quality-ratchet.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/cli" "$MOCK_BIN"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh"
    chmod +x "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh"

    REPORT_FILE="$TMP_DIR/report.json"
    export REPORT_FILE
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
cat "$REPORT_FILE"
GO
    chmod +x "$MOCK_BIN/go"
    export PATH="$MOCK_BIN:$PATH"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_report() {
    local metric="$1"
    cat > "$REPORT_FILE" <<JSON
{
  "queries": 20,
  "hits": 13,
  "missing_ground_truth": 1,
  "any_relevant_at_k": $metric,
  "avg_precision_at_k": 0.60
}
JSON
}

@test "retrieval ratchet passes when metric meets threshold" {
    write_report "0.65"

    cd "$FAKE_REPO"
    run bash scripts/check-retrieval-quality-ratchet.sh

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS retrieval quality ratchet"* ]]
    [[ "$output" == *"any_relevant_at_k=0.65"* ]]
}

@test "retrieval ratchet warns below threshold before strict turn count" {
    write_report "0.40"

    cd "$FAKE_REPO"
    run bash scripts/check-retrieval-quality-ratchet.sh

    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN retrieval quality ratchet"* ]]
    [[ "$output" == *"indexed_turns=0"* ]]
}

@test "retrieval ratchet fails below threshold after strict turn count" {
    write_report "0.40"
    mkdir -p "$FAKE_REPO/.agents/ao/sessions/turns"
    touch "$FAKE_REPO/.agents/ao/sessions/turns/t1.md"

    cd "$FAKE_REPO"
    export AGENTOPS_RETRIEVAL_RATCHET_STRICT_TURNS=1
    run bash scripts/check-retrieval-quality-ratchet.sh

    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL retrieval quality ratchet"* ]]
    [[ "$output" == *"indexed_turns=1"* ]]
}
