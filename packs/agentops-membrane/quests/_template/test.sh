#!/usr/bin/env bash
# test.sh — executable acceptance harness for quest {{QUEST}}.
#
# It exercises each CONTRACT.md clause as a concrete command assertion and exits
# NONZERO until every clause passes. Against the shipped placeholder impl.sh it
# MUST fail — that is the whole point: a quest starts red, and only a real build
# turns it green. The builder implements impl.sh until this exits 0; the membrane
# close gate never runs this file (it judges the diff against CONTRACT.md), but a
# green ./test.sh is the builder's own evidence and the contract's Verify hook.
#
# The planner scaffolds this; the builder implements against it. Keep one
# assertion per numbered CONTRACT clause, in order.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL="$HERE/impl.sh"
fails=0

assert_eq() { # <label> <expected> <actual>
  if [ "$2" = "$3" ]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1 (expected '$2', got '$3')"
    fails=$((fails + 1))
  fi
}

# --- Clause 1 --- (planner: replace with the real check for CONTRACT clause 1)
out1="$("$IMPL" clause1 2>/dev/null || true)"
assert_eq "clause 1: <describe>" "SATISFIED_1" "$out1"

# --- Clause 2 --- (planner: replace with the real check for CONTRACT clause 2)
out2="$("$IMPL" clause2 2>/dev/null || true)"
assert_eq "clause 2: <describe>" "SATISFIED_2" "$out2"

if [ "$fails" -gt 0 ]; then
  echo "TEST: $fails clause(s) FAILING (quest {{QUEST}} not yet satisfied)"
  exit 1
fi
echo "TEST: all clauses satisfied (quest {{QUEST}})"
exit 0
