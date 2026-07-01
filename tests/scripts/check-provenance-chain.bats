#!/usr/bin/env bats
#
# Behavioral test for the provenance.chain local gate
# (age-gate-the-ungated-egwt.9), scripts/check-provenance-chain.sh + its backing
# `ao provenance verify`. Asserts the tamper-detection windshield actually fires:
#   - the real gate PASSES against the repo's own committed ledger (wired + green);
#   - a HEALTHY fixture chain (a real slice from HEAD's ledger) verifies clean (exit 0);
#   - a TAMPERED fixture (one flipped hash byte) is CAUGHT (non-zero) and names the entry;
#   - a MISSING ledger is an intact empty chain (exit 0) — a fresh clone must not fail;
#   - a healthy run finishes fast (linear JSONL scan, no rebuilds beyond the binary).
#
# FIXTURE FIDELITY: the healthy fixture is a real slice copied from HEAD's
# committed ledger (which begins at the chain root, prev_hash=""), so it is a
# genuine chain the production writer emitted — never a hand-built shape.

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
    GATE="$ROOT/scripts/check-provenance-chain.sh"
    LEDGER="$ROOT/docs/provenance/ledger.jsonl"
}

# Build a fixture repo root (git-marked so ao's resolveLedgerPath finds it) with
# a HEALTHY 3-record chain copied from the head of the real committed ledger.
_make_healthy_fixture() {
    local dir="$1"
    ( cd "$dir" && git init -q . )
    mkdir -p "$dir/docs/provenance"
    head -3 "$LEDGER" > "$dir/docs/provenance/ledger.jsonl"
}

@test "real gate passes against the repo's committed ledger" {
    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"chain intact"* ]]
    [[ "$output" == *"PROVENANCE_CHAIN_GATE: OK"* ]]
}

@test "healthy fixture chain (real slice from HEAD) verifies clean" {
    tmpdir="$(mktemp -d)"
    _make_healthy_fixture "$tmpdir"
    run bash -c "cd '$tmpdir' && '$AO_BIN' provenance verify"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK:"* ]]
    [[ "$output" == *"3 record"* ]]
    rm -rf "$tmpdir"
}

@test "tampered fixture (flipped hash byte) is caught and names the entry" {
    tmpdir="$(mktemp -d)"
    _make_healthy_fixture "$tmpdir"
    # Flip one character in line 2's hash without recomputing — breaks both its
    # self-hash and line 3's prev_hash link.
    python3 - "$tmpdir/docs/provenance/ledger.jsonl" <<'PY'
import json, sys
p = sys.argv[1]
lines = open(p).read().rstrip("\n").split("\n")
e = json.loads(lines[1])
h = list(e["hash"])
h[0] = "0" if h[0] != "0" else "1"
e["hash"] = "".join(h)
lines[1] = json.dumps(e)
open(p, "w").write("\n".join(lines) + "\n")
PY
    run bash -c "cd '$tmpdir' && '$AO_BIN' provenance verify"
    [ "$status" -ne 0 ]
    [[ "$output" == *"BROKEN"* ]]
    [[ "$output" == *"line 2"* ]]
    rm -rf "$tmpdir"
}

@test "missing ledger is an intact empty chain (fresh clone must not fail)" {
    tmpdir="$(mktemp -d)"
    ( cd "$tmpdir" && git init -q . )
    # No docs/provenance/ledger.jsonl at all.
    run bash -c "cd '$tmpdir' && '$AO_BIN' provenance verify"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 record"* ]]
    rm -rf "$tmpdir"
}

@test "healthy gate run finishes fast" {
    start="$(date +%s)"
    run bash "$GATE"
    end="$(date +%s)"
    [ "$status" -eq 0 ]
    # Generous ceiling: the scan is linear and the binary is pre-built here.
    [ "$((end - start))" -le 10 ]
}
