#!/usr/bin/env bash
# check-architecture-doc-drift.sh — Wave C mechanical acceptance for stale architecture surfaces.
#
# Fails when reconciled docs regress to bd-era tracker wording or BC5 lists
# hooks as a center-of-gravity skill slug.
#
# Exit codes: 0 = clean, 1 = drift, 2 = usage error.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

ports_doc="$repo_root/docs/architecture/ports-and-adapters.md"
bc_yaml="$repo_root/docs/contracts/bounded-contexts.yaml"

failures=0

fail() {
    echo "ARCHITECTURE_DOC_DRIFT: FAIL: $*" >&2
    failures=$((failures + 1))
}

if rg -n 'anticipating `bd`' "$ports_doc" >/dev/null 2>&1; then
    fail "$ports_doc still contains anticipating \`bd\` wording"
fi

if rg -n '^[[:space:]]*- hooks[[:space:]]*$' "$bc_yaml" >/dev/null 2>&1; then
    fail "$bc_yaml BC5 center_of_gravity must not list bare hooks entry"
fi

if ! bash "$repo_root/scripts/check-bounded-contexts-drift.sh" --check >/dev/null; then
    fail "bounded-contexts drift gate failed (run scripts/check-bounded-contexts-drift.sh --check)"
fi

if [[ "$failures" -gt 0 ]]; then
    exit 1
fi

echo "ARCHITECTURE_DOC_DRIFT: PASS (Wave C architecture surfaces reconciled)"
exit 0
