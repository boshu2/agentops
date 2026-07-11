#!/usr/bin/env bats
#
# Tests for scripts/check-docs-cli-snippets.sh — the docs.cli-snippets gate that
# resolves every `ao …` command cited in a LIVE doc against the live cobra tree
# and fails on a removed/renamed command, with a FILENAME-pinned shrink-only
# baseline (age-gate-the-ungated-egwt.4).
#
# Pattern: build a fixture "repo" in BATS_TEST_TMPDIR that carries its own copy
# of the check script + shared libs (so the script's ROOT resolves to the
# fixture, exactly as docs-scope.bats does), seed a docs/ tree + a baseline, and
# assert the two-way ratchet. The archive-tagged `ao` binary is built once and
# injected via AGENTOPS_AO_BIN (the script's documented fast path).

setup_file() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    # Build the archive-tagged ao once for the whole file; inject via env.
    AO_BIN_FILE="$BATS_FILE_TMPDIR/ao"
    ( cd "$REPO_ROOT/cli" && go build -tags "flywheel legacy" -o "$AO_BIN_FILE" ./cmd/ao )
    export AGENTOPS_AO_BIN="$AO_BIN_FILE"
}

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export AGENTOPS_AO_BIN="$BATS_FILE_TMPDIR/ao"

    # Fixture repo: script + shared libs, so ROOT="$SCRIPT_DIR/.." == the fixture.
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts/lib" "$FIX/docs" "$FIX/cli"
    cp "$REPO_ROOT/scripts/check-docs-cli-snippets.sh" "$FIX/scripts/"
    cp "$REPO_ROOT/scripts/lib/docs-scope.sh" "$FIX/scripts/lib/"
    cp "$REPO_ROOT/scripts/lib/ao-snippet-resolve.sh" "$FIX/scripts/lib/"
    cp "$REPO_ROOT/scripts/lib/ao_snippet_resolve.py" "$FIX/scripts/lib/"
    # shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.6)
    cp "$REPO_ROOT/scripts/lib/ratchet.sh" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-docs-cli-snippets.sh"
    export FIX
    BASELINE="$FIX/scripts/.docs-cli-snippets-baseline"
    export BASELINE
}

# Run the fixture's check with a given baseline content ($1 = baseline lines,
# may be empty). Populates $status / $output.
run_check() {
    printf '%s' "$1" > "$BASELINE"
    run env DOCS_CLI_SNIPPETS_BASELINE="$BASELINE" bash "$FIX/scripts/check-docs-cli-snippets.sh"
}

# ---- dead-command detection --------------------------------------------------

@test "a live doc citing a removed ao command FAILS, naming file + token" {
    printf '# Guide\n\nRun `ao factory start --goal "x"` to begin.\n' > "$FIX/docs/guide.md"
    run_check ""
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/guide.md"* ]]
    [[ "$output" == *"ao factory"* ]]
}

@test "a live doc citing only LIVE ao commands PASSES" {
    printf '# Guide\n\nRun `ao gate check --fast` and `ao session bootstrap`.\n' > "$FIX/docs/guide.md"
    run_check ""
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "extraction covers a fenced code block line, not just inline spans" {
    { printf '# Guide\n\n'; printf '```bash\n'; printf 'ao rpi phased "goal"\n'; printf '```\n'; } > "$FIX/docs/guide.md"
    run_check ""
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/guide.md"* ]]
    [[ "$output" == *"ao rpi"* ]]
}

@test "plain prose mentioning ao (no code span, no leading 'ao ') is NOT scanned" {
    printf '# Guide\n\nThe factory command was removed; use the loop instead.\n' > "$FIX/docs/guide.md"
    run_check ""
    [ "$status" -eq 0 ]
}

@test "a line DESCRIBING a command removal is exempt (removal-lang)" {
    printf '# Guide\n\nThere is no `ao hooks` command any more; 3.0 is hookless.\n' > "$FIX/docs/guide.md"
    run_check ""
    [ "$status" -eq 0 ]
}

# ---- banner / historical exemption (shared docs-scope lib) --------------------

@test "a doc with a RETIRED banner in the first 15 lines is exempt from scanning" {
    printf '# Old Runbook (RETIRED)\n\nRun `ao factory start` here (historical).\n' > "$FIX/docs/old.md"
    run_check ""
    [ "$status" -eq 0 ]
}

# ---- baseline ratchet (two-way) ----------------------------------------------

@test "a baselined offender PASSES (allowlisted)" {
    printf '# Guide\n\nRun `ao factory start --goal "x"`.\n' > "$FIX/docs/guide.md"
    run_check $'docs/guide.md\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "a NON-baselined offender FAILS even when other files ARE baselined" {
    printf '# A\n\nRun `ao factory start`.\n' > "$FIX/docs/a.md"
    printf '# B\n\nRun `ao rpi phased`.\n' > "$FIX/docs/b.md"
    run_check $'docs/a.md\n'    # only a.md allowlisted; b.md is a new offender
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/b.md"* ]]
}

@test "a STALE baseline entry (no longer triggers any finding) FAILS demanding prune" {
    printf '# Clean\n\nRun `ao gate check --fast`.\n' > "$FIX/docs/clean.md"
    run_check $'docs/clean.md\n'   # clean.md has no dead command, but is baselined
    [ "$status" -eq 1 ]
    [[ "$output" == *"no longer trigger"* ]]
    [[ "$output" == *"docs/clean.md"* ]]
}

@test "a baseline entry for a DELETED file is stale and FAILS" {
    # No docs/gone.md exists at all.
    printf '# Live\n\nRun `ao lookup --query x`.\n' > "$FIX/docs/live.md"
    run_check $'docs/gone.md\n'
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/gone.md"* ]]
}

# ---- the REAL repo passes with its seeded baseline ---------------------------

@test "the real repo passes with its committed baseline (gate lands green)" {
    run env AGENTOPS_AO_BIN="$BATS_FILE_TMPDIR/ao" bash "$REPO_ROOT/scripts/check-docs-cli-snippets.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
