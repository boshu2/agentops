#!/usr/bin/env bash
# Run the complete land-queue regression suite without depending on GitHub Actions.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v bats >/dev/null 2>&1; then
  echo "land-queue-test: bats is required" >&2
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
