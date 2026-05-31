#!/usr/bin/env bash
# check-provenance-orphans.sh — no-orphan provenance gate (ag-x31t.6).
#
# Drives the `ao provenance trace --orphans --strict` audit (which generalizes
# goals_trace_orphans onto the provenance graph) against the seeded orphan
# fixtures and asserts the gate behaves correctly:
#
#   1. Each failing-by-design fixture in tests/fixtures/provenance/ has one
#      engineered artifact node with NO inbound authored/inferred edge, so the
#      gate MUST exit non-zero and name that orphan (per expected-orphans.json).
#   2. The same graph, once an inbound edge wires the orphan back to a
#      directive, MUST exit zero — the gate flips green when provenance lands.
#
# This is the CI surface of the no-orphan gate: it proves the detector is wired
# and functional. The committed fixtures are failing-by-design demonstrations of
# the three audit gaps the epic (ag-x31t) closes (phantom scenario-hash-stability
# gate, retired-but-present pre-push-gate.sh, stale "65 jobs"), so the gate runs
# them through the audit rather than letting them fail an unconditional run.
#
# Bounded-context: BC4-Factory. Evidence: .github/workflows/validate.yml.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURES="$ROOT/tests/fixtures/provenance"
MANIFEST="$FIXTURES/expected-orphans.json"

# Resolve the ao binary: prefer the freshly-built CI binary, fall back to PATH.
AO="${AO_BIN:-}"
if [[ -z "$AO" ]]; then
    if [[ -x "$ROOT/cli/bin/ao" ]]; then
        AO="$ROOT/cli/bin/ao"
    else
        AO="ao"
    fi
fi

fail() {
    echo "PROVENANCE_ORPHAN_GATE: $*" >&2
    exit 1
}

command -v jq >/dev/null 2>&1 || fail "jq is required"
[[ -f "$MANIFEST" ]] || fail "manifest missing: $MANIFEST"

echo "=== no-orphan provenance gate (ao provenance trace --orphans --strict) ==="

# 1. Each seeded fixture must be CAUGHT by the strict gate (non-zero exit) and
#    name its declared orphan artifact id.
mapfile -t files < <(jq -r '.fixtures[].file' "$MANIFEST")
[[ ${#files[@]} -eq 3 ]] || fail "expected 3 seeded fixtures, got ${#files[@]}"

for f in "${files[@]}"; do
    graph="$FIXTURES/$f"
    [[ -f "$graph" ]] || fail "fixture file missing: $graph"
    want_id="$(jq -r --arg f "$f" '.fixtures[] | select(.file==$f) | .orphan_artifact_id' "$MANIFEST")"

    set +e
    out="$("$AO" provenance trace --orphans --strict --graph "$graph" 2>&1)"
    rc=$?
    set -e

    if [[ $rc -eq 0 ]]; then
        fail "gate did NOT fire on failing-by-design fixture $f (expected non-zero exit)"
    fi
    if ! grep -qF "$want_id" <<<"$out"; then
        fail "gate output for $f did not name orphan artifact $want_id; got: $out"
    fi
    echo "  caught orphan in $f -> $want_id (exit $rc)"
done

# 2. Wire the first fixture's orphan with an inbound edge; the gate must pass.
wired_src="$FIXTURES/$(jq -r '.fixtures[0].file' "$MANIFEST")"
wired_id="$(jq -r '.fixtures[0].orphan_artifact_id' "$MANIFEST")"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat "$wired_src" >"$tmp"
printf '{"record":"edge","edge_type":"bead_produced_artifact","from_id":"d-wired","to_id":"%s","confidence":"high","evidence":"GOALS.md"}\n' "$wired_id" >>"$tmp"

if ! "$AO" provenance trace --orphans --strict --graph "$tmp" >/dev/null 2>&1; then
    fail "gate did NOT pass once orphan $wired_id was wired with an inbound edge"
fi
echo "  wired graph passes: $wired_id -> exit 0"

echo "PROVENANCE_ORPHAN_GATE: OK (3 orphans caught, wired graph passes)"
