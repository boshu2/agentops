#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=3

# Check 1: rotate-logs.sh handles empty directory gracefully (exits 0)
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
emptydir="$tmpdir/empty-logs"
mkdir -p "$emptydir"
if bash scripts/rotate-logs.sh "$emptydir" 2>/dev/null; then
  score=$((score + 1))
fi

# Check 2: rotate-logs.sh still works on directory with actual log files
logdir="$tmpdir/real-logs"
mkdir -p "$logdir"
# Create some fake log files with old timestamps (10 days = older than 7-day
# retention threshold but younger than 14-day delete threshold, so they get
# compressed but NOT deleted)
for i in 1 2 3; do
  echo "log line $i" > "$logdir/app-$i.log"
  touch -d "10 days ago" "$logdir/app-$i.log"
done
# Also create a recent log that should NOT be compressed
echo "recent" > "$logdir/recent.log"
if bash scripts/rotate-logs.sh "$logdir" 7 2>/dev/null; then
  # Verify old logs were compressed and recent log remains
  compressed_count=$(find "$logdir" -maxdepth 1 -name '*.log.gz' 2>/dev/null | wc -l)
  if [[ "$compressed_count" -gt 0 ]] && [[ -f "$logdir/recent.log" ]]; then
    score=$((score + 1))
  fi
fi

# Check 3: shellcheck passes on rotate-logs.sh (if shellcheck available)
if command -v shellcheck &>/dev/null; then
  if shellcheck scripts/rotate-logs.sh >/dev/null 2>&1; then
    score=$((score + 1))
  fi
else
  # If shellcheck not available, check for basic bash hygiene
  if grep -qE '^\s*set\s+-[a-z]*e' scripts/rotate-logs.sh; then
    score=$((score + 1))
  fi
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
