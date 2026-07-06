#!/usr/bin/env bats
# verification-economics-report.bats — age-verification-economics-ebec.2
#
# Fixture fidelity: ledger fixtures below round-trip the REAL persisted record
# shapes (a verdict edge as emitted by pawl-verdict.sh on 2026-07-06 @ 8a7f55ce9,
# and the wasGeneratedBy genesis shape), never a hand-invented schema. Git
# fixtures use the REAL auto-bind subject format.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/verification-economics-report.sh"
    TMP="$BATS_TEST_TMPDIR/ve"
    mkdir -p "$TMP"
}

# 4 verdict edges (3 CONFIRMED / 1 REFUTED; families gpt,gpt,claude+gpt,gpt;
# months 2026-06 x1 CONFIRMED, 2026-07 x 2 CONFIRMED + 1 REFUTED) plus 2
# non-verdict records that must be filtered out.
make_ledger() {
    cat > "$1" <<'JSONL'
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-8jf97","from_type":"bead","to_id":"feat/ag-8jf97-provenance-ledger","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"_beads/issues.jsonl#ag-8jf97","trust_tier":"authored","ts":"2026-06-13T23:55:06Z","prev_hash":"","payload_hash":"p0","hash":"h0"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fix-a@aaaaaaa","from_type":"verdict","to_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-fix-a disposition=CONFIRMED","bead_id":"age-fix-a","trust_tier":"inferred","ts":"2026-06-20T10:00:00Z","reviewer_family":"gpt","evidence_path":"/tmp/pawl-evidence/age-fix-a-codex.txt","prev_hash":"h0","payload_hash":"p1","hash":"h1"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fix-b@bbbbbbb","from_type":"verdict","to_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-fix-b disposition=CONFIRMED","bead_id":"age-fix-b","trust_tier":"inferred","ts":"2026-07-01T11:00:00Z","reviewer_family":"gpt","evidence_path":"/tmp/pawl-evidence/age-fix-b-codex.txt","prev_hash":"h1","payload_hash":"p2","hash":"h2"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fix-c@ccccccc","from_type":"verdict","to_id":"cccccccccccccccccccccccccccccccccccccccc","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-fix-c disposition=CONFIRMED","bead_id":"age-fix-c","trust_tier":"inferred","ts":"2026-07-02T12:00:00Z","reviewer_family":"claude+gpt","evidence_path":"/tmp/pawl-evidence/age-fix-c-opus.txt","prev_hash":"h2","payload_hash":"p3","hash":"h3"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fix-d@ddddddd","from_type":"verdict","to_id":"dddddddddddddddddddddddddddddddddddddddd","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict age-fix-d disposition=REFUTED","bead_id":"age-fix-d","trust_tier":"inferred","ts":"2026-07-03T13:00:00Z","reviewer_family":"gpt","evidence_path":"/tmp/pawl-evidence/age-fix-d-codex.txt","prev_hash":"h3","payload_hash":"p4","hash":"h4"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"age-fix-a","from_type":"bead","to_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"_beads/issues.jsonl#age-fix-a","trust_tier":"authored","ts":"2026-07-03T14:00:00Z","prev_hash":"h4","payload_hash":"p5","hash":"h5"}
JSONL
}

# Real auto-bind subject shape: 2 CONFIRMED + 1 REFUTED + 1 unrelated commit.
make_repo() {
    git init -q "$1"
    git -C "$1" config user.email t@t.local
    git -C "$1" config user.name t
    git -C "$1" commit --allow-empty -qm "chore(provenance): bind pawl CONFIRMED verdict for age-fix-a #trivial"
    git -C "$1" commit --allow-empty -qm "feat(x): unrelated work (age-fix-z)"
    git -C "$1" commit --allow-empty -qm "chore(provenance): bind pawl REFUTED verdict for age-fix-d #trivial"
    git -C "$1" commit --allow-empty -qm "chore(provenance): bind pawl CONFIRMED verdict for age-fix-b #trivial"
}

@test "reports ledger verdict totals, refute rate, family and month breakdowns" {
    make_ledger "$TMP/ledger.jsonl"
    make_repo "$TMP/repo"
    run bash "$SCRIPT" --repo "$TMP/repo" --ledger "$TMP/ledger.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"verdict edges: 4 (CONFIRMED: 3 / REFUTED: 1)"* ]]
    [[ "$output" == *"refute rate: 25.0% (1/4)"* ]]
    [[ "$output" == *"gpt: 3"* ]]
    [[ "$output" == *"claude+gpt: 1"* ]]
    [[ "$output" == *"2026-07  2/1"* ]]
    [[ "$output" == *"UNMEASURED"* ]]
    [[ "$output" == *"age-verification-economics-ebec.1"* ]]
}

@test "counts git verdict-bind subjects as a volume cross-check" {
    make_ledger "$TMP/ledger.jsonl"
    make_repo "$TMP/repo"
    run bash "$SCRIPT" --repo "$TMP/repo" --ledger "$TMP/ledger.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"git binds: 3 total (2 CONFIRMED / 1 REFUTED / 0 other)"* ]]
}

@test "--json emits machine-readable exact counts" {
    make_ledger "$TMP/ledger.jsonl"
    make_repo "$TMP/repo"
    run bash "$SCRIPT" --repo "$TMP/repo" --ledger "$TMP/ledger.jsonl" --json
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.ledger.verdicts == 4 and .ledger.confirmed == 3 and .ledger.refuted == 1'
    echo "$output" | jq -e '.ledger.refute_rate == 0.25'
    echo "$output" | jq -e '.ledger.by_family.gpt == 3 and .ledger.by_family["claude+gpt"] == 1'
    echo "$output" | jq -e '.ledger.by_month["2026-07"].confirmed == 2 and .ledger.by_month["2026-07"].refuted == 1'
    echo "$output" | jq -e '.git_binds.total == 3 and .git_binds.other == 0'
    echo "$output" | jq -e '.cost.status == "UNMEASURED" and .cost.meter_bead == "age-verification-economics-ebec.1"'
}

@test "dead instrument: zero verdict edges in ledger fails closed" {
    printf '%s\n' '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-8jf97","from_type":"bead","to_id":"feat/x","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"_beads/issues.jsonl#ag-8jf97","trust_tier":"authored","ts":"2026-06-13T23:55:06Z","prev_hash":"","payload_hash":"p0","hash":"h0"}' > "$TMP/noverdicts.jsonl"
    make_repo "$TMP/repo2"
    run bash "$SCRIPT" --repo "$TMP/repo2" --ledger "$TMP/noverdicts.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"dead instrument"* ]]
}

@test "missing ledger fails with remedy" {
    make_repo "$TMP/repo3"
    run bash "$SCRIPT" --repo "$TMP/repo3" --ledger "$TMP/does-not-exist.jsonl"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
}
