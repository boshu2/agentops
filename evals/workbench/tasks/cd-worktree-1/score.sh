#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-worktree-per-bead.sh"
total=3
score=0

bad="$(mktemp)"; good="$(mktemp)"
cat > "$bad" <<'SH'
cd /Users/bo/dev/agentops
git checkout -b task/ag-demo
SH
cat > "$good" <<'SH'
git worktree add ../agentops-wt-ag-demo -b task/ag-demo origin/main
cd ../agentops-wt-ag-demo
SH

if [[ -f "$SCRIPT" ]]; then
  score=$((score + 1))
  chmod +x "$SCRIPT" 2>/dev/null || true
  rc=0; bash "$SCRIPT" ag-demo "$bad" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 1 ]] && score=$((score + 1))
  rc=0; bash "$SCRIPT" ag-demo "$good" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 0 ]] && score=$((score + 1))
fi
rm -f "$bad" "$good"

pass=false; [[ "$score" -eq "$total" ]] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
