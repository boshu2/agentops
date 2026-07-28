#!/usr/bin/env bats
# Tests for scripts/check-skill-python-ratchet.sh — the ADR-0016 shipped-Python
# ratchet.
#
# LIVENESS IS THE POINT. This gate exists because ADR-0016 declared shipping
# Python inside a skill "a gate failure" while no gate ran, so a suite that only
# proved the green path would reproduce the exact defect being fixed. Every
# negative below is a seeded witness: the gate must be shown to FAIL on the
# thing it claims to catch, not merely to pass on a clean tree.
#
# Witness inventory (each maps to one failure the gate promises to detect):
#   N1  new Python on a skill's execution path            -> exit 1
#   N2  new Python under skills/*/tests/                  -> exit 0 (exempt class)
#   N3  a change that allowlists its OWN new file         -> exit 1 (growth guard)
#   N4  a pinned file removed but its line kept           -> exit 1 (must prune)
#   N5  nested skills/*/scripts/<sub>/*.py                -> exit 1 (depth is not an escape)
#   N6  deleting shipped Python                           -> exit 0 (the wanted direction)

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    # Git injects GIT_DIR/GIT_WORK_TREE into hook-launched processes; a leaked
    # GIT_DIR would point the fixture's `git init` at a real repository and
    # rewrite its config. Scrub before touching git at all (.claude/rules/go.md,
    # ek8v).
    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
          GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR
    mkdir -p "$TMP_DIR/scripts/lib" "$TMP_DIR/skills/alpha/scripts" "$TMP_DIR/skills/alpha/tests"
    cp "$REPO_ROOT/scripts/check-skill-python-ratchet.sh" "$TMP_DIR/scripts/"
    cp "$REPO_ROOT/scripts/lib/preamble.sh" "$TMP_DIR/scripts/lib/preamble.sh"
    cp "$REPO_ROOT/scripts/lib/ratchet.sh" "$TMP_DIR/scripts/lib/ratchet.sh"
    chmod +x "$TMP_DIR/scripts/check-skill-python-ratchet.sh"

    # Baseline: one pre-existing execution-path file, pinned. This is the
    # grandfathered tree the ratchet must leave alone.
    printf 'print("legacy")\n' > "$TMP_DIR/skills/alpha/scripts/legacy.py"
    cat > "$TMP_DIR/scripts/.skill-python-grandfather" <<'EOF'
# fixture snapshot
skills/alpha/scripts/legacy.py
EOF
    (
        cd "$TMP_DIR"
        git init -q
        git config user.email t@t.t
        git config user.name t
        git add -A
        git commit -qm seed
    )
}

teardown() {
    rm -rf "$TMP_DIR"
}

run_gate() {
    ( cd "$TMP_DIR" && bash scripts/check-skill-python-ratchet.sh --scope "${1:-head}" )
}

commit_all() {
    ( cd "$TMP_DIR" && git add -A && git commit -qm "${1:-change}" )
}

# --- baseline -----------------------------------------------------------------

@test "clean grandfathered tree passes and prints the surviving count" {
    run run_gate head
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: skill-python ratchet"* ]]
    # The count is the ratchet's public number — it must be visible, not implied.
    [[ "$output" == *"1 grandfathered file(s) remain"* ]]
}

@test "modifying a grandfathered file is allowed" {
    printf 'print("legacy v2")\n' > "$TMP_DIR/skills/alpha/scripts/legacy.py"
    commit_all "touch legacy"
    run run_gate head
    [ "$status" -eq 0 ]
}

# --- N1: the headline negative ------------------------------------------------

@test "N1: new Python on a skill execution path fails" {
    printf 'print("new")\n' > "$TMP_DIR/skills/alpha/scripts/fresh.py"
    commit_all "add fresh.py"
    run run_gate head
    [ "$status" -eq 1 ]
    [[ "$output" == *"skills/alpha/scripts/fresh.py"* ]]
    [[ "$output" == *"ADR-0016"* ]]
    # The repair hint must route to `ao`, not to the allowlist.
    [[ "$output" == *'`ao`'* ]]
    [[ "$output" == *"NOT a repair"* ]]
}

