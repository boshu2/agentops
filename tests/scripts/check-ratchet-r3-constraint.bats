#!/usr/bin/env bats
#
# Tests for scripts/check-ratchet-r3-constraint.sh — the Ratchet R3 gate
# ("no learning without a constraint"). Runs against a synthetic learnings
# fixture (NOT the operator's gitignored .agents/learnings/ corpus), so the
# test is hermetic and committed-and-gated in CI even though the real corpus
# is local-only.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-ratchet-r3-constraint.sh"

    TMP_DIR="$(mktemp -d)"
    LEARN="$TMP_DIR/learnings"
    mkdir -p "$LEARN"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Helper: write a learning file with given maturity + body.
write_learning() {
    local name="$1" maturity="$2" body="$3"
    cat > "$LEARN/$name" <<EOF
---
type: learning
maturity: $maturity
confidence: high
---

$body
EOF
}

@test "durable learning that cites a script gate passes" {
    write_learning "2026-06-06-durable-with-gate.md" "established" \
        "Encoded as a gate in scripts/check-ratchet-r3-constraint.sh so it can't regress."

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"R3 gate passed"* ]]
}

@test "durable learning that cites a frontmatter constraint field passes" {
    cat > "$LEARN/2026-06-06-fm-constraint.md" <<'EOF'
---
type: learning
maturity: canonical
enforced_by: .github/workflows/validate.yml::process-hygiene
---

The doctrine rule is now a CI gate.
EOF

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"R3 gate passed"* ]]
}

@test "durable learning with NO constraint warns but exits 0 (warn-first)" {
    write_learning "2026-06-06-durable-no-constraint.md" "established" \
        "We learned that warm caches matter, but never compiled it into anything."

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN R3"* ]]
    [[ "$output" == *"WARN-ONLY"* ]]
}

@test "durable learning with NO constraint FAILS under --strict" {
    write_learning "2026-06-06-durable-no-constraint.md" "established" \
        "We learned that warm caches matter, but never compiled it into anything."

    run bash "$SCRIPT" --strict "$LEARN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL R3"* ]]
    [[ "$output" == *"R3 gate FAILED (strict)"* ]]
}

@test "RATCHET_R3_BLOCKING=true is equivalent to --strict" {
    write_learning "2026-06-06-durable-no-constraint.md" "candidate" \
        "Durable claim with no gate, test, or SKILL behind it."

    RATCHET_R3_BLOCKING=true run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"R3 gate FAILED (strict)"* ]]
}

@test "provisional learning without a constraint is exempt" {
    write_learning "2026-06-06-provisional.md" "provisional" \
        "Early observation, not yet durable, no constraint expected."

    run bash "$SCRIPT" --strict "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 missing a constraint"* ]]
}

@test "durable learning citing a test surface passes" {
    write_learning "2026-06-06-durable-test.md" "stable" \
        "Regression-guarded in cli/internal/ratchet/gate_test.go::TestR3."

    run bash "$SCRIPT" --strict "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"R3 gate passed"* ]]
}

@test "durable learning citing a SKILL step passes" {
    write_learning "2026-06-06-durable-skill.md" "established" \
        "Encoded as a step in skills/evolve/SKILL.md."

    run bash "$SCRIPT" --strict "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"R3 gate passed"* ]]
}

@test "empty frontmatter field does NOT count as a constraint citation" {
    cat > "$LEARN/2026-06-06-empty-field.md" <<'EOF'
---
type: learning
maturity: established
constraint:
---

Has a constraint key but no value — must still be flagged.
EOF

    run bash "$SCRIPT" --strict "$LEARN"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL R3"* ]]
}

@test "missing learnings directory is a clean no-op" {
    run bash "$SCRIPT" "$TMP_DIR/does-not-exist"
    [ "$status" -eq 0 ]
    [[ "$output" == *"nothing to check"* ]]
}

@test "mixed corpus reports correct durable + flagged counts" {
    write_learning "2026-06-06-a-good.md"  "established" "Gated via scripts/foo.sh."
    write_learning "2026-06-06-b-bad.md"   "established" "No constraint here."
    write_learning "2026-06-06-c-prov.md"  "provisional" "Still forming."

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"2 durable-tier, 1 missing a constraint"* ]]
}
