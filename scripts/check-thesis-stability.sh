#!/usr/bin/env bash
# check-thesis-stability.sh — Thesis-Stability Gate for the Reconciliation
# Engine arc (soc-r3y8b, between Wave 1 and Wave 2).
#
# Bead: soc-mt50 (PR-E, Wave 1H of epic soc-xlw8)
# Plan: .agents/plans/2026-05-07-drain-open-next-work-items.md §soc-mt50
#
# What it does
# ------------
# Diffs the current README.md / PRODUCT.md / GOALS.md hero sections against
# the snapshot frozen at Wave 0 close (.agents/reconcile/wave-0-thesis-snapshot.md).
# A diff = thesis drift; the operator must consciously accept it (and
# re-validate Waves 2-4) before proceeding, OR re-brainstorm Waves 2-4.
#
# Hero extraction (per pre-mortem M2):
#   awk 'NR==1, /^## / {if (!/^## /) print}' <file>
# This anchors to ^## (with the H2 boundary literal) and excludes the
# boundary line from the captured hero. Code-fence-safe because it matches
# only line-leading "## ".
#
# Exit codes:
#   0 = no drift (proceed to Wave 2)
#   1 = drift detected (operator decision required)
#   2 = precondition error (snapshot missing, source files missing, or
#       snapshot contains a WAVE_0_TODO marker)
#
# Usage:
#   bash scripts/check-thesis-stability.sh
#   bash scripts/check-thesis-stability.sh --json
#   bash scripts/check-thesis-stability.sh --help

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SNAPSHOT="${THESIS_STABILITY_SNAPSHOT:-$REPO_ROOT/.agents/reconcile/wave-0-thesis-snapshot.md}"
DECISION_TEMPLATE="$REPO_ROOT/.agents/reconcile/thesis-stability-decision.md"

JSON=0
case "${1:-}" in
    -h|--help)
        cat <<USAGE
Usage: bash scripts/check-thesis-stability.sh [--json]

Gates the Reconciliation Engine arc transition from Wave 1 to Wave 2 by
diffing current README/PRODUCT/GOALS hero sections against the Wave 0
snapshot at $SNAPSHOT.

Exit 0 = no drift; 1 = drift (operator decision); 2 = precondition error.
USAGE
        exit 0
        ;;
    --json)
        JSON=1
        ;;
    "")
        ;;
    *)
        echo "ERROR: unknown argument: $1" >&2
        exit 2
        ;;
esac

# Hero extractor — keep the awk pattern in lockstep with the snapshot
# header's documented contract.
extract_hero() {
    awk 'NR==1, /^## / {if (!/^## /) print}' "$1"
}

# Snapshot section extractor — pulls the fenced code block under
# "## <FILE>.md hero".
extract_snapshot_section() {
    local file="$1"
    awk -v target="## ${file} hero" '
        $0 == target { in_section = 1; next }
        in_section && /^## / { exit }
        in_section && /^```/ { in_block = !in_block; next }
        in_section && in_block { print }
    ' "$SNAPSHOT"
}

# Precondition: snapshot exists.
if [[ ! -f "$SNAPSHOT" ]]; then
    echo "ERROR: snapshot file missing: $SNAPSHOT" >&2
    echo "       Run PR-E (soc-mt50) to seed the snapshot from Wave 0 closure SHA." >&2
    exit 2
fi

# Precondition: snapshot is not a TODO stub. The marker must be on its own
# line to count — backtick-wrapped or quoted prose mentions of the marker
# are explanatory text, not stubs.
if grep -qE '^<!-- WAVE_0_TODO -->[[:space:]]*$' "$SNAPSHOT"; then
    echo "ERROR: snapshot contains WAVE_0_TODO marker — gate cannot evaluate." >&2
    echo "       Operator must finalize $SNAPSHOT before this gate is meaningful." >&2
    exit 2
fi

# Precondition: source files exist.
declare -a missing=()
for f in README.md PRODUCT.md GOALS.md; do
    [[ -f "$REPO_ROOT/$f" ]] || missing+=("$f")
done
if (( ${#missing[@]} > 0 )); then
    echo "ERROR: source files missing: ${missing[*]}" >&2
    exit 2
fi

drift_count=0
declare -a drifted=()

for f in README PRODUCT GOALS; do
    src="$REPO_ROOT/${f}.md"
    current="$(extract_hero "$src")"
    pinned="$(extract_snapshot_section "${f}.md")"
    if [[ -z "$pinned" ]]; then
        echo "ERROR: snapshot is missing the '${f}.md hero' section." >&2
        exit 2
    fi
    if [[ "$current" != "$pinned" ]]; then
        drifted+=("${f}.md")
        drift_count=$((drift_count + 1))
        if [[ "$JSON" -eq 0 ]]; then
            echo "DRIFT: ${f}.md hero differs from Wave 0 snapshot" >&2
            diff <(printf '%s\n' "$pinned") <(printf '%s\n' "$current") >&2 || true
            echo "" >&2
        fi
    fi
done

if (( drift_count == 0 )); then
    if [[ "$JSON" -eq 1 ]]; then
        echo '{"verdict":"pass","drifted":[]}'
    else
        echo "thesis-stability: PASS (no drift in README/PRODUCT/GOALS hero sections vs snapshot)"
    fi
    exit 0
fi

if [[ "$JSON" -eq 1 ]]; then
    printf '{"verdict":"fail","drifted":['
    sep=""
    for d in "${drifted[@]}"; do
        printf '%s"%s"' "$sep" "$d"
        sep=","
    done
    printf '],"decision_template":"%s"}\n' "$DECISION_TEMPLATE"
else
    echo ""
    echo "thesis-stability: FAIL (${drift_count} surface(s) drifted: ${drifted[*]})"
    echo ""
    echo "Operator action required. Two paths:"
    echo "  1. Accept drift: thesis has evolved. Document acceptance in"
    echo "     $DECISION_TEMPLATE"
    echo "     and re-validate Waves 2-4 acceptance criteria against the new thesis."
    echo "  2. Re-brainstorm Waves 2-4: thesis change invalidates downstream"
    echo "     plan; restart from /brainstorm before continuing."
    echo ""
    echo "If the drift is incidental (e.g. typo fix), regenerate the snapshot"
    echo "from a known-good SHA via git and document the rationale in the"
    echo "decision template."
fi
exit 1
