#!/usr/bin/env bats
# L2 tests for scripts/efc-power-gate.py — the read-only EFC-transfer power-gate
# readiness check (age-k2w). Asserts the frozen §55 gate: >=50 runs AND >=15 in the
# DECOUPLED failure class (escapes = CONFIRMED-then-higher-attempt-REFUTED, not
# disposition). The verdict must be INCONCLUSIVE until BOTH thresholds are met.

setup() {
    SCRIPT="$(git rev-parse --show-toplevel)/scripts/efc-power-gate.py"
    TMP="$(mktemp -d)"
    LEDGER="$TMP/yield.jsonl"
}

teardown() { rm -rf "$TMP"; }

# gen_ledger <clean_runs> <escaped_beads> > ledger
# clean_runs: that many runs, each a single CONFIRMED bead (no escape).
# escaped_beads: that many beads, each CONFIRMED@1 then REFUTED@2 (an escape).
gen_ledger() {
    python3 - "$1" "$2" <<'PY'
import json, sys
clean, escaped = int(sys.argv[1]), int(sys.argv[2])
def gv(run, bead, disp, attempt):
    return json.dumps({"event": "gate-verdict", "run_id": run, "bead_id": bead,
                       "ts": "2026-06-22T00:00:00Z",
                       "body": {"disposition": disp, "attempt": attempt}})
out = []
for i in range(clean):
    out.append(gv(f"run-c{i}", f"clean-{i}", "CONFIRMED", 1))
for i in range(escaped):
    out.append(gv(f"run-e{i}", f"esc-{i}", "CONFIRMED", 1))
    out.append(gv(f"run-e{i}", f"esc-{i}", "REFUTED", 2))
print("\n".join(out))
PY
}

@test "INCONCLUSIVE when failure class is too small (N met, escapes < 15)" {
    gen_ledger 60 3 > "$LEDGER"   # 60+3 = 63 runs (>=50) but only 3 escapes
    run python3 "$SCRIPT" --ledger "$LEDGER"
    [ "$status" -eq 2 ]
    [[ "$output" == *"INCONCLUSIVE"* ]]
    [[ "$output" == *"decoupled-failures"* ]]
}

@test "INCONCLUSIVE when too few runs (escapes met, N < 50)" {
    gen_ledger 5 16 > "$LEDGER"   # 16 escapes (>=15) but only 21 runs (<50)
    run python3 "$SCRIPT" --ledger "$LEDGER"
    [ "$status" -eq 2 ]
    [[ "$output" == *"INCONCLUSIVE"* ]]
    [[ "$output" == *"runs 21/50"* ]]
}

@test "RUNNABLE when both thresholds are met (>=50 runs, >=15 escapes)" {
    gen_ledger 50 16 > "$LEDGER"   # 66 runs, 16 escapes -> gate met
    run python3 "$SCRIPT" --ledger "$LEDGER"
    [ "$status" -eq 0 ]
    [[ "$output" == *"RUNNABLE"* ]]
    [[ "$output" == *"may now be built/run"* ]]
}

@test "escape requires a HIGHER-attempt REFUTED, not just any REFUTED (no circularity via rework)" {
    # REFUTED@1 then CONFIRMED@2 is normal rework, NOT an escape -> not a decoupled failure.
    python3 - "$LEDGER" <<'PY'
import json, sys
out = []
for i in range(60):
    out.append(json.dumps({"event":"gate-verdict","run_id":f"r{i}","bead_id":f"b{i}",
                           "ts":"t","body":{"disposition":"CONFIRMED","attempt":1}}))
for i in range(20):  # rework, not escapes
    out += [json.dumps({"event":"gate-verdict","run_id":f"rw{i}","bead_id":f"rw{i}",
                        "ts":"t","body":{"disposition":"REFUTED","attempt":1}}),
            json.dumps({"event":"gate-verdict","run_id":f"rw{i}","bead_id":f"rw{i}",
                        "ts":"t","body":{"disposition":"CONFIRMED","attempt":2}})]
open(sys.argv[1],"w").write("\n".join(out))
PY
    run python3 "$SCRIPT" --ledger "$LEDGER"
    [ "$status" -eq 2 ]   # 80 runs but 0 escapes (rework isn't a failure) -> INCONCLUSIVE
    [[ "$output" == *"0 escapes"* ]]
}

@test "missing ledger errors cleanly (exit 1)" {
    run python3 "$SCRIPT" --ledger "$TMP/does-not-exist.jsonl"
    [ "$status" -eq 1 ]
}
