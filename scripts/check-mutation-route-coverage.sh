#!/usr/bin/env bash
# scripts/check-mutation-route-coverage.sh
#
# PER PRE-MORTEM AMENDMENT A2 (security, soc-8inr.5): proves that
# registerMutationRoute is the ONLY path through which mutation HTTP routes
# are registered in cli/internal/daemon/. A developer could otherwise call
# mux.HandleFunc("/v1/foo", handler) directly and bypass the auth wrapper
# entirely.
#
# CI gate: this script must exit 0. Wired into scripts/pre-push-gate.sh.
#
# Heuristic: every mux.HandleFunc callsite in cli/internal/daemon/ that lives
# outside auth.go fails the gate. auth.go is the single allowed home for
# raw mux registration via registerMutationRoute / registerReadOnlyRoute
# helpers; all other files MUST go through those helpers.
#
# Read-only routes registered via registerReadOnlyRoute are intentionally
# exempt from the auth wrapper but still go through auth.go (single
# registration choke-point), so this gate covers both.

set -euo pipefail

if REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" && [[ -n "$REPO_ROOT" ]]; then
    :
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi
SERVER_DIR="$REPO_ROOT/cli/internal/daemon"

if [[ ! -d "$SERVER_DIR" ]]; then
    echo "ERROR: daemon source dir not found: $SERVER_DIR" >&2
    exit 2
fi

# Find all mux.HandleFunc callsites in non-test daemon files. Exclude auth.go
# (the registration choke-point) and *_test.go (tests may register fixtures).
violations=$(grep -rEn 'mux\.HandleFunc\b' "$SERVER_DIR" \
    --include='*.go' \
    --exclude='*_test.go' \
    | grep -v '/auth\.go:' \
    | grep -v '^[^:]*:[0-9]\+://' \
    || true)

# Strip pure comment lines (// at start, after optional whitespace).
violations=$(echo "$violations" | awk -F: '{
    line=""
    for (i=3; i<=NF; i++) line = line (i==3?"":":") $i
    gsub(/^[ \t]+/, "", line)
    if (line ~ /^\/\//) next
    print
}')

if [[ -n "$violations" ]]; then
    echo "ERROR: mux.HandleFunc called outside auth.go in cli/internal/daemon/" >&2
    echo "       (registerMutationRoute / registerReadOnlyRoute is the ONLY allowed path)" >&2
    echo "" >&2
    echo "$violations" >&2
    echo "" >&2
    echo "If a route is read-only, register via registerReadOnlyRoute(mux, path, handler)." >&2
    echo "If a route is a mutation, register via registerMutationRoute(mux, path, policy, handler)." >&2
    echo "" >&2
    echo "PER PRE-MORTEM AMENDMENT A2 (soc-8inr.5): direct mux.HandleFunc bypasses" >&2
    echo "the auth wrapper. The gate refuses to merge any route that escapes the helpers." >&2
    exit 1
fi

echo "OK: all mux.HandleFunc callsites in cli/internal/daemon/ live in auth.go (registerMutationRoute / registerReadOnlyRoute scope)"
exit 0