# --- N2: the recorded carve-out ----------------------------------------------

@test "N2: new Python under skills/*/tests/ passes (exempt class)" {
    printf 'def test_x():\n    assert True\n' > "$TMP_DIR/skills/alpha/tests/test_fresh.py"
    commit_all "add test"
    run run_gate head
    [ "$status" -eq 0 ]
}

# --- N3: self-allowlisting ----------------------------------------------------

@test "N3: a change cannot allowlist its own new file (growth guard)" {
    printf 'print("new")\n' > "$TMP_DIR/skills/alpha/scripts/fresh.py"
    echo "skills/alpha/scripts/fresh.py" >> "$TMP_DIR/scripts/.skill-python-grandfather"
    commit_all "add fresh.py + self-allowlist"
    run run_gate head
    [ "$status" -eq 1 ]
    [[ "$output" == *"only SHRINKS"* ]]
    [[ "$output" == *"skills/alpha/scripts/fresh.py"* ]]
}

# --- N4: the shrink direction -------------------------------------------------

@test "N4: a pinned file that no longer exists must be pruned" {
    rm "$TMP_DIR/skills/alpha/scripts/legacy.py"
    commit_all "promote legacy into ao"
    run run_gate head
    [ "$status" -eq 1 ]
    [[ "$output" == *"no longer exist"* ]]
    [[ "$output" == *"skills/alpha/scripts/legacy.py"* ]]
}

@test "N4b: pruning the line together with the file passes" {
    rm "$TMP_DIR/skills/alpha/scripts/legacy.py"
    cat > "$TMP_DIR/scripts/.skill-python-grandfather" <<'EOF'
# fixture snapshot
EOF
    commit_all "promote legacy into ao + prune"
    run run_gate head
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 grandfathered file(s) remain"* ]]
}

# --- N5: nesting is not an escape hatch ---------------------------------------

@test "N5: nested skills/*/scripts/<sub>/*.py fails" {
    mkdir -p "$TMP_DIR/skills/alpha/scripts/binary"
    printf 'print("nested")\n' > "$TMP_DIR/skills/alpha/scripts/binary/deep.py"
    commit_all "add nested py"
    run run_gate head
    [ "$status" -eq 1 ]
    [[ "$output" == *"skills/alpha/scripts/binary/deep.py"* ]]
}

# --- N5b: the glob-crosses-slash trap ----------------------------------------

# Inside `[[ ]]`, bash pattern `*` matches `/`, so a `skills/*/scripts/*.py`
# glob ALSO matches skills/<slug>/tests/scripts/<f>.py — silently governing the
# class the ADR amendment exempts. The detector is an anchored regex for exactly
# this reason; this pins it.
@test "N5b: a scripts/ dir nested under tests/ stays exempt" {
    mkdir -p "$TMP_DIR/skills/alpha/tests/scripts"
    printf 'print("test helper")\n' > "$TMP_DIR/skills/alpha/tests/scripts/helper.py"
    commit_all "add test-local helper"
    run run_gate head
    [ "$status" -eq 0 ]
}

# --- N6: the direction the gate wants -----------------------------------------

@test "N6: deleting shipped Python is never a violation" {
    printf 'print("new")\n' > "$TMP_DIR/skills/alpha/scripts/fresh.py"
    commit_all "add fresh.py"
    rm "$TMP_DIR/skills/alpha/scripts/fresh.py"
    commit_all "remove fresh.py"
    run run_gate head
    [ "$status" -eq 0 ]
}

# --- scope / usage ------------------------------------------------------------

@test "an invalid --scope is a loud usage error, not a silent pass" {
    run run_gate bogus
    [ "$status" -eq 2 ]
    [[ "$output" == *"Invalid --scope"* ]]
}

@test "non-Python and non-skill changes are not governed" {
    printf 'x\n' > "$TMP_DIR/skills/alpha/scripts/helper.sh"
    printf 'y\n' > "$TMP_DIR/README.md"
    commit_all "sh glue + docs"
    run run_gate head
    [ "$status" -eq 0 ]
}
