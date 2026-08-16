#!/usr/bin/env bats

# Regression test for the skill-builder deep audit Pass 3 (rubric scoring) — soc-ads5v.
# Pass 3 folds the 10-category static package-readiness rubric
# (docs/reference/skill-quality-rubric.md) into audit-report.json via
# score_agentops_skill.py --audit-block. The score is advisory: it must NOT
# change the PASS/WARN/FAIL verdict or imply safety/effectiveness evaluation.
# Scoring must be deterministic + explainable (each category gets a 0-3 score
# and a reason).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    AUDIT="$REPO_ROOT/skills/skill-builder/scripts/audit.sh"
    SCORE="$REPO_ROOT/skills/skill-builder/scripts/score_agentops_skill.py"
    SCHEMA="$REPO_ROOT/skills/skill-builder/schemas/audit-report.json"

    # The 10 scored categories, verbatim from the rubric.
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
metadata:
  tier: meta
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
    [[ "$output" == *'"scope": "static-package-readiness"'* ]]
    [[ "$output" == *'"safety_gate_evaluated": false'* ]]
    [[ "$output" == *'"effectiveness_evaluated": false'* ]]
}

@test "default scorer JSON labels its limited evidence scope" {
    run python3 "$SCORE" "$FIXTURE"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"scope": "static-package-readiness"'* ]]
    [[ "$output" == *'"safety_gate_evaluated": false'* ]]
    [[ "$output" == *'"effectiveness_evaluated": false'* ]]
}

@test "audit.sh folds a rubric block with all 10 scored categories into the report" {
    run bash "$AUDIT" "$FIXTURE" --json "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"/30"* ]]

    python3 - "$TMP_DIR/report.json" <<PY
import json, sys
report = json.load(open(sys.argv[1]))
rubric = report["rubric"]
assert rubric is not None, "rubric must be present"
assert rubric["scope"] == "static-package-readiness"
assert rubric["safety_gate_evaluated"] is False
assert rubric["effectiveness_evaluated"] is False
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

@test "static readiness scoring is deterministic across runs" {
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

    # Optional package absence is uncertainty (1), never automatic solid (2).
    run python3 "$SCORE" "$FIXTURE"
    [ "$status" -eq 0 ]
    SCORE_OUTPUT="$output" python3 - <<'PY'
import json
import os

report = json.loads(os.environ["SCORE_OUTPUT"])
assert report["total_score"] == 15
assert report["rating"] == "B"
assert report["scores"]["validation"] == 0
assert report["scores"]["self_test"] == 1
assert report["scores"]["helper_scripts"] == 1
assert report["scores"]["assets_templates"] == 1
assert report["scores"]["subagents_roles"] == 1
metrics = report["metrics"]
assert metrics["script_files"] == 0
assert metrics["asset_files"] == 0
assert metrics["subagent_files"] == 0
PY

    # The verdict is still driven only by Pass 1+2.
    rating="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['rubric']['rating'])" "$TMP_DIR/report.json")"
    [[ "$rating" = "B" ]]
}

@test "static readiness band boundaries match the 0-30 rubric" {
    PYTHONDONTWRITEBYTECODE=1 SCORE_PATH="$SCORE" python3 - <<'PY'
import importlib.util
import os

spec = importlib.util.spec_from_file_location("score_agentops_skill", os.environ["SCORE_PATH"])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

expected = {
    0: "C", 10: "C", 11: "B", 20: "B",
    21: "A", 26: "A", 27: "S", 30: "S",
}
for score, band in expected.items():
    assert module.readiness_rating(score) == band, (score, band)
PY
}

@test "canonical plan and execution skills keep explicit output contracts" {
    for skill in plan implement using-flywheel; do
        local report="$TMP_DIR/$skill.json"
        run bash "$AUDIT" "$REPO_ROOT/skills/$skill" --json "$report"
        [ "$status" -eq 0 ]
        run jq -e '
            [.pass2.checks[]
             | select(.id == "output-spec-explicit")
             | .status] == ["pass"]
        ' "$report"
        [ "$status" -eq 0 ]
    done
}

@test "report stays valid against the audit schema when rubric is emitted" {
    run bash "$AUDIT" "$FIXTURE" --json "$TMP_DIR/report.json"
    [ "$status" -eq 0 ]
    run python3 - "$TMP_DIR/report.json" "$SCHEMA" <<'PY'
import json
import sys

import jsonschema

with open(sys.argv[1], encoding="utf-8") as report_file:
    report = json.load(report_file)
with open(sys.argv[2], encoding="utf-8") as schema_file:
    schema = json.load(schema_file)
jsonschema.Draft7Validator(schema).validate(report)
PY
    [ "$status" -eq 0 ]
}
