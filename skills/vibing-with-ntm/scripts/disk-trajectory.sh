#!/usr/bin/env bash
# disk-trajectory.sh — track disk % per tick and warn on upward trajectory
#
# Usage: disk-trajectory.sh [mount_point]   (default: /)
#
# Appends a timestamped sample to /tmp/vibing-disk.log, then computes:
#   - current %
#   - delta vs last sample
#   - 3-sample trend
# Warns when >=50% used AND delta per tick >3pp (trajectory fires before threshold).
#
# Per OBSERVABILITY.md "Disk trajectory beats absolute threshold" + OC-032 + AP-50.

set -u
MOUNT="${1:-/}"
LOG="/tmp/vibing-disk.log"

NOW=$(date +%s)
PCT=$(df -P "$MOUNT" | awk 'NR==2 {gsub("%","",$5); print $5}')
echo "$NOW $PCT" >> "$LOG"

echo "Mount=$MOUNT  current_use=${PCT}%"

# Compute delta vs previous sample
LAST2=$(tail -2 "$LOG")
if [ "$(echo "$LAST2" | wc -l)" -ge 2 ]; then
  PREV_PCT=$(echo "$LAST2" | head -1 | awk '{print $2}')
  DELTA=$((PCT - PREV_PCT))
  echo "delta_from_last_tick=${DELTA}pp"
else
  DELTA=0
  echo "delta_from_last_tick=(first sample)"
fi

# 3-sample trend
TAIL3=$(tail -3 "$LOG" | awk '{print $2}' | tr '\n' ' ')
echo "last_3_samples: $TAIL3"

# Warning logic
if [ "$PCT" -ge 50 ] && [ "$DELTA" -gt 3 ]; then
  echo "⚠ TRAJECTORY WARN: used=${PCT}%, delta=+${DELTA}pp/tick — dispatch cargo-clean / prune nudges to heaviest panes before threshold fires"
  echo "  Recipe: OC-032 (per-pane CARGO_TARGET_DIR isolation) + targeted sweep (not box-wide rm)"
elif [ "$PCT" -ge 85 ]; then
  echo "⚠ ABSOLUTE WARN: used=${PCT}% — fuzz corpora / build caches will start failing; prune now"
else
  echo "OK (no trajectory warning)"
fi
