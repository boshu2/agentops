#!/usr/bin/env bash
# doctor-evolve.sh — preflight checks for /evolve loop preconditions.
#
# /evolve has many silent-failure modes. When GOALS.md is unparseable,
# .agents/evolve/session-state.json has stale claims, scripts/cron-tune-
# cadence.sh is missing, or the next-work.jsonl tail is schema-invalid,
# the loop degrades quietly instead of stopping. This script runs the
# precondition checklist explicitly so the agent — or operator — knows
# what's wrong before a cycle starts.
#
# Checks (each PASS/WARN/FAIL):
#   1. GOALS.md exists and parses (yaml fallback to .yaml)
#   2. .agents/evolve/session-state.json present + valid JSON + required keys
#   3. scripts/cron-tune-cadence.sh present + executable + reports STAY/UP/DOWN
#   4. .agents/rpi/next-work.jsonl exists + tail line is JSON
#   5. No STOP / DORMANT / KILL markers
#   6. No stale claim_status=in_progress claims (claimed > 24h ago)
#   7. cycle-history.jsonl tail entry has required fields
#
# Flags:
#   --strict     fail (exit 1) on any WARN as well as FAIL
#   --json       machine-readable summary
#   --quiet      print only failures
#
# Exit codes:
#   0 — all checks pass (or pass with warnings, without --strict)
#   1 — at least one FAIL (or WARN under --strict)
#   2 — usage error

set -euo pipefail

STRICT=0
JSON=0
QUIET=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    --json) JSON=1 ;;
    --quiet) QUIET=1 ;;
    -h|--help) usage 0 ;;
    *) echo "doctor-evolve: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# We collect findings as TSV: status<tab>check<tab>detail
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

emit() {
  printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$TMP"
}

# --- Check 1: GOALS file ---
goals_file=""
if [ -f "$ROOT/GOALS.md" ]; then
  goals_file="$ROOT/GOALS.md"
elif [ -f "$ROOT/GOALS.yaml" ]; then
  goals_file="$ROOT/GOALS.yaml"
fi
if [ -z "$goals_file" ]; then
  emit FAIL goals "no GOALS.md or GOALS.yaml at $ROOT — /evolve cannot measure fitness"
else
  if [ "${goals_file##*.}" = "yaml" ] && command -v yq >/dev/null 2>&1; then
    if yq eval . "$goals_file" >/dev/null 2>&1; then
      emit PASS goals "GOALS.yaml parseable"
    else
      emit FAIL goals "GOALS.yaml is not valid yaml"
    fi
  else
    emit PASS goals "$(basename "$goals_file") present"
  fi
fi

# --- Check 2: session-state.json ---
state_file="$ROOT/.agents/evolve/session-state.json"
if [ ! -f "$state_file" ]; then
  emit WARN session-state "missing $state_file (loop will initialize on first cycle)"
else
  if ! jq -e . "$state_file" >/dev/null 2>&1; then
    emit FAIL session-state "$state_file is not valid JSON"
  else
    # Required keys per evolve doctrine.
    missing=""
    for k in session_pr_count batch_prs heartbeat_streak; do
      if ! jq -e "has(\"$k\")" "$state_file" >/dev/null 2>&1; then
        missing="$missing $k"
      fi
    done
    if [ -n "$missing" ]; then
      emit WARN session-state "missing keys:$missing"
    else
      emit PASS session-state "valid JSON + required keys present"
    fi
  fi
fi

# --- Check 3: cron-tune-cadence.sh ---
tune_script="$ROOT/scripts/cron-tune-cadence.sh"
if [ ! -x "$tune_script" ]; then
  emit FAIL cron-tune "$tune_script missing or not executable"
else
  out="$(bash "$tune_script" heartbeat 2>/dev/null | tail -1 || true)"
  case "$out" in
    STAY|TUNE_UP|TUNE_DOWN) emit PASS cron-tune "responds with $out" ;;
    *) emit FAIL cron-tune "unexpected output: $out" ;;
  esac
fi

# --- Check 4: next-work.jsonl tail ---
nw_file="$ROOT/.agents/rpi/next-work.jsonl"
if [ ! -f "$nw_file" ]; then
  emit WARN next-work "no $nw_file (evolve will skip harvested-work rung)"
