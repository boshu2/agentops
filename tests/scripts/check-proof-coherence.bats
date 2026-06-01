#!/usr/bin/env bats

# Proof-coherence gate (reconciliation engine Wave 3C; soc-57hbj). For each L2/L3
# claim-ledger row, every evidence_event_ref must resolve to a daemon event that
# exists AND carries verdict: pass. A dangling ref, a failed verdict, or an L2/L3
# row with zero refs is incoherent (exit 1). L0/L1 rows are not gated. With no
# ledger present the gate is a PASS no-op. Bad flag / missing input = exit 2.
#
# All fixtures are hermetic: built in a temp dir and passed via --ledger/--events
# so the test never reads the live repo ledger or daemon stream.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-proof-coherence.sh"
    TMP_DIR="$(mktemp -d)"

    # --- daemon event stream: one passing event, one failing event ----------
    cat > "$TMP_DIR/events.jsonl" <<'EOF'
{"event_type":"agent_update.criterion_verdict","run_id":"run-001","payload":{"criterion_id":"context-on-passes","verdict":"pass"}}
{"event_type":"agent_update.criterion_verdict","run_id":"run-002","payload":{"criterion_id":"context-off-fails","verdict":"fail"}}

garbage-not-json
EOF

    # --- coherent ledger: L2 row references the passing event; L1 row ignored
    cat > "$TMP_DIR/coherent.json" <<'EOF'
{
  "rows": [
    { "claim_id": "C-l1-ungated", "promotion_state": "L1",
      "evidence_event_refs": [] },
    { "claim_id": "C-worker-context", "promotion_state": "L2",
      "evidence_event_refs": [
        { "event_type": "agent_update.criterion_verdict", "run_id": "run-001", "criterion_id": "context-on-passes" }
      ] }
  ]
}
EOF

    # --- dangling: L2 row references a run_id that has no daemon event -------
    cat > "$TMP_DIR/dangling.json" <<'EOF'
{
  "rows": [
    { "claim_id": "C-dangling", "promotion_state": "L2",
      "evidence_event_refs": [
        { "event_type": "agent_update.criterion_verdict", "run_id": "run-999", "criterion_id": "ghost" }
      ] }
  ]
}
EOF

    # --- failed: L3 row references the event whose verdict is fail ----------
    cat > "$TMP_DIR/failed.json" <<'EOF'
{
  "rows": [
    { "claim_id": "C-failed", "promotion_state": "L3",
      "evidence_event_refs": [
        { "event_type": "agent_update.criterion_verdict", "run_id": "run-002", "criterion_id": "context-off-fails" }
      ] }
  ]
}
EOF

    # --- unbacked: L2 row with zero evidence_event_refs ---------------------
    cat > "$TMP_DIR/unbacked.json" <<'EOF'
{ "rows": [ { "claim_id": "C-unbacked", "promotion_state": "L2", "evidence_event_refs": [] } ] }
EOF

    # --- only L0/L1 rows: nothing gated, must pass even with empty refs -----
    cat > "$TMP_DIR/lowtier.json" <<'EOF'
{
  "rows": [
    { "claim_id": "C-l0", "promotion_state": "L0", "evidence_event_refs": [] },
    { "claim_id": "C-l1", "promotion_state": "L1",
      "evidence_event_refs": [ { "event_type": "x", "run_id": "run-999" } ] }
  ]
}
EOF

    # --- malformed ledger (not valid JSON) ----------------------------------
    printf 'this is not json {' > "$TMP_DIR/bad.json"
}

teardown() { rm -rf "$TMP_DIR"; }

@test "coherent L2 row referencing a passing event exits 0" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/coherent.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"1 L2/L3 row(s) coherent"* ]]
    [[ "$output" == *"C-worker-context"* ]]
    [[ "$output" == *"-> pass"* ]]
}

@test "dangling ref (no matching daemon event) exits 1 with DANGLING" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/dangling.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"DANGLING"* ]]
    [[ "$output" == *"C-dangling"* ]]
    [[ "$output" == *"run_id=run-999"* ]]
}

@test "referenced event with verdict fail exits 1 with FAILED" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/failed.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAILED"* ]]
    [[ "$output" == *"verdict='fail'"* ]]
    [[ "$output" == *"C-failed"* ]]
}

@test "L2 row with zero evidence_event_refs is UNBACKED, exits 1" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/unbacked.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"UNBACKED"* ]]
    [[ "$output" == *"C-unbacked"* ]]
}

@test "only L0/L1 rows are not gated, exits 0 even with a dangling L1 ref" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/lowtier.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 L2/L3 row(s) coherent"* ]]
}

@test "explicitly-passed unreadable event stream is an env error, exits 2" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/coherent.json" --events "$TMP_DIR/no-such.jsonl"
    [ "$status" -eq 2 ]
    [[ "$output" == *"not readable"* ]]
}

@test "empty event stream makes a referenced ref dangling, exits 1" {
    : > "$TMP_DIR/empty.jsonl"
    run bash "$SCRIPT" --ledger "$TMP_DIR/coherent.json" --events "$TMP_DIR/empty.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"DANGLING"* ]]
}

@test "no ledger present is a PASS no-op, exits 0" {
    # Auto-discovery finds no ledger when run from a repo with none. Use an empty
    # git repo as repo_root surrogate so git rev-parse --show-toplevel resolves there.
    EMPTY_ROOT="$(mktemp -d)"
    git init -q "$EMPTY_ROOT"
    run bash -c "cd '$EMPTY_ROOT' && bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no claim ledger present"* ]]
    rm -rf "$EMPTY_ROOT"
}

@test "json mode emits a machine-readable fail report for incoherence" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/dangling.json" --events "$TMP_DIR/events.jsonl" --json
    [ "$status" -eq 1 ]
    echo "$output" | jq -e '.gate == "proof-coherence"' >/dev/null
    echo "$output" | jq -e '.status == "fail"' >/dev/null
    echo "$output" | jq -e '.incoherent_count == 1' >/dev/null
    echo "$output" | jq -e '.incoherent[0].kind == "dangling"' >/dev/null
    echo "$output" | jq -e '.incoherent[0].claim_id == "C-dangling"' >/dev/null
}

@test "json mode emits a pass report when coherent" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/coherent.json" --events "$TMP_DIR/events.jsonl" --json
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.status == "pass"' >/dev/null
    echo "$output" | jq -e '.rows_checked == 1' >/dev/null
    echo "$output" | jq -e '.incoherent | length == 0' >/dev/null
}

@test "malformed ledger JSON is a usage error, exits 2" {
    run bash "$SCRIPT" --ledger "$TMP_DIR/bad.json" --events "$TMP_DIR/events.jsonl"
    [ "$status" -eq 2 ]
    [[ "$output" == *"not valid JSON"* ]]
}

@test "unknown flag is a usage error, exits 2" {
    run bash "$SCRIPT" --bogus
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown argument"* ]]
}

@test "--help exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"proof-coherence"* ]]
}