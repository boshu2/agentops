#!/usr/bin/env bash
# test-aggregate-observation-log.sh — L1 + L2 tests for
# scripts/aggregate-observation-log.sh.
#
# Bead: soc-ejq2 (PR-D, Wave 1G of epic soc-xlw8).
#
# L1 unit tests (jq logic, exercised via the script with --dry-run + fixture
# inputs):
#   * dedup by run_id collapses duplicates
#   * schema validation rejects null run_id
#   * push-to-main observations (pr_number=null) get merged_anyway=false and
#     ledger_updated=false
# L2 integration:
#   * --dry-run against the good fixtures emits expected summary
#   * full (non-dry-run) write produces a parseable JSONL with the backfilled
#     fields and idempotent re-runs
#
# The aggregator is invoked in fixture mode (AGGREGATE_OBS_FIXTURE_DIR) which
# bypasses `gh` entirely.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/aggregate-observation-log.sh"
FIXTURES="$ROOT/tests/fixtures/observation-log"

if [ ! -x "$SCRIPT" ]; then
    echo "FAIL: aggregator not executable at $SCRIPT" >&2
    exit 1
fi

PASS=0
FAIL=0
TMP_BASE="$(mktemp -d -t test-aggregate-observation-log.XXXXXX)"
trap 'rm -rf "$TMP_BASE"' EXIT

ok()   { PASS=$((PASS + 1)); echo "  PASS  $*"; }
nope() { FAIL=$((FAIL + 1)); echo "  FAIL  $*" >&2; }

# -----------------------------------------------------------------------------
echo "Test 1: --help exits 0 with usage"
if out="$(bash "$SCRIPT" --help 2>&1)" && grep -q "Usage:" <<<"$out"; then
    ok "help text printed"
else
    nope "help missing or non-zero exit"
fi

# -----------------------------------------------------------------------------
echo "Test 2: rejects malformed observations (null run_id)"
F2="$(mktemp -d -p "$TMP_BASE" malformed.XXXXXX)"
cp "$FIXTURES/obs-good-1.json" "$F2/"
cp "$FIXTURES/obs-malformed-null-runid.json" "$F2/"
if AGGREGATE_OBS_FIXTURE_DIR="$F2" bash "$SCRIPT" --dry-run \
        > "$TMP_BASE/t2.out" 2> "$TMP_BASE/t2.err"; then
    nope "expected non-zero exit on malformed observation, got 0"
else
    rc=$?
    if [ "$rc" -eq 2 ] && grep -q "malformed observation" "$TMP_BASE/t2.err"; then
        ok "rc=2 with diagnostic on null run_id"
    else
        nope "expected rc=2 with 'malformed observation' diag, got rc=$rc"
        cat "$TMP_BASE/t2.err" >&2 || true
    fi
fi

# -----------------------------------------------------------------------------
echo "Test 3: empty fixture dir -> empty-but-valid output, exit 0"
F3="$(mktemp -d -p "$TMP_BASE" empty.XXXXXX)"
SANDBOX3="$(mktemp -d -p "$TMP_BASE" sb3.XXXXXX)"
(
    cd "$SANDBOX3"
    mkdir -p .agents/reconcile scripts
    cp "$SCRIPT" scripts/aggregate-observation-log.sh
    AGGREGATE_OBS_FIXTURE_DIR="$F3" bash scripts/aggregate-observation-log.sh \
        > "$TMP_BASE/t3.out" 2> "$TMP_BASE/t3.err"
)
if [ -f "$SANDBOX3/.agents/reconcile/observation-log.jsonl" ] \
        && [ ! -s "$SANDBOX3/.agents/reconcile/observation-log.jsonl" ]; then
    ok "empty input produced empty-but-valid log"
else
    nope "expected empty file at .agents/reconcile/observation-log.jsonl"
    ls -la "$SANDBOX3/.agents/reconcile/" >&2 || true
fi

# -----------------------------------------------------------------------------
echo "Test 4: dedup on run_id collapses duplicates"
F4="$(mktemp -d -p "$TMP_BASE" dedup.XXXXXX)"
cp "$FIXTURES/obs-good-1.json" "$F4/"
cp "$FIXTURES/obs-duplicate.json" "$F4/"
cp "$FIXTURES/obs-good-2.json" "$F4/"
SANDBOX4="$(mktemp -d -p "$TMP_BASE" sb4.XXXXXX)"
(
    cd "$SANDBOX4"
    mkdir -p .agents/reconcile scripts
    cp "$SCRIPT" scripts/aggregate-observation-log.sh
    AGGREGATE_OBS_FIXTURE_DIR="$F4" bash scripts/aggregate-observation-log.sh \
        > "$TMP_BASE/t4.out" 2> "$TMP_BASE/t4.err"
)
LOG4="$SANDBOX4/.agents/reconcile/observation-log.jsonl"
LINES4="$(wc -l < "$LOG4" | tr -d ' ')"
if [ "$LINES4" = "2" ]; then
    ok "3 inputs -> 2 unique run_ids"
