#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "only canonical premortem and postmortem roots exist" {
  for root in skills skills-codex images/gemini/skills; do
    [ -f "$REPO_ROOT/$root/premortem/SKILL.md" ]
    [ -f "$REPO_ROOT/$root/postmortem/SKILL.md" ]
    for alias in pre-mortem pre_mortem post-mortem post_mortem; do
      [ ! -e "$REPO_ROOT/$root/$alias" ]
    done
  done
}

@test "generated skill inventories contain one canonical mortem identity each" {
  run python3 - "$REPO_ROOT" <<'PY'
import json
from pathlib import Path
import sys

root = Path(sys.argv[1])
for relative, list_key in (
    ("skills/catalog.json", "skills"),
    ("images/claude/manifest.json", "skills"),
    ("images/codex/manifest.json", "skills"),
):
    data = json.loads((root / relative).read_text())
    names = [row.get("name", row.get("slug")) for row in data[list_key]]
    assert names.count("premortem") == 1, (relative, names.count("premortem"))
    assert names.count("postmortem") == 1, (relative, names.count("postmortem"))
    assert not set(names).intersection({"pre-mortem", "pre_mortem", "post-mortem", "post_mortem"})
PY
  [ "$status" -eq 0 ]
}

@test "metadata-derived mesh is current" {
  run python3 "$REPO_ROOT/scripts/generate-skill-mesh.py" --check
  [ "$status" -eq 0 ]
}
