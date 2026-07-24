#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_REPO="$(mktemp -d)"
  mkdir -p "$FIXTURE_REPO/skills/example"
  cat >"$FIXTURE_REPO/skills/example/SKILL.md" <<'EOF'
---
name: wrong-name
description: "Fixture with a deliberately mismatched package name."
skill_api_version: 1
metadata:
  disposition: change
---

# Example
EOF
}

teardown() {
  rm -rf "$FIXTURE_REPO"
}

@test "strict heal rejects a source package whose declared name mismatches its path" {
  run env HEAL_REPO_ROOT="$FIXTURE_REPO" \
    bash "$REPO_ROOT/skills/skill-builder/scripts/heal.sh" \
    --check --strict "$FIXTURE_REPO/skills/example"

  [ "$status" -eq 1 ]
  [[ "$output" == *"[NAME_MISMATCH]"* ]]
  [[ "$output" == *"name must be 'example'"* ]]
}
