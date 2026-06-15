#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-br-beads-dir.sh"
total=3
score=0

bad="$(mktemp)"; good="$(mktemp)"
cat > "$bad" <<'SH'
br update ag-123 --claim
BEADS_DIR="$PWD/_beads" br show ag-123
SH
cat > "$good" <<'SH'
BEADS_DIR="$PWD/_beads" br update ag-123 --claim
BEADS_DIR=/repo/_beads br close ag-123 --reason done
SH

if [[ -f "$SCRIPT" ]]; then
  score=$((score + 1))
  chmod +x "$SCRIPT" 2>/dev/null || true
  rc=0; bash "$SCRIPT" "$bad" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 1 ]] && score=$((score + 1))
  rc=0; bash "$SCRIPT" "$good" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 0 ]] && score=$((score + 1))
fi
rm -f "$bad" "$good"

pass=false; [[ "$score" -eq "$total" ]] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
