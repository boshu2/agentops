#!/usr/bin/env bash
# check-skill-probe-headroom.sh — ADVISORY skill.probe-headroom gate.
#
# WHY: `evals/skill-probes/LEDGER.md` reports 0/12 product/judgment skills
# measured, and the reason is not that the probes were never run — it is that
# the probes that DID run came back INERT with no way to tell which kind of
# INERT it was. `INERT` compares the two arms and says they matched. It cannot
# say whether the skill changed nothing (a real null, worth a ledger row) or
# whether the control arm already aced the scenario so there was nothing left
# to measure (a void row: the measurement failed, not the skill). Only the
# CONTROL arm's absolute rate separates the two, and re-running a saturated
# scenario at a lower effort produces ledger rows instead of knowledge.
#
# WHAT it checks (two parts):
#
#   1. HERMETIC SELF-TEST (blocking within this script). Two committed
#      scorecard pairs under tests/fixtures/skill-probes/ are driven through
#      cli/cmd/probe-headroom. Both pairs carry the SAME probe verdict
#      (INERT), so nothing that reads only the verdict can separate them:
#        * saturated/ — control aces two effort levels -> must be SATURATED
#        * headroom/  — control scores 0.50            -> must be SEPARATED
#      If the detector stops discriminating, this script fails. That is the
#      RED-fixture flip, re-run on every invocation instead of asserted once.
#
#   2. ADVISORY SWEEP. Every probe group under docs/evals/scorecards/ is
#      classified and SATURATED groups are NAMED. This never fails the run:
#      a saturated historical group is a true finding about the ledger, not a
#      regression introduced by the change under test.
#
# HONESTY: headroom is not quality. A SEPARATED classification says the
# scenario left room to measure in, nothing about whether the skill is good.
# A SATURATED classification says the row is void, nothing about whether the
# skill works — the honest reading is "at this altitude the behavior is native
# to the model", which is itself a result and belongs in the ledger.
#
# ADVISORY-FIRST: registered Blocking:false in cli/internal/gates so it
# surfaces as WARN. The Blocking:false -> true flip is made later, deliberately,
# on measured evidence — never on a calendar.
#
# Usage:
#   bash scripts/check-skill-probe-headroom.sh              # self-test + sweep
#   bash scripts/check-skill-probe-headroom.sh --no-scan    # self-test only
#
# Env overrides (test seams):
#   PROBE_HEADROOM_BIN         pre-built helper binary (default: build from source)
#   PROBE_HEADROOM_FIXTURES    fixture root (default: $REPO_ROOT/tests/fixtures/skill-probes)
#   PROBE_HEADROOM_SCORECARDS  sweep root (default: $REPO_ROOT/docs/evals/scorecards)
#
# Exit: 0 = detector discriminates (sweep findings are advisory)
#       1 = the detector is blind (a fixture pair classified wrong)
#       2 = misuse / unreadable input
#
# practices: [continuous-integration, measurement-over-assertion]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

FIXTURES="${PROBE_HEADROOM_FIXTURES:-$REPO_ROOT/tests/fixtures/skill-probes}"
SCORECARDS="${PROBE_HEADROOM_SCORECARDS:-$REPO_ROOT/docs/evals/scorecards}"
SCAN=1

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --no-scan) SCAN=0; shift;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

fail() {
    echo "PROBE_HEADROOM_GATE: $*" >&2
    exit 1
}

for dir in "$FIXTURES/saturated" "$FIXTURES/headroom"; do
    [[ -d "$dir" ]] || { echo "fixture pair missing: $dir" >&2; exit 2; }
done

# Resolve the helper: prefer a pre-built binary (CI may build it once), else
# build from source into a temp dir. Hermetic — no model dispatch, no network.
HELPER="${PROBE_HEADROOM_BIN:-}"
if [[ -z "$HELPER" ]]; then
    if [[ -x "$REPO_ROOT/cli/bin/probe-headroom" ]]; then
        HELPER="$REPO_ROOT/cli/bin/probe-headroom"
    else
        _phbin=""
        with_tmpdir _phbin probe-headroom   # assigns $_phbin via printf -v
        HELPER="$_phbin/probe-headroom"
        ( cd "$REPO_ROOT/cli" && go build -o "$HELPER" ./cmd/probe-headroom ) \
            || { echo "could not build probe-headroom helper" >&2; exit 2; }
    fi
fi

echo "=== skill probe headroom gate (advisory) ==="

# classify_pair DIR -> prints the helper output, sets PAIR_RC to its exit code.
classify_pair() {
    local dir="$1" rc=0 out
    set +e
    out="$("$HELPER" "$dir"/*.json 2>&1)"
    rc=$?
    set -e
    PAIR_OUT="$out"
    PAIR_RC="$rc"
}

# 1. The saturated pair must be FLAGGED (exit 3, classification SATURATED).
classify_pair "$FIXTURES/saturated"
if [[ "$PAIR_RC" -ne 3 ]]; then
    fail "saturated fixture pair was NOT flagged (exit $PAIR_RC, want 3) — the detector is blind: $PAIR_OUT"
fi
grep -qF "PROBE_HEADROOM: SATURATED" <<<"$PAIR_OUT" \
    || fail "saturated fixture pair did not report SATURATED: $PAIR_OUT"
echo "  saturated pair: flagged (exit $PAIR_RC)"

# 2. The headroom pair must NOT be flagged (exit 0, classification SEPARATED).
#    Both pairs are INERT, so a detector that reads the verdict fails here.
classify_pair "$FIXTURES/headroom"
if [[ "$PAIR_RC" -ne 0 ]]; then
    fail "INERT-with-headroom fixture pair WAS flagged (exit $PAIR_RC, want 0) — an honest null is not a saturated row: $PAIR_OUT"
fi
grep -qF "PROBE_HEADROOM: SEPARATED" <<<"$PAIR_OUT" \
    || fail "headroom fixture pair did not report SEPARATED: $PAIR_OUT"
echo "  headroom pair:  not flagged (exit $PAIR_RC)"

if [[ "$SCAN" -eq 1 && -d "$SCORECARDS" ]]; then
    echo "--- advisory sweep: $SCORECARDS ---"
    if ! "$HELPER" --scan "$SCORECARDS"; then
        echo "PROBE_HEADROOM_GATE: sweep could not read $SCORECARDS" >&2
        exit 2
    fi
fi

echo "PROBE_HEADROOM_GATE: OK (saturated pair flagged, INERT-with-headroom pair not)"
exit 0
