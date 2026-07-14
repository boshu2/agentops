#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
schema="$repo_root/schemas/verdict.v1.schema.json"
request_checker="$skill_dir/scripts/validation-request.py"
request_schemas=(
  "$repo_root/schemas/validation-candidate.v1.schema.json"
  "$repo_root/schemas/validation-gate-registry.v1.schema.json"
  "$repo_root/schemas/validation-request.v1.schema.json"
  "$repo_root/schemas/validation-receipt.v1.schema.json"
)

grep -q '^name: validate$' "$skill_dir/SKILL.md"
grep -Fq 'Validate ends at proof.' "$skill_dir/SKILL.md"
grep -Fq 'Structured observations are part of the immutable verdict' "$skill_dir/SKILL.md"
grep -Fq '**Mode-budget assertion:** 8 modes.' "$skill_dir/SKILL.md"
grep -Fq 'vibe` trigger maps to `--mode=post-impl`' "$skill_dir/SKILL.md"
grep -Fq 'Inventory size is never a rigor or validator-count' "$skill_dir/SKILL.md"
grep -Fq 'A diagnostic or release FAIL remains nonbinding to semantic validation' "$skill_dir/SKILL.md"
grep -Fq 'there is no automatic crash recovery.' "$skill_dir/SKILL.md"
grep -Fq 'permit `VALIDATE_SINGLE_FRESH`' "$skill_dir/SKILL.md"
grep -Fq 'Validate does not meter, reserve, or authorize semantic work through a second adapter.' "$skill_dir/SKILL.md"
grep -Fq 'Diagnostic and release `FAIL` remain' "$skill_dir/SKILL.md"
grep -q '^Feature: Validate emits immutable proof only$' "$skill_dir/references/validate.feature"
grep -q '^  Scenario: Factual readiness permits one fresh validator$' "$skill_dir/references/validate.feature"

PYTHONDONTWRITEBYTECODE=1 python3 "$request_checker" --help >/dev/null
test ! -e "$skill_dir/scripts/validation-budget.py"
test ! -e "$repo_root/schemas/validation-budget-receipt.v1.schema.json"
SCHEMAS="$(IFS=:; echo "${request_schemas[*]}")" python3 - <<'PY'
import json
import os
from pathlib import Path

from jsonschema import Draft202012Validator

for raw_path in os.environ["SCHEMAS"].split(":"):
    schema = json.loads(Path(raw_path).read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
PY

SCHEMA="$schema" python3 - <<'PY'
import copy
import json
import os
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker

schema = json.loads(Path(os.environ["SCHEMA"]).read_text(encoding="utf-8"))
validator = Draft202012Validator(schema, format_checker=FormatChecker())
valid = {
    "verdict_id": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
    "bead_id": "01ARZ3NDEKTSV4RRFFQ69G5FAW",
    "verdict": "PASS",
    "confidence": "HIGH",
    "briefing_learnings": [],
    "findings": [],
    "observations": [{
        "kind": "strength",
        "summary": "Acceptance command passed on the pinned commit",
        "evidence_ref": ".agents/evidence/acceptance.txt",
    }],
    "not_checked": [],
    "validated_at": "2026-07-13T12:00:00Z",
    "validator_session": "judge-1",
    "schema_version": 1,
}
validator.validate(valid)

missing = copy.deepcopy(valid)
missing.pop("observations")
if not list(validator.iter_errors(missing)):
    raise SystemExit("verdict schema accepted missing structured observations")

malformed = copy.deepcopy(valid)
malformed["observations"][0].pop("evidence_ref")
if not list(validator.iter_errors(malformed)):
    raise SystemExit("verdict schema accepted an uncited observation")

self_judged = copy.deepcopy(valid)
self_judged["validator_session"] = "author"
if self_judged["verdict"] == "PASS" and self_judged["validator_session"] == "author":
    pass
else:
    raise SystemExit("self-judge negative fixture is malformed")
PY

if grep -Eiq 'ao (pawl|land|ratchet record|flywheel|forge)|git (commit|push)|br (close|update)|AUTO-REDO' "$skill_dir/SKILL.md"; then
  echo 'validate contract contains forbidden post-proof execution' >&2
  exit 1
fi

echo 'validate skill contract: PASS'
