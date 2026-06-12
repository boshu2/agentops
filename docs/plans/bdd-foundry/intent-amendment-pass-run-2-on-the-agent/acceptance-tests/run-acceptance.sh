#!/usr/bin/env bash
# run-acceptance.sh — ONE command that runs EVERY amendment-pass (run-2)
# acceptance scenario, B74–B94 (B94 = the drift-guard split of B85's
# rollout-evidence concern). bdd-foundry Phase 2 (ATDD): the suite is RED
# by design until the amendment-pass work is built.
#
#   bash docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests/run-acceptance.sh
#
# Discipline (inherited from B73, applied to this suite): no skip/pending/
# focus markers; totality — every scenario B74..B94 present exactly once;
# hermetic — sandbox fixtures in mktemp dirs, real-repo checks via disposable
# clones/scratch copies only (B92), tracker reads from the MAIN checkout only.
# The base (run-1) suite keeps its own entry point; B75's audit-red.sh is the
# standing red-on-assertion gate that runs it.
set -euo pipefail

SUITE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for tool in bats jq python3 git br; do
  command -v "$tool" > /dev/null || { echo "FATAL: $tool not on PATH (fail-closed — never skip)" >&2; exit 2; }
done

# ── no skip / pending / focus markers anywhere in the suite ──
if grep -nE '^[[:space:]]*skip([[:space:]]|$)' "$SUITE_DIR"/*.bats; then
  echo "FATAL: skip marker found — the suite must be total" >&2
  exit 2
fi
if grep -n 'bats:focus' "$SUITE_DIR"/*.bats; then
  echo "FATAL: focus marker found — the suite must be total" >&2
  exit 2
fi

# ── totality: every appended scenario B74..B94 present exactly once ──
# (B94 is the drift-guard split of B85's rollout-evidence concern; one
# scenario id per test, so every bead ACCEPTANCE filter selects exactly one.)
fail=""
for n in $(seq 74 94); do
  c="$(grep -h "@test \"B$n:" "$SUITE_DIR"/*.bats | wc -l | tr -d ' ')"
  [ "$c" -eq 1 ] || fail="$fail B$n(x$c)"
done
if [ -n "$fail" ]; then
  echo "FATAL: scenarios missing or duplicated in the suite:$fail" >&2
  exit 2
fi

echo "== land.sh amendment-pass (run 2) acceptance suite: B74–B94 =="
bats --tap "$SUITE_DIR"
