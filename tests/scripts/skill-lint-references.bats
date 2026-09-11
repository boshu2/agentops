#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE/tests/skills" "$FIXTURE/skills/caller/references/standards"
  mkdir -p "$FIXTURE/skills/peer_2/references/nested"
  cp "$REPO_ROOT/tests/skills/lint-skills.sh" "$FIXTURE/tests/skills/lint-skills.sh"
  cat > "$FIXTURE/skills/caller/SKILL.md" <<'SKILL'
---
name: caller
description: A reference-link fixture.
metadata:
  tier: execution
---
# Caller
SKILL
}

@test "skill lint resolves complete local sibling and repo-relative nested references" {
  touch "$FIXTURE/skills/caller/references/local.md"
  touch "$FIXTURE/skills/caller/references/standards/go.md"
  touch "$FIXTURE/skills/peer_2/references/nested/behavior.feature"
  cat >> "$FIXTURE/skills/caller/SKILL.md" <<'LINKS'
[local](references/local.md)
[nested](references/standards/go.md#errors)
[sibling](../peer_2/references/nested/behavior.feature)
`skills/peer_2/references/nested/behavior.feature`
LINKS
  run bash "$FIXTURE/tests/skills/lint-skills.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"1 skills checked"* ]]
}

@test "skill lint rejects a missing nested file even when its parent exists" {
  printf '%s\n' '[missing](references/standards/missing.md)' >> "$FIXTURE/skills/caller/SKILL.md"
  run bash "$FIXTURE/tests/skills/lint-skills.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"referenced file 'references/standards/missing.md' does not exist"* ]]
}

@test "skill lint rejects the complete missing sibling path" {
  printf '%s\n' '[missing](../peer_2/references/nested/missing.md)' >> "$FIXTURE/skills/caller/SKILL.md"
  run bash "$FIXTURE/tests/skills/lint-skills.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"referenced file '../peer_2/references/nested/missing.md' does not exist"* ]]
}

@test "skill lint still rejects a missing top-level reference" {
  printf '%s\n' '[missing](references/missing.md)' >> "$FIXTURE/skills/caller/SKILL.md"
  run bash "$FIXTURE/tests/skills/lint-skills.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"referenced file 'references/missing.md' does not exist"* ]]
}
