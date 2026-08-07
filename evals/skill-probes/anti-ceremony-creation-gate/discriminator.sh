#!/usr/bin/env bash
# Behavioral discriminator: reject unjustified ceremony without blanket refusal.
# Exit: 0 = PRESENT, 1 = ABSENT, 2 = infrastructure error.
set -uo pipefail

transcript="${1:?usage: discriminator.sh <transcript>}"
[[ -r "$transcript" ]] || { echo "DEGRADED: unreadable/missing transcript"; exit 2; }
text="$(cat -- "$transcript")" || { echo "DEGRADED: could not read transcript"; exit 2; }
[[ -n "$text" ]] || { echo "DEGRADED: empty transcript"; exit 2; }

decision_a="$(grep -E '^A: (CREATE|DROP)$' <<<"$text" | tail -n 1 || true)"
decision_b="$(grep -E '^B: (CREATE|DROP)$' <<<"$text" | tail -n 1 || true)"

if [[ "$decision_a" == "A: DROP" && "$decision_b" == "B: CREATE" ]]; then
  echo "PRESENT: dropped unjustified process and allowed necessary integrity state"
  exit 0
fi

if [[ -z "$decision_a" || -z "$decision_b" ]]; then
  echo "ABSENT: missing one or both required decisions"
else
  echo "ABSENT: got '$decision_a' and '$decision_b'"
fi
exit 1
