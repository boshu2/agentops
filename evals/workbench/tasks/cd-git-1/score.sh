#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-no-main-push.sh"
total=3
score=0

bad="$(mktemp)"; good="$(mktemp)"
cat > "$bad" <<'SH'
git push origin main
git push -u origin task/ag-good
SH
cat > "$good" <<'SH'
git push -u origin task/ag-good
git push origin HEAD:refs/heads/task/ag-good
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
