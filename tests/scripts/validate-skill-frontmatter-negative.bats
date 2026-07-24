#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_ROOT="$(mktemp -d)"
  mkdir -p "$FIXTURE_ROOT/example"
  cp "$REPO_ROOT/skills/skill-builder/SKILL.md" "$FIXTURE_ROOT/example/SKILL.md"
}

teardown() {
  rm -rf "$FIXTURE_ROOT"
}

@test "strict frontmatter rejects a missing contract relationship field" {
  python3 - "$FIXTURE_ROOT/example/SKILL.md" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
lines = path.read_text().splitlines()
path.write_text("\n".join(line for line in lines if not line.startswith("context_rel:")) + "\n")
PY

  run env \
    SKILL_FRONTMATTER_SKILLS_ROOT="$FIXTURE_ROOT" \
    "$REPO_ROOT/scripts/validate-skill-frontmatter.sh" --strict

  [ "$status" -eq 1 ]
  [[ "$output" == *"missing optional field: context_rel"* ]]
  [[ "$output" == *"STRICT FAIL"* ]]
}
