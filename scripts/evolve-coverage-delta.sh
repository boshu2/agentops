#!/usr/bin/env bash
set -euo pipefail

# evolve-coverage-delta.sh
# Measures Go test coverage and computes delta from last evolve cycle.
# Stores baseline in .agents/evolve/coverage-baseline.json
#
# Output format: "Coverage: 51.6% (+1.2% from last cycle)"

usage() {
  cat <<USAGE
Usage: $0 [--dir <path>] [--update]

Measures test coverage and shows delta from last evolve cycle.

Options:
  --dir     Go module directory (default: cli/)
  --update  Update the baseline after reporting (default: false)

Output: "Coverage: 51.6% (+1.2% from last cycle)"
Baseline stored at: .agents/evolve/coverage-baseline.json
USAGE
}

DIR="cli/"
UPDATE_BASELINE=false
BASELINE_FILE=".agents/evolve/coverage-baseline.json"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)    DIR="$2"; shift 2 ;;
    --update) UPDATE_BASELINE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "go not installed; skipping coverage delta."
  exit 0
fi

if [[ ! -d "$DIR" ]]; then
  echo "Directory $DIR does not exist; skipping."
  exit 0
fi

echo "=== Coverage Delta Measurement ==="

# Run tests with coverage and capture output
COVERAGE_OUTPUT=$(cd "$DIR" && go test -cover ./... 2>&1 || true)

# Extract per-package coverage percentages and compute average.
# Require the trailing "%" so we don't match Go's event-coverage line for
# packages with no _test.go files (e.g. cmd/ao under coverpkg builds emits
# `coverage: 1/12 events (informational)` — without the % anchor that "1"
# was previously averaged in as 1.0% and dragged the project average down).
CURRENT_PCT=$(echo "$COVERAGE_OUTPUT" | \
  grep -oE 'coverage: [0-9]+(\.[0-9]+)?%' | \
  grep -oE '[0-9]+(\.[0-9]+)?' | \
  awk '{sum += $1; count++} END { if (count > 0) printf "%.1f", sum/count; else print "0.0" }')

if [[ -z "$CURRENT_PCT" ]]; then
  echo "Could not determine coverage percentage."
  exit 0
fi

# Read previous baseline
PREV_PCT=""
if [[ -f "$BASELINE_FILE" ]]; then
  if command -v jq >/dev/null 2>&1; then
    PREV_PCT=$(jq -r '.coverage_pct // empty' "$BASELINE_FILE" 2>/dev/null || true)
  fi
fi

# Compute delta and format output
if [[ -n "$PREV_PCT" ]]; then
  DELTA=$(awk -v cur="$CURRENT_PCT" -v prev="$PREV_PCT" 'BEGIN {
    d = cur - prev
    printf "%+.1f", d
  }')
  echo "Coverage: ${CURRENT_PCT}% (${DELTA}% from last cycle)"
else
  echo "Coverage: ${CURRENT_PCT}% (no prior baseline)"
fi

# Optionally update baseline
if [[ "$UPDATE_BASELINE" == "true" ]]; then
  mkdir -p "$(dirname "$BASELINE_FILE")"
  GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
  TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  printf '{"coverage_pct": %s, "timestamp": "%s", "git_sha": "%s"}\n' \
    "$CURRENT_PCT" "$TIMESTAMP" "$GIT_SHA" \
    > "$BASELINE_FILE"
  echo "Baseline updated: $BASELINE_FILE"
fi

exit 0
