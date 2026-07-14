#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "factual registry admits only declared deterministic proof kinds" {
  run env REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import json
import os
from pathlib import Path

from jsonschema import Draft202012Validator

root = Path(os.environ["REPO_ROOT"])
schema = json.loads(
    (root / "schemas/validation-gate-registry.v1.schema.json").read_text()
)
validator = Draft202012Validator(schema)
allowed = {
    "syntax",
    "schema",
    "identity",
    "paths",
    "generated_drift",
    "executable_assertion",
    "evidence_integrity",
}
for proof_kind in sorted(allowed):
    registry = {
        "schema_version": 1,
        "gates": [{
            "id": "fact",
            "lane": "mandatory",
            "proof_kind": proof_kind,
            "argv": ["bash", "scripts/fact.sh", "--json"],
            "backing": [{"path": "scripts/fact.sh", "sha256": "0" * 64}],
        }],
    }
    errors = list(validator.iter_errors(registry))
    if errors:
        raise SystemExit(f"allowed proof kind {proof_kind!r} was rejected: {errors[0].message}")

semantic = {
    "schema_version": 1,
    "gates": [{
        "id": "semantic-prose-score",
        "lane": "mandatory",
        "proof_kind": "semantic_prose",
        "argv": ["bash", "scripts/score-prose.sh", "--json"],
        "backing": [{"path": "scripts/score-prose.sh", "sha256": "0" * 64}],
    }],
}
if not list(validator.iter_errors(semantic)):
    raise SystemExit("semantic prose was accepted as deterministic proof")
PY

  [ "$status" -eq 0 ]
}

@test "missing registry backing has an explicit registry-integrity class" {
  run env REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import json
import os
from pathlib import Path

schema = json.loads(
    (Path(os.environ["REPO_ROOT"]) / "schemas/validation-receipt.v1.schema.json").read_text()
)
error = schema["$defs"]["preflightError"]
required = set(error["required"])
if "defect_class" not in required:
    raise SystemExit("preflight errors do not require a defect class")
enum = set(error["properties"]["defect_class"]["enum"])
if "registry_integrity" not in enum:
    raise SystemExit("registry_integrity is not a declared defect class")
PY

  [ "$status" -eq 0 ]
}

@test "changed-scope gate runs structural integrity without semantic scoring" {
  run bash "$REPO_ROOT/scripts/regen-changed-scope.sh" --list \
    --file skills/validate/SKILL.md

  [ "$status" -eq 0 ]
  [[ "$output" == *"changed skill structural integrity"* ]]
  [[ "$output" == *"skills/heal-skill/scripts/heal.sh --check --strict skills/validate"* ]]
  [[ "$output" != *"skills/heal-skill/scripts/audit.sh"* ]]
  [[ "$output" != *"deep conformance"* ]]
}

@test "equivalent prose rephrasing cannot fail deterministic Validate proof" {
  local scratch="$BATS_TEST_TMPDIR/rephrased-validate"
  mkdir -p "$scratch/skills" "$scratch/schemas"
  cp -R "$REPO_ROOT/skills/validate" "$scratch/skills/validate"
  cp "$REPO_ROOT/schemas/verdict.v1.schema.json" \
    "$REPO_ROOT"/schemas/validation-*.schema.json "$scratch/schemas/"

  for document in \
    "$scratch/skills/validate/SKILL.md" \
    "$scratch/skills/validate/references/canonical-validation-protocol.md"; do
    awk '
      !/Semantic prose scores/ &&
      !/exact-wording preferences/ &&
      !/advisory semantic observation/ &&
      !/deterministic authority/
    ' "$document" >"$document.rephrased"
    mv "$document.rephrased" "$document"
  done
  printf '\nEquivalent wording: machines establish facts; independent reviewers judge meaning.\n' \
    >>"$scratch/skills/validate/SKILL.md"
  printf '\nEquivalent wording: advisory comments about meaning cannot authorize or block delivery.\n' \
    >>"$scratch/skills/validate/references/canonical-validation-protocol.md"

  run bash "$scratch/skills/validate/scripts/validate.sh"

  [ "$status" -eq 0 ]
  [[ "$output" == *"validate skill contract: PASS"* ]]
}
