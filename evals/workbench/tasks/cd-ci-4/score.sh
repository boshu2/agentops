#!/usr/bin/env bash
# cd-ci-4 grader (deterministic, no LLM). Runs the agent-produced
# check-removed-job-assertions.sh against a referenced and an absent job id.
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
SCRIPT="$WORKDIR/check-removed-job-assertions.sh"
total=3
score=0

root="$(mktemp -d)"
mkdir -p "$root/evals" "$root/docs"
echo '{"artifact_contains": ["foo-gate"]}' > "$root/evals/x.json"
echo "foo-gate is required" > "$root/docs/ci.md"

if [[ -f "$SCRIPT" ]]; then
  score=$((score + 1))
  chmod +x "$SCRIPT" 2>/dev/null || true
  rc=0; bash "$SCRIPT" "foo-gate" "$root" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 1 ]] && score=$((score + 1))    # still-referenced job -> must exit 1
  rc=0; bash "$SCRIPT" "absent-gate" "$root" >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 0 ]] && score=$((score + 1))    # unreferenced job -> must exit 0
fi
rm -rf "$root"

pass=false; [[ "$score" -eq "$total" ]] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
