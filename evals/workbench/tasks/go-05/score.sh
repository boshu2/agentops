#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=4

# Check 1: go test ./... passes
if go test ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: go vet ./... clean
if go vet ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 3: No exec.Command with unsanitized shell invocation
# Flag: exec.Command("sh", "-c", ...) or exec.Command("bash", "-c", ...)
# This pattern passes arbitrary user input to a shell
vuln_found=false
if grep -rn 'exec\.Command\s*(' --include='*.go' "$WORKDIR" | grep -qP '"(sh|bash|cmd)".*"-c"'; then
  vuln_found=true
fi
if [ "$vuln_found" = "false" ]; then
  score=$((score + 1))
fi

# Check 4: The run/command functionality still exists in some form OR
# the vulnerable code was cleanly removed (code compiles and no dead imports)
# Either: (a) a safe command runner exists, or (b) the feature was removed and code is clean
if go build ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
