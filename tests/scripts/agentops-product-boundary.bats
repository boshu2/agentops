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
