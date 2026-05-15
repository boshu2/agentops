#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    REPO="$TMP_DIR/repo"
    VALIDATOR="$REPO/scripts/validate-practice-citations.sh"

    mkdir -p "$REPO/scripts"
    cp "$ROOT/scripts/validate-practice-citations.sh" "$VALIDATOR"
    chmod +x "$VALIDATOR"

    cat > "$REPO/PRACTICE.md" <<'EOF'
# Practice Fixture

## Practice slugs (canonical registry)

| Slug | Era | What it names |
|------|-----|---------------|
| `adr` | fixture | Architecture decisions |
| `snapshot-testing` | fixture | Golden artifacts |
| `ddd-bounded-context` | fixture | Bounded contexts |
| `tdd` | fixture | Test-first development |
| `ai-assisted-dev` | fixture | AI-assisted development |
| `design-by-contract` | fixture | Contracts |

## End
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "strict mode ignores declaration-optional scripts without practice headers" {
    cat > "$REPO/scripts/no-citation.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
EOF

    run bash "$VALIDATOR" --strict

    [ "$status" -eq 0 ]
    [[ "$output" == *"declaration-optional without practices: 1"* ]]
    [[ "$output" == *"No findings."* ]]
}

@test "strict mode accepts known slugs in script practice headers" {
    cat > "$REPO/scripts/check-good.sh" <<'EOF'
#!/usr/bin/env bash
# practices: [tdd, design-by-contract]
set -euo pipefail
EOF

    run bash "$VALIDATOR" --strict

    [ "$status" -eq 0 ]
    [[ "$output" == *"No findings."* ]]
}

@test "strict mode rejects unknown slugs in script practice headers" {
    cat > "$REPO/scripts/check-bad.sh" <<'EOF'
#!/usr/bin/env bash
# practices: [unknown-script-practice]
set -euo pipefail
EOF

    run bash "$VALIDATOR" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"INVALID_SLUG: scripts/check-bad.sh cites unknown slug \"unknown-script-practice\""* ]]
}

@test "json report includes invalid script slug findings" {
    cat > "$REPO/scripts/check-bad.sh" <<'EOF'
#!/usr/bin/env bash
# practices: [unknown-script-practice]
set -euo pipefail
EOF

    run bash "$VALIDATOR" --strict --json

    [ "$status" -eq 1 ]
    [[ "$output" == *"\"mode\": \"strict\""* ]]
    [[ "$output" == *"INVALID_SLUG"* ]]
    [[ "$output" == *"scripts/check-bad.sh cites unknown slug \\\"unknown-script-practice\\\""* ]]
}
