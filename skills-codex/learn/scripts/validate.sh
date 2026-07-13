#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
schema="$skill_dir/schemas/learn-receipt.schema.json"

grep -q '^name: learn$' "$skill_dir/SKILL.md"
grep -Fq 'The input verdict is immutable.' "$skill_dir/SKILL.md"
grep -Fq 'input_verdict_digest' "$skill_dir/SKILL.md"
grep -Fq 'Postmortem is optional and runs only for retrospective causal analysis.' "$skill_dir/SKILL.md"
grep -q '^Feature: Learn bookkeeps an immutable verdict$' "$skill_dir/references/learn.feature"

SCHEMA="$schema" python3 - <<'PY'
import copy
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
    "input_verdict_digest": "sha256:" + "a" * 64,
    "artifact": ".agents/rpi/phase-4-summary.md",
    "observations": [{
        "kind": "strength",
        "summary": "Acceptance fixture passed",
        "evidence_ref": "tests/integration/example.sh",
        "disposition": "record",
    }],
}
validator.validate(valid)

mutable = copy.deepcopy(valid)
mutable["verdict"] = "PASS"
if not list(validator.iter_errors(mutable)):
    raise SystemExit("Learn schema accepted a mutable verdict field")

unbound = copy.deepcopy(valid)
unbound.pop("input_verdict_digest")
if not list(validator.iter_errors(unbound)):
    raise SystemExit("Learn schema accepted an unbound verdict")
PY

echo 'learn skill contract: PASS'
