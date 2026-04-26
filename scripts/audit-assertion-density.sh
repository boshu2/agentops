#!/usr/bin/env bash
# audit-assertion-density.sh
# Analyze Go test files for assertion density.
# Usage:
#   bash scripts/audit-assertion-density.sh                  # Report all *_test.go
#   bash scripts/audit-assertion-density.sh --check          # Gate mode
#   bash scripts/audit-assertion-density.sh --scope coverage # Legacy scope (cov*_test.go)
#   bash scripts/audit-assertion-density.sh --scope '*_extra_test.go' <dir>
set -euo pipefail

CHECK_MODE=false
THRESHOLD=1.5
# Default to *_test.go so the audit covers any test file that ships with
# the change, not just the legacy *coverage*_test.go pattern (cov*_test.go
# is banned by CLAUDE.md, so the old default audited an empty set on a
# clean checkout — see post-mortem #5).
SCOPE='*_test.go'

POSITIONAL=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --check) CHECK_MODE=true; shift ;;
        --scope)
            case "$2" in
                coverage) SCOPE='*coverage*_test.go' ;;
                all)      SCOPE='*_test.go' ;;
                *)        SCOPE="$2" ;;
            esac
            shift 2
            ;;
        *) POSITIONAL+=("$1"); shift ;;
    esac
done

TARGET_DIR="${POSITIONAL[0]:-cli/cmd/ao}"

# For each test file matching SCOPE, compute assertion density.
HOLLOW_COUNT=0
TOTAL_FILES=0

for f in $(find "$TARGET_DIR" -name "$SCOPE" -type f | sort); do
    TOTAL_FILES=$((TOTAL_FILES + 1))
    # `grep -c` writes 0 to stdout AND exits 1 when there are no matches on
    # GNU grep, so a trailing `|| echo 0` would double the output ("0\n0\n")
    # and break the awk math below. Drop the fallback; the count is reliable.
    FUNCS=$(grep -c "^func Test" "$f" 2>/dev/null || true)
    ASSERTS=$(grep -cE 't\.(Error|Fatal|Errorf|Fatalf)|assert\.|require\.|goleak\.' "$f" 2>/dev/null || true)
    : "${FUNCS:=0}"
    : "${ASSERTS:=0}"

    if [[ "$FUNCS" -gt 0 ]]; then
        # Use awk for float division
        RATIO=$(awk "BEGIN {printf \"%.1f\", $ASSERTS / $FUNCS}")
        HOLLOW=$(awk "BEGIN {print ($RATIO < $THRESHOLD) ? 1 : 0}")

        if [[ "$HOLLOW" -eq 1 ]]; then
            HOLLOW_COUNT=$((HOLLOW_COUNT + 1))
            echo "HOLLOW: $f (ratio=$RATIO, funcs=$FUNCS, asserts=$ASSERTS)"
        else
            echo "OK:     $f (ratio=$RATIO)"
        fi
    fi
done

echo ""
echo "Summary: $HOLLOW_COUNT hollow / $TOTAL_FILES test files (scope=$SCOPE)"

if [[ "$CHECK_MODE" == "true" && "$HOLLOW_COUNT" -gt 0 ]]; then
    echo "FAIL: $HOLLOW_COUNT files below threshold ($THRESHOLD)"
    exit 1
fi
