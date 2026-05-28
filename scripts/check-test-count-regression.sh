#!/usr/bin/env bash
set -euo pipefail

# check-test-count-regression.sh
# Test-count non-regression ratchet for cli/internal/ packages (ag-h2z).
#
# For each cli/internal/<pkg> with a changed _test.go file between BASE_REF and
# HEAD, sum `^func Test` declarations across the package's _test.go files at the
# base ref and at HEAD. A net per-package decrease fails the gate unless a
# `Test-Removal-Reason:` trailer appears in any commit body in the range. Counts
# are summed per package (not per file) so moving a test between files in the
# same package is net-zero.
#
# Environment:
#   BASE_REF                           base ref to diff against (default origin/main)
#   AGENTOPS_TEST_COUNT_NOREGRESS=skip emergency bypass (logged, exit 0)
#
# Exit codes:
#   0 - pass / not applicable / skipped
#   1 - net test-count regression without a Test-Removal-Reason: trailer
#
# FAIL diagnostics go to stderr; PASS/SKIP summaries go to stdout.

if [[ "${AGENTOPS_TEST_COUNT_NOREGRESS:-}" == "skip" ]]; then
    echo "SKIP: test-count non-regression gate disabled via AGENTOPS_TEST_COUNT_NOREGRESS=skip"
    exit 0
fi

BASE_REF="${BASE_REF:-origin/main}"

if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "SKIP: not inside a git repository"
    exit 0
fi

# Fail-open when the base ref cannot be resolved (shallow/local clone). The gate
# fails closed only on a real, verifiable net decrease.
if ! base_sha="$(git rev-parse --verify --quiet "${BASE_REF}^{commit}")"; then
    echo "SKIP: base ref '${BASE_REF}' is unresolvable (shallow clone or missing fetch) — fail-open"
    exit 0
fi

# Changed cli/internal _test.go files between base and HEAD (three-dot: changes
# on HEAD since the merge base, i.e. PR-diff semantics).
mapfile -t changed_test_files < <(
    git diff --name-only "${base_sha}...HEAD" -- cli/internal 2>/dev/null \
        | grep -E '^cli/internal/.*_test\.go$' || true
)

if [[ "${#changed_test_files[@]}" -eq 0 ]]; then
    echo "PASS: no cli/internal/*_test.go changes — test-count gate not applicable"
    exit 0
fi

# Unique package directories of the changed test files.
declare -A pkg_seen=()
pkgs=()
for f in "${changed_test_files[@]}"; do
    pkg="$(dirname "$f")"
    if [[ -z "${pkg_seen[$pkg]:-}" ]]; then
        pkg_seen[$pkg]=1
        pkgs+=("$pkg")
    fi
done

# A single Test-Removal-Reason: trailer anywhere in the range is the deliberate
# escape hatch for the whole push.
trailer_present=0
if git log "${base_sha}..HEAD" --format='%B' 2>/dev/null \
    | grep -qiE '^[[:space:]]*Test-Removal-Reason:[[:space:]]*[^[:space:]]'; then
    trailer_present=1
fi

# test_names_at_ref <ref> <pkg-dir> — sorted-unique `Test*` function names across
# the package's direct _test.go files present at that ref.
test_names_at_ref() {
    local ref="$1" pkg="$2" file
    while IFS= read -r file; do
        [[ -z "$file" ]] && continue
        git show "${ref}:${file}" 2>/dev/null \
            | grep -oE '^func Test[A-Za-z0-9_]*' \
            | sed 's/^func //' || true
    done < <(
        git ls-tree -r --name-only "$ref" -- "$pkg" 2>/dev/null \
            | grep -E "^${pkg}/[^/]+_test\.go$" || true
    ) | sort -u
}

count_lines() {
    # Count non-empty lines from stdin.
    grep -c . || true
}

failures=()
for pkg in "${pkgs[@]}"; do
    base_names="$(test_names_at_ref "$base_sha" "$pkg")"
    head_names="$(test_names_at_ref "HEAD" "$pkg")"

    base_count=0
    [[ -n "$base_names" ]] && base_count="$(printf '%s\n' "$base_names" | count_lines)"
    head_count=0
    [[ -n "$head_names" ]] && head_count="$(printf '%s\n' "$head_names" | count_lines)"

    if (( head_count < base_count )); then
        removed="$(comm -23 <(printf '%s\n' "$base_names") <(printf '%s\n' "$head_names") \
            | sed '/^$/d' | paste -sd, - | sed 's/,/, /g')"
        delta=$(( head_count - base_count ))
        failures+=("${pkg}: ${base_count} -> ${head_count} (${delta} tests, removed: ${removed})")
    else
        echo "PASS: ${pkg}: ${base_count} -> ${head_count}"
    fi
done

if [[ "${#failures[@]}" -gt 0 ]]; then
    if [[ "$trailer_present" -eq 1 ]]; then
        echo "PASS: net test-count decrease allowed by a Test-Removal-Reason: trailer"
        printf '  %s\n' "${failures[@]}"
        exit 0
    fi
    {
        echo "FAIL: test-count regression in cli/internal/ without a Test-Removal-Reason: trailer"
        printf '  %s\n' "${failures[@]}"
        echo ""
        echo "If this removal is deliberate, add a 'Test-Removal-Reason: <one-line>' trailer"
        echo "to a commit in this branch, or set AGENTOPS_TEST_COUNT_NOREGRESS=skip to bypass."
    } >&2
    exit 1
fi

echo "PASS: test-count non-regression gate — no per-package regressions"
exit 0
