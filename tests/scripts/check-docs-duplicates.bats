#!/usr/bin/env bats
#
# Tests for scripts/check-docs-duplicates.sh — the docs.duplicates anti-regrowth
# gate (age-gate-the-ungated-egwt.12). The gate shasums every LIVE doc and fails,
# naming BOTH files, on any byte-identical pair where both files exceed the
# MIN_DUP_LINES threshold.
#
# Fixture-repo pattern per tests/scripts/check-docs-demoted-claims.bats: stand up
# a self-contained repo (scripts/ + scripts/lib/{preamble,docs-scope}.sh + docs/)
# in BATS_TEST_TMPDIR and run the real script inside it. The script sources the
# hardened preamble (REPO_ROOT) then docs-scope, and pins DOCS_ROOT to REPO_ROOT,
# so a fixture repo exercises the true code paths.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-docs-duplicates.sh"
    LIB="$REPO_ROOT/scripts/lib/docs-scope.sh"
    PREAMBLE="$REPO_ROOT/scripts/lib/preamble.sh"
    [ -r "$SCRIPT" ]
    [ -r "$LIB" ]
    [ -r "$PREAMBLE" ]

    # Build a fresh fixture repo per test. `git init` makes the preamble's
    # REPO_ROOT resolution deterministic ($FIX), never a surrounding checkout.
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts/lib" "$FIX/docs/workflows"
    git -C "$FIX" init -q
    cp "$SCRIPT" "$FIX/scripts/"
    cp "$LIB" "$FIX/scripts/lib/"
    cp "$PREAMBLE" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-docs-duplicates.sh"
    RUN="$FIX/scripts/check-docs-duplicates.sh"
}

# Helper: write a doc of N lines (each a distinct numbered line so content is
# deterministic and reproducible across a copy).
write_lines() {
    local path="$1" n="$2" i
    : > "$path"
    for ((i = 1; i <= n; i++)); do
        printf 'line %d of the guide with some body text to make it substantive\n' "$i" >> "$path"
    done
}

# ---- over-threshold byte-identical pair FAILS naming both --------------------

@test "a byte-identical pair over the line threshold FAILS naming both files" {
    write_lines "$FIX/docs/workflows/original.md" 80
    cp "$FIX/docs/workflows/original.md" "$FIX/docs/workflows/copy.md"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"docs/workflows/original.md"* ]]
    [[ "$output" == *"docs/workflows/copy.md"* ]]
}

# ---- under-threshold identical pair PASSES ----------------------------------

@test "an identical pair UNDER the line threshold PASSES (small-doc floor)" {
    write_lines "$FIX/docs/small-a.md" 10
    cp "$FIX/docs/small-a.md" "$FIX/docs/small-b.md"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- distinct docs PASS -----------------------------------------------------

@test "two distinct large docs PASS (not byte-identical)" {
    write_lines "$FIX/docs/one.md" 80
    write_lines "$FIX/docs/two.md" 80
    # Make two.md differ by one line so it is not byte-identical to one.md.
    printf 'a distinguishing final line\n' >> "$FIX/docs/two.md"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- real-repo run: the deleted duplicate stays deleted (anti-regrowth) ------

@test "real repo run exits 0 (proves the byte-dup stays deleted)" {
    run bash "$REPO_ROOT/scripts/check-docs-duplicates.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
