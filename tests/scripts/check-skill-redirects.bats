#!/usr/bin/env bats
#
# Tests for the folded-skill redirect-validity gate (age-rhlx):
# scripts/check-skill-redirects.sh asserts that every `merged-into` disposition
# in docs/contracts/skill-dispositions.yaml resolves — following the
# merged-into chain — to a LIVE skill (skills/<name>/SKILL.md), with no cycles.
#
# WHY: a folded skill is a redirect. If a later rename/prune deletes the fold
# TARGET, the redirect silently points at a 404 and inbound traffic dead-ends.
# This gate keeps the folded-skill map's targets real forever.
#
# The gate derives repo_root from `git rev-parse --show-toplevel`, so each case
# stamps a minimal repo fixture (a ledger + skills/<name>/SKILL.md dirs) and
# asserts the verdict.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GATE="$REPO_ROOT/scripts/check-skill-redirects.sh"
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts" "$FIX/docs/contracts" "$FIX/skills"
    cp "$GATE" "$FIX/scripts/check-skill-redirects.sh"
    chmod +x "$FIX/scripts/check-skill-redirects.sh"
    git -C "$FIX" init -q
    git -C "$FIX" config user.email t@t.t
    git -C "$FIX" config user.name t
}

# live_skill <name> — create a live skills/<name>/SKILL.md
live_skill() { mkdir -p "$FIX/skills/$1"; printf -- '---\nname: %s\n---\nbody\n' "$1" > "$FIX/skills/$1/SKILL.md"; }

run_gate() { ( cd "$FIX" && bash scripts/check-skill-redirects.sh ); }

@test "passes when every merged-into target resolves to a live skill" {
    live_skill discovery
    live_skill review
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  brainstorm:
    state:        merged-into
    merged-into:  discovery
  bug-hunt:
    state:        merged-into
    merged-into:  review
  review:
    state:        active
YAML
    run run_gate
    [ "$status" -eq 0 ]
}

@test "fails when a fold target does not exist as a live skill" {
    live_skill discovery
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  brainstorm:
    state:        merged-into
    merged-into:  discovery
  bug-hunt:
    state:        merged-into
    merged-into:  review
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"bug-hunt"* ]]
    [[ "$output" == *"review"* ]]
}

@test "resolves a transitive chain (a -> b -> live)" {
    live_skill crank
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  ship-loop:
    state:        merged-into
    merged-into:  burndown
  burndown:
    state:        merged-into
    merged-into:  crank
YAML
    run run_gate
    [ "$status" -eq 0 ]
}

@test "fails on a cycle in the merged-into chain" {
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  a:
    state:        merged-into
    merged-into:  b
  b:
    state:        merged-into
    merged-into:  a
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"cycle"* ]]
}

@test "fails on a cycle even when a node in it still has a live SKILL.md" {
    # Regression (pawl codex catch): a -> b -> a where b also has a stale live
    # SKILL.md must NOT short-circuit as resolved. A folded skill is declared
    # gone; it is never a valid redirect terminal, so the cycle is still caught.
    live_skill b
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  a:
    state:        merged-into
    merged-into:  b
  b:
    state:        merged-into
    merged-into:  a
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"cycle"* ]]
}

@test "a cut/retired skill needs no target" {
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
skills:
  reverse-engineer-rpi:
    state:        cut
YAML
    run run_gate
    [ "$status" -eq 0 ]
}
