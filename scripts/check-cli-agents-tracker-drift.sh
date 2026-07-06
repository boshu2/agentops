#!/usr/bin/env bash
# check-cli-agents-tracker-drift.sh — Fail when cli/AGENTS.md drifts off br.
#
# cli/AGENTS.md is a pointer stub to root AGENTS.md + br invocation. THIS repo's
# tracker is br; this gate blocks live bd examples used as this repo's tracker,
# stale linked-worktree `$PWD/_beads` examples, and hard-coded private-ledger git
# paths.
#
# Two-store nuance (age-gc-adoption-u0he): bd/dolt is NOT retired globally — it is
# the gascity SUBSTRATE store, a different layer from this repo's br tracker. A
# line that legitimately documents that substrate store and carries the
# `gascity-substrate` marker is exempt from the bd-command check below.
#
# Usage:
#   bash scripts/check-cli-agents-tracker-drift.sh [--agents-file PATH]
#
# Exit codes: 0 = clean, 1 = drift findings, 2 = usage error.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
agents_file="$repo_root/cli/AGENTS.md"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --agents-file)
            shift
            agents_file="${1:-}"
            ;;
        -h|--help)
            sed -n '1,12p' "$0"
            exit 0
            ;;
        *)
            echo "check-cli-agents-tracker-drift: unknown arg: $1" >&2
            exit 2
            ;;
    esac
    shift
done

if [[ ! -f "$agents_file" ]]; then
    echo "CLI_AGENTS_TRACKER_DRIFT: FAIL: missing file: $agents_file" >&2
    exit 1
fi

failures=0

fail() {
    echo "CLI_AGENTS_TRACKER_DRIFT: FAIL: $*" >&2
    failures=$((failures + 1))
}

# Substrate carve-out (age-gc-adoption-u0he): bd/dolt is the gascity SUBSTRATE
# store, not this repo's tracker. Strip lines explicitly annotated
# `gascity-substrate` before scanning — documenting the substrate layer is
# legitimate, only drifting THIS repo's tracking back to bd is not.
bd_scan_body="$(grep -viF 'gascity-substrate' "$agents_file" || true)"

if printf '%s\n' "$bd_scan_body" | grep -Eiq '\bbd (ready|show|update|close|prime|onboard|vc|dolt|sync|dep)\b'; then
    fail "$agents_file must not document live bd commands as this repo's tracker (br is the tracker; bd/dolt is the gascity substrate store)"
fi

if printf '%s\n' "$bd_scan_body" | grep -Eiq '\bbd dolt\b'; then
    fail "$agents_file must not document bd dolt as this repo's tracker (bd/dolt is the gascity substrate store, a different layer)"
fi

if grep -Fq 'BEADS_DIR=$PWD/_beads' "$agents_file"; then
    fail "$agents_file must not hard-code BEADS_DIR=\$PWD/_beads; use BEADS_DIR=\"\$(ao beads dir)\""
fi

if grep -Fq 'git -C _beads' "$agents_file"; then
    fail "$agents_file must not hard-code git -C _beads; use git -C \"\$(ao beads dir)\""
fi

if grep -Fq 'git add _beads' "$agents_file"; then
    fail "$agents_file must not stage _beads from the public repo"
fi

if ! grep -Fq '../AGENTS.md' "$agents_file"; then
    fail "$agents_file must link to root AGENTS.md as source of truth"
fi

if ! grep -Fq 'BEADS_DIR="$(ao beads dir)" br ready' "$agents_file"; then
    fail "$agents_file must document BEADS_DIR=\"\$(ao beads dir)\" br ready"
fi

if ! grep -Fq 'codebase-overview.md' "$agents_file"; then
    fail "$agents_file must link to docs/architecture/codebase-overview.md"
fi

if [[ "$failures" -gt 0 ]]; then
    exit 1
fi

echo "CLI_AGENTS_TRACKER_DRIFT: PASS ($agents_file is br-only pointer stub)"
exit 0
