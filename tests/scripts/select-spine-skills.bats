#!/usr/bin/env bats
# Tests for scripts/select-spine-skills.sh and the install.sh --tier spine lever
# (age-h4y3): a spine-only install ships just the proven spine skills
# (spine: true frontmatter) instead of the whole bundle.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SELECTOR="$ROOT/scripts/select-spine-skills.sh"
    INSTALL="$ROOT/scripts/install.sh"
    [ -f "$SELECTOR" ] || skip "select-spine-skills.sh missing"
}

@test "selector returns the known spine skills from the live skills/ tree" {
    run bash "$SELECTOR" "$ROOT/skills"
    [ "$status" -eq 0 ]
    # Representative spine skills (spine: true) must be present.
    echo "$output" | grep -qx "beads-br"
    echo "$output" | grep -qx "converge"
    echo "$output" | grep -qx "council"
    # Permanent compatibility pointers for a spine skill must survive the same
    # bundle pruning as their canonical target.
    echo "$output" | grep -qx "premortem"
    echo "$output" | grep -qx "pre-mortem"
    echo "$output" | grep -qx "pre_mortem"
}

@test "selector excludes non-spine (experimental/corpus) skills" {
    run bash "$SELECTOR" "$ROOT/skills"
    [ "$status" -eq 0 ]
    # autodev + evolve are experimental-tier (not spine: true).
    ! echo "$output" | grep -qx "autodev"
    ! echo "$output" | grep -qx "evolve"
}

@test "selector counts frontmatter spine:true, not a body/prose mention, and sorts" {
    fx="$BATS_TEST_TMPDIR/skills"
    mkdir -p "$fx"/{spine-b,spine-a,exp-prose,exp-none,legacy-spine,legacy-exp}
    printf -- '---\nname: spine-b\nspine: true\n---\nbody\n' >"$fx/spine-b/SKILL.md"
    printf -- '---\nname: spine-a\nspine: true\n---\nbody\n' >"$fx/spine-a/SKILL.md"
    # spine: true appears only in the BODY — must NOT be selected.
    printf -- '---\nname: exp-prose\nspine: false\n---\nsee spine: true note\n' >"$fx/exp-prose/SKILL.md"
    printf -- '---\nname: exp-none\n---\nbody\n' >"$fx/exp-none/SKILL.md"
    printf -- '---\nname: legacy-spine\nredirect_to: spine-a\nimplementation: false\n---\nbody\n' >"$fx/legacy-spine/SKILL.md"
    printf -- '---\nname: legacy-exp\nredirect_to: exp-none\nimplementation: false\n---\nbody\n' >"$fx/legacy-exp/SKILL.md"

    run bash "$SELECTOR" "$fx"
    [ "$status" -eq 0 ]
    # The canonical spine skills plus only the alias targeting a spine skill,
    # all sorted. An alias cannot promote an experimental target.
    [ "$output" = "$(printf 'legacy-spine\nspine-a\nspine-b')" ]
}

@test "selector errors without a skills-root argument" {
    run bash "$SELECTOR"
    [ "$status" -eq 2 ]
}

@test "install.sh rejects an invalid --tier value" {
    [ -f "$INSTALL" ] || skip "install.sh missing"
    run bash "$INSTALL" --tier bogus
    [ "$status" -eq 2 ]
    echo "$output" | grep -q "Invalid --tier"
}

@test "install.sh --help documents the spine tier" {
    [ -f "$INSTALL" ] || skip "install.sh missing"
    run bash "$INSTALL" --help
    [ "$status" -eq 0 ]
    echo "$output" | grep -q -- "--tier"
    echo "$output" | grep -q "spine"
}
