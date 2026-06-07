#!/usr/bin/env bash
# bench.sh — capture a timing baseline artifact for the hot path.
#
# EXECUTE this (do not read-as-reference). Run from the TARGET repo's working dir.
# It writes profile/<label>.json with median wall-clock over N runs so later
# iterations have something concrete to beat.
#
# Usage:   bash bench.sh <label> [-- <command...>]
#   label            : baseline | candidate | <name>  (artifact key)
#   -- <command...>  : the benchmark command to time (repeatable, deterministic)
#
# Config via env:
#   OPT_RUNS    number of timed runs (default 11)
#   OPT_WARMUP  warmup runs to discard (default 3)
#   OPT_PROFILE_DIR  output dir (default ./profile)
#
# Exit: 0 wrote artifact; 2 no command given; 3 command failed.
set -euo pipefail

label="${1:-}"
if [[ -z "$label" ]]; then
  echo "bench: usage: bench.sh <label> -- <command...>" >&2
  exit 2
fi
shift || true
if [[ "${1:-}" == "--" ]]; then shift; fi
if [[ $# -eq 0 ]]; then
  echo "bench: no benchmark command given (bench.sh $label -- <command...>)" >&2
  exit 2
fi

RUNS="${OPT_RUNS:-11}"
WARMUP="${OPT_WARMUP:-3}"
PROFILE_DIR="${OPT_PROFILE_DIR:-./profile}"
mkdir -p "$PROFILE_DIR"
out="$PROFILE_DIR/$label.json"

now_ns() { date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))'; }

# warmup (discarded)
for ((i=0; i<WARMUP; i++)); do "$@" >/dev/null 2>&1 || { echo "bench: command failed during warmup" >&2; exit 3; }; done

times_ns=()
for ((i=0; i<RUNS; i++)); do
  s="$(now_ns)"
  "$@" >/dev/null 2>&1 || { echo "bench: command failed on run $i" >&2; exit 3; }
  e="$(now_ns)"
  times_ns+=( $(( e - s )) )
done

# median + min/max via sort
sorted="$(printf '%s\n' "${times_ns[@]}" | sort -n)"
count="$(printf '%s\n' "$sorted" | wc -l | tr -d ' ')"
mid=$(( count / 2 ))
median_ns="$(printf '%s\n' "$sorted" | sed -n "$((mid+1))p")"
min_ns="$(printf '%s\n' "$sorted" | head -1)"
max_ns="$(printf '%s\n' "$sorted" | tail -1)"
# noise band as a fraction of median
band_ns=$(( max_ns - min_ns ))

cmd_str="$*"
cat > "$out" <<JSON
{
  "label": "$label",
  "command": "${cmd_str//\"/\\\"}",
  "runs": $RUNS,
  "warmup": $WARMUP,
  "median_ns": $median_ns,
  "min_ns": $min_ns,
  "max_ns": $max_ns,
  "noise_band_ns": $band_ns,
  "captured_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON

echo "bench: wrote $out  median=${median_ns}ns  band=${band_ns}ns  (n=$RUNS)"
echo "bench: NEXT — read the profile, rank frames, then run optguard.sh --baseline before changing code."
