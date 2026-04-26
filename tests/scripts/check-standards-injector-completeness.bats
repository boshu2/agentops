#!/usr/bin/env bats

# Regression tests for scripts/check-standards-injector-completeness.sh.
# Each test builds a self-contained tmpdir with a fake standards-injector.sh
# and a refs/ directory, then runs the gate via the env-var overrides that
# the script supports (STANDARDS_INJECTOR_HOOK, STANDARDS_REFERENCES_DIR).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-standards-injector-completeness.sh"

    TMP_DIR="$(mktemp -d)"
    HOOK_FILE="$TMP_DIR/standards-injector.sh"
    REFS_DIR="$TMP_DIR/refs"
    mkdir -p "$REFS_DIR"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Helper: write a fake hook with a given case body. Wraps the snippet in the
# minimum boilerplate the parser expects (`case "$EXT" in ... esac`).
write_hook() {
    {
        printf '#!/usr/bin/env bash\n'
        printf 'EXT="${1##*.}"\n'
        printf 'case "$EXT" in\n'
        printf '%s\n' "$1"
        printf '    *)         exit 0 ;;\n'
        printf 'esac\n'
    } > "$HOOK_FILE"
}

# Helper: create empty reference files for the listed langs.
make_refs() {
    for lang in "$@"; do
        : > "$REFS_DIR/$lang.md"
    done
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "passes when every mapped language has a reference file" {
    write_hook '    py)        LANG="python" ;;
    go)        LANG="go" ;;
    sh)        LANG="shell" ;;'
    make_refs python go shell

    run env STANDARDS_INJECTOR_HOOK="$HOOK_FILE" STANDARDS_REFERENCES_DIR="$REFS_DIR" "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK"* ]]
}

@test "fails and names the missing language when one reference is absent" {
    write_hook '    py)        LANG="python" ;;
    js)        LANG="javascript" ;;
    sh)        LANG="shell" ;;'
    make_refs python shell  # javascript intentionally missing

    run env STANDARDS_INJECTOR_HOOK="$HOOK_FILE" STANDARDS_REFERENCES_DIR="$REFS_DIR" "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"javascript"* ]]
    # Should not falsely flag the languages that do have refs.
    [[ "$output" != *"- python"* ]]
    [[ "$output" != *"- shell"* ]]
}

@test "handles | alternation in case patterns (ts|tsx maps to one lang)" {
    write_hook '    ts|tsx)    LANG="typescript" ;;
    yaml|yml)  LANG="yaml" ;;'
    make_refs typescript yaml

    run env STANDARDS_INJECTOR_HOOK="$HOOK_FILE" STANDARDS_REFERENCES_DIR="$REFS_DIR" "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"2 languages"* ]]
}

@test "fails with parser error code 2 when case block has no LANG mappings" {
    {
        printf '#!/usr/bin/env bash\n'
        printf 'case "$EXT" in\n'
        printf '    *) exit 0 ;;\n'
        printf 'esac\n'
    } > "$HOOK_FILE"

    run env STANDARDS_INJECTOR_HOOK="$HOOK_FILE" STANDARDS_REFERENCES_DIR="$REFS_DIR" "$SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"parser"* ]] || [[ "$output" == *"no LANG"* ]]
}

@test "real repo passes the completeness gate" {
    # Sanity check: the repo as committed must be in a passing state.
    run "$SCRIPT"
    [ "$status" -eq 0 ]
}
