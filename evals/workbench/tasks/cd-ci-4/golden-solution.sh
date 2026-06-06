#!/usr/bin/env bash
# Golden reference solution for cd-ci-4 (used by the grader-discrimination test,
# NOT given to the agent). Fails if <job-id> is still referenced under <search-root>.
set -euo pipefail
job="${1:?Usage: check-removed-job-assertions.sh <job-id> <search-root>}"
root="${2:?}"
if grep -rqs -- "$job" "$root"; then
  grep -rls -- "$job" "$root"
  exit 1
fi
exit 0
