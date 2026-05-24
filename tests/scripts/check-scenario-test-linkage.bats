#!/usr/bin/env bats

# Regression tests for scripts/check-scenario-test-linkage.sh (soc-63xfx).
# Builds a self-contained fake repo (scripts/, skills/<name>/references/, and a
# tests/ tree for @covered-by targets), then runs the gate inside it. The script
# resolves REPO_ROOT relative to its own location, so copying it into the fake
# repo's scripts/ dir scopes every check to the fixture.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-scenario-test-linkage.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" \
             "$FAKE_REPO/skills/foo/references" \
             "$FAKE_REPO/skills/bar/references" \
             "$FAKE_REPO/tests/e2e"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-scenario-test-linkage.sh"
    chmod +x "$FAKE_REPO/scripts/check-scenario-test-linkage.sh"
    FAKE_SCRIPT="$FAKE_REPO/scripts/check-scenario-test-linkage.sh"

    # A real test file that @covered-by can resolve to, with a named function.
    cat > "$FAKE_REPO/tests/e2e/foo.sh" <<'EOF'
#!/usr/bin/env bash
TestFooCoverage() { :; }
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Write a feature file. $1=skill, $2=tag-line-or-empty, rest=scenario names.
write_feature() {
    local skill="$1" tag="$2"; shift 2
    local f="$FAKE_REPO/skills/$skill/references/$skill.feature"
    {
        printf '# fake spec\n\nFeature: %s feature\n\n' "$skill"
        for scen in "$@"; do
            [ -n "$tag" ] && printf '  %s\n' "$tag"
            printf '  Scenario: %s\n    When x\n    Then y\n\n' "$scen"
        done
    } > "$f"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "PASS: scenario with a resolving @covered-by path" {
    write_feature foo "@covered-by:tests/e2e/foo.sh" "covered scenario"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"1 linked"* ]]
}

@test "PASS: scenario with @covered-by path::Name when the name exists" {
    write_feature foo "@covered-by:tests/e2e/foo.sh::TestFooCoverage" "named cover"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "PASS: allowlisted doc-only feature with no tags" {
    printf 'skills/foo/references/foo.feature\nskills/bar/references/bar.feature\n' \
        > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature foo "" "untagged one" "untagged two"
    write_feature bar "" "untagged three"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"allowlisted"* ]]
}

@test "FAIL: scenario with no tag and file not allowlisted" {
    write_feature foo "" "orphan scenario"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no @covered-by tag and the file is not allowlisted"* ]]
}

@test "FAIL: dangling @covered-by path (file missing)" {
    write_feature foo "@covered-by:tests/e2e/missing.sh" "dangling scenario"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"dangling"* ]]
    [[ "$output" == *"test path does not exist"* ]]
}

@test "FAIL: dangling @covered-by path::Name (name missing in file)" {
    write_feature foo "@covered-by:tests/e2e/foo.sh::TestDoesNotExist" "named dangling"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"not found in"* ]]
}

@test "FAIL: allowlisted file that also carries a @covered-by tag (ambiguous)" {
    write_feature foo "@covered-by:tests/e2e/foo.sh" "tagged but allowlisted"
    printf 'skills/foo/references/foo.feature\nskills/bar/references/bar.feature\n' \
        > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"allowlisted (doc-only) file but also declares @covered-by"* ]]
}

@test "FAIL: stale allowlist entry (listed file does not exist)" {
    write_feature foo "@covered-by:tests/e2e/foo.sh" "covered"
    printf 'skills/foo/references/foo.feature\n' > /dev/null  # foo is linked, not allowlisted
    printf 'skills/ghost/references/ghost.feature\nskills/bar/references/bar.feature\n' \
        > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no longer exists"* ]]
}

@test "--warn-only downgrades a failure to exit 0" {
    write_feature foo "" "orphan scenario"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT" --warn-only
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
}

@test "--json emits a machine-readable summary" {
    write_feature foo "@covered-by:tests/e2e/foo.sh" "covered"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT" --json
    [ "$status" -eq 0 ]
    [[ "$output" == *'"result":"pass"'* ]]
    [[ "$output" == *'"scenarios_linked":1'* ]]
}

@test "file-level tag (above Feature) covers all scenarios" {
    local f="$FAKE_REPO/skills/foo/references/foo.feature"
    {
        printf '# fake spec\n\n'
        printf '@covered-by:tests/e2e/foo.sh\n'
        printf 'Feature: foo feature\n\n'
        printf '  Scenario: one\n    When x\n    Then y\n\n'
        printf '  Scenario: two\n    When x\n    Then y\n'
    } > "$f"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    write_feature bar "" "doc only"
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"2 linked"* ]]
}

@test "misuse: unknown flag exits 2" {
    write_feature bar "" "x"
    printf 'skills/bar/references/bar.feature\n' > "$FAKE_REPO/scripts/.scenario-linkage-allow"
    run bash "$FAKE_SCRIPT" --bogus
    [ "$status" -eq 2 ]
}
