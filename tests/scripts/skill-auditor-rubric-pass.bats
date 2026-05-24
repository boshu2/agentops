#!/usr/bin/env bats

# Regression test for skill-auditor Pass 3 (rubric scoring) — soc-ads5v.
# Pass 3 folds the 10-category Skill Quality Rubric
# (docs/reference/skill-quality-rubric.md) into audit-report.json via
# score_agentops_skill.py --audit-block. The score is advisory: it must NOT
# change the PASS/WARN/FAIL verdict. Scoring must be deterministic + explainable
# (each category gets a 0-3 score and a reason).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    AUDIT="$REPO_ROOT/skills/skill-auditor/scripts/audit.sh"
    SCORE="$REPO_ROOT/skills/skill-auditor/scripts/score_agentops_skill.py"

    # The 10 rubric categories, verbatim from docs/reference/skill-quality-rubric.md.
    EXPECTED_CATEGORIES=(
        trigger_quality kernel_clarity progressive_disclosure helper_scripts
        validation self_test assets_templates subagents_roles safety_boundaries
        packaging
    )

    TMP_DIR="$(mktemp -d)"
    FIXTURE="$TMP_DIR/sample-skill"
    mkdir -p "$FIXTURE"
    cat > "$FIXTURE/SKILL.md" <<'EOF'
---
name: sample-skill
description: 'Do a thing. Use when you need to validate a sample.'
---
# /sample-skill — sample

## ⚠️ Critical Constraints

- **Never** mutate the target. **Why:** read-only contract.

## Output Specification

**Format:** JSON written to a file. **Filename:** result.json
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "scorer exposes --audit-block mode for Pass 3" {
    run python3 "$SCORE" "$FIXTURE" --audit-block
    [ "$status" -eq 0 ]
    [[ "$output" == *'"total_score"'* ]]
    [[ "$output" == *'"max_score": 30'* ]]
    [[ "$output" == *'"advisory": true'* ]]
}

@test "audit.sh folds a rubric block with all 10 categories into the report" {
    run bash "$AUDIT" "$FIXTURE" --json "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]

    python3 - "$TMP_DIR/report.json" <<PY
import json, sys
report = json.load(open(sys.argv[1]))
rubric = report["rubric"]
assert rubric is not None, "rubric must be present"
assert rubric["max_score"] == 30, rubric["max_score"]
assert rubric["advisory"] is True
expected = "${EXPECTED_CATEGORIES[*]}".split()
got = [c["category"] for c in rubric["categories"]]
assert got == expected, f"category drift: {got} != {expected}"
for c in rubric["categories"]:
    assert 0 <= c["score"] <= 3, c
    assert c["reason"], f"missing reason for {c['category']}"
assert 0 <= rubric["total_score"] <= 30
assert rubric["rating"] in ("C", "B", "A", "S")
print("rubric block OK")
PY
}

@test "rubric scoring is deterministic across runs" {
    run python3 "$SCORE" "$FIXTURE" --audit-block
    [ "$status" -eq 0 ]
    first="$output"
    run python3 "$SCORE" "$FIXTURE" --audit-block
    [ "$status" -eq 0 ]
    [ "$output" = "$first" ]
}

@test "Pass 3 rubric is advisory and does not change the verdict" {
    # Fixture has a single-line description without Triggers markers -> Pass 2
    # description/trigger checks WARN. Verdict should be WARN regardless of the
    # rubric score.
    run bash "$AUDIT" "$FIXTURE" --json "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]
    verdict="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['verdict'])" "$TMP_DIR/report.json")"
    [ "$verdict" = "WARN" ]

    # The rubric score for this minimal fixture is low (no scripts/refs/self-test)
    # but the verdict is driven only by Pass 1+2.
    rating="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['rubric']['rating'])" "$TMP_DIR/report.json")"
    [ "$rating" = "C" ]
}

@test "report stays valid JSON when the rubric block is emitted" {
    run bash "$AUDIT" "$FIXTURE" --json "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]
    run python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]
}
