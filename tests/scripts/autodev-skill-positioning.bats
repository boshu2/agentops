#!/usr/bin/env bats
#
# soc-ozoqh scenario 1: "autodev reads as config not loop".
#
# The canonical vocabulary rule skills/domain/references/autodev.md:9,13 says
# autodev is the config/intent layer the loop reads — NOT a loop — and that its
# positioning must "never lead with 'bounded autonomous dev loops'". This gate
# pins the autodev skill description + its generated mirrors to that rule so the
# corpus cannot drift back to loop-framing.
#
# Scoped to the autodev skill + generated catalogs. The style guide
# skills/domain/references/autodev.md is intentionally NOT checked: it QUOTES the
# banned phrase as the anti-pattern it forbids, so banning it there is self-defeating.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    SKILL="$REPO_ROOT/skills/autodev/SKILL.md"
    DESC_LINE="$(grep -m1 '^description:' "$SKILL")"
}

BANNED="bounded autonomous dev loops"

@test "autodev skill description drops the loop-framing phrase" {
    [[ "$DESC_LINE" != *"$BANNED"* ]]
}

@test "autodev skill description leads with the PROGRAM.md/AUTODEV.md contract" {
    [[ "$DESC_LINE" == *"PROGRAM.md"* || "$DESC_LINE" == *"AUTODEV.md"* ]]
    [[ "$DESC_LINE" == *"contract"* ]]
}

@test "autodev skill body never frames autodev as running the loop unattended" {
    run grep -c "$BANNED" "$SKILL"
    [ "$output" -eq 0 ]
    # canonical rule: "The loop consumes it; it does not run it."
    run grep -n "runs the loop unattended" "$SKILL"
    [ "$status" -ne 0 ]
}

@test "autodev skill body names it as the config layer the loop reads" {
    run grep -niE "config(/intent)? layer" "$SKILL"
    [ "$status" -eq 0 ]
}

@test "derived skill catalogs drop the loop-framing phrase for autodev" {
    for f in skills/catalog.json registry.json docs/contracts/context-map.md; do
        run grep -c "$BANNED" "$REPO_ROOT/$f"
        [ "$output" -eq 0 ]
    done
}

@test "the codex autodev mirror drops the loop-framing phrase" {
    run grep -c "$BANNED" "$REPO_ROOT/skills-codex/autodev/SKILL.md"
    [ "$output" -eq 0 ]
}
