#!/usr/bin/env bats
#
# Tests for the artifact-classification schema validator
# (scripts/validate-skill-disposition-schema.sh, ag-4akl8 S0).
#
# The validator enforces the v4 additive schema on
# docs/contracts/skill-dispositions.yaml: every active `- skill:` row and every
# `workflows:` entry carries a `kind` in the closed enum {skill, workflow, loop}
# and a `capability_class` in the closed capability enum. A row with an unknown
# `kind` (or capability_class) is rejected by NAME so the offending row is
# obvious.
#
# Heredocs build a throwaway fixture yaml in BATS_TEST_TMPDIR so the canonical
# ledger is never mutated; DISP_YAML reaches the validator via the environment.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    VALIDATOR="$REPO_ROOT/scripts/validate-skill-disposition-schema.sh"
}

@test "canonical ledger passes the schema validator" {
    run bash "$VALIDATOR"
    [ "$status" -eq 0 ]
}

@test "an unknown kind value is rejected and the row is named" {
    fixture="$BATS_TEST_TMPDIR/disp.yaml"
    cat > "$fixture" <<'YAML'
historical: {}
workflows: {}
dispositions:
  - skill:            ok-skill
    domain:           "BC1 Corpus"
    hexagonal_role:   supporting
    disposition:      keep
    kind:             skill
    runtime_targets:  [claude, codex]
    parity_policy:    required
    capability_class: corpus
    path:             skills/ok-skill/SKILL.md
    aliases:          []
    supersedes:       null
    rationale:        "ok"
  - skill:            bad-kind-skill
    domain:           "BC1 Corpus"
    hexagonal_role:   supporting
    disposition:      keep
    kind:             gizmo
    runtime_targets:  [claude, codex]
    parity_policy:    required
    capability_class: corpus
    path:             skills/bad-kind-skill/SKILL.md
    aliases:          []
    supersedes:       null
    rationale:        "bad"
YAML
    DISP_YAML="$fixture" run bash "$VALIDATOR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bad-kind-skill"* ]]
    [[ "$output" == *"gizmo"* ]]
}

@test "an unknown capability_class value is rejected and the row is named" {
    fixture="$BATS_TEST_TMPDIR/disp.yaml"
    cat > "$fixture" <<'YAML'
historical: {}
workflows: {}
dispositions:
  - skill:            bad-cap-skill
    domain:           "BC1 Corpus"
    hexagonal_role:   supporting
    disposition:      keep
    kind:             skill
    runtime_targets:  [claude, codex]
    parity_policy:    required
    capability_class: nonsense
    path:             skills/bad-cap-skill/SKILL.md
    aliases:          []
    supersedes:       null
    rationale:        "bad"
YAML
    DISP_YAML="$fixture" run bash "$VALIDATOR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bad-cap-skill"* ]]
}

@test "a missing required additive field is rejected and the row is named" {
    fixture="$BATS_TEST_TMPDIR/disp.yaml"
    cat > "$fixture" <<'YAML'
historical: {}
workflows: {}
dispositions:
  - skill:            missing-field-skill
    domain:           "BC1 Corpus"
    hexagonal_role:   supporting
    disposition:      keep
    kind:             skill
    capability_class: corpus
    path:             skills/missing-field-skill/SKILL.md
    rationale:        "missing runtime_targets + parity_policy"
YAML
    DISP_YAML="$fixture" run bash "$VALIDATOR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"missing-field-skill"* ]]
}

@test "workflows entries are validated for kind too" {
    fixture="$BATS_TEST_TMPDIR/disp.yaml"
    cat > "$fixture" <<'YAML'
historical: {}
workflows:
  bad-workflow:
    kind:             contraption
    domain:           "BC3 Loop"
    hexagonal_role:   driving-adapter
    runtime_targets:  [claude]
    parity_policy:    exempt
    capability_class: orchestration
    aliases:          []
    path:             .claude/workflows/bad-workflow.js
    rationale:        "bad"
dispositions: []
YAML
    DISP_YAML="$fixture" run bash "$VALIDATOR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bad-workflow"* ]]
}

# --- BC6 + additive-no-rename scenarios (ag-4akl8 S0) ---------------------------

@test "BC6 passes the bounded-context drift gate with six contexts" {
    run bash "$REPO_ROOT/scripts/check-bounded-contexts-drift.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"6 BCs"* ]]
    # BC6 must appear in BOTH registry docs (the gate fails otherwise, but
    # assert the membership directly so the scenario is pinned).
    grep -q "BC6 Orchestration" "$REPO_ROOT/docs/reference/agentops-skill-domain-map.md"
    grep -q "BC6 Orchestration" "$REPO_ROOT/docs/reference/agentops-hexagonal-architecture-map.md"
}

@test "BC6 with no same-named cli command does not trip SKU coverage" {
    # The sku_catalog coverage check must pass even though BC6 members
    # (ntm/swarm/agent-mail/...) own no same-named `ao` subcommand: cli-command
    # coverage is conditional (carve-out), BC-skill coverage is the floor.
    run python3 - <<'PY'
import os, sys
sys.path.insert(0, os.path.join(os.environ["REPO_ROOT"], "scripts", "lib"))
import sku_catalog
assert "BC6" in sku_catalog.BOUNDED_CONTEXTS, "BC6 not in the enum"
# Synthesize a catalog where BC6 has an active skill but no cli-command SKU.
catalog = {
    "capabilities": [
        {"type": "skill", "name": "ntm", "status": "active", "bounded_context": "BC6"},
        {"type": "skill", "name": "compile", "status": "active", "bounded_context": "BC1"},
        {"type": "skill", "name": "review", "status": "active", "bounded_context": "BC2"},
        {"type": "skill", "name": "rpi", "status": "active", "bounded_context": "BC3"},
        {"type": "skill", "name": "skill-builder", "status": "active", "bounded_context": "BC4"},
        {"type": "skill", "name": "push", "status": "active", "bounded_context": "BC5"},
        # BC1 owns a cli command; BC6 owns none.
        {"type": "cli-command", "name": "compile", "bounded_context": "BC1"},
    ]
}
# Fill loop-move coverage so the only thing under test is BC coverage.
for move, cands in sku_catalog.LOOP_MOVES.items():
    catalog["capabilities"].append(
        {"type": "skill", "name": cands[0], "status": "active", "bounded_context": "BC3"}
    )
failures = sku_catalog.check_coverage(catalog)
bc6_fails = [f for f in failures if "BC6" in f]
assert not bc6_fails, f"BC6 tripped coverage: {bc6_fails}"
print("ok")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "the additive change leaves renamed-consumer parsers byte-untouched" {
    # resolve-skill-path.sh, skills_retire.go, heal.sh must match origin/main.
    for f in \
        scripts/lib/resolve-skill-path.sh \
        cli/cmd/ao/skills_retire.go \
        skills/heal-skill/scripts/heal.sh \
        skills-codex/heal-skill/scripts/heal.sh; do
        run git -C "$REPO_ROOT" diff --quiet origin/main -- "$f"
        [ "$status" -eq 0 ]
    done
    # The filename + row key + domain-string format are unchanged.
    grep -q "^  - skill:" "$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
    grep -q 'domain:.*"BC6 Orchestration"' "$REPO_ROOT/docs/contracts/skill-dispositions.yaml"
}
