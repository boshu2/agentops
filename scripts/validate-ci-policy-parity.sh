#!/usr/bin/env bash
# validate-ci-policy-parity.sh — golden-file diff between AGENTS.md CI table
# and the generator output (docs/contracts/ci-jobs.yaml + validate.yml).
#
# soc-3oij: the AGENTS table is now generated. This script is a thin wrapper
# around scripts/generate-ci-jobs-table.sh --check. The generator also
# enforces:
#   - Every job in validate.yml's summary.needs has a manifest entry
#   - Every continue-on-error: true job is rendered as (non-blocking)
#   - Manifest does not contain orphan jobs absent from summary.needs
#
# History: pre-soc-3oij, this script did its own awk/grep parity logic
# (~190 lines). The hand-edited AGENTS table was the source of truth, which
# meant adding a new validate-* job required hand-editing 5 files in lockstep
# (caught us twice in May 2026 — see PR #315 add-validate-job scaffolder).
# Now the manifest is the source of truth and AGENTS.md is regenerated.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GENERATOR="$SCRIPT_DIR/generate-ci-jobs-table.sh"

if [[ ! -x "$GENERATOR" ]]; then
    echo "CI_POLICY_PARITY: generator missing or not executable: $GENERATOR" >&2
    exit 2
fi

if "$GENERATOR" --check; then
    exit 0
else
    rc=$?
    echo ""
    echo "CI_POLICY_PARITY: drift detected by golden-file gate."
    echo "Action: run scripts/generate-ci-jobs-table.sh --write to refresh AGENTS.md," >&2
    echo "        or edit docs/contracts/ci-jobs.yaml if a job's reason/failure changed." >&2
    exit "$rc"
fi
