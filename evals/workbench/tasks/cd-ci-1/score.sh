#!/usr/bin/env bash
# cd-ci-1 grader (deterministic, no LLM). Runs the agent-produced
# check-no-advisory-tier.sh against a violating and a clean fixture.
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-no-advisory-tier.sh"
total=3
score=0

bad="$(mktemp)"; cat > "$bad" <<'YML'
jobs:
  lint:
    continue-on-error: true
  summary:
    needs: [lint, correctness]
YML
good="$(mktemp)"; cat > "$good" <<'YML'
jobs:
  lint:
    continue-on-error: true
  summary:
    needs: [correctness]
YML

if [[ -f "$SCRIPT" ]]; then
  score=$((score + 1))
  chmod +x "$SCRIPT" 2>/dev/null || true
  rc=0; bash "$SCRIPT" "$bad" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 1 ]] && score=$((score + 1))   # violating fixture -> must exit 1
  rc=0; bash "$SCRIPT" "$good" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 0 ]] && score=$((score + 1))   # clean fixture -> must exit 0
fi
rm -f "$bad" "$good"

pass=false; [[ "$score" -eq "$total" ]] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
