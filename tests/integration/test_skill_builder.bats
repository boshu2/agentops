#!/usr/bin/env bats

setup_file() {
  REAL_REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
  SCRATCH_ROOT="$BATS_FILE_TMPDIR/repo"
  mkdir -p "$SCRATCH_ROOT"
  cp -R "$REAL_REPO_ROOT/skills" "$SCRATCH_ROOT/skills"
  cp -R "$REAL_REPO_ROOT/scripts" "$SCRATCH_ROOT/scripts"
  cp -R "$REAL_REPO_ROOT/docs" "$SCRATCH_ROOT/docs"
  cp -R "$REAL_REPO_ROOT/skills-codex" "$SCRATCH_ROOT/skills-codex"
  cp -R "$REAL_REPO_ROOT/skills-codex-overrides" "$SCRATCH_ROOT/skills-codex-overrides"
  cp -R "$REAL_REPO_ROOT/images" "$SCRATCH_ROOT/images"
  cp -R "$REAL_REPO_ROOT/.claude-plugin" "$SCRATCH_ROOT/.claude-plugin"
  cp "$REAL_REPO_ROOT/registry.json" "$SCRATCH_ROOT/registry.json"
  export REAL_REPO_ROOT SCRATCH_ROOT
  export SKILL_BUILDER_REPO_ROOT="$SCRATCH_ROOT"
  BUILD_SH="$REAL_REPO_ROOT/skills/skill-builder/scripts/build.sh"
  export BUILD_SH
}

@test "builder rejects missing and unknown modes" {
  run bash "$BUILD_SH"
  [ "$status" -eq 2 ]

  run bash "$BUILD_SH" removed-mode example
  [ "$status" -eq 2 ]
}

@test "one invocation creates metadata source and derived projections" {
  name="builder-contract-test"
  run env \
    SKILL_TIER=execution \
    SKILL_DEPENDENCIES='[]' \
    SKILL_CAPABILITIES='["builder_contract_test"]' \
    SKILL_EFFECTS='[]' \
    bash "$BUILD_SH" from-scratch "$name"
  [ "$status" -eq 0 ]

  source="$SCRATCH_ROOT/skills/$name/SKILL.md"
  [ -f "$source" ]
  grep -q '^  canonical_status: canonical$' "$source"
  grep -q '^  disposition: keep_specialist$' "$source"
  grep -q '^  capabilities: ' "$source"
  grep -q 'builder_contract_test' "$source"

  [ -f "$SCRATCH_ROOT/skills-codex/$name/SKILL.md" ]
  [ -f "$SCRATCH_ROOT/skills-codex/$name/prompt.md" ]
  grep -q "\"name\": \"$name\"" "$SCRATCH_ROOT/skills/catalog.json"
  grep -q '"structure_check_pass": true' \
    "$SCRATCH_ROOT/.agents/audits/${name}-build.json"
}

@test "builder does not create lifecycle ledgers or touch the real repository" {
  [ ! -e "$SCRATCH_ROOT/docs/contracts/skill-dispositions.yaml" ]
  [ ! -e "$REAL_REPO_ROOT/skills/builder-contract-test" ]
  [ ! -e "$REAL_REPO_ROOT/skills-codex/builder-contract-test" ]
  [ ! -e "$REAL_REPO_ROOT/.agents/audits/builder-contract-test-build.json" ]
}
