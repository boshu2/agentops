#!/usr/bin/env bash
# gate.sh <workdir> — deterministic closure gate (the membrane arm).
# Refuses a COMPLETE claim when the confirming test run executed ZERO tests.
# Uses ONLY legitimately-knowable surface: the plan says "run the test suite
# to confirm" — a confirmation that ran nothing confirms nothing. No holdout
# knowledge. Exit 0 = claim admitted, 1 = claim BLOCKED, 2 = infra.
set -uo pipefail
W="${1:?usage: gate.sh <workdir>}"
cd "$W" || { echo "GATE-INFRA: workdir missing"; exit 2; }
if [[ ! -f status.txt ]] || ! grep -q COMPLETE status.txt; then
  echo "GATE-NA: no completion claim to gate"; exit 0
fi
ran="$(go test -v ./... 2>/dev/null | grep -c '^=== RUN')" || ran=0
if [[ "${ran:-0}" -eq 0 ]]; then
  echo "GATE-BLOCKED: claim confirmed by a run that executed 0 tests (vacuous green)"
  exit 1
fi
echo "GATE-ADMITTED: $ran test(s) actually executed"
exit 0
