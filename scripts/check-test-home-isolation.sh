#!/usr/bin/env bash
#
# check-test-home-isolation.sh — soc-y1bk
#
# Lint that prevents Go tests from leaking into the operator's real ~/.claude,
# ~/.codex, ~/.agents, or any other $HOME-rooted state. Broader sibling of
# scripts/check-home-isolation.sh (which is harvest/RunIngest-specific).
#
# Rule:
#   Any *_test.go file under cli/ that touches paths derived from $HOME (via
#   os.UserHomeDir, os.Getenv("HOME"), filepath.Join(home, ".claude" | ".codex" |
#   ".agents") etc.) MUST also isolate HOME via either:
#     (a) a per-test t.Setenv("HOME", ...), OR
#     (b) a package-level TestMain that sets HOME before m.Run().
#
# Counter-rule: tests must not use os.Setenv("HOME", ...) inside a test
# function. The Go testing.T helper t.Setenv is the only safe form because it
# serializes env writes through testing's package-private mutex — required for
# `go test -race -shuffle=on -count=N` to be deterministic.
#
# Kill switch: CHECK_TEST_HOME_ISOLATION_DISABLED=1 to bypass locally.
#
# Exit codes:
#   0 = pass (zero offenders)
#   1 = fail (one or more offenders)
#   2 = script error (bad invocation, missing cli/ dir)

set -euo pipefail

if [[ "${CHECK_TEST_HOME_ISOLATION_DISABLED:-}" == "1" ]]; then
    echo "check-test-home-isolation: disabled via CHECK_TEST_HOME_ISOLATION_DISABLED=1"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI_DIR="${REPO_ROOT}/cli"

if [[ ! -d "$CLI_DIR" ]]; then
    echo "check-test-home-isolation: cli/ not found at ${CLI_DIR}" >&2
    exit 2
fi

# Trigger: a test file that reaches into $HOME.
#   - os.UserHomeDir()
#   - os.Getenv("HOME")
#   - filepath.Join(home..., ".claude" | ".codex" | ".agents")
# Any of these means the test exercises code that resolves $HOME.
TRIGGER_PATTERN='os\.UserHomeDir\(\)|os\.Getenv\("HOME"\)|filepath\.Join\(home[A-Za-z_]*, "\.claude"|filepath\.Join\(home[A-Za-z_]*, "\.codex"|filepath\.Join\(home[A-Za-z_]*, "\.agents"'

# Per-file isolation: a t.Setenv("HOME", ...) call OR an os.Setenv("HOME", ...)
# call inside a TestMain. We detect both and let TestMain alone count.
PER_TEST_ISOLATION='t\.Setenv\("HOME"'

# Anti-pattern: os.Setenv("HOME", ...) inside a regular test function.
# We greenlight it ONLY when used inside TestMain (the package-wide setup
# helper, which runs before any t.Setenv-aware test machinery exists).
RAW_HOME_SETENV='os\.Setenv\("HOME"'

failed=0
offending_files=()
raw_setenv_offenders=()

# Detect: does a *_test.go in this package contain a TestMain that sets HOME?
package_has_testmain_isolation() {
    local pkg_dir="$1"
    local f
    for f in "$pkg_dir"/*_test.go; do
        [[ -f "$f" ]] || continue
        # Check if file has a TestMain function AND sets HOME within it.
        if grep -q '^func TestMain' "$f" 2>/dev/null && \
           grep -qE "${PER_TEST_ISOLATION}|${RAW_HOME_SETENV}" "$f" 2>/dev/null; then
            return 0
        fi
    done
    return 1
}

# Detect raw os.Setenv("HOME", ...) outside of TestMain. This is the load-bearing
# signal for soc-y1bk: under -race -shuffle=on, raw os.Setenv races with other
# tests reading env. t.Setenv goes through testing's env mutex.
file_has_raw_home_setenv_outside_testmain() {
    local file="$1"
    awk '
        /^func TestMain/ { in_testmain=1 }
        in_testmain && /^}/ { in_testmain=0 }
        !in_testmain && /os\.Setenv\("HOME"/ { print NR; exit_code=1 }
        END { exit exit_code }
    ' "$file"
    return $?
}

while IFS= read -r file; do
    # Hard fail if a test file uses os.Setenv("HOME", ...) outside TestMain.
    # Capture the offending line numbers for diagnostics.
    if grep -qE "$RAW_HOME_SETENV" "$file" 2>/dev/null; then
        if ! file_has_raw_home_setenv_outside_testmain "$file" >/dev/null 2>&1; then
            # awk returned non-zero meaning a raw os.Setenv was found outside TestMain.
            # awk's exit_code=1 is INVERTED above — fix the test:
            :
        fi
        # Use a simpler grep-based heuristic: if there's a raw os.Setenv("HOME"
        # and the file does NOT have a TestMain (so it can't be inside one),
        # OR if the count of raw os.Setenv lines exceeds the number inside TestMain.
        # For simplicity, treat any os.Setenv("HOME" outside the literal lines
        # between `func TestMain` and the next top-level `}` as an offense.
        raw_lines="$(awk '
            /^func TestMain/ { in_tm=1; next }
            in_tm && /^}$/ { in_tm=0; next }
            !in_tm && /os\.Setenv\("HOME"/ { print NR ":" $0 }
        ' "$file")"
        if [[ -n "$raw_lines" ]]; then
            raw_setenv_offenders+=("$file")
            offending_files+=("$file (raw os.Setenv(\"HOME\") outside TestMain)")
            failed=$((failed + 1))
            continue
        fi
    fi

    # Skip if no trigger pattern in this file (no $HOME access at all).
    if ! grep -qE "$TRIGGER_PATTERN" "$file" 2>/dev/null; then
        continue
    fi

    # Per-file t.Setenv("HOME", ...) — isolation present. Pass.
    if grep -qE "$PER_TEST_ISOLATION" "$file" 2>/dev/null; then
        continue
    fi

    # Package-level TestMain with HOME set? Pass.
    pkg_dir="$(dirname "$file")"
    if package_has_testmain_isolation "$pkg_dir"; then
        continue
    fi

    # Neither per-file nor package-level isolation — fail.
    offending_files+=("$file (touches \$HOME without t.Setenv or TestMain)")
    failed=$((failed + 1))
done < <(find "$CLI_DIR" -name "*_test.go" -type f 2>/dev/null | sort)

if [[ $failed -gt 0 ]]; then
    echo "check-test-home-isolation: FAIL ($failed offender file(s))" >&2
    echo "" >&2
    for entry in "${offending_files[@]}"; do
        echo "  $entry" >&2
    done
    echo "" >&2
    echo "Fix: add 't.Setenv(\"HOME\", t.TempDir())' at the top of the offending test," >&2
    echo "or add a package-level TestMain that sets HOME before m.Run()." >&2
    echo "" >&2
    echo "Background (soc-y1bk):" >&2
    echo "  Tests that read \$HOME without isolation poison the operator's real" >&2
    echo "  ~/.claude, ~/.codex, or ~/.agents tree. Tests that use raw" >&2
    echo "  os.Setenv(\"HOME\", ...) inside a test function race with other tests" >&2
    echo "  under 'go test -race -shuffle=on -count=N'. Use t.Setenv: it" >&2
    echo "  serializes env writes through the testing package's env mutex and" >&2
    echo "  auto-restores the prior value at test cleanup." >&2
    exit 1
fi

echo "check-test-home-isolation: PASS (no test files leak \$HOME)"
exit 0
