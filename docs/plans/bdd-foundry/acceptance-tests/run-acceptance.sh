#!/usr/bin/env bash
# run-acceptance.sh — ONE command that runs EVERY land.sh acceptance scenario.
# bdd-foundry Phase 2. B73 pins the repo-level entry point at
# tests/landing/run-acceptance.sh; the implementation phase wires that path to
# delegate here (or moves this file there). Until then:
#
#   bash docs/plans/bdd-foundry/acceptance-tests/run-acceptance.sh
#
# Contract (B73): any skip/pending/focus marker fails the whole run; the suite
# runs twice and both runs must produce identical pass/fail results
# (deterministic); all fixtures are hermetic temp dirs + the sandbox-marked
# bare remote — zero writes outside fixture roots, no network.
set -euo pipefail

SUITE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

command -v bats >/dev/null || { echo "FATAL: bats not on PATH" >&2; exit 2; }
command -v jq   >/dev/null || { echo "FATAL: jq not on PATH" >&2; exit 2; }

# ── no skip / pending / focus markers anywhere in the suite ──
if grep -nE '^[[:space:]]*skip([[:space:]]|$)' "$SUITE_DIR"/*.bats; then
  echo "FATAL: skip marker found — the suite must be total (B73)" >&2
  exit 2
fi
if grep -n 'bats:focus' "$SUITE_DIR"/*.bats; then
  echo "FATAL: focus marker found — the suite must be total (B73)" >&2
  exit 2
fi

# ── totality: every scenario B1..B73 present exactly once ──
missing=""
for n in $(seq 1 73); do
  grep -qh "@test \"B$n:" "$SUITE_DIR"/*.bats || missing="$missing B$n"
done
if [ -n "$missing" ]; then
  echo "FATAL: scenarios missing from the suite:$missing" >&2
  exit 2
fi

run_once() { # run-tag → writes per-test TAP to a results file, returns bats status
  local tag="$1" status=0
  bats --tap "$SUITE_DIR" > "$SUITE_DIR/.results-$tag.tap" 2>&1 || status=$?
  # normalized pass/fail map (strip timing/noise) for the determinism diff
  grep -E '^(ok|not ok) ' "$SUITE_DIR/.results-$tag.tap" \
    | sed -E 's/^(ok|not ok) [0-9]+ /\1 /' > "$SUITE_DIR/.results-$tag.norm"
  return $status
}

echo "== land.sh acceptance suite: run 1/2 =="
st1=0; run_once run1 || st1=$?
cat "$SUITE_DIR/.results-run1.tap"

echo "== land.sh acceptance suite: run 2/2 (determinism check — runs twice) =="
st2=0; run_once run2 || st2=$?

if ! diff -u "$SUITE_DIR/.results-run1.norm" "$SUITE_DIR/.results-run2.norm"; then
  echo "FATAL: the two runs produced different pass/fail results — nondeterministic suite (B73)" >&2
  exit 2
fi

rm -f "$SUITE_DIR"/.results-run*.tap "$SUITE_DIR"/.results-run*.norm
[ "$st1" -eq 0 ] && [ "$st2" -eq 0 ]
