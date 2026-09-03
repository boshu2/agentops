#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  GATE="$REPO_ROOT/scripts/validate-codex-api-conformance.sh"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/skills-codex"
  mkdir -p "$FIXTURE_ROOT/portable-skill"
}

write_skill() {
  printf '%s' "$1" > "$FIXTURE_ROOT/portable-skill/SKILL.md"
}

@test "checked-in Codex release projection is portable" {
  run bash "$GATE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS [portable] 54 package(s)"* ]]
}

@test "portable gate accepts the minimal Agent Skills package" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\n---\n# Portable skill\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS [portable] 1 package(s)"* ]]
}

@test "portable gate rejects host-only frontmatter" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\noutput_contract: result.v1\n---\n# Portable skill\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"host-only frontmatter fields: output_contract"* ]]
}

@test "portable gate rejects non-string metadata and comma-delimited tools" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\nmetadata:\n  effects: [write]\nallowed-tools: Read, Grep\n---\n# Portable skill\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"metadata must map strings to strings"* ]]
  [[ "$output" == *"allowed-tools must be a nonempty space-separated string"* ]]
}

@test "portable gate rejects missing and escaping resource links" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\n---\n# Portable skill\n[missing](references/missing.md)\n[escape](../../../../outside.md)\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"resource link does not resolve"* ]]
  [[ "$output" == *"resource link escapes catalog"* ]]
}

@test "portable gate rejects a missing image resource" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\n---\n# Portable skill\n![diagram](assets/missing.png)\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"resource link does not resolve: SKILL.md -> assets/missing.png"* ]]
}

@test "portable gate rejects a symlinked package" {
  write_skill $'---\nname: portable-skill\ndescription: Use when a portable fixture is needed.\n---\n# Portable skill\n'
  external="$BATS_TEST_TMPDIR/external"
  mkdir -p "$external"
  printf '%s' $'---\nname: linked-skill\ndescription: Use when a linked fixture is needed.\n---\n# Linked skill\n' > "$external/SKILL.md"
  ln -s "$external" "$FIXTURE_ROOT/linked-skill"

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"skill package directory must not be a symlink"* ]]
}

@test "portable gate rejects duplicate frontmatter keys" {
  write_skill $'---\nname: wrong-name\nname: portable-skill\ndescription: Use when a portable fixture is needed.\n---\n# Portable skill\n'

  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"found duplicate key"* ]]
}

@test "twins never turn a path segment after a placeholder into a skill invocation" {
  # Regression for `<run-id>/codebase-recon.json` becoming `<run-id>$codebase-recon.json`:
  # a closing '>' precedes a path segment, never an invocation.
  run grep -rln '>\$' "$REPO_ROOT"/skills-codex/*/SKILL.md
  [ "$status" -eq 1 ]
}
