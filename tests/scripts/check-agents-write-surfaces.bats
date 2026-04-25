#!/usr/bin/env bats

# Regression tests for scripts/check-agents-write-surfaces.sh
# Builds a self-contained fake repo with cli/, scripts/, hooks/, lib/, skills/,
# docs/contracts/, and runs the lint inside it. Each scenario isolates one
# contract dimension so failures point at one rule.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-agents-write-surfaces.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/cli/cmd/ao" "$FAKE_REPO/cli/internal" \
             "$FAKE_REPO/hooks" "$FAKE_REPO/lib" "$FAKE_REPO/skills" \
             "$FAKE_REPO/docs/contracts"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-agents-write-surfaces.sh"
    chmod +x "$FAKE_REPO/scripts/check-agents-write-surfaces.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Helper: write a minimal contract doc with the given allowlist entries.
write_contract() {
    local doc="$FAKE_REPO/docs/contracts/agents-write-surfaces.md"
    {
        printf '# .agents/ Write Surfaces\n\n'
        printf '<!-- BEGIN agents-write-surfaces-allowlist -->\n'
        for entry in "$@"; do
            printf '%s\n' "$entry"
        done
        printf '<!-- END agents-write-surfaces-allowlist -->\n'
    } > "$doc"
}

@test "check-agents-write-surfaces.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "passes when every referenced subdir is allowlisted" {
    write_contract ao learnings patterns
    cat > "$FAKE_REPO/cli/internal/foo.go" <<'EOF'
package foo

const ChainDir = ".agents/ao"
const LearningsDir = ".agents/learnings"
EOF
    cat > "$FAKE_REPO/scripts/touch-pattern.sh" <<'EOF'
#!/usr/bin/env bash
mkdir -p .agents/patterns
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"contract OK"* ]]
}

@test "fails when production code references a subdir not in allowlist" {
    write_contract ao
    cat > "$FAKE_REPO/cli/internal/bad.go" <<'EOF'
package bad

const Stash = ".agents/widgets/output.json"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *"undocumented"* ]]
    [[ "$output" == *"widgets"* ]]
    [[ "$output" == *"cli/internal/bad.go"* ]]
}

@test "allows skill-owned subdirs without allowlist entry" {
    write_contract ao
    mkdir -p "$FAKE_REPO/skills/my-skill"
    : > "$FAKE_REPO/skills/my-skill/SKILL.md"
    cat > "$FAKE_REPO/cli/internal/skill_writer.go" <<'EOF'
package skillwriter

const SkillDir = ".agents/my-skill"
const ChainDir = ".agents/ao"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 0 ]
}

@test "fails when skill-named subdir is referenced but skill does not exist" {
    write_contract ao
    cat > "$FAKE_REPO/cli/internal/orphan.go" <<'EOF'
package orphan

const Dir = ".agents/ghost-skill/state.json"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *"ghost-skill"* ]]
}

@test "ignores Go test files" {
    write_contract ao
    cat > "$FAKE_REPO/cli/internal/foo_test.go" <<'EOF'
package foo

const FixtureDir = ".agents/test-fixture/data"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 0 ]
}

@test "exits 2 when contract doc is missing" {
    rm -f "$FAKE_REPO/docs/contracts/agents-write-surfaces.md"
    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"contract doc missing"* ]]
}

@test "exits 2 when allowlist block is empty" {
    write_contract
    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"empty"* ]]
}

@test "exits 2 on malformed allowlist entries" {
    local doc="$FAKE_REPO/docs/contracts/agents-write-surfaces.md"
    {
        printf '<!-- BEGIN agents-write-surfaces-allowlist -->\n'
        printf 'ok-name\n'
        printf 'BAD/SLASH\n'
        printf '<!-- END agents-write-surfaces-allowlist -->\n'
    } > "$doc"

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 2 ]
    [[ "$output" == *"malformed"* ]]
}

@test "supports inline comments and blank lines in allowlist" {
    local doc="$FAKE_REPO/docs/contracts/agents-write-surfaces.md"
    {
        printf '<!-- BEGIN agents-write-surfaces-allowlist -->\n'
        printf '\n'
        printf '# core state\n'
        printf 'ao\n'
        printf '\n'
        printf 'learnings   # promoted artifacts\n'
        printf '<!-- END agents-write-surfaces-allowlist -->\n'
    } > "$doc"
    cat > "$FAKE_REPO/cli/internal/foo.go" <<'EOF'
package foo

const A = ".agents/ao"
const B = ".agents/learnings"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh
    [ "$status" -eq 0 ]
}

@test "--json emits machine-readable summary" {
    write_contract ao
    cat > "$FAKE_REPO/cli/internal/bad.go" <<'EOF'
package bad

const Dir = ".agents/widgets"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh --json
    [ "$status" -eq 1 ]
    echo "$output" | jq -e '.status == "fail"'
    echo "$output" | jq -e '.undocumented | index("widgets")'
    echo "$output" | jq -e '.source_locations.widgets | index("cli/internal/bad.go")'
}

@test "--json emits ok status when everything documented" {
    write_contract ao
    cat > "$FAKE_REPO/cli/internal/foo.go" <<'EOF'
package foo

const A = ".agents/ao"
EOF

    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh --json
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.status == "ok"'
    echo "$output" | jq -e '.undocumented | length == 0'
}

@test "--help prints usage and exits 0" {
    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage:"* ]]
}

@test "rejects unknown flags with exit 2" {
    cd "$FAKE_REPO"
    run bash scripts/check-agents-write-surfaces.sh --bogus
    [ "$status" -eq 2 ]
}

@test "repo allowlist entries are referenced or explicitly lifecycle-only" {
    local lifecycle_only=""
    cd "$REPO_ROOT"
    run bash "$SCRIPT" --json
    [ "$status" -eq 0 ]

    missing="$(
        awk '
          /<!-- BEGIN agents-write-surfaces-allowlist -->/ { inside=1; next }
          /<!-- END agents-write-surfaces-allowlist -->/ { inside=0; next }
          inside {
            sub(/[[:space:]]+#.*$/, "")
            gsub(/^[[:space:]]+|[[:space:]]+$/, "")
            if ($0 != "" && $0 !~ /^#/) print $0
          }
        ' "$REPO_ROOT/docs/contracts/agents-write-surfaces.md" \
          | while IFS= read -r entry; do
              if [[ " $lifecycle_only " == *" $entry "* ]]; then
                  continue
              fi
              if ! echo "$output" | jq -e --arg entry "$entry" '.source_locations[$entry] | length > 0' >/dev/null; then
                  echo "$entry"
              fi
            done
    )"
    [ -z "$missing" ]
}
