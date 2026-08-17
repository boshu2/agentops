#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skill="$skill_dir/SKILL.md"

grep -q '^name: fitness$' "$skill"
grep -q '^## Constraints$' "$skill"
grep -Fq 'optionally_write_goal_snapshot' "$skill"
grep -Fq 'optionally_write_rendered_spec' "$skill"
grep -Fq '| `validate` | goals | none |' "$skill"
grep -Fq '| `measure` | goals + declared gate evidence | fixed baseline snapshot |' "$skill"
grep -Fq 'A render target that is the goals source' "$skill"
grep -Fq 'observed `reads`, `writes`, and `stdout`' "$skill"
test -x "$skill_dir/scripts/validate-output.sh"

fixture_dir="$(mktemp -d)"
trap 'rm -rf -- "$fixture_dir"' EXIT
python3 - "$fixture_dir" <<'PY'
import copy
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
digest = "e" * 64
goals = str(root / "project" / "GOALS.md")
base = {
    "schema_version": "fitness-command-receipt.v1",
    "subcommand": "validate",
    "authorization_id": "caller:fitness-fixture",
    "goals_source": goals,
    "goals_digest_before": digest,
    "goals_digest_after": digest,
    "reads": [goals],
    "writes": [],
    "stdout": True,
    "render_root": None,
    "render_target": None,
    "target_existed_before": None,
    "overwrite_authorization_id": None,
    "status": "complete",
}
def emit(name, value):
    (root / name).write_text(json.dumps(value), encoding="utf-8")
emit("validate.json", base)
measure = copy.deepcopy(base)
measure.update({"subcommand": "measure", "writes": [str(root / "project" / ".agents" / "ao" / "goals" / "baselines" / "latest.json")]})
emit("measure.json", measure)
render = copy.deepcopy(base)
render_root = root / "project" / ".agents" / "scratch" / "goals"
render_target = render_root / "goals.feature"
render.update({"subcommand": "render", "writes": [str(render_target)], "stdout": False, "render_root": str(render_root), "render_target": str(render_target), "target_existed_before": False})
emit("render.json", render)
read_only_write = copy.deepcopy(base)
read_only_write["writes"] = [str(root / "leak")]
emit("read-only-write.json", read_only_write)
goals_target = copy.deepcopy(render)
goals_target.update({"render_target": goals, "writes": [goals]})
emit("goals-target.json", goals_target)
missing_overwrite = copy.deepcopy(render)
missing_overwrite["target_existed_before"] = True
emit("missing-overwrite.json", missing_overwrite)
mutated = copy.deepcopy(base)
mutated["goals_digest_after"] = "f" * 64
emit("mutated-goals.json", mutated)
PY
for accepted in validate measure render; do
  bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$accepted.json"
done
for rejected in read-only-write goals-target missing-overwrite mutated-goals; do
  if bash "$skill_dir/scripts/validate-output.sh" "$fixture_dir/$rejected.json"; then
    echo "fitness contract accepted negative fixture: $rejected" >&2
    exit 1
  fi
done

echo 'fitness skill contract: PASS'
