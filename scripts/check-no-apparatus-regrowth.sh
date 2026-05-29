#!/usr/bin/env bash
# check-no-apparatus-regrowth.sh — anti-regeneration fitness gate (GOALS.md directive D15)
#
# STAY-REMOVED guard, NOT a size metric. The teardown (epic ag-097 and its
# waves) deletes over-projected apparatus — dead packages, the gascity compat
# cluster, duplicated projections. Nothing in the fitness function rewards
# keeping the system small, so the /evolve loop could rebuild what the teardown
# removed. This gate makes "the slop we removed stays removed" a MEASURED
# outcome: it FAILS only when a path the teardown explicitly removed comes BACK.
#
# It does NOT count lines/files/jobs and does NOT penalize legitimate new
# growth — it only fires on regrowth of specifically-removed surfaces, which is
# a real user outcome, not a code metric (GOALS.md "## Anti Stars": "Goals that
# measure code metrics instead of user outcomes").
#
# The removed-surface list is the committed manifest scripts/removed-apparatus.txt
# (one path per line, repo-root-relative; `#` comments and blank lines ignored).
# Future teardown waves APPEND to that manifest rather than editing this script.
#
# Exit 0 = PASS (every removed surface stays gone).
# Exit 1 = FAIL (at least one removed surface regrew) with a per-path message.
#
# Flags:
#   --json              emit a machine-readable result to stdout
#   --manifest <path>   override the manifest path (default scripts/removed-apparatus.txt)
#   --root <path>       override the repo root used to resolve manifest entries
#   -h, --help          show this help
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST=""
JSON=0

usage() {
    sed -n '2,28p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json) JSON=1; shift ;;
        --manifest) MANIFEST="$2"; shift 2 ;;
        --root) REPO_ROOT="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "check-no-apparatus-regrowth: unknown argument: $1 (try --help)" >&2; exit 2 ;;
    esac
done

if [[ -z "$MANIFEST" ]]; then
    MANIFEST="$SCRIPT_DIR/removed-apparatus.txt"
fi

if [[ ! -f "$MANIFEST" ]]; then
    echo "check-no-apparatus-regrowth: FAIL — manifest not found: $MANIFEST" >&2
    exit 1
fi

regrown=()
checked=0

while IFS= read -r raw || [[ -n "$raw" ]]; do
    # Strip inline comments and surrounding whitespace; skip blanks/comments.
    line="${raw%%#*}"
    line="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    [[ -z "$line" ]] && continue
    checked=$((checked + 1))
    if [[ -e "$REPO_ROOT/$line" ]]; then
        regrown+=("$line")
    fi
done < "$MANIFEST"

if [[ "$JSON" -eq 1 ]]; then
    # Build a JSON array of regrown paths without requiring jq.
    arr=""
    for p in "${regrown[@]:-}"; do
        [[ -z "$p" ]] && continue
        [[ -n "$arr" ]] && arr="$arr,"
        arr="$arr\"$p\""
    done
    status="pass"
    [[ "${#regrown[@]}" -gt 0 ]] && status="fail"
    printf '{"gate":"no-apparatus-regrowth","status":"%s","checked":%d,"regrown":[%s]}\n' \
        "$status" "$checked" "$arr"
fi

if [[ "${#regrown[@]}" -gt 0 ]]; then
    if [[ "$JSON" -ne 1 ]]; then
        echo "check-no-apparatus-regrowth: FAIL — ${#regrown[@]} teardown-removed surface(s) regrew:"
        for p in "${regrown[@]}"; do
            printf '  - %s (the teardown removed this; it must stay removed)\n' "$p"
        done
        echo
        echo "If a return is intentional, remove the path from $MANIFEST in the same change and explain why."
    fi
    exit 1
fi

if [[ "$JSON" -ne 1 ]]; then
    echo "check-no-apparatus-regrowth: PASS — all $checked teardown-removed surface(s) stay removed"
fi
exit 0
