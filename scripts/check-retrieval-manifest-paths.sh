#!/usr/bin/env bash
# check-retrieval-manifest-paths.sh — validate every ground_truth path in a
# retrieval-eval manifest resolves against `git ls-tree HEAD`.
#
# Catches the failure mode where a generated/merged manifest references files
# that never existed on any branch (see f-2026-05-02-004). Without this gate,
# such a manifest passes schema validation but silently collapses retrieval
# quality metrics (any_relevant_at_k → ~0).
#
# Usage:
#   scripts/check-retrieval-manifest-paths.sh <manifest.json> [<manifest.json> ...]
#
# Exit codes:
#   0 — every ground_truth path in every manifest resolves at HEAD
#   1 — one or more paths missing (details on stderr)
#   2 — usage error (no manifests, jq missing, file unreadable)

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "usage: $0 <manifest.json> [<manifest.json> ...]" >&2
    exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "FAIL retrieval manifest paths: jq is required" >&2
    exit 2
fi

if ! command -v git >/dev/null 2>&1; then
    echo "FAIL retrieval manifest paths: git is required" >&2
    exit 2
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

fail=0
total=0
for manifest in "$@"; do
    if [[ ! -f "$manifest" ]]; then
        echo "FAIL retrieval manifest paths: $manifest: not a file" >&2
        fail=1
        continue
    fi

    # Support both canonical {queries:[{ground_truth:[]}]} and legacy
    # [{relevant:[]}] shapes — same coverage as the Go loader.
    if jq -e 'type == "array"' "$manifest" >/dev/null 2>&1; then
        paths="$(jq -r '.[] | (.relevant // [])[]' "$manifest")"
    else
        paths="$(jq -r '.queries[] | (.ground_truth // [])[]' "$manifest")"
    fi

    if [[ -z "$paths" ]]; then
        echo "WARN retrieval manifest paths: $manifest has zero ground_truth paths" >&2
        continue
    fi

    while IFS= read -r path; do
        [[ -z "$path" ]] && continue
        total=$((total + 1))
        # Strip ./ prefix if present
        path="${path#./}"
        if ! git -C "$REPO_ROOT" ls-tree --name-only HEAD -- "$path" 2>/dev/null | grep -qFx "$path"; then
            echo "FAIL retrieval manifest paths: $manifest: ground_truth path missing at HEAD: $path" >&2
            fail=1
        fi
    done <<<"$paths"
done

if [[ $fail -ne 0 ]]; then
    exit 1
fi

echo "PASS retrieval manifest paths: $total ground_truth path(s) validated across $# manifest(s)"
exit 0
