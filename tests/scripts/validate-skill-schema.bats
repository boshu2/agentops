#!/usr/bin/env bats
# Tests for scripts/validate-skill-schema.sh — the skill.schema blocking gate.
#
# WHY THIS EXISTS: skill.schema gates every SKILL.md in the corpus and, until
# now, had no test at all. Nothing had ever demonstrated it can FAIL. A gate
# proven only against a green tree is indistinguishable from `exit 0` — replace
# its validator with a no-op and CI stays green. This file is the negative
# witness, and it prunes skill.schema from
# scripts/.gate-negative-witness-grandfather (the check-liveness ratchet in
# cli/internal/gates/checks/negative_witness_test.go).
#
# The script anchors REPO_ROOT to `dirname $0/..`, so a fixture tree with its
# own scripts/ + schemas/ + skills/ exercises the real validator against
# controlled input rather than against the live corpus.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    mkdir -p "$TMP_DIR/scripts" "$TMP_DIR/schemas" "$TMP_DIR/skills"
    cp "$REPO_ROOT/scripts/validate-skill-schema.sh" "$TMP_DIR/scripts/"
    cp "$REPO_ROOT/schemas/skill-frontmatter.v1.schema.json" "$TMP_DIR/schemas/"
    chmod +x "$TMP_DIR/scripts/validate-skill-schema.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_skill() {  # write_skill <slug> <frontmatter-body>
    mkdir -p "$TMP_DIR/skills/$1"
    { printf -- '---\n'; printf '%s\n' "$2"; printf -- '---\n\n# %s\n' "$1"; } \
        > "$TMP_DIR/skills/$1/SKILL.md"
}

run_gate() {
    ( cd "$TMP_DIR" && bash scripts/validate-skill-schema.sh )
}

# Descriptions are single-quoted: the corpus convention embeds `Triggers: "x"`
# inside the value, and unquoted that colon makes YAML read a nested mapping —
# which fails the gate for a PARSE error rather than the schema violation each
# negative below is meant to witness.
valid_frontmatter() {
    printf "name: alpha\ndescription: 'A valid fixture skill. Triggers: \"alpha\".'\nskill_api_version: 1"
}

@test "a schema-valid skill passes" {
    write_skill alpha "$(valid_frontmatter)"
    run run_gate
    [ "$status" -eq 0 ]
}

# --- negative witnesses: the gate must be shown to FAIL ----------------------

@test "NEGATIVE: a skill missing the required name field fails" {
    write_skill alpha "$(valid_frontmatter)"
    write_skill broken "description: 'Missing its name. Triggers: \"broken\".'
skill_api_version: 1"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *broken* ]]
}

@test "NEGATIVE: a skill missing the required description field fails" {
    write_skill alpha "$(valid_frontmatter)"
    write_skill broken 'name: broken
skill_api_version: 1'
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *broken* ]]
}

@test "NEGATIVE: a skill missing skill_api_version fails" {
    write_skill alpha "$(valid_frontmatter)"
    write_skill broken "name: broken
description: 'Has no api version. Triggers: \"broken\".'"
    run run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *broken* ]]
}

@test "NEGATIVE: one bad skill fails the run even when others are valid" {
    write_skill alpha "$(valid_frontmatter)"
    write_skill beta "$(printf "name: beta\ndescription: 'Another valid one. Triggers: \"beta\".'\nskill_api_version: 1")"
    write_skill broken "description: 'no name here'
skill_api_version: 1"
    run run_gate
    [ "$status" -eq 1 ]
}
