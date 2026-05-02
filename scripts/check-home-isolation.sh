#!/usr/bin/env bash
#
# check-home-isolation.sh
#
# Warn-then-fail CI gate: any Go test file in cli/ that references
# harvest.Promote, harvest.DiscoverRigs, RunIngest, or any code path
# that transitively writes to $HOME/.agents/learnings MUST also
# isolate HOME via t.Setenv("HOME", ...) or a package-level TestMain.
#
# Without this guard, tests silently poison the operator's real
# global hub on every run. This exact bug was caught during Phase 3
# validation of the Dream nightly compounder on 2026-04-09 — 150
# synthetic fixture files leaked into ~/.agents/learnings/learning/
# and had to be manually deleted.
#
# Kill switch: set CHECK_HOME_ISOLATION_DISABLED=1 to bypass locally.
#
# Exit codes:
#   0 = pass (zero offending test files)
#   1 = fail (one or more offending test files)
#   2 = script error (bad invocation, missing cli/ dir)

set -euo pipefail

if [[ "${CHECK_HOME_ISOLATION_DISABLED:-}" == "1" ]]; then
    echo "check-home-isolation: disabled via CHECK_HOME_ISOLATION_DISABLED=1"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI_DIR="${REPO_ROOT}/cli"

if [[ ! -d "$CLI_DIR" ]]; then
    echo "check-home-isolation: cli/ not found at ${CLI_DIR}" >&2
    exit 2
fi

# Patterns that indicate a test path might write to the global hub.
TRIGGER_PATTERN='harvest\.Promote|harvest\.DiscoverRigs|RunIngest\('

# Patterns that indicate the test file isolates HOME.
# Either a per-test t.Setenv("HOME", ...) OR a package-level TestMain
# that sets HOME counts as isolation.
HOME_ISOLATION_PATTERN='t\.Setenv\("HOME"|os\.Setenv\("HOME"'

# Additionally, if a test file lives in a package that has a
# TestMain isolation (e.g., cli/internal/overnight/overnight_testmain_test.go),
# tests in that package inherit the TestMain guarantee and don't
# need per-test calls.
#
# Detect this by checking each test file's parent directory for any
# test file that contains TestMain + Setenv HOME.

failed=0
offending_files=()

while IFS= read -r file; do
    # Skip if no trigger pattern in this file.
    if ! grep -qE "$TRIGGER_PATTERN" "$file" 2>/dev/null; then
        continue
    fi

    # Per-file isolation present? Pass.
    if grep -qE "$HOME_ISOLATION_PATTERN" "$file" 2>/dev/null; then
        continue
    fi

    # Package-level TestMain with HOME isolation? Walk the package dir.
    pkg_dir="$(dirname "$file")"
    testmain_isolated=0
    for tm in "$pkg_dir"/*_test.go; do
        [[ -f "$tm" ]] || continue
        if grep -q "TestMain" "$tm" 2>/dev/null && \
           grep -qE "$HOME_ISOLATION_PATTERN" "$tm" 2>/dev/null; then
            testmain_isolated=1
            break
        fi
    done
    if [[ $testmain_isolated -eq 1 ]]; then
        continue
    fi

    # Neither per-file nor package-level isolation — fail this file.
    offending_files+=("$file")
    failed=$((failed + 1))
done < <(find "$CLI_DIR" -name "*_test.go" -type f 2>/dev/null)

if [[ $failed -gt 0 ]]; then
    echo "check-home-isolation: FAIL ($failed file(s) use harvest.* without HOME isolation)" >&2
    echo "" >&2
    for f in "${offending_files[@]}"; do
        echo "  $f" >&2
    done
    echo "" >&2
    echo "Fix: add 't.Setenv(\"HOME\", t.TempDir())' at the top of each test function," >&2
    echo "or add a package-level TestMain that sets HOME before m.Run()." >&2
    echo "" >&2
    echo "Background: Phase 3 validation of the Dream nightly compounder caught this" >&2
    echo "exact bug. Tests that call harvest.Promote or RunIngest without isolation" >&2
    echo "write to ~/.agents/learnings/ for real, poisoning the operator's corpus." >&2
    exit 1
fi

echo "check-home-isolation: PASS (no test files missing HOME isolation)"
exit 0
