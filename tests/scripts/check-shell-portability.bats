#!/usr/bin/env bats
# Tests for scripts/check-shell-portability.sh — the static guard against the
# one shell-portability bug class with no safe inline form: GNU `find -printf`.
#
# Context: four real `find -printf` instances (age-iue5 #850, age-7jm6) shipped
# undetected because no gate catches the class and Linux CI succeeds at runtime
# (only BSD/macOS find errors). This lint catches reintroduction on every
# platform. These specs pin its detection, its exclusions (comments, the
# allow-marker, the linter's own file), and its exit contract.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-shell-portability.sh"
    TMP_DIR="$(mktemp -d)"
    mkdir -p "$TMP_DIR/scripts"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "the real scripts/ tree is clean (exit 0, reports a count)" {
    run bash "$SCRIPT" --root "$REPO_ROOT/scripts"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK shell-portability"* ]]
    [[ "$output" == *"no GNU-only"* ]]
}

@test "a real find -printf use FAILS and names the offending file:line" {
    printf '#!/usr/bin/env bash\nx=$(find . -printf "%%f\\n")\n' > "$TMP_DIR/scripts/bad.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL shell-portability"* ]]
    [[ "$output" == *"bad.sh:2:"* ]]
}

@test "a comment mentioning -printf does NOT trip the lint (exit 0)" {
    printf '#!/usr/bin/env bash\n# this comment explains -printf is GNU-only\necho ok\n' \
        > "$TMP_DIR/scripts/commented.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK shell-portability"* ]]
}

@test "the # portability-ok marker allows a deliberate, justified use (exit 0)" {
    printf '#!/usr/bin/env bash\ny=$(find . -printf "%%p\\n") # portability-ok deliberate\n' \
        > "$TMP_DIR/scripts/marked.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 0 ]
}

@test "mixed tree flags ONLY the real use, not the comment or marked lines" {
    printf '#!/usr/bin/env bash\nx=$(find . -printf "%%f\\n")\n' > "$TMP_DIR/scripts/bad.sh"
    printf '#!/usr/bin/env bash\n# -printf note\necho ok\n' > "$TMP_DIR/scripts/commented.sh"
    printf '#!/usr/bin/env bash\nz=$(find . -printf x) # portability-ok\n' > "$TMP_DIR/scripts/marked.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    [[ "$output" == *"bad.sh"* ]]
    [[ "$output" != *"commented.sh"* ]]
    [[ "$output" != *"marked.sh"* ]]
}

@test "the linter excludes its own file from the scan" {
    # The linter necessarily names the pattern in its own code/messages; copying
    # it into the scanned root must NOT make it flag itself.
    cp "$SCRIPT" "$TMP_DIR/scripts/check-shell-portability.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 0 ]
}

@test "an EXTENSIONLESS shell script (shebang, no .sh) with find -printf is caught" {
    # Hooks and other executable helpers may be extensionless shell. Detection
    # must follow the shebang.
    printf '#!/usr/bin/env bash\nx=$(find . -printf "%%f\\n")\n' > "$TMP_DIR/scripts/myhook"
    chmod +x "$TMP_DIR/scripts/myhook"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    [[ "$output" == *"myhook"* ]]
}

@test "the marker as a STRING (not a trailing comment) does NOT exempt a real use" {
    # Defect the pawl caught: a bare 'portability-ok' anywhere on the line could
    # hide a real find -printf. The marker must be a '#'-led comment to count.
    printf '#!/usr/bin/env bash\nx=$(find . -printf "portability-ok")\n' > "$TMP_DIR/scripts/sneaky.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    [[ "$output" == *"sneaky.sh"* ]]
}

@test "a zsh-shebang extensionless script with find -printf is caught (find is shell-agnostic)" {
    printf '#!/usr/bin/env zsh\nx=$(find . -printf "%%f")\n' > "$TMP_DIR/scripts/zhook"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    [[ "$output" == *"zhook"* ]]
}

@test "the marker exempts only its own line, not other unmarked uses in the same file" {
    # A '# portability-ok' comment exempts the line it sits on (in shell, '#'
    # comments to end-of-line, so dropping the whole line is correct). It does
    # NOT exempt other lines — a separate unmarked find -printf still flags.
    printf '#!/usr/bin/env bash\nx=$(find . -printf "%%f") # portability-ok\ny=$(find . -printf "%%p")\n' \
        > "$TMP_DIR/scripts/partial.sh"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 1 ]
    # line 2 (marked) is exempted; line 3 (unmarked) is still flagged.
    [[ "$output" == *"partial.sh:3:"* ]]
    [[ "$output" != *"partial.sh:2:"* ]]
}

@test "an extensionless NON-shell file (no shell shebang) is ignored" {
    # A python/other extensionless script that happens to contain the token must
    # not be linted (we only guard shell).
    printf '#!/usr/bin/env python3\n# find . -printf here is just text\n' > "$TMP_DIR/scripts/pyhook"
    run bash "$SCRIPT" --root "$TMP_DIR/scripts"
    [ "$status" -eq 0 ]
}

@test "unknown flag exits 2 (usage error)" {
    run bash "$SCRIPT" --nonsense
    [ "$status" -eq 2 ]
}

@test "missing root dir exits 2" {
    run bash "$SCRIPT" --root "$TMP_DIR/does-not-exist"
    [ "$status" -eq 2 ]
}
