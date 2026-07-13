#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
schema="$skill_dir/schemas/learn-receipt.schema.json"

grep -q '^name: learn$' "$skill_dir/SKILL.md"
grep -q '^## Critical Constraints$' "$skill_dir/SKILL.md"
grep -q 'phase-4-summary.md' "$skill_dir/SKILL.md"
grep -q '^Feature: Learn records the fourth lifecycle receipt$' "$skill_dir/references/learn.feature"

SCHEMA="$schema" python3 - <<'PY'
import json
import os
from pathlib import Path

from jsonschema import Draft202012Validator

schema = json.loads(Path(os.environ["SCHEMA"]).read_text(encoding="utf-8"))
validator = Draft202012Validator(schema)
valid = {
    "schema_version": "learn-receipt.v1",
    "phase": "learn",
    "skill": "learn",
    "status": "DONE",
    "input_verdict_ref": ".agents/council/validate.md",
    "artifact": ".agents/rpi/phase-4-summary.md",
    "observations": [
        {"summary": "Preserve the acceptance fixture", "evidence_ref": "tests/integration/example.sh"}
    ],
}
validator.validate(valid)
invalid = dict(valid)
invalid["phase"] = "validate"
if not list(validator.iter_errors(invalid)):
    raise SystemExit("learn schema accepted a non-Learn phase")
PY

echo "learn skill contract: PASS"
