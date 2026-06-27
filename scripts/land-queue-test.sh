#!/usr/bin/env bash
# Run the complete land-queue regression suite without depending on GitHub Actions.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v bats >/dev/null 2>&1; then
  echo "land-queue-test: bats is required" >&2
  exit 127
fi

# The e2e acceptance suite (tests/land-queue/e2e-acceptance.bats) skips itself when
# jq or go is absent. If this "complete" runner only required bats, it could exit 0
# while the integrated acceptance proof was silently skipped — a fail-open acceptance
# gate. Require the e2e suite's own hard deps here so a green run always EXECUTED the
# final proof (refuted defect, age-landq-self dogfood).
missing=()
for dep in jq go; do
  command -v "$dep" >/dev/null 2>&1 || missing+=("$dep")
done
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "land-queue-test: missing required dep(s) for the e2e acceptance proof: ${missing[*]}" >&2
  echo "land-queue-test: refusing to report a green suite that would SKIP the integrated acceptance test (fail-open guard)" >&2
  exit 127
fi

cd "$REPO_ROOT"

suites=(
  tests/land-queue/postrebase-pawl-stamp.bats
  tests/land-queue/branch-submit.bats
  tests/land-queue/land-lane.bats
  tests/land-queue/flaky-retry.bats
  tests/land-queue/assert-no-actions.bats
  tests/land-queue/e2e-acceptance.bats
)

echo "land-queue-test: running ${#suites[@]} Bats suites"
bats "${suites[@]}"
