#!/usr/bin/env bats
#
# Tests for scripts/check-docs-learning-references.sh — verifies that
# specific-dated .agents/learnings/YYYY-MM-DD-*.md references in
# docs/plans/ and docs/learnings/ have a matching docs/learnings/
# mirror OR an explicit (local-only)/(documentary)/(template) annotation.
#
# Sibling pattern: matches tests/scripts/check-three-gap-supergate.bats
# (cycle 56 operator-exemplar shape) — stub a temp repo with the script,
# create docs/plans + docs/learnings fixtures per test, run + assert
# exit code + presence of expected strings.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_HOME="$(mktemp -d)"

    SHIM_ROOT="$TMP_HOME/repo"
    mkdir -p "$SHIM_ROOT/scripts" "$SHIM_ROOT/docs/plans" "$SHIM_ROOT/docs/learnings"
    cp "$REPO_ROOT/scripts/check-docs-learning-references.sh" "$SHIM_ROOT/scripts/"
    chmod +x "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
}

teardown() {
    rm -rf "$TMP_HOME"
}

@test "mirror exists for dated reference → PASS" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
This plan cites `.agents/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md` as rationale.
EOF
    : > "$SHIM_ROOT/docs/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md"
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "missing mirror, no annotation → FAIL with fix hint" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
This plan cites `.agents/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md` for the parallel-session ownership pattern.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"docs/plans/foo.md"* ]]
    [[ "$output" == *"docs/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md"* ]]
    [[ "$output" == *"fix:"* ]]
}

@test "missing mirror but (local-only) annotation → PASS" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
Rationale: `.agents/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md` (local-only) — operator's machine has the full file.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "missing mirror but (documentary) annotation → PASS" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
For historical context, `.agents/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md` (documentary) is mentioned in the swarm skill.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "missing mirror but (template) annotation → PASS" {
    cat > "$SHIM_ROOT/docs/learnings/template-example.md" <<'EOF'
Example reference: `.agents/learnings/2026-01-15-example-session.md` (template) shows the expected shape.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "truncated date-prefix path (no .md suffix) → skipped silently" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
Some prose mentions `.agents/learnings/2026-05-11-...` as a path family without a full filename.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "reference outside docs/plans + docs/learnings → silent" {
    mkdir -p "$SHIM_ROOT/docs/rfcs"
    cat > "$SHIM_ROOT/docs/rfcs/foo.md" <<'EOF'
This RFC references `.agents/learnings/2026-04-12-tier1-forge-parallel-session-hazards.md` documentarily.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "--warn-only flag downgrades FAIL to WARN exit 0" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
Missing-mirror reference: `.agents/learnings/2026-04-12-some-learning.md` with no annotation.
EOF
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh" --warn-only
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
}

@test "multiple references, some mirrored some not → FAIL counts unmirrored" {
    cat > "$SHIM_ROOT/docs/plans/foo.md" <<'EOF'
First ref `.agents/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md` is mirrored.
Second ref `.agents/learnings/2026-04-12-missing-learning.md` is not mirrored or annotated.
EOF
    : > "$SHIM_ROOT/docs/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md"
    run bash "$SHIM_ROOT/scripts/check-docs-learning-references.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"1 un-mirrored"* ]]
}
