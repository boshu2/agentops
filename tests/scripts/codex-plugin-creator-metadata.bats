#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/validate-codex-plugin-creator-metadata.sh"
  FIXTURE="$BATS_TEST_TMPDIR/plugin"
  mkdir -p "$FIXTURE/.codex-plugin" "$FIXTURE/plugins"
  cp "$REPO_ROOT/.codex-plugin/plugin.json" "$FIXTURE/.codex-plugin/plugin.json"
  cp "$REPO_ROOT/plugins/marketplace.json" "$FIXTURE/plugins/marketplace.json"
}

update_manifest() {
  jq "$1" "$FIXTURE/.codex-plugin/plugin.json" > "$FIXTURE/updated.json"
  mv "$FIXTURE/updated.json" "$FIXTURE/.codex-plugin/plugin.json"
}

@test "current skills-only Codex package satisfies discovery metadata" {
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -eq 0 ]
}

@test "valid description updates do not require changing the validator" {
  update_manifest '.interface.shortDescription = "Portable skills for engineering."'
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -eq 0 ]
}

@test "missing discovery description is rejected" {
  update_manifest 'del(.interface.shortDescription)'
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -ne 0 ]
}

@test "Codex plugin must not claim separately installed hooks" {
  update_manifest '.interface.capabilities += ["Hooks"]'
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -ne 0 ]
}

@test "unsupported hooks manifest wiring is rejected" {
  update_manifest '.hooks = "./hooks/hooks.json"'
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -ne 0 ]
}

@test "marketplace missing its installation policy is rejected" {
  jq 'del(.plugins[0].policy)' "$FIXTURE/plugins/marketplace.json" > "$FIXTURE/updated.json"
  mv "$FIXTURE/updated.json" "$FIXTURE/plugins/marketplace.json"
  run bash "$SCRIPT" --repo-root "$FIXTURE"
  [ "$status" -ne 0 ]
}
