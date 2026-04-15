#!/usr/bin/env bash
set -euo pipefail

# check-cmd-ao-coverage.sh — Enforce a per-package coverage floor for
# cli/cmd/ao. Reads an existing Go coverage profile (default cli/coverage.out)
# rather than re-running the test suite, so it composes with the existing
# `go test -coverprofile=coverage.out -covermode=atomic ./...` step in CI.
#
# Threshold ratchet: cmd/ao is currently 76.8% statement-weighted against
# the full-suite coverage.out (2026-04-15). Source epic
# `evolve-cycle-6-coverage-85pct` is driving this to 85%. Each time real
# coverage rises, bump MIN_COVERAGE here in lockstep.
#
# Usage:
#   scripts/check-cmd-ao-coverage.sh                       # default profile + threshold
#   scripts/check-cmd-ao-coverage.sh --profile path/to.out
#   scripts/check-cmd-ao-coverage.sh --min 80
#   MIN_COVERAGE=80 scripts/check-cmd-ao-coverage.sh
#
# Exits 0 on pass, 1 on shortfall, 2 on misuse / missing profile.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PROFILE="${COVERAGE_PROFILE:-$REPO_ROOT/cli/coverage.out}"
MIN_COVERAGE="${MIN_COVERAGE:-76}"
PKG_PREFIX="github.com/boshu2/agentops/cli/cmd/ao/"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --profile)
            PROFILE="$2"
            shift 2
            ;;
        --min)
            MIN_COVERAGE="$2"
            shift 2
            ;;
        -h|--help)
            sed -n '3,20p' "$0"
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1" >&2
            exit 2
            ;;
    esac
done

if [[ ! -f "$PROFILE" ]]; then
    echo "ERROR: coverage profile not found at $PROFILE" >&2
    echo "Hint: run 'cd cli && go test -coverprofile=coverage.out -covermode=atomic ./...' first." >&2
    exit 2
fi

# Filter the profile to lines whose source path starts with the cmd/ao package
# prefix, then aggregate covered/total statements. The Go cover profile format
# is space-separated:
#
#     mode: atomic
#     <import-path>/<file>.go:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>
#
# So $1 is the location, $2 is numStmt, $3 is execution count.
stats=$(
    awk -v prefix="$PKG_PREFIX" '
        index($1, prefix) == 1 {
            n = $2 + 0
            total += n
            if ($3 + 0 > 0) covered += n
        }
        END { printf "%d %d\n", covered+0, total+0 }
    ' "$PROFILE"
)
covered=${stats% *}
total=${stats#* }

if [[ "$total" -eq 0 ]]; then
    echo "ERROR: no statements found for package prefix $PKG_PREFIX in $PROFILE" >&2
    echo "Hint: ensure the test run included ./cmd/ao/..." >&2
    exit 2
fi

pct=$(awk -v c="$covered" -v t="$total" 'BEGIN { printf "%.1f", (c / t) * 100 }')
pct_int=$(awk -v c="$covered" -v t="$total" 'BEGIN { printf "%d", (c * 100) / t }')

echo "cmd/ao coverage: ${pct}% (${covered}/${total} statements, threshold: ${MIN_COVERAGE}%)"

if [[ "$pct_int" -lt "$MIN_COVERAGE" ]]; then
    echo "FAIL: cmd/ao coverage ${pct}% is below the ${MIN_COVERAGE}% floor."
    echo "  Add tests under cli/cmd/ao/ until coverage clears the gate, or"
    echo "  bump MIN_COVERAGE in scripts/check-cmd-ao-coverage.sh if the floor was raised intentionally."
    exit 1
fi

echo "PASS: cmd/ao coverage ${pct}% meets ${MIN_COVERAGE}% floor."
