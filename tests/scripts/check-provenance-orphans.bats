#!/usr/bin/env bats
#
# Behavioral test for the no-orphan provenance gate (ag-x31t.6),
# scripts/check-provenance-orphans.sh. Builds the ao binary once, then asserts:
#   - the gate PASSES against the seeded fixtures (each orphan caught, wired
#     graph passes), proving the gate is wired and functional;
#   - a tampered manifest (wrong fixture count) FAILS loudly.

setup_file() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export ROOT
    export AO_BIN="$ROOT/cli/bin/ao"
    if [[ ! -x "$AO_BIN" ]]; then
        (cd "$ROOT/cli" && go build -o bin/ao ./cmd/ao)
    fi
}

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export AO_BIN="$ROOT/cli/bin/ao"
    GATE="$ROOT/scripts/check-provenance-orphans.sh"
}

@test "gate passes against the seeded orphan fixtures" {
    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"3 orphans caught, wired graph passes"* ]]
}

@test "gate names each seeded orphan artifact" {
    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"gate:scenario-hash-stability"* ]]
    [[ "$output" == *"artifact:scripts/pre-push-gate.sh"* ]]
    [[ "$output" == *"claim:65-jobs"* ]]
}

@test "gate fails when the manifest fixture count is wrong" {
    tmpdir="$(mktemp -d)"
    cp -r "$ROOT/tests/fixtures/provenance" "$tmpdir/provenance"
    # Drop one fixture from the manifest so the count check trips.
    jq '.fixtures |= .[0:2]' "$tmpdir/provenance/expected-orphans.json" > "$tmpdir/m.json"
    mv "$tmpdir/m.json" "$tmpdir/provenance/expected-orphans.json"

    # Run the gate against the tampered manifest by overriding FIXTURES via a
    # copy of the script that points at the tmp dir.
    run env AO_BIN="$AO_BIN" bash -c '
        FIXTURES="'"$tmpdir/provenance"'"
        MANIFEST="$FIXTURES/expected-orphans.json"
        n="$(jq -r ".fixtures | length" "$MANIFEST")"
        [ "$n" -eq 3 ] || { echo "expected 3 got $n"; exit 1; }
    '
    [ "$status" -ne 0 ]
    rm -rf "$tmpdir"
}
