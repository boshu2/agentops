#!/usr/bin/env bats
#
# Tests for scripts/check-scripts-ao-invocations.sh — the scripts.ao-invocations
# gate that resolves every literal `ao <command...>` invocation in a supported
# script/test against the live cobra tree, fails on removed/renamed commands,
# and rejects all retired-command waivers (age-owcs, age-ghk3i.5).
#
# Pattern (mirrors check-docs-cli-snippets.bats): build a fixture "repo" in
# BATS_TEST_TMPDIR that carries its own copy of the check script + shared libs
# (so the script's ROOT resolves to the fixture), seed a scripts/ tree + a
# baseline tombstone, and assert fail-closed behavior. The archive-tagged `ao` binary is
# built once and injected via AGENTOPS_AO_BIN (the script's documented fast path).

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
    mkdir -p "$FIX/scripts/lib" "$FIX/tests" "$FIX/cli"
    cp "$REPO_ROOT/scripts/check-scripts-ao-invocations.sh" "$FIX/scripts/"
    cp "$REPO_ROOT/scripts/lib/ao-snippet-resolve.sh" "$FIX/scripts/lib/"
    cp "$REPO_ROOT/scripts/lib/ao_snippet_resolve.py" "$FIX/scripts/lib/"
    # shared ratchet mechanics (age-ratchet-lib-extraction-bv7d.5)
    cp "$REPO_ROOT/scripts/lib/ratchet.sh" "$FIX/scripts/lib/"
    chmod +x "$FIX/scripts/check-scripts-ao-invocations.sh"
    export FIX
    BASELINE="$FIX/scripts/.scripts-ao-invocations-baseline"
    export BASELINE
}

# Run the fixture's check with a given baseline content ($1 = baseline lines, may
# be empty). Populates $status / $output.
run_check() {
    printf '%s' "$1" > "$BASELINE"
    run env SCRIPTS_AO_INVOCATIONS_BASELINE="$BASELINE" bash "$FIX/scripts/check-scripts-ao-invocations.sh"
}

# ---- dead-command detection --------------------------------------------------

@test "an induced retired call fails closed and names its file" {
    printf '#!/usr/bin/env bash\nrun_json_capture out err ao --output json daemon jobs wait 123\n' > "$FIX/scripts/bad.sh"
    run_check ""
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/bad.sh"* ]]
    [[ "$output" == *"ao daemon"* ]]
}

@test "an inline suppress pragma cannot waive a retired call" {
    printf '#!/usr/bin/env bash\nao rpi status  # ao-resolve: ignore\n' > "$FIX/scripts/bad.sh"
    run_check ""
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/bad.sh"* ]]
}

@test "a live ao invocation PASSES" {
    printf '#!/usr/bin/env bash\nao gate check --fast\nao session bootstrap\n' > "$FIX/scripts/good.sh"
    run_check ""
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# ---- false-positive guards (soundness over recall) ---------------------------

@test "a dead command inside a heredoc body is NOT flagged" {
    { printf '#!/usr/bin/env bash\n'; printf "cat <<'EOF'\n"; printf 'ao rpi status\n'; printf 'EOF\n'; } > "$FIX/scripts/here.sh"
    run_check ""
    [ "$status" -eq 0 ]
}

@test "a dead command inside a quoted string (prose) is NOT flagged" {
    printf '#!/usr/bin/env bash\necho "the (ao rpi status) command was removed"\n' > "$FIX/scripts/prose.sh"
    run_check ""
    [ "$status" -eq 0 ]
}

@test "a dead command in a comment line is NOT flagged" {
    printf '#!/usr/bin/env bash\n# ao rpi status was the old way\ntrue\n' > "$FIX/scripts/comment.sh"
    run_check ""
    [ "$status" -eq 0 ]
}

# ---- retired-command waivers are forbidden -----------------------------------

@test "a baseline entry cannot waive a retired call" {
    printf '#!/usr/bin/env bash\nao rpi status\n' > "$FIX/scripts/bad.sh"
    run_check $'scripts/bad.sh\n'
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/bad.sh"* ]]
}

@test "a NON-baselined offender FAILS even when another file IS baselined" {
    printf '#!/usr/bin/env bash\nao rpi status\n' > "$FIX/scripts/a.sh"
    printf '#!/usr/bin/env bash\nif ! ao daemon jobs list; then true; fi\n' > "$FIX/tests/b.sh"
    run_check $'scripts/a.sh\n'    # only a.sh allowlisted; tests/b.sh is a new offender
    [ "$status" -eq 1 ]
    [[ "$output" == *"tests/b.sh"* ]]
    [[ "$output" == *"ao daemon"* ]]
}

@test "a baseline entry for a clean file fails as a retired-command waiver" {
    printf '#!/usr/bin/env bash\nao gate check --fast\n' > "$FIX/scripts/clean.sh"
    run_check $'scripts/clean.sh\n'   # clean.sh has no dead command, but is baselined
    [ "$status" -eq 1 ]
    [[ "$output" == *"waiver"* ]]
    [[ "$output" == *"scripts/clean.sh"* ]]
}

@test "a baseline entry for a DELETED file is stale and FAILS" {
    # No scripts/gone.sh exists at all.
    printf '#!/usr/bin/env bash\nao lookup --query x\n' > "$FIX/scripts/live.sh"
    run_check $'scripts/gone.sh\n'
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/gone.sh"* ]]
}

# ---- the real repo carries no retired-command waivers ------------------------

@test "current supported scripts have zero retired command waivers" {
    EMPTY_BASELINE="$BATS_TEST_TMPDIR/empty-baseline"
    : > "$EMPTY_BASELINE"
    run env AGENTOPS_AO_BIN="$BATS_FILE_TMPDIR/ao" \
        SCRIPTS_AO_INVOCATIONS_BASELINE="$EMPTY_BASELINE" \
        bash "$REPO_ROOT/scripts/check-scripts-ao-invocations.sh"
    if [ "$status" -ne 0 ]; then
        printf '%s\n' "$output" >&3
    fi
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"zero retired command waivers"* ]]

    run awk 'NF && $1 !~ /^#/' "$REPO_ROOT/scripts/.scripts-ao-invocations-baseline"
    [ -z "$output" ]
}

# ---- ratchet-lib migration contract (age-ratchet-lib-extraction-bv7d.5) ------

@test "missing python3 is a loud environment error (rc 2), not a silent pass (fail-closed deps)" {
    SHIM="$BATS_TEST_TMPDIR/noPy"
    mkdir -p "$SHIM"
    printf '#!/usr/bin/env bash\nexit 127\n' > "$SHIM/python3"
    chmod +x "$SHIM/python3"
    printf '#!/usr/bin/env bash\nao version\n' > "$FIX/scripts/anything.sh"
    : > "$BASELINE"
    run env PATH="$SHIM:$PATH" bash "$FIX/scripts/check-scripts-ao-invocations.sh"
    [ "$status" -eq 2 ]
    [[ -n "$output" ]]
    [[ "$output" == *"cannot certify"* ]]
}
