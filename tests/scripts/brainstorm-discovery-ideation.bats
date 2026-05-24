#!/usr/bin/env bats

# Regression tests for the idea-wizard generate-winnow methodology wrapped into
# /brainstorm + /discovery (ag-yw0).
#
# This is a documentation/wiring gate: the generate-winnow methodology lives in
# SKILL.md prose + references + .feature scenarios (skills are markdown contracts,
# not executable code), so these tests assert the methodology is present and wired
# across the Claude skills, the ported references, and the Codex twins.
#
# Detailed logging (per the operationalize discipline being documented): each test
# echoes what it is checking before asserting, so a failure log names the exact
# missing surface.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    BRAINSTORM="$REPO_ROOT/skills/brainstorm/SKILL.md"
    DISCOVERY="$REPO_ROOT/skills/discovery/SKILL.md"
    CODEX_BRAINSTORM="$REPO_ROOT/skills-codex/brainstorm/SKILL.md"
    CODEX_DISCOVERY="$REPO_ROOT/skills-codex/discovery/SKILL.md"
    REF_IDEATION="$REPO_ROOT/skills/brainstorm/references/ideation-mode.md"
    REF_RUBRIC="$REPO_ROOT/skills/brainstorm/references/idea-rubric.md"
    REF_BEADS="$REPO_ROOT/skills/brainstorm/references/bead-operationalization.md"
    FEAT_BRAINSTORM="$REPO_ROOT/skills/brainstorm/references/brainstorm.feature"
    FEAT_DISCOVERY="$REPO_ROOT/skills/discovery/references/discovery.feature"
}

# Assert a file contains a literal substring, logging the probe first.
assert_has() {
    local file="$1" needle="$2"
    echo "# checking: $(basename "$(dirname "$file")")/$(basename "$file") contains: $needle" >&3
    grep -qF -- "$needle" "$file"
}

# Assert a file does NOT match an extended-regex pattern (anti-leak guard).
# Uses `run` + status so it fails the test on any bats version (no `run !`).
assert_lacks_regex() {
    local file="$1" pattern="$2"
    echo "# anti-leak: $(basename "$(dirname "$file")")/$(basename "$file") must NOT match: $pattern" >&3
    run grep -qE -- "$pattern" "$file"
    [ "$status" -ne 0 ]
}

# Assert a file does NOT contain a literal substring (anti-leak guard).
assert_lacks() {
    local file="$1" needle="$2"
    echo "# anti-leak: $(basename "$(dirname "$file")")/$(basename "$file") must NOT contain: $needle" >&3
    run grep -qF -- "$needle" "$file"
    [ "$status" -ne 0 ]
}

@test "all four SKILL.md surfaces and three ported references exist" {
    for f in "$BRAINSTORM" "$DISCOVERY" "$CODEX_BRAINSTORM" "$CODEX_DISCOVERY" \
             "$REF_IDEATION" "$REF_RUBRIC" "$REF_BEADS"; do
        echo "# exists? $f" >&3
        [ -f "$f" ]
    done
}

@test "brainstorm SKILL documents ideation mode with the generate-winnow funnel" {
    assert_has "$BRAINSTORM" "Ideation Mode"
    assert_has "$BRAINSTORM" "--ideate"
    assert_has "$BRAINSTORM" "Generate 30"
    assert_has "$BRAINSTORM" "best 5"
    assert_has "$BRAINSTORM" "next best 10"
    assert_has "$BRAINSTORM" "portfolio of "
    assert_has "$BRAINSTORM" "ranked best-to-worst"
}

@test "brainstorm SKILL preserves the existing four-phase goal-clarification flow" {
    # Additive guarantee: the original phases must remain present unchanged.
    assert_has "$BRAINSTORM" "Phase 1: Assess Clarity"
    assert_has "$BRAINSTORM" "Phase 2: Understand the Idea"
    assert_has "$BRAINSTORM" "Phase 3: Explore Approaches"
    assert_has "$BRAINSTORM" "Phase 4: Capture Design"
    assert_has "$BRAINSTORM" "Phase 3b: Adversarial Critique"
}

@test "brainstorm SKILL links all three ported references" {
    assert_has "$BRAINSTORM" "references/ideation-mode.md"
    assert_has "$BRAINSTORM" "references/idea-rubric.md"
    assert_has "$BRAINSTORM" "references/bead-operationalization.md"
}

