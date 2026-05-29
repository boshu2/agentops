#!/usr/bin/env bash
# check-outcomes-holdout-leak.sh — deny-by-default holdout-leak gate (ag-hdqu0.6,
# Outcomes guard layer 4). An Outcomes rubric/score payload is a derived artifact
# that crosses the cloud boundary; Managed Agents are NOT ZDR, so a payload must
# NEVER carry holdout ground truth. This is the authoritative CI check (a script,
# not a hook) backing the by-construction (ProjectRubric) and re-scan
# (Rubric.ContainsAny) guards.
#
# Usage: check-outcomes-holdout-leak.sh <payload.json> [more.json ...]
# Exit:  0 = every payload is holdout-safe
#        1 = a payload carries a target/ground_truth key, or is unreadable (deny-by-default)
#        2 = usage error (no arguments)
set -euo pipefail

# A JSON KEY named target or ground_truth (value-position "target" text in a
# description does not match — the trailing colon requires key position).
FORBIDDEN_REGEX='"(target|ground_truth)"[[:space:]]*:'

if [ "$#" -eq 0 ]; then
	echo "usage: $0 <payload.json> [more.json ...]" >&2
	exit 2
fi

status=0
for payload in "$@"; do
	if [ ! -r "$payload" ]; then
		echo "FAIL: ${payload}: unreadable — denied by default (cannot prove holdout-safe)" >&2
		status=1
		continue
	fi
	if grep -Eq "$FORBIDDEN_REGEX" "$payload"; then
		echo "FAIL: ${payload}: contains a holdout key (target/ground_truth); Outcomes payloads must be holdout-safe (Managed Agents are not ZDR)" >&2
		status=1
	else
		echo "ok: ${payload}: holdout-safe"
	fi
done

exit "$status"
