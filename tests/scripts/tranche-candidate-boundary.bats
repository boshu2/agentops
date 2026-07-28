#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

scan_active_lifecycle_authority() {
  local file="$1"
  grep -Eni \
    'Pawl|plan-Pawl|plan-pawl|candidate-packet\.v1|validation[- ]budget|reservation|admission|private[ -]retry|ratchet|council|tracker[ -]closure|delivery authority|semantic pre-push|ao[[:space:]]+land|full repository gate|final semantic verdict' \
    "$file"
}

@test "crank is not a live skill in source or runtime projections" {
  for path in \
    skills/crank/SKILL.md \
    skills-codex/crank/SKILL.md \
    images/gemini/skills/crank/SKILL.md; do
    [ ! -e "$REPO_ROOT/$path" ] || {
      echo "retired crank skill remains live at $path" >&2
      return 1
    }
  done

  run python3 - "$REPO_ROOT" <<'PY'
from pathlib import Path
import json
import sys

root = Path(sys.argv[1])
catalogs = (
    ("skills/catalog.json", "name"),
    ("skills-codex-overrides/catalog.json", "name"),
    ("images/claude/manifest.json", "slug"),
    ("images/codex/manifest.json", "slug"),
)
for relative, key in catalogs:
    data = json.loads((root / relative).read_text(encoding="utf-8"))
    rows = data.get("skills") or []
    names = [row.get(key) or row.get("name") or row.get("slug") for row in rows]
    if "crank" in names:
        raise SystemExit(f"{relative} still advertises crank")
PY
  [ "$status" -eq 0 ]
}

@test "implement stops at subject manifest and check receipts" {
  skill="$REPO_ROOT/skills/implement/SKILL.md"
  grep -Fq "exactly one bounded experiment" "$skill"
  grep -Fq 'subject-manifest.v1' "$skill"
  grep -Fq "Return the manifest digest, author context ID, and exact check receipts" "$skill"
  grep -Fq "response or runtime channel. Stop." "$skill"
  grep -Fq "Do not commit, push, claim, close, release, land, reserve, retry, or invoke a" "$skill"
  grep -Fq "semantic validator" "$skill"

  run scan_active_lifecycle_authority "$skill"
  [ "$status" -eq 1 ] || {
    echo "$output" >&2
    return 1
  }
}

@test "boundary scan catches old per-wave proof and closure language" {
  fixture="$BATS_TEST_TMPDIR/old-implement.md"
  printf '%s\n' \
    "Run Pawl closure after each wave." \
    "Require plan-Pawl, validation budget reservation, and tracker closure." \
    "Dispatch council review before delivery authority or ao land." >"$fixture"

  run scan_active_lifecycle_authority "$fixture"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Pawl"* ]]
  [[ "$output" == *"validation budget"* ]]
  [[ "$output" == *"ao land"* ]]
}
