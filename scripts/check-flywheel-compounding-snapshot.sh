#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, snapshot-testing, continuous-integration]
# CI gate: validate a corpus-state evidence snapshot for the
# flywheel-compounding directive. Unlike check-flywheel-compounding.sh
# (which runs the live multi-session computation and needs a populated
# corpus), this gate reads a tracked snapshot file and validates:
#
#   1. Snapshot file exists
#   2. Snapshot is not older than $AGENTOPS_FLYWHEEL_SNAPSHOT_MAX_DAYS (default 14)
#   3. evidence.escape_velocity_compounding == true
#
# Operator refresh: bash scripts/snapshot-flywheel-compounding.sh
# Override: AGENTOPS_FLYWHEEL_SNAPSHOT_SKIP=1
# Pair: scripts/snapshot-flywheel-compounding.sh

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SNAPSHOT_PATH="${SNAPSHOT_PATH:-$REPO_ROOT/docs/releases/flywheel-compounding-snapshot.json}"
MAX_DAYS="${AGENTOPS_FLYWHEEL_SNAPSHOT_MAX_DAYS:-14}"

if [ "${AGENTOPS_FLYWHEEL_SNAPSHOT_SKIP:-0}" = "1" ]; then
    echo "check-flywheel-compounding-snapshot: SKIP (AGENTOPS_FLYWHEEL_SNAPSHOT_SKIP=1)"
    exit 0
fi

if [ ! -f "$SNAPSHOT_PATH" ]; then
    echo "check-flywheel-compounding-snapshot: FAIL — snapshot missing at $SNAPSHOT_PATH"
    echo "  fix: bash scripts/snapshot-flywheel-compounding.sh && git add docs/releases/flywheel-compounding-snapshot.json"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "check-flywheel-compounding-snapshot: SKIP (jq not available)"
    exit 0
fi

RECORDED_AT="$(jq -r '.recorded_at // ""' "$SNAPSHOT_PATH" 2>/dev/null)"
COMPOUNDING="$(jq -r '.evidence.escape_velocity_compounding // false' "$SNAPSHOT_PATH" 2>/dev/null)"

if [ -z "$RECORDED_AT" ]; then
    echo "check-flywheel-compounding-snapshot: FAIL — recorded_at missing or unreadable"
    echo "  path: $SNAPSHOT_PATH"
    exit 1
fi

# Parse ISO-8601 to epoch (GNU date)
RECORDED_EPOCH="$(date -d "$RECORDED_AT" +%s 2>/dev/null || true)"
if [ -z "$RECORDED_EPOCH" ]; then
    # BSD-style fallback
    RECORDED_EPOCH="$(date -j -f '%Y-%m-%dT%H:%M:%SZ' "$RECORDED_AT" +%s 2>/dev/null || true)"
fi
if [ -z "$RECORDED_EPOCH" ]; then
    echo "check-flywheel-compounding-snapshot: FAIL — could not parse recorded_at: $RECORDED_AT"
    exit 1
fi

NOW=$(date +%s)
AGE_SECS=$(( NOW - RECORDED_EPOCH ))
AGE_DAYS=$(( AGE_SECS / 86400 ))
MAX_SECS=$(( MAX_DAYS * 86400 ))

if [ "$AGE_SECS" -gt "$MAX_SECS" ]; then
    echo "check-flywheel-compounding-snapshot: FAIL — snapshot is ${AGE_DAYS}d old (>${MAX_DAYS}d threshold)"
    echo "  path: $SNAPSHOT_PATH"
    echo "  fix:  bash scripts/snapshot-flywheel-compounding.sh && git add -- ..."
    exit 1
fi

if [ "$COMPOUNDING" != "true" ]; then
    echo "check-flywheel-compounding-snapshot: FAIL — escape_velocity_compounding=$COMPOUNDING (must be true)"
    echo "  path:        $SNAPSHOT_PATH"
    echo "  recorded_at: $RECORDED_AT"
    echo "  fix:         corpus needs more citation activity; rerun snapshot after compounding work lands"
    exit 1
fi

echo "check-flywheel-compounding-snapshot: PASS (${AGE_DAYS}d old, compounding=true, threshold ${MAX_DAYS}d)"
exit 0
