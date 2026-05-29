#!/usr/bin/env bats
# ag-cw2y item 4: skill-builder must add a skills-codex-overrides/catalog.json
# entry for a new skill so it is one-shot-green against
# validate-codex-override-coverage ("source skill missing from Codex catalog").
# Default treatment is parity_only (canonical-derived codex form). Idempotent +
# repo-root-injectable.

setup() {
  HELPER="$BATS_TEST_DIRNAME/../../scripts/append-codex-override-entry.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/skills-codex-overrides"
  cat > "$FIX/skills-codex-overrides/catalog.json" <<'EOF'
{
  "version": 1,
  "description": "fixture",
  "waves": [ { "id": "catalog-parity", "title": "Catalog parity" } ],
  "skills": [
    { "name": "existing", "treatment": "parity_only", "wave": "catalog-parity", "reason": "already here" }
  ]
}
EOF
}

teardown() { rm -rf "$FIX"; }

@test "appends a parity_only catalog entry for a new skill" {
  run bash "$HELPER" newskill "$FIX"
  [ "$status" -eq 0 ]
  run python3 -c "import json,sys; d=json.load(open('$FIX/skills-codex-overrides/catalog.json')); e=[s for s in d['skills'] if s['name']=='newskill']; assert len(e)==1; assert e[0]['treatment']=='parity_only'; assert e[0]['wave']=='catalog-parity'; print('ok')"
  [[ "$output" == *"ok"* ]]
}

@test "produced catalog remains valid JSON" {
  bash "$HELPER" newskill "$FIX"
  run python3 -c "import json; json.load(open('$FIX/skills-codex-overrides/catalog.json')); print('valid')"
  [[ "$output" == *"valid"* ]]
}

@test "is idempotent — no duplicate entry" {
  bash "$HELPER" newskill "$FIX"
  bash "$HELPER" newskill "$FIX"
  run python3 -c "import json; d=json.load(open('$FIX/skills-codex-overrides/catalog.json')); print(sum(1 for s in d['skills'] if s['name']=='newskill'))"
  [[ "$output" == *"1"* ]]
}

@test "does not duplicate an already-cataloged skill" {
  run bash "$HELPER" existing "$FIX"
  [ "$status" -eq 0 ]
  run python3 -c "import json; d=json.load(open('$FIX/skills-codex-overrides/catalog.json')); print(sum(1 for s in d['skills'] if s['name']=='existing'))"
  [[ "$output" == *"1"* ]]
}
