#!/usr/bin/env bash
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-generated-edits.sh"
total=3
score=0

bad="$(mktemp)"; good="$(mktemp)"
printf 'registry.json\ndocs/SKILLS.md\n' > "$bad"
printf 'skills/example/SKILL.md\nregistry.json\n' > "$good"

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
