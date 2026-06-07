#!/usr/bin/env bash
# optguard.sh — the rollback gate. EXECUTE this (do not read-as-reference).
# Run from the TARGET repo's working dir, once per optimization iteration.
#
# It enforces the two non-negotiable rules:
#   1. outputs must NOT drift (correctness)
#   2. timing must improve beyond the noise band (real win)
# Its EXIT CODE is the verdict the agent acts on.
#
# Usage:
#   bash optguard.sh --baseline -- <command...>   # snapshot outputs + timing BEFORE the change
#   bash optguard.sh --verify   -- <command...>   # re-run AFTER the change; compare
#
# Config via env:
#   OPT_RUNS / OPT_WARMUP   passed through to timing (default 11 / 3)
#   OPT_PROFILE_DIR         state dir (default ./profile)
#   OPT_FLOAT_TOLERANCE     if set, numeric outputs within this abs diff are "identical"
#
# Exit codes (the gate):
#   0  outputs identical AND faster than the noise band  -> KEEP the change
#   1  outputs drifted (correctness regression)          -> ROLL BACK, log ledger
#   2  no measurable win (<= noise band) or regression   -> ROLL BACK, log ledger
#   3  usage / missing baseline snapshot                 -> fix invocation, re-run
set -euo pipefail

mode="${1:-}"
shift || true
if [[ "${1:-}" == "--" ]]; then shift; fi

PROFILE_DIR="${OPT_PROFILE_DIR:-./profile}"
RUNS="${OPT_RUNS:-11}"
WARMUP="${OPT_WARMUP:-3}"
out_snap="$PROFILE_DIR/.optguard.out"
time_snap="$PROFILE_DIR/.optguard.median_ns"
band_snap="$PROFILE_DIR/.optguard.band_ns"

now_ns() { date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))'; }

time_command() {
  # echoes median_ns and band_ns; captures the command's stdout into $1
  local capture="$1"; shift
  for ((i=0; i<WARMUP; i++)); do "$@" >/dev/null 2>&1 || true; done
  local t=() s e
  : > "$capture"
  for ((i=0; i<RUNS; i++)); do
    s="$(now_ns)"
    "$@" > "$capture.run" 2>&1 || true
    e="$(now_ns)"
    t+=( $(( e - s )) )
  done
  mv "$capture.run" "$capture"
  local sorted count mid med mn mx
  sorted="$(printf '%s\n' "${t[@]}" | sort -n)"
  count="$(printf '%s\n' "$sorted" | wc -l | tr -d ' ')"
  mid=$(( count / 2 ))
  med="$(printf '%s\n' "$sorted" | sed -n "$((mid+1))p")"
  mn="$(printf '%s\n' "$sorted" | head -1)"
  mx="$(printf '%s\n' "$sorted" | tail -1)"
  echo "$med $(( mx - mn ))"
}

case "$mode" in
  --baseline)
    [[ $# -gt 0 ]] || { echo "optguard: --baseline needs -- <command...>" >&2; exit 3; }
    mkdir -p "$PROFILE_DIR"
    read -r med band < <(time_command "$out_snap" "$@")
    echo "$med"  > "$time_snap"
    echo "$band" > "$band_snap"
    echo "optguard: baseline snapshot saved (median=${med}ns band=${band}ns, outputs hashed)."
    echo "optguard: NOW apply ONE optimization, then run: optguard.sh --verify -- <same command>"
    exit 0
    ;;
  --verify)
    [[ $# -gt 0 ]] || { echo "optguard: --verify needs -- <command...>" >&2; exit 3; }
    if [[ ! -f "$out_snap" || ! -f "$time_snap" ]]; then
      echo "optguard: no baseline snapshot found — run --baseline first." >&2
      exit 3
    fi
    base_med="$(cat "$time_snap")"
    base_band="$(cat "$band_snap" 2>/dev/null || echo 0)"
    new_out="$PROFILE_DIR/.optguard.out.new"
    read -r new_med new_band < <(time_command "$new_out" "$@")

    # --- correctness gate ---
    if [[ -n "${OPT_FLOAT_TOLERANCE:-}" ]]; then
      if ! python3 - "$out_snap" "$new_out" "$OPT_FLOAT_TOLERANCE" <<'PY'
import sys, re
a=open(sys.argv[1]).read(); b=open(sys.argv[2]).read(); tol=float(sys.argv[3])
num=re.compile(r'-?\d+\.?\d*(?:[eE][-+]?\d+)?')
na=num.findall(a); nb=num.findall(b)
# non-numeric skeleton must match exactly
if num.sub('§',a)!=num.sub('§',b): sys.exit(1)
if len(na)!=len(nb): sys.exit(1)
for x,y in zip(na,nb):
    if abs(float(x)-float(y))>tol: sys.exit(1)
sys.exit(0)
PY
      then
        echo "optguard: FAIL(1) — outputs drifted beyond float tolerance. ROLL BACK." >&2
        exit 1
      fi
    else
      if ! cmp -s "$out_snap" "$new_out"; then
        echo "optguard: FAIL(1) — outputs differ from baseline. ROLL BACK and log the ledger." >&2
        exit 1
      fi
    fi

    # --- performance gate: improvement must exceed the noise band ---
    # require new median to be faster than baseline by at least the wider band.
    margin="$base_band"; [[ "$new_band" -gt "$margin" ]] && margin="$new_band"
    threshold=$(( base_med - margin ))
    if [[ "$new_med" -lt "$threshold" ]]; then
      speedup="$(python3 -c "print(f'{$base_med/$new_med:.2f}x')" 2>/dev/null || echo '>1x')"
      echo "optguard: PASS(0) — outputs identical, ${base_med}ns -> ${new_med}ns ($speedup, beats noise band ${margin}ns). KEEP."
      # promote snapshot so the next iteration compares against this kept state
      cp "$new_out" "$out_snap"; echo "$new_med" > "$time_snap"; echo "$new_band" > "$band_snap"
      exit 0
    else
      echo "optguard: FAIL(2) — no win (${base_med}ns -> ${new_med}ns, within noise band ${margin}ns). ROLL BACK and log the ledger." >&2
      exit 2
    fi
    ;;
  *)
    echo "optguard: usage: optguard.sh --baseline|--verify -- <command...>" >&2
    exit 3
    ;;
esac