else
    nope "expected 2 lines after dedup, got $LINES4"
    cat "$LOG4" >&2 || true
fi

# Verify the two run_ids are 1234567890 and 1234567891.
RUN_IDS4="$(jq -r '.run_id' "$LOG4" | sort | tr '\n' ',' )"
if [ "$RUN_IDS4" = "1234567890,1234567891," ]; then
    ok "preserved both unique run_ids"
else
    nope "wrong run_id set: $RUN_IDS4"
fi

# -----------------------------------------------------------------------------
echo "Test 5: push-to-main (pr_number=null) sets merged_anyway+ledger_updated to false"
LINE5_NULL="$(jq -c 'select(.pr_number == null)' "$LOG4")"
M5="$(jq -r '.merged_anyway' <<<"$LINE5_NULL")"
L5="$(jq -r '.ledger_updated' <<<"$LINE5_NULL")"
if [ "$M5" = "false" ] && [ "$L5" = "false" ]; then
    ok "null-pr observation has both fields false"
else
    nope "expected false/false on null-pr; got merged_anyway=$M5 ledger_updated=$L5"
fi

# -----------------------------------------------------------------------------
echo "Test 6: PR observation also has both backfill fields (false in fixture mode)"
LINE6_PR="$(jq -c 'select(.pr_number != null)' "$LOG4")"
M6="$(jq -r '.merged_anyway' <<<"$LINE6_PR")"
L6="$(jq -r '.ledger_updated' <<<"$LINE6_PR")"
if [ "$M6" = "false" ] && [ "$L6" = "false" ]; then
    ok "pr-bearing observation has both fields backfilled (false in fixture mode)"
else
    nope "expected false/false in fixture mode; got merged_anyway=$M6 ledger_updated=$L6"
fi

# -----------------------------------------------------------------------------
echo "Test 7: --dry-run does NOT write file"
F7="$(mktemp -d -p "$TMP_BASE" dryrun.XXXXXX)"
cp "$FIXTURES/obs-good-1.json" "$F7/"
SANDBOX7="$(mktemp -d -p "$TMP_BASE" sb7.XXXXXX)"
(
    cd "$SANDBOX7"
    mkdir -p .agents/reconcile scripts
    cp "$SCRIPT" scripts/aggregate-observation-log.sh
    AGGREGATE_OBS_FIXTURE_DIR="$F7" bash scripts/aggregate-observation-log.sh --dry-run \
        > "$TMP_BASE/t7.out" 2> "$TMP_BASE/t7.err"
)
if [ ! -e "$SANDBOX7/.agents/reconcile/observation-log.jsonl" ] \
        && grep -q "dry-run" "$TMP_BASE/t7.out"; then
    ok "dry-run skipped write and printed summary"
else
    nope "dry-run should not have written the file"
    ls -la "$SANDBOX7/.agents/reconcile/" >&2 || true
fi

# -----------------------------------------------------------------------------
echo "Test 8: idempotent re-run produces identical output"
SANDBOX8="$(mktemp -d -p "$TMP_BASE" sb8.XXXXXX)"
(
    cd "$SANDBOX8"
    mkdir -p .agents/reconcile scripts
    cp "$SCRIPT" scripts/aggregate-observation-log.sh
    AGGREGATE_OBS_FIXTURE_DIR="$F4" bash scripts/aggregate-observation-log.sh \
        > /dev/null 2> /dev/null
    cp .agents/reconcile/observation-log.jsonl /tmp/.t8.first
    AGGREGATE_OBS_FIXTURE_DIR="$F4" bash scripts/aggregate-observation-log.sh \
        > /dev/null 2> /dev/null
    cp .agents/reconcile/observation-log.jsonl /tmp/.t8.second
)
if cmp -s /tmp/.t8.first /tmp/.t8.second; then
    ok "second run produced byte-identical output"
else
    nope "second run differed from first"
    diff /tmp/.t8.first /tmp/.t8.second >&2 || true
fi
rm -f /tmp/.t8.first /tmp/.t8.second

# -----------------------------------------------------------------------------
echo
echo "Summary: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
