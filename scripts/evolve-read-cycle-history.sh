#!/usr/bin/env bash
# evolve-read-cycle-history.sh — wrapper around `ao loop history` for evolve
# scripts and references. Routes cycle-history.jsonl reads through the typed
# BC3 LoopReaderPort (cli/cmd/ao/loop_reader_adapter.go, cycle 108
# productionLoopReader) per soc-y5vh.4, replacing inline `tail`/`awk`/`jq`
# shell-outs over the raw JSONL.
#
# Why: the JSONL is mutated by both the writer adapter and any operator who
# `cat >> cycle-history.jsonl`s a row. Going through `ao loop history`
# enforces the typed port's read shape (one JSON CycleEntry per line) and
# gives downstream consumers a stable contract independent of the on-disk
# format.
#
# practices: [hexagonal-architecture, ddd-bounded-context, code-complete]
#
# Usage:
#   evolve-read-cycle-history.sh                       # default: last 3 entries
#   evolve-read-cycle-history.sh recent [N]            # last N entries (default 3)
#   evolve-read-cycle-history.sh latest                # most recent entry only
#   evolve-read-cycle-history.sh range START END       # cycles START..END (inclusive)
#
# Exits 0 with one JSON CycleEntry per line on stdout. Exits 2 on bad usage.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Prefer the repo-local ao binary; fall back to PATH. Mirrors the resolution
# pattern in scripts/check-three-gap-supergate.sh and scripts/release-smoke-test.sh.
if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO="$REPO_ROOT/cli/bin/ao"
elif command -v ao >/dev/null 2>&1; then
    AO="ao"
else
    echo "ERROR: ao binary not found; build with 'cd cli && make build' first" >&2
    exit 2
fi

mode="${1:-recent}"

case "$mode" in
    recent)
        limit="${2:-3}"
        if ! [[ "$limit" =~ ^[0-9]+$ ]]; then
            echo "ERROR: 'recent' limit must be a non-negative integer (got '$limit')" >&2
            exit 2
        fi
        exec "$AO" loop history --limit "$limit"
        ;;
    latest)
        exec "$AO" loop history --latest
        ;;
    range)
        start="${2:-0}"
        end="${3:-0}"
        if ! [[ "$start" =~ ^[0-9]+$ ]] || ! [[ "$end" =~ ^[0-9]+$ ]]; then
            echo "ERROR: 'range' START and END must be non-negative integers" >&2
            exit 2
        fi
        exec "$AO" loop history --start "$start" --end "$end"
        ;;
    -h|--help)
        sed -n '3,22p' "$0"
        exit 0
        ;;
    *)
        echo "ERROR: unknown mode: '$mode'" >&2
        echo "usage: $0 {recent [N]|latest|range START END}" >&2
        exit 2
        ;;
esac
