#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "advisory Review owns no acceptance effects or hard dependencies" {
  run python3 - "$REPO_ROOT" <<'PYCODE'
from pathlib import Path
import sys
import yaml

root = Path(sys.argv[1])
def metadata(name):
    return yaml.safe_load((root / "skills" / name / "SKILL.md").read_text().split("---", 2)[1])
review, validate = metadata("review"), metadata("validate")
assert review["user-invocable"] and review["metadata"]["graph_root"]
assert review["metadata"]["dependencies"] == []
assert review["metadata"]["effects"] == []
assert review["produces"] == []
assert "judge_acceptance" not in review["metadata"]["capabilities"]
assert "verdict.v2" in validate["produces"]
assert "judge_acceptance" in validate["metadata"]["capabilities"]
assert all("verdict.v2" not in metadata(p.parent.name).get("produces", [])
           for p in (root / "skills").glob("*/SKILL.md") if p.parent.name != "validate")
PYCODE
  [ "$status" -eq 0 ]
}

# Run check_core_schemas from scripts/check-cathedral-cut-conformance.py over a
# schemas/ fixture; an AssertionError becomes exit 1 with its message.
run_core_schema_check() {
  run python3 - \
    "$REPO_ROOT/scripts/check-cathedral-cut-conformance.py" \
    "$1" <<'PY'
import importlib.util
from pathlib import Path
import sys

spec = importlib.util.spec_from_file_location("cathedral_cut", sys.argv[1])
module = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(module)
module.ROOT = Path(sys.argv[2])
try:
    module.check_core_schemas()
except AssertionError as exc:
    print(exc)
    raise SystemExit(1)
PY
}

@test "cathedral-cut conformance rejects retired lifecycle state in any schema" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture"
  cp -R "$REPO_ROOT/schemas" "$fixture/schemas"

  run_core_schema_check "$fixture"
  [ "$status" -eq 0 ]

  printf '%s\n' '{"type": "object", "properties": {"retries": {"type": "integer"}}}' \
    >"$fixture/schemas/planted.v1.schema.json"
  run_core_schema_check "$fixture"
  [ "$status" -eq 1 ]
  [[ "$output" == *"planted.v1.schema.json: retired lifecycle state ['retries']"* ]]
}

@test "legacy operating-loop workflow is a routing tombstone" {
  wf="$REPO_ROOT/workflows/operating-loop.js"
  grep -Fq "throw new Error" "$wf"
  grep -Fq "skills/rpi/SKILL.md" "$wf"
  grep -Fq "ship-beads.js" "$wf"
  # No live seven-move dispatch remains.
  run grep -F "agent(" "$wf"
  [ "$status" -eq 1 ]
}

@test "skill-mesh rejects a second verdict producer, an advisory judge and a non-rpi dependency" {
  run python3 - "$REPO_ROOT/scripts/check-skill-mesh.py" <<'EOF'
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("skill_mesh", sys.argv[1])
mesh = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mesh)

def skill(deps=(), produces=(), effects=("x",), caps=()):
    return {"produces": list(produces), "metadata": {"dependencies": list(deps), "effects": list(effects), "capabilities": list(caps)}}

base = {
    "rpi": skill(deps=("plan", "implement", "validate")),
    "plan": skill(), "implement": skill(),
    "validate": skill(produces=("verdict.v2",), caps=("judge_acceptance",)),
}
assert mesh.check_graph_and_authority(base) == [], mesh.check_graph_and_authority(base)
cases = {
    "exactly one skill must produce verdict.v2": dict(base, review=skill(produces=("verdict.v2",), caps=("judge_acceptance",))),
    "advisory skill (no produces, no effects) declares judge_acceptance": dict(base, review=skill(effects=(), caps=("judge_acceptance",))),
    "only rpi may declare hard dependencies": dict(base, review=skill(deps=("plan",))),
}
for want, skills in cases.items():
    got = mesh.check_graph_and_authority(skills)
    assert any(want in message for message in got), (want, got)
print("ok")
EOF
  [ "$status" -eq 0 ]
  [ "$output" = "ok" ]
}

@test "schema docs reject a deprecated schema inside a current block, and verdict enums stay tri-state" {
  fixture="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$fixture/docs/contracts"
  cp -R "$REPO_ROOT/schemas" "$fixture/schemas"
  cp "$REPO_ROOT/docs/SCHEMAS.md" "$fixture/docs/SCHEMAS.md"
  cp "$REPO_ROOT/docs/contracts/index.md" "$fixture/docs/contracts/index.md"
  run python3 - "$REPO_ROOT/scripts/check-cathedral-cut-conformance.py" "$fixture" <<'EOF'
import importlib.util
import json
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location("cathedral_cut", sys.argv[1])
cut = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cut)
root = Path(sys.argv[2])
cut.ROOT = root
cut.check_schema_index_docs()
cut.check_core_schemas()

doc = root / "docs" / "SCHEMAS.md"
text = doc.read_text()
row = "rpi-report.v1.schema.json"
assert row in text
lines = text.split("\n")
at = next(i for i, line in enumerate(lines) if row in line)
lines.insert(at + 1, lines[at].replace("rpi-report.v1", "plan-packet.v1"))
doc.write_text("\n".join(lines))
try:
    cut.check_schema_index_docs()
    raise SystemExit("planted deprecated row in the current block passed")
except AssertionError as exc:
    assert "mixes deprecated" in str(exc), exc

verdict = root / "schemas" / "verdict.v2.schema.json"
schema = json.loads(verdict.read_text())
schema["properties"]["criteria"]["items"]["properties"]["result"]["enum"].append("WARN")
verdict.write_text(json.dumps(schema))
try:
    cut.check_core_schemas()
    raise SystemExit("WARN in criteria[].result passed")
except AssertionError as exc:
    assert "not the tri-state" in str(exc), exc
print("ok")
EOF
  [ "$status" -eq 0 ]
  [ "$output" = "ok" ]
}
