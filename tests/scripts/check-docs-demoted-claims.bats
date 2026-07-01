#!/usr/bin/env bats
#
# Tests for scripts/check-docs-demoted-claims.sh — the docs.demoted-claims
# honesty gate (age-gate-the-ungated-egwt.6). SIBLING of the retired-tech gate;
# the banned lexicon is the ADR-0004/ADR-0011 demoted-claim vocabulary
# (peer-review-uncited / multiplier-uncited / compounding-as-proven).
#
# Fixture-repo pattern per tests/scripts/docs-scope.bats: stand up a self-
# contained repo (scripts/ + scripts/lib/docs-scope.sh + docs/ + a baseline) in
# BATS_TEST_TMPDIR and run the real script inside it. The script resolves its
# lib and baseline from its own $ROOT, so a fixture repo exercises the true
# code paths — including the exit codes that the future Blocking flip depends on.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-docs-demoted-claims.sh"
    LIB="$REPO_ROOT/scripts/lib/docs-scope.sh"
    [ -r "$SCRIPT" ]
    [ -r "$LIB" ]

    # Build a fresh fixture repo per test.
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts/lib" "$FIX/docs" "$FIX/docs/adr" "$FIX/docs/evals"
    cp "$SCRIPT" "$FIX/scripts/"
    cp "$LIB" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-docs-demoted-claims.sh"
    RUN="$FIX/scripts/check-docs-demoted-claims.sh"
}

# Helper: write the fixture baseline (FILENAME-pinned; one path per line).
seed_baseline() {
    printf '%s\n' "$@" > "$FIX/scripts/.docs-demoted-claims-baseline"
}

# ---- regression / clean ------------------------------------------------------

@test "empty baseline + a clean live doc passes" {
    printf '# Clean\n\nThe verification loop is proven.\n' > "$FIX/docs/clean.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- class (c): compounding-as-proven ---------------------------------------

@test "non-baselined doc asserting the flywheel thesis proven FAILS (names file + class + ADR)" {
    printf '# Doc\n\nThe flywheel thesis is validated.\n' > "$FIX/docs/overclaim.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/overclaim.md"* ]]
    [[ "$output" == *"compounding-as-proven"* ]]
    # the repair hint points at the demoting ADRs
    [[ "$output" == *"ADR-0004"* ]]
    [[ "$output" == *"ADR-0011"* ]]
}

@test "corpus-is-the-moat asserted indicatively FAILS" {
    printf '# Doc\n\nThe corpus is the moat. The tool is replaceable.\n' > "$FIX/docs/moat.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/moat.md"* ]]
}

# ---- class (b): uncited multiplier ------------------------------------------

@test "uncited multiplier with speedup language FAILS" {
    printf '# Doc\n\nGit metrics: 40x speedup from parallelization.\n' > "$FIX/docs/mult.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/mult.md"* ]]
    [[ "$output" == *"multiplier-uncited"* ]]
}

# ---- class (a): uncited peer-review -----------------------------------------

@test "bare peer-review claim without a citation FAILS" {
    printf '# Doc\n\nScientific foundation: Peer-reviewed.\n' > "$FIX/docs/pr.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/pr.md"* ]]
    [[ "$output" == *"peer-review-uncited"* ]]
}

@test "peer-review claim WITH an adjacent author-year citation PASSES" {
    printf '# Doc\n\n| 17%%/week decay | Darr et al. (1995) | Empirical, peer-reviewed |\n' > "$FIX/docs/cited.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- line-level hedge exemption ---------------------------------------------

@test "a hedged line (names it an unproven hypothesis) PASSES" {
    printf '# Doc\n\nThat the corpus is the moat is an explicitly unproven hypothesis.\n' > "$FIX/docs/hedged.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a conditional compounding claim (When X, knowledge compounds) PASSES" {
    printf '# Doc\n\nWhen retrieval times usage beats decay, knowledge compounds.\n' > "$FIX/docs/cond.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- evidence/eval-file exemption -------------------------------------------

@test "a docs/evals file reporting a measured delta is exempt (PASSES)" {
    printf '# Measured\n\nThe treatment ran 40x faster than control in the A/B.\n' > "$FIX/docs/evals/ab.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- ADR exemption (out of scope entirely) ----------------------------------

@test "an ADR file asserting compounding is exempt (out of scope, PASSES)" {
    printf '# ADR-0099\n\nThe flywheel thesis is validated. The corpus is the moat.\n' \
        > "$FIX/docs/adr/ADR-0099-thing.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- baselined offender is tolerated ----------------------------------------

@test "a baselined offending file PASSES (grandfathered)" {
    printf '# Doc\n\nThe corpus is the moat.\n' > "$FIX/docs/legacy.md"
    seed_baseline "docs/legacy.md"
    run bash "$RUN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- shrink ratchet: stale baseline entry -----------------------------------

@test "a baselined file with ZERO findings FAILS demanding a prune (shrink ratchet)" {
    printf '# Clean now\n\nThe verification loop is proven.\n' > "$FIX/docs/wascleaned.md"
    seed_baseline "docs/wascleaned.md"
    run bash "$RUN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"PRUNE"* ]]
    [[ "$output" == *"docs/wascleaned.md"* ]]
}

# ---- future blocking flip: exit code is already correct ----------------------

# The gate is registered advisory (Blocking:false) for one clean cycle. This
# proves the SCRIPT's own exit code is already 1 on a seeded offender, so when
# the seed.go row flips Blocking:true it will actually block — no code change to
# the script needed for the flip.
@test "seeded offender: script exit code is ready for the future Blocking flip" {
    printf '# Doc\n\nThe flywheel thesis is empirically confirmed.\n' > "$FIX/docs/future.md"
    : > "$FIX/scripts/.docs-demoted-claims-baseline"
    run bash "$RUN"
    [ "$status" -eq 1 ]   # non-zero today ⇒ blocks the day Blocking flips true
}

# ---- real-repo run with the real baseline -----------------------------------

@test "real repo run with the shipped baseline exits 0" {
    run bash "$REPO_ROOT/scripts/check-docs-demoted-claims.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
