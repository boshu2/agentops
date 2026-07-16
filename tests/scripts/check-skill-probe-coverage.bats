#!/usr/bin/env bats
#
# Tests for scripts/check-skill-probe-coverage.sh — the advisory
# skill.probe-coverage gate (age-e508.1).
#
# The gate NAMES every product-/judgment-tier skill that lacks a behavioral
# probe RESULT in the MEASURED ledger of skills/SKILL-TIERS.md. It is
# advisory-first: default mode reports findings but exits 0 (warn); --strict
# flips to a hard fail (the same warn-then-fail flip discipline as the egwt
# gates). "Has a probe result" = a ledger row whose verdict is BEHAVIORAL or
# INERT; an UNMEASURED verdict or an absent row is NOT a result.
#
# The gate is fixture-driven via env overrides (SKILL_PROBE_SKILLS_DIR,
# SKILL_PROBE_TIERS_FILE) so a fixture skills tree + tiers file can be pointed at
# without copying the repo.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    GATE="$REPO_ROOT/scripts/check-skill-probe-coverage.sh"

    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/skills"
    export SKILL_PROBE_SKILLS_DIR="$FIX/skills"
    export SKILL_PROBE_TIERS_FILE="$FIX/SKILL-TIERS.md"
}

# make_skill <name> <tier> — write a minimal SKILL.md carrying a metadata tier.
make_skill() {
    local name="$1" tier="$2"
    mkdir -p "$SKILL_PROBE_SKILLS_DIR/$name"
    cat > "$SKILL_PROBE_SKILLS_DIR/$name/SKILL.md" <<EOF
---
name: $name
description: fixture skill $name
metadata:
  tier: $tier
---
# $name
EOF
}

make_redirect() {
    local name="$1"
    mkdir -p "$SKILL_PROBE_SKILLS_DIR/$name"
    cat > "$SKILL_PROBE_SKILLS_DIR/$name/SKILL.md" <<EOF
---
name: $name
implementation: false
---
Use the canonical skill instead.
EOF
}

# write_ledger <rows...> — write a SKILL-TIERS.md carrying a MEASURED probe
# ledger. Each arg is a table row body "skill | probe | date | verdict".
write_ledger() {
    {
        echo "# Skill Tier Taxonomy"
        echo
        echo "## Behavioral Probe Ledger (MEASURED)"
        echo
        echo "| Skill | Probe ID | Date | Verdict |"
        echo "|-------|----------|------|---------|"
        local row
        for row in "$@"; do
            echo "| $row |"
        done
    } > "$SKILL_PROBE_TIERS_FILE"
}

@test "a product-tier skill absent from the ledger is NAMED and --strict FAILS" {
    make_skill foo product
    write_ledger   # empty ledger
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"foo"* ]]
}

@test "default (advisory) mode NAMES the skill but exits 0 (warn-first)" {
    make_skill foo product
    write_ledger
    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"foo"* ]]
    [[ "$output" == *"WARN"* ]]
}

@test "a judgment-tier skill with no probe is flagged too" {
    make_skill val judgment
    write_ledger
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"val"* ]]
}

@test "a product-tier skill WITH a BEHAVIORAL ledger row is NOT flagged" {
    make_skill foo product
    write_ledger "foo | probe-foo | 2026-07-08 | BEHAVIORAL"
    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
    [[ "$output" != *"foo lacks"* ]]
}

@test "an INERT ledger verdict counts as a measured result (not flagged)" {
    make_skill foo product
    write_ledger "foo | probe-foo | 2026-07-08 | INERT"
    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
}

@test "an UNMEASURED ledger verdict does NOT count — still flagged" {
    make_skill foo product
    write_ledger "foo | probe-foo | 2026-07-08 | UNMEASURED"
    run bash "$GATE" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"foo"* ]]
}

@test "execution-tier skills are exempt (not required to carry a probe)" {
    make_skill bar execution
    write_ledger
    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
    [[ "$output" != *"bar"* ]]
}

@test "redirect-only skills are exempt and cannot abort the advisory scan" {
    make_skill foo product
    make_redirect legacy-foo
    write_ledger "foo | probe-foo | 2026-07-08 | BEHAVIORAL"
    run bash "$GATE" --strict
    [ "$status" -eq 0 ]
    [[ "$output" != *"legacy-foo"* ]]
}

@test "a missing ledger file degrades to advisory (product/judgment flagged, exit 0 default)" {
    make_skill foo product
    rm -f "$SKILL_PROBE_TIERS_FILE"
    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"foo"* ]]
}

@test "the real repo gate is advisory: exits 0 in default mode even with unmeasured skills" {
    unset SKILL_PROBE_SKILLS_DIR SKILL_PROBE_TIERS_FILE
    run bash "$GATE"
    [ "$status" -eq 0 ]
}
