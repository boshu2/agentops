#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
schema="$skill_dir/schemas/learn-receipt.schema.json"

grep -q '^name: learn$' "$skill_dir/SKILL.md"
grep -Fq 'The input verdict is immutable.' "$skill_dir/SKILL.md"
grep -Fq 'input_verdict_digest' "$skill_dir/SKILL.md"
grep -Fq 'Postmortem is optional and runs only for retrospective causal analysis.' "$skill_dir/SKILL.md"
grep -Fq 'plan_impact' "$skill_dir/SKILL.md"
grep -Fq 'returns it to the' "$skill_dir/SKILL.md"
grep -Fq 'does not invoke Premortem' "$skill_dir/SKILL.md"
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
    "remaining_work": True,
    "plan_impact": {
        "disposition": "no_change",
        "summary": "No remaining-plan mutation is required",
        "evidence_refs": [".agents/council/validate.md"],
        "proposed_changes": [],
    },
    "observations": [{
        "kind": "strength",
        "summary": "Acceptance fixture passed",
        "evidence_ref": "tests/integration/example.sh",
        "disposition": "record",
    }],
    "producer_candidates": [],
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

material = copy.deepcopy(valid)
material["plan_impact"] = {
    "disposition": "material_change",
    "summary": "A plan assumption was invalidated",
    "evidence_refs": [".agents/council/validate.md"],
    "proposed_changes": ["Split the next slice"],
}
validator.validate(material)

missing_change = copy.deepcopy(material)
missing_change["plan_impact"]["proposed_changes"] = []
if not list(validator.iter_errors(missing_change)):
    raise SystemExit("Learn schema accepted material_change without proposed changes")

terminal = copy.deepcopy(valid)
terminal["remaining_work"] = False
terminal["plan_impact"] = {
    "disposition": "terminal",
    "summary": "No work remains",
    "evidence_refs": [".agents/council/validate.md"],
    "proposed_changes": [],
}
validator.validate(terminal)

recurring = copy.deepcopy(valid)
recurring["producer_candidates"] = [{
    "id": "producer-0123456789ab",
    "class_key": "v1:docs/stale-surface",
    "summary": "A retired surface remained in active documentation.",
    "recurrence_count": 2,
    "advisory": True,
    "evidence": [
        {"observation_id": "obs-a", "objective_id": "objective-a", "evidence_ref": "a.md"},
        {"observation_id": "obs-b", "objective_id": "objective-b", "evidence_ref": "b.md"},
    ],
}]
validator.validate(recurring)

inflated = copy.deepcopy(recurring)
inflated["producer_candidates"][0]["recurrence_count"] = 1
if not list(validator.iter_errors(inflated)):
    raise SystemExit("Learn schema accepted a producer candidate without recurrence")
PY

echo 'learn skill contract: PASS'
