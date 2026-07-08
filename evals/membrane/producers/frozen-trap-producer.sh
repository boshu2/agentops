#!/usr/bin/env bash
# frozen-trap-producer.sh — a DETERMINISTIC producer for scripts/eval-membrane.sh.
#
# Instead of driving a stochastic weak model (local-mlx-producer.sh), this
# producer overlays a FROZEN weak-producer solution — the exact code a weak
# producer would have shipped — into the staged workspace. That makes the
# PRODUCER arm reproducible byte-for-byte, so scripts/membrane-calibrate.sh can
# re-measure the SAME corpus over and over and attribute any change to the
# MEMBRANE (the thing under calibration), never to producer noise.
#
# Frozen solutions live in evals/membrane/frozen/<task>/<pkg>/<file>.go and are
# overlaid on top of the task's setup.sh scaffold (replacing the TODO stub,
# leaving the visible test in place). Each false-done trap passes the visible
# test but fails the hidden oracle; each control is a genuine true-done.
#
# Invoked by eval-membrane.sh as:  bash -c "$PRODUCER_CMD" _ "$workspace" ...
# so $1 = workspace. $2 (prompt) and $3 (timeout) are ignored — the output is
# frozen, not generated. A workspace with no matching frozen solution exits
# nonzero so eval-membrane records the task as degraded (fail-safe), never a
# silent no-op that would score the bare stub.
set -euo pipefail

WORKDIR="${1:?usage: frozen-trap-producer.sh <workspace> [prompt] [timeout]}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FROZEN_ROOT="$(cd "$SCRIPT_DIR/../frozen" && pwd)"

# Map a workspace package directory -> the frozen task that owns it. Detecting by
# the staged package dir (not go.mod parsing) is robust and needs no extra tools.
pairs="dedup:fd-no-mutate topscores:fd-buried-req scale:fd-regression stats:cleaner-median truncate:hard-utf8-truncate"

task=""
for pair in $pairs; do
	dir="${pair%%:*}"
	name="${pair##*:}"
	if [ -d "$WORKDIR/$dir" ]; then
		task="$name"
		break
	fi
done

if [ -z "$task" ]; then
	echo "frozen-trap-producer: no frozen solution matches workspace $WORKDIR" >&2
	exit 3
fi

src="$FROZEN_ROOT/$task"
if [ ! -d "$src" ]; then
	echo "frozen-trap-producer: missing frozen solution dir $src" >&2
	exit 3
fi

# Overlay the frozen solution files on top of the scaffold (replacing the stub).
cp -R "$src"/. "$WORKDIR"/
echo "frozen-trap-producer: overlaid $task" >&2
