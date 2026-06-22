#!/usr/bin/env bats
# Tests for scripts/ingest-ci-deterministic-catches.sh (i4m, part 2: CI direct-persist).
# Hooks don't fire in CI, so CI's go-gate-shadow emits a deterministic catch and
# uploads the (gitignored, local) yield ledger as an artifact. This script folds
# that artifact into a maintainer's LOCAL gauge ledger — no commit-back.
#
# L2: the green case builds the artifact via the REAL production emitter
# (emit-deterministic-catch.sh) and drives the REAL ao binary end-to-end (ingest
# -> ao yield gauge counts the catch), proving CI catches reach the gauge.

setup_file() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    if [ -n "${AO_TEST_BIN:-}" ] && [ -x "${AO_TEST_BIN}" ]; then
        AO_BIN_RESOLVED="$AO_TEST_BIN"
    elif [ -x "$REPO_ROOT/cli/bin/ao" ]; then
        AO_BIN_RESOLVED="$REPO_ROOT/cli/bin/ao"
    else
        AO_BIN_RESOLVED="$BATS_FILE_TMPDIR/ao"
        ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN_RESOLVED" ./cmd/ao ) || AO_BIN_RESOLVED=""
    fi
    export REPO_ROOT AO_BIN_RESOLVED
}

setup() {
    INGEST="$REPO_ROOT/scripts/ingest-ci-deterministic-catches.sh"
    EMIT="$REPO_ROOT/scripts/emit-deterministic-catch.sh"
    # Local maintainer project (its own git repo + local yield ledger).
    LOCAL="$BATS_TEST_TMPDIR/local"
    mkdir -p "$LOCAL"
    ( cd "$LOCAL" && git init -q && git config user.email t@t && git config user.name t \
        && echo seed > seed.txt && git add . && git commit -qm "seed ag-local1 work" )
    LEDGER="$LOCAL/.agents/yield/yield-ledger.jsonl"
    # CI "artifact" dir: a separate repo where the production emitter wrote a
    # deterministic catch (fixture fidelity — real emit, not a hand-built line).
    ART="$BATS_TEST_TMPDIR/artifact"
    mkdir -p "$ART"
    ( cd "$ART" && git init -q && git config user.email t@t && git config user.name t \
        && echo x > x.txt && git add . && git commit -qm "ci change ag-ci1 work" )
}

emit_ci_catch() {  # emit a deterministic catch into the artifact dir (real emitter)
    bash -c "cd '$ART' && AO_BIN='$AO_BIN_RESOLVED' AO_YIELD_RUN_ID=ci-deterministic bash '$EMIT'" >/dev/null
}

@test "script exists and is executable" {
    [ -f "$INGEST" ]
    [ -x "$INGEST" ]
}

@test "green: a CI catch artifact ingests into the local gauge (catch_rate=1)" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    emit_ci_catch
    [ -f "$ART/.agents/yield/yield-ledger.jsonl" ]
    run env AO_BIN="$AO_BIN_RESOLVED" bash "$INGEST" "$ART" --ledger "$LEDGER"
    [ "$status" -eq 0 ]
    [[ "$output" == *"1 new"* ]]
    grep -q '"event":"gate-verdict"' "$LEDGER"
    grep -q 'deterministic' "$LEDGER"
    # The maintainer's gauge now counts the CI catch.
    run bash -c "cd '$LOCAL' && '$AO_BIN_RESOLVED' yield gauge --run ci-deterministic --json"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"catch_rate": 1'* ]]
}

@test "idempotent: re-ingesting the same artifact does not double-count" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    emit_ci_catch
    env AO_BIN="$AO_BIN_RESOLVED" bash "$INGEST" "$ART" --ledger "$LEDGER" >/dev/null
    before="$(wc -l < "$LEDGER")"
    run env AO_BIN="$AO_BIN_RESOLVED" bash "$INGEST" "$ART" --ledger "$LEDGER"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 new"* ]]
    after="$(wc -l < "$LEDGER")"
    [ "$before" -eq "$after" ]
}

@test "ignores non-deterministic / non-gate-verdict lines in an artifact" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    mkdir -p "$ART/.agents/yield"
    printf '%s\n' '{"event":"other","run_id":"x"}' \
        '{"event":"gate-verdict","run_id":"r","ts":"t","body":{"head_sha":"abc","mode":"multi-model","disposition":"CONFIRMED"}}' \
        > "$ART/.agents/yield/yield-ledger.jsonl"
    run env AO_BIN="$AO_BIN_RESOLVED" bash "$INGEST" "$ART" --ledger "$LEDGER"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 new"* ]]
    [ ! -s "$LEDGER" ]
}

@test "soundness: a malformed deterministic line is REJECTED, never corrupting the gauge ledger" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    # A line that PASSES the structural pre-filter (deterministic + REFUTED +
    # head_sha + run_id) but has an UNKNOWN field the closed-schema reader rejects.
    # The ingest must refuse the batch and leave the local ledger loadable.
    mkdir -p "$ART/.agents/yield"
    printf '%s\n' \
        '{"event":"gate-verdict","bead_id":"ag-x","run_id":"ci-deterministic","ts":"2026-06-22T00:00:00Z","body":{"difficulty":1,"disposition":"REFUTED","head_sha":"abc1234","attempt":1,"mode":"deterministic","author_context_id":"pre-push-gate","refuter_families":[],"author_family":"deterministic-gate","cross_family":false,"author_ne_reviewer":true,"evidence_present":true,"BOGUS_UNKNOWN_FIELD":"x"}}' \
        > "$ART/.agents/yield/yield-ledger.jsonl"
    run env AO_BIN="$AO_BIN_RESOLVED" bash "$INGEST" "$ART" --ledger "$LEDGER"
    [ "$status" -ne 0 ]
    [[ "$output" == *"refusing to append"* ]]
    # ledger must remain empty + still loadable by the gauge (not corrupted).
    [ ! -s "$LEDGER" ]
    run bash -c "cd '$LOCAL' && '$AO_BIN_RESOLVED' yield gauge --run ci-deterministic --json"
    [ "$status" -eq 0 ]
}
