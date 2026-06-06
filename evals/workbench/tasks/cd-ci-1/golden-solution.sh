#!/usr/bin/env bash
# Golden reference solution for cd-ci-1 (used by the grader-discrimination test,
# NOT given to the agent). Fails if any continue-on-error:true job is in summary.needs.
set -euo pipefail
YML="${1:?Usage: check-no-advisory-tier.sh <validate.yml>}"
awk '
  /^  [A-Za-z0-9_-]+:[ \t]*$/ { cur=$1; sub(/:$/, "", cur) }
  /continue-on-error:[ \t]*true/ { coe[cur]=1 }
  /needs:[ \t]*\[/ {
    line=$0; sub(/.*\[/, "", line); sub(/\].*/, "", line)
    gsub(/[ ,]+/, "\n", line); m=split(line, arr, "\n")
    for (i=1; i<=m; i++) if (arr[i] != "") needs[arr[i]]=1
  }
  END { found=0; for (j in coe) if (needs[j]) { print "advisory PR check: " j; found=1 } exit(found ? 1 : 0) }
' "$YML"
