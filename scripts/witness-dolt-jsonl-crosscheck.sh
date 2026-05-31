#!/usr/bin/env bash
# witness-dolt-jsonl-crosscheck.sh — Dolt->JSONL witness cross-check gate (ag-lmdx.3).
#
# The watcher that watches the watcher. drrebuild (ag-lmdx.3 sibling, #646)
# proves the Dolt context graph can be REBUILT from the git-committed JSONL
# witness + git blobs. This gate proves the INVERSE: re-deriving the witness
# FROM the Dolt projection reproduces the committed, git-anchored hash chain.
# Dolt history is rewritable (reset/--force/root), so it is NOT tamper-evidence;
# the git-anchored JSONL witness is. A Dolt rewrite that did not also rewrite
# the committed witness is caught here, and nowhere else.
#
# CI has no live Dolt, so the gate is HERMETIC: it drives committed fixtures
# (a faithful Dolt export and a tampered one) through the re-derive+hash-compare
# helper and asserts the gate behaves correctly:
#
#   1. The FAITHFUL Dolt rows re-derive to the committed witness  -> exit 0.
#   2. The TAMPERED Dolt rows (one rewritten row) do NOT          -> exit non-zero,
#      and the helper reports DIVERGED.
#
# This proves the detector is wired and discriminating. The hash discipline is
# the schema's exactly (cli/internal/drwitness reuses cli/internal/drrebuild),
# so the fixtures are real chains, not invented ones.
#
# Bounded-context: BC4-Factory. Evidence: .github/workflows/validate.yml.
#
# Usage: witness-dolt-jsonl-crosscheck.sh
# Exit:  0 = gate behaves correctly (faithful passes, tampered fails)
#        1 = gate misbehaved (faithful failed, or tampered passed)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURES="$ROOT/tests/fixtures/witness"
FAITHFUL="$FIXTURES/dolt-rows-faithful.jsonl"
TAMPERED="$FIXTURES/dolt-rows-tampered.jsonl"
WITNESS="$FIXTURES/committed-witness.jsonl"

fail() {
	echo "WITNESS_CROSSCHECK_GATE: $*" >&2
	exit 1
}

for f in "$FAITHFUL" "$TAMPERED" "$WITNESS"; do
	[[ -f "$f" ]] || fail "fixture missing: $f"
done

# Resolve the witness-crosscheck helper: prefer a pre-built binary (CI may build
# it once), else build it from source into a temp dir. Hermetic — no live Dolt.
HELPER="${WITNESS_BIN:-}"
TMPBIN=""
cleanup() { [[ -n "$TMPBIN" ]] && rm -rf "$TMPBIN"; return 0; }
trap cleanup EXIT

if [[ -z "$HELPER" ]]; then
	if [[ -x "$ROOT/cli/bin/witness-crosscheck" ]]; then
		HELPER="$ROOT/cli/bin/witness-crosscheck"
	else
		TMPBIN="$(mktemp -d)"
		HELPER="$TMPBIN/witness-crosscheck"
		( cd "$ROOT/cli" && go build -o "$HELPER" ./cmd/witness-crosscheck ) \
			|| fail "could not build witness-crosscheck helper"
	fi
fi

echo "=== Dolt->JSONL witness cross-check gate (ag-lmdx.3) ==="

# 1. Faithful Dolt projection re-derives to the committed witness -> exit 0.
if ! out="$("$HELPER" "$FAITHFUL" "$WITNESS" 2>&1)"; then
	fail "faithful Dolt projection did NOT cross-check clean (expected pass): $out"
fi
grep -qF "OK" <<<"$out" || fail "faithful run did not report OK: $out"
echo "  faithful: $out"

# 2. Tampered Dolt projection must FAIL the cross-check (non-zero) and say so.
set +e
out="$("$HELPER" "$TAMPERED" "$WITNESS" 2>&1)"
rc=$?
set -e
if [[ $rc -eq 0 ]]; then
	fail "tampered Dolt projection PASSED the cross-check (gate is blind): $out"
fi
grep -qF "DIVERGED" <<<"$out" || fail "tampered run did not report DIVERGED: $out"
echo "  tampered: caught (exit $rc) -> $out"

echo "WITNESS_CROSSCHECK_GATE: OK (faithful passes, tampered fails)"