@test "idea-rubric reference carries all ten evaluation dimensions" {
    for dim in Robust Reliable Performant Intuitive User-friendly Ergonomic \
               Useful Compelling Accretive Pragmatic; do
        assert_has "$REF_RUBRIC" "$dim"
    done
}

@test "bead-operationalization reference uses bd and bans br/bv tracker refs" {
    assert_has "$REF_BEADS" "bd create"
    assert_has "$REF_BEADS" "bd dep add"
    assert_has "$REF_BEADS" "DO NOT OVERSIMPLIFY"
    assert_has "$REF_BEADS" "DO NOT LOSE FEATURES"
    # idea-wizard's br/bv tracker must NOT leak into the ported repo-native docs.
    assert_lacks_regex "$REF_BEADS"    '\b(br|bv)\s+(list|create|dep|ready|--robot)'
    assert_lacks_regex "$REF_IDEATION" '\b(br|bv)\s+(list|create|dep|ready|--robot)'
    assert_lacks_regex "$REF_RUBRIC"   '\b(br|bv)\s+(list|create|dep|ready|--robot)'
}

@test "ideation-mode reference documents the mode-selection rule and grounding" {
    assert_has "$REF_IDEATION" "When to use which mode"
    assert_has "$REF_IDEATION" "AGENTS.md"
    assert_has "$REF_IDEATION" "bd list --json"
    assert_has "$REF_IDEATION" "bd list --status closed --json"
}

@test "discovery SKILL wires the open-ended ideate path with operationalize and refine" {
    assert_has "$DISCOVERY" "Open-Ended Path"
    assert_has "$DISCOVERY" "--ideate"
    assert_has "$DISCOVERY" "/brainstorm --ideate"
    assert_has "$DISCOVERY" "Operationalize"
    assert_has "$DISCOVERY" "self-documenting"
    assert_has "$DISCOVERY" "Refine in plan space"
    assert_has "$DISCOVERY" "4-5 refinement passes"
}

@test "discovery SKILL preserves the strict-delegation contract" {
    # Additive guarantee: the contract that predates this change must remain.
    assert_has "$DISCOVERY" "Strict Delegation Contract"
    assert_has "$DISCOVERY" "strict-delegation-contract.md"
    # The new path must explicitly keep delegation (no inlining the 30-idea gen).
    assert_has "$DISCOVERY" "do NOT inline the 30-idea generation"
}

@test "codex twins mirror ideation mode with codex notation and no Claude primitives" {
    assert_has "$CODEX_BRAINSTORM" "Ideation Mode"
    assert_has "$CODEX_BRAINSTORM" '$brainstorm --ideate'
    assert_has "$CODEX_DISCOVERY" "Open-Ended Path"
    assert_has "$CODEX_DISCOVERY" '$brainstorm --ideate'
    # Codex bodies must not leak Claude-era primitives (parity rule).
    assert_lacks "$CODEX_BRAINSTORM" "AskUserQuestion"
    assert_lacks "$CODEX_DISCOVERY" "AskUserQuestion"
    # Codex must use $skill notation, not /skill, for the new content.
    assert_lacks "$CODEX_BRAINSTORM" "/brainstorm --ideate"
    assert_lacks "$CODEX_DISCOVERY" "/brainstorm --ideate"
}

@test "feature specs add ideation and operationalize scenarios additively" {
    assert_has "$FEAT_BRAINSTORM" "triggers ideation mode"
    assert_has "$FEAT_BRAINSTORM" "winnows ruthlessly to a ranked five"
    assert_has "$FEAT_BRAINSTORM" "expands the portfolio to fifteen"
    assert_has "$FEAT_DISCOVERY" "generate-winnow path"
    assert_has "$FEAT_DISCOVERY" "operationalizes the winnowed portfolio"
    assert_has "$FEAT_DISCOVERY" "refines beads in plan space"
    # The original scenarios must survive untouched (scenario-hash stability).
    assert_has "$FEAT_BRAINSTORM" "a goal is clarified through the four phases"
    assert_has "$FEAT_DISCOVERY" "Discovery delegates to Plan"
}