else
  if ! tail -1 "$nw_file" | jq -e . >/dev/null 2>&1; then
    emit FAIL next-work "tail line of next-work.jsonl is not valid JSON"
  else
    emit PASS next-work "tail line is valid JSON"
  fi
fi

# --- Check 5: STOP / DORMANT / KILL markers ---
markers_seen=""
for m in .agents/evolve/STOP .agents/evolve/DORMANT; do
  [ -f "$ROOT/$m" ] && markers_seen="$markers_seen $m"
done
[ -f "$HOME/.config/evolve/KILL" ] && markers_seen="$markers_seen ~/.config/evolve/KILL"
if [ -n "$markers_seen" ]; then
  emit WARN markers "loop-halt markers present:$markers_seen"
else
  emit PASS markers "no STOP/DORMANT/KILL markers"
fi

# --- Check 6: stale in-flight claims ---
if [ -f "$nw_file" ]; then
  now_epoch="$(date -u +%s)"
  # Per-line jq so a single malformed line doesn't kill the whole check
  # under set -e + pipefail.
  stale=0
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    is_stale="$(printf '%s' "$line" | jq -r --argjson now "$now_epoch" '
      select(.claim_status == "in_progress") |
      select((.claimed_at // null) != null) |
      .claimed_at | fromdateiso8601 as $t |
      select(($now - $t) > 86400) | "stale"
    ' 2>/dev/null || true)"
    [ "$is_stale" = "stale" ] && stale=$((stale + 1))
  done < "$nw_file"
  if [ "$stale" -gt 0 ]; then
    emit WARN stale-claims "$stale claim(s) in_progress > 24h"
  else
    emit PASS stale-claims "no claims older than 24h"
  fi
fi

# --- Check 7: cycle-history tail ---
ch_file="$ROOT/.agents/evolve/cycle-history.jsonl"
if [ ! -f "$ch_file" ]; then
  emit WARN cycle-history "no $ch_file (first-run state OK)"
else
  tail_entry="$(tail -1 "$ch_file" 2>/dev/null)"
  if [ -z "$tail_entry" ]; then
    emit WARN cycle-history "tail line is empty"
  elif ! printf '%s' "$tail_entry" | jq -e . >/dev/null 2>&1; then
    emit FAIL cycle-history "tail line is not valid JSON"
  else
    # required: cycle + result
    missing=""
    for k in cycle result; do
      if ! printf '%s' "$tail_entry" | jq -e ".$k != null" >/dev/null 2>&1; then
        missing="$missing $k"
      fi
    done
    if [ -n "$missing" ]; then
      emit WARN cycle-history "tail missing fields:$missing"
    else
      cycle_num="$(printf '%s' "$tail_entry" | jq -r .cycle)"
      emit PASS cycle-history "last cycle $cycle_num well-formed"
    fi
  fi
fi

# --- Emit results ---
pass=$(awk -F'\t' '$1=="PASS"' "$TMP" | wc -l | tr -d ' ')
warn=$(awk -F'\t' '$1=="WARN"' "$TMP" | wc -l | tr -d ' ')
fail=$(awk -F'\t' '$1=="FAIL"' "$TMP" | wc -l | tr -d ' ')

if [ "$JSON" -eq 1 ]; then
  findings_json="$(awk -F'\t' '{
    printf "{\"status\":\"%s\",\"check\":\"%s\",\"detail\":\"%s\"}\n", $1, $2, $3
  }' "$TMP" | jq -s .)"
  jq -nc --argjson pass "$pass" --argjson warn "$warn" --argjson fail "$fail" \
    --argjson findings "$findings_json" \
    '{pass:$pass, warn:$warn, fail:$fail, findings:$findings}'
else
  printf 'doctor-evolve: %d PASS / %d WARN / %d FAIL\n' "$pass" "$warn" "$fail"
  if [ "$QUIET" -eq 1 ]; then
    awk -F'\t' '$1=="FAIL" || $1=="WARN"' "$TMP" | sed 's/\t/ — /; s/\t/ — /'
  else
    awk -F'\t' '{ printf "  %-4s %-15s %s\n", $1, $2, $3 }' "$TMP"
  fi
fi

if [ "$fail" -gt 0 ]; then
  exit 1
fi
if [ "$STRICT" -eq 1 ] && [ "$warn" -gt 0 ]; then
  exit 1
fi
exit 0
