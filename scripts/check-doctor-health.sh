#!/usr/bin/env bash
set -euo pipefail

# check-doctor-health.sh
# Validates that `ao doctor` reports no P0 (required) findings.
# Used by ci-local-release.sh to catch path/namespace drift.
#
# `ao doctor --json` emits a single engine Report and exits:
#   0 = healthy (no findings)
#   1 = findings present (severity P0..P3)
#   >1 = doctor itself errored (unsafe-refused, I/O error, …)
#
# Exit codes (this script):
#   0 = no P0 findings (healthy, or degraded with only P1..P3 findings)
#   1 = a P0 finding, or doctor errored / binary unavailable

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AO_BIN="$REPO_ROOT/cli/bin/ao"
TEMP_AO_BIN=""

# shellcheck disable=SC2329  # Invoked via trap.
cleanup() {
    if [[ -n "$TEMP_AO_BIN" && -f "$TEMP_AO_BIN" ]]; then
        rm -f "$TEMP_AO_BIN"
    fi
}
trap cleanup EXIT

if [[ ! -x "$AO_BIN" ]]; then
    TEMP_AO_BIN="$(mktemp "${TMPDIR:-/tmp}/ao-doctor.XXXXXX")"
    if ! (
        cd "$REPO_ROOT/cli"
        go build -o "$TEMP_AO_BIN" ./cmd/ao
    ); then
        echo "ao binary not found at $AO_BIN and temporary build failed" >&2
        exit 1
    fi
    AO_BIN="$TEMP_AO_BIN"
fi

# Run doctor in JSON mode for machine parsing. Exit 1 (findings present) is an
# expected outcome, not a script failure — capture the code instead of aborting.
# stdout is the engine Report JSON; stderr is kept separate so it never
# pollutes the JSON we parse.
doctor_rc=0
err_file="$(mktemp "${TMPDIR:-/tmp}/ao-doctor-err.XXXXXX")"
output=$("$AO_BIN" doctor --json 2>"$err_file") || doctor_rc=$?
doctor_err="$(cat "$err_file")"
rm -f "$err_file"

if [[ "$doctor_rc" -gt 1 ]]; then
    echo "ao doctor errored (exit $doctor_rc)" >&2
    [[ -n "$doctor_err" ]] && echo "$doctor_err" >&2
    echo "$output" >&2
    exit 1
fi

if ! echo "$output" | jq -e . >/dev/null 2>&1; then
    echo "ao doctor --json did not produce valid JSON" >&2
    echo "$output" >&2
    exit 1
fi

total=$(echo "$output" | jq -r '.summary.total_findings // 0')
p0=$(echo "$output" | jq -r '[.findings[]? | select(.severity == "P0")] | length')

echo "Doctor: ${total} finding(s), ${p0} required (P0)"

# Fail only on required (P0) findings.
if [[ "$p0" -gt 0 ]]; then
    echo ""
    echo "Required (P0) finding(s):"
    echo "$output" | jq -r '.findings[]? | select(.severity == "P0") | "  \(.id): \(.title)"'
    exit 1
fi

# Report non-blocking findings but don't fail.
if [[ "$total" -gt 0 ]]; then
    echo ""
    echo "Findings (non-blocking):"
    echo "$output" | jq -r '.findings[]? | "  [\(.severity)] \(.id): \(.title)"'
fi

exit 0
