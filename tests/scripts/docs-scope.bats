#!/usr/bin/env bats
#
# Tests for scripts/lib/docs-scope.sh — the shared LIVE-doc scope resolver
# (docs_scope_live_files) and the historical-exemption test
# (docs_scope_is_exempt) extracted from check-docs-no-retired-tech.sh
# (age-gate-the-ungated-egwt.1).
#
# Sibling pattern: matches tests/scripts/check-workflow-no-retired-tracker.bats
# and test-check-docs-learning-references.bats — build a fixture docs tree in
# BATS_TEST_TMPDIR, point the lib at it via the injectable DOCS_ROOT seam, and
# assert scope membership + exemption verdicts.
#
# The lib is SOURCED (not executed); it deliberately does NOT set strict mode on
# behalf of a caller, so these tests source it directly and call its functions.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    LIB="$REPO_ROOT/scripts/lib/docs-scope.sh"
    [ -r "$LIB" ]
    # shellcheck source=/dev/null
    . "$LIB"

    # Fixture docs tree under an injected DOCS_ROOT.
    export DOCS_ROOT="$BATS_TEST_TMPDIR/fix"
    mkdir -p "$DOCS_ROOT/docs" \
             "$DOCS_ROOT/docs/adr" \
             "$DOCS_ROOT/docs/audits" \
             "$DOCS_ROOT/docs/plans"

    # A plain live doc (in scope, not exempt).
    printf '# Overview\n\nThis is a live doc.\n' > "$DOCS_ROOT/docs/live.md"

    # A live doc that self-declares historical via a first-line banner.
    printf '# Old Design (RETIRED)\n\nSuperseded content.\n' > "$DOCS_ROOT/docs/banner.md"

    # A doc whose RETIRED banner sits at line 20 (OUTSIDE the first 15 lines).
    { for i in $(seq 1 19); do printf 'filler line %s\n' "$i"; done; \
      printf 'This section is RETIRED and no longer used.\n'; } \
      > "$DOCS_ROOT/docs/late-banner.md"

    # A doc under docs/adr/ (excluded from scope AND exempt).
    printf '# ADR-0099 Some Decision\n\nContext.\n' > "$DOCS_ROOT/docs/adr/ADR-0099-thing.md"

    # A doc under a dated-snapshot excluded dir (docs/audits/).
    printf '# Audit snapshot\n' > "$DOCS_ROOT/docs/audits/2026-07-01-snapshot.md"

    # A migration-named doc (exempt by filename glob) — put it in scope so its
    # filename exemption is what matters.
    printf '# Moving off the old thing\n' > "$DOCS_ROOT/docs/some-migration-notes.md"
}

# ---- docs_scope_live_files (scope membership) -------------------------------

@test "live doc is included in the live-doc set" {
    run docs_scope_live_files
    [ "$status" -eq 0 ]
    [[ "$output" == *"docs/live.md"* ]]
}

@test "file in a dated-snapshot excluded dir is NOT in the live set" {
    run docs_scope_live_files
    [ "$status" -eq 0 ]
    [[ "$output" != *"docs/audits/2026-07-01-snapshot.md"* ]]
}

@test "docs/adr file is NOT in the live set (excluded from scope)" {
    run docs_scope_live_files
    [ "$status" -eq 0 ]
    [[ "$output" != *"docs/adr/ADR-0099-thing.md"* ]]
}

@test "emitted paths are relative and begin with docs/" {
    run docs_scope_live_files
    [ "$status" -eq 0 ]
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" == docs/* ]]
    done <<< "$output"
}

# ---- docs_scope_is_exempt (exemption verdict) -------------------------------

@test "banner-exempt doc (RETIRED in first 15 lines) is exempt" {
    run docs_scope_is_exempt "docs/banner.md"
    [ "$status" -eq 0 ]
}

@test "docs/adr file is exempt" {
    run docs_scope_is_exempt "docs/adr/ADR-0099-thing.md"
    [ "$status" -eq 0 ]
}

@test "plain live doc is NOT exempt" {
    run docs_scope_is_exempt "docs/live.md"
    [ "$status" -eq 1 ]
}

@test "migration-named doc is exempt by filename glob" {
    run docs_scope_is_exempt "docs/some-migration-notes.md"
    [ "$status" -eq 0 ]
}

# EDGE: a RETIRED banner at line 20 (outside the first 15 lines) must NOT exempt.
@test "RETIRED banner at line 20 (outside first 15 lines) is NOT exempt" {
    run docs_scope_is_exempt "docs/late-banner.md"
    [ "$status" -eq 1 ]
}

# The exemption test also resolves a path directly (already readable) — the
# check script cd's to the root and passes docs/... paths that resolve as-is.
@test "exemption resolves a directly-readable path too" {
    ( cd "$DOCS_ROOT" && docs_scope_is_exempt "docs/banner.md" )
}

# ---- extracting-script integration (lib sourcing survives invocation modes) --

# REGRESSION (pawl catch, first land attempt): the check script cd's to $ROOT
# before sourcing; a relative ${BASH_SOURCE[0]} path (invoked as
# `cd scripts && ./check-docs-no-retired-tech.sh`) then resolved the lib from
# $ROOT/lib/ instead of $ROOT/scripts/lib/ and the gate died before scanning.
# The lib must be resolved via the pre-cd absolutized $ROOT.
@test "check script sources the lib when invoked from inside scripts/" {
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts/lib" "$FIX/docs"
    cp "$REPO_ROOT/scripts/check-docs-no-retired-tech.sh" "$FIX/scripts/"
    cp "$REPO_ROOT/scripts/lib/docs-scope.sh" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-docs-no-retired-tech.sh"
    printf '# Clean live doc\n' > "$FIX/docs/live.md"
    run bash -c 'cd "$1/scripts" && ./check-docs-no-retired-tech.sh' _ "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"scanned 1 live docs"* ]]
    [[ "$output" == *"PASS"* ]]
}

@test "check script sources the lib when invoked from the repo root" {
    FIX="$BATS_TEST_TMPDIR/repo2"
    mkdir -p "$FIX/scripts/lib" "$FIX/docs"
    cp "$REPO_ROOT/scripts/check-docs-no-retired-tech.sh" "$FIX/scripts/"
    cp "$REPO_ROOT/scripts/lib/docs-scope.sh" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-docs-no-retired-tech.sh"
    printf '# Clean live doc\n' > "$FIX/docs/live.md"
    run bash -c 'cd "$1" && scripts/check-docs-no-retired-tech.sh' _ "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"scanned 1 live docs"* ]]
    [[ "$output" == *"PASS"* ]]
}
