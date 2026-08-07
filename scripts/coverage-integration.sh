#!/usr/bin/env bash
set -euo pipefail

# coverage-integration.sh — Build a coverage-instrumented ao binary and run it.
#
# Uses Go 1.20+ `go build -cover` to produce a binary that writes coverage
# data to GOCOVERDIR on exit.  Run typical CLI commands against the binary,
# then summarise coverage with `go tool covdata`.
#
# Usage:
#   scripts/coverage-integration.sh              # run integration + report
#   scripts/coverage-integration.sh --report     # report only (re-use existing .coverdata)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_DIR="$REPO_ROOT/cli"

COVER_BIN="$CLI_DIR/bin/ao-cover"
COVER_DIR="$REPO_ROOT/.coverdata"

if ! command -v go >/dev/null 2>&1; then
    echo "SKIP: go is not installed"
    exit 0
fi

report_only=false
if [[ "${1:-}" == "--report" ]]; then
    report_only=true
fi

if [[ "$report_only" == false ]]; then
    # Clean previous coverage data
    rm -rf "$COVER_DIR"
    mkdir -p "$COVER_DIR"

    echo "Building coverage-instrumented binary..."
    (cd "$CLI_DIR" && go build -cover -o "$COVER_BIN" ./cmd/ao)

    echo "Running integration commands..."
    export GOCOVERDIR="$COVER_DIR"

    # Run safe read-only commands that exercise major code paths
    "$COVER_BIN" version 2>/dev/null || true
    "$COVER_BIN" ratchet status 2>/dev/null || true
    "$COVER_BIN" doctor 2>/dev/null || true

    echo "Integration commands complete."
fi

if [[ ! -d "$COVER_DIR" ]] || [[ -z "$(ls -A "$COVER_DIR" 2>/dev/null)" ]]; then
    echo "No coverage data found at $COVER_DIR"
    echo "Run without --report first."
    exit 1
fi

echo ""
echo "=== Coverage Report (integration tests) ==="
go tool covdata percent -i="$COVER_DIR" 2>&1 || {
    echo "WARN: go tool covdata failed — may need Go 1.20+"
    exit 0
}

echo ""
echo "Coverage data written to $COVER_DIR"
echo "For function-level detail: go tool covdata func -i=$COVER_DIR"
