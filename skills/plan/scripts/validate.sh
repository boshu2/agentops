#!/usr/bin/env bash
set -euo pipefail
skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$skill_dir/../.." && pwd)"
grep -q '^name: plan$' "$skill_dir/SKILL.md"
grep -Fq 'PlanPacket' "$skill_dir/SKILL.md"
python3 -m json.tool "$repo_root/schemas/plan-packet.v1.schema.json" >/dev/null
python3 - "$repo_root/schemas/plan-packet.v1.schema.json" <<'PY'
import copy
import json
import sys

from jsonschema import Draft202012Validator

schema = json.load(open(sys.argv[1], encoding="utf-8"))
validator = Draft202012Validator(schema)
digest = "a" * 64
packet = {
    "schema_version": "plan-packet.v1",
    "intent": "Validate one bounded behavior",
    "intent_digest": digest,
    "acceptance_digest": digest,
    "active_behavior": "Plan requires normal and edge scenarios",
    "scenarios": [
        {"id": "normal", "kind": "normal", "given": "a plan", "when": "it is validated", "then": "normal behavior is covered"},
        {"id": "edge", "kind": "edge", "given": "an edge", "when": "it is validated", "then": "edge behavior is covered"},
    ],
    "non_goals": [],
    "required_evidence": ["schema validation"],
    "write_scope": {"include": ["schemas/plan-packet.v1.schema.json"], "exclude": []},
    "first_acceptance_check": {"command": "skills/plan/scripts/validate.sh"},
}
assert not list(validator.iter_errors(packet))
all_edge = copy.deepcopy(packet)
all_edge["scenarios"][0]["kind"] = "edge"
assert list(validator.iter_errors(all_edge)), "PlanPacket must require at least one normal scenario"
PY
! grep -Eiq 'ao |\bbr\b|beads|claim|queue|lease|delivery|release' "$skill_dir/SKILL.md"
echo 'plan skill contract: PASS'
