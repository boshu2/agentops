#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
}

@test "skill mesh projections match SKILL metadata" {
  run python3 "$REPO_ROOT/scripts/generate-skill-mesh.py" --check
  [ "$status" -eq 0 ]
}

@test "every live skill has one complete metadata disposition" {
  run python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import sys
import yaml

root = Path(sys.argv[1])
required = {"tier", "dependencies", "capabilities", "effects", "canonical_status", "disposition"}
for path in sorted((root / "skills").glob("*/SKILL.md")):
    data = yaml.safe_load(path.read_text(encoding="utf-8").split("---", 2)[1])
    metadata = data.get("metadata") or {}
    missing = required - set(metadata)
    if missing:
        raise SystemExit(f"{path}: missing {sorted(missing)}")
    if metadata["disposition"] not in {
        "keep", "keep_off_path", "keep_strategy", "keep_optional_adapter", "keep_specialist"
    }:
        raise SystemExit(f"{path}: invalid disposition")
PY
  [ "$status" -eq 0 ]
}

@test "core graph is exactly RPI to Plan Implement Validate" {
  run python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import sys
import yaml

root = Path(sys.argv[1])
actual = {}
for name in ("rpi", "plan", "implement", "validate"):
    path = root / "skills" / name / "SKILL.md"
    data = yaml.safe_load(path.read_text(encoding="utf-8").split("---", 2)[1])
    actual[name] = set((data.get("metadata") or {}).get("dependencies") or [])
expected = {"rpi": {"plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
if actual != expected:
    raise SystemExit(f"core graph mismatch: {actual!r}")
PY
  [ "$status" -eq 0 ]
}

@test "there is no parallel handwritten disposition ledger" {
  [ ! -e "$REPO_ROOT/docs/contracts/skill-dispositions.yaml" ]
}
