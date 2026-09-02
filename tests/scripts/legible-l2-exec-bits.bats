#!/usr/bin/env bats
#
# Tests for scripts/check-shell-exec-bits.sh — the shell.exec-bits advisory
# gate. Both branches are exercised against a throwaway git repo built in
# BATS_TEST_TMPDIR, so the assertions are about the gate's rule, not about the
# current state of this repository.
#
# Branch 1: a shebang-bearing *.sh tracked at 100644 must FAIL.
# Branch 2: a shebang-less *.sh under a lib/ directory at 100644 must PASS.
#
# The git discovery env is scrubbed before every `git init`: git exports
# GIT_DIR into hook-launched processes, and a fixture `git init` that inherits
# it rewrites the SHARED .git/config (core.bare=true), bricking every linked
# worktree (.claude/rules/go.md, ek8v).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    GATE="$REPO_ROOT/scripts/check-shell-exec-bits.sh"
    export GATE

    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX \
        GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE

    FIX="$BATS_TEST_TMPDIR/fixture-repo"
    mkdir -p "$FIX/scripts/lib" "$FIX/tests"
    git -C "$FIX" init -q
    git -C "$FIX" config user.name "exec-bits fixture"
    git -C "$FIX" config user.email "exec-bits@fixture.invalid"
    export FIX
}

# track FILE MODE — write CONTENT (stdin) to $FIX/FILE and stage it at MODE.
track() {
    local rel="$1" mode="$2"
    mkdir -p "$FIX/$(dirname "$rel")"
    cat > "$FIX/$rel"
    chmod "$mode" "$FIX/$rel"
    git -C "$FIX" add "$rel"
    if [ "$mode" = 644 ]; then
        git -C "$FIX" update-index --chmod=-x "$rel"
    else
        git -C "$FIX" update-index --chmod=+x "$rel"
    fi
}

@test "clean fixture: executable entry point plus sourced lib passes" {
    track scripts/entry.sh 755 <<'SH'
#!/usr/bin/env bash
echo entry
SH
    track scripts/lib/helper.sh 644 <<'SH'
# shellcheck shell=bash
helper() { echo helper; }
SH

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK: shell entry points"* ]]
}

@test "shebang-bearing script tracked at 100644 fails and is named" {
    track scripts/entry.sh 755 <<'SH'
#!/usr/bin/env bash
echo entry
SH
    track tests/run-thing.sh 644 <<'SH'
#!/usr/bin/env bash
echo thing
SH

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"not tracked executable"* ]]
    [[ "$output" == *"100644 tests/run-thing.sh"* ]]
    # The clean sibling must not be reported.
    [[ "$output" != *"scripts/entry.sh"* ]]
    [[ "$output" == *"git update-index --chmod=+x"* ]]
}

@test "shebang-less lib file at 100644 is accepted, outside lib/ it is not" {
    track scripts/lib/sourced.sh 644 <<'SH'
# shellcheck shell=bash
sourced() { echo sourced; }
SH
    track tests/lib/also-sourced.sh 644 <<'SH'
# shellcheck shell=bash
also() { echo also; }
SH

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]

    # Same content, wrong home: no shebang and not under lib/.
    track scripts/stray.sh 644 <<'SH'
# shellcheck shell=bash
stray() { echo stray; }
SH

    run bash "$GATE" "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"without a shebang live outside a lib/ directory"* ]]
    [[ "$output" == *"100644 scripts/stray.sh"* ]]
    [[ "$output" != *"scripts/lib/sourced.sh"* ]]
}

@test "shell files outside scripts/ and tests/ are out of scope" {
    track scripts/entry.sh 755 <<'SH'
#!/usr/bin/env bash
echo entry
SH
    track skills/whatever.sh 644 <<'SH'
#!/usr/bin/env bash
echo out of scope
SH

    run bash "$GATE" "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" != *"skills/whatever.sh"* ]]
}
