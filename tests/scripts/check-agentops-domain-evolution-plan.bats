#!/usr/bin/env bats
# Tests for scripts/check-agentops-domain-evolution-plan.sh.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-agentops-domain-evolution-plan.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/docs/reference" \
        "$FAKE_REPO/skills/alpha" "$FAKE_REPO/skills/beta"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-agentops-domain-evolution-plan.sh"
    chmod +x "$FAKE_REPO/scripts/check-agentops-domain-evolution-plan.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_valid_fixture() {
    cat > "$FAKE_REPO/skills/alpha/SKILL.md" <<'EOF'
---
name: alpha
description: Alpha skill.
---
# Alpha
EOF
    cat > "$FAKE_REPO/skills/beta/SKILL.md" <<'EOF'
---
name: beta
description: Beta skill.
---
# Beta
EOF
    cat > "$FAKE_REPO/docs/reference/agentops-domain-evolution-bdd.md" <<'EOF'
# AgentOps Domain Evolution BDD

Feature: evolve the context compiler
Scenario: classify skills into domains
Given AgentOps is a context compiler and SDLC control plane
When agents run ao evolve with landing-policy off
Then they drive small provable changes with the source-built CLI
EOF
    cat > "$FAKE_REPO/docs/reference/agentops-skill-domain-map.md" <<'EOF'
# AgentOps Skill Domain Map

AgentOps is a context compiler for small provable changes in the SDLC control plane.

| Skill | Bounded context | Disposition |
| --- | --- | --- |
| `alpha` | BC1 Corpus | keep |
| `beta` | BC2 Validation | update |

| Summary | Count |
| --- | ---: |
| Skills audited | 2 |

BC1 Corpus
BC2 Validation
BC3 Loop
BC4 Factory
BC5 Runtime

keep update refactor merge-review cut-review
EOF
    cat > "$FAKE_REPO/docs/reference/agentops-hexagonal-architecture-map.md" <<'EOF'
# AgentOps Hexagonal Architecture Map

AgentOps is a context compiler and SDLC control plane for small provable changes.

BC1 Corpus
BC2 Validation
BC3 Loop
BC4 Factory
BC5 Runtime

HypothesisLedgerPort
ConvergenceCheckPort
CorpusReaderPort
GateRunnerPort
HarnessPort

ao evolve
landing-policy off
source-built CLI
EOF
    cat > "$FAKE_REPO/docs/reference/agentops-domain-evolution-plan.md" <<'EOF'
# AgentOps Domain Evolution Plan

Use soc-y5vh evidence to run the context compiler as an SDLC control plane.
The loop lands small provable changes with ao evolve, landing-policy off, and
the source-built CLI.
EOF
}

@test "passes when BDD, domain map, architecture map, and plan align" {
    write_valid_fixture

    run "$FAKE_REPO/scripts/check-agentops-domain-evolution-plan.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: domain evolution plan covers 2 skills"* ]]
}

@test "fails when the domain map misses a skill" {
    write_valid_fixture
    perl -0pi -e 's/\| `beta` \| BC2 Validation \| update \|\n//' \
        "$FAKE_REPO/docs/reference/agentops-skill-domain-map.md"

    run "$FAKE_REPO/scripts/check-agentops-domain-evolution-plan.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"domain map missing skill: beta"* ]]
}
